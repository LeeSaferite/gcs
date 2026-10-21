// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

// Each node type has a method-less shadow type (type checkFields Check, and so on) holding the same fields. A node is
// written and read through its shadow, which keeps json/v2 from calling the node's own MarshalJSONTo or
// UnmarshalJSONFrom again. Writing puts the wire type ahead of the shadow's fields. Reading is strict: it rejects
// unknown members, and it replaces json/v2's handling of the nested nodes, operands, criteria and enums with the
// decoders below, which reject what the lenient defaults would turn into something else (an unknown comparison would
// become "any", for one). A Ref that holds a tree that fails to decode keeps it as read; see Ref.UnmarshalJSONFrom.

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

// Decode decodes a tree. Decoding is strict: an unknown node type, an unknown member, an unknown key for an enum or a
// comparison, a missing node and a tree nested deeper than MaxDepth are all errors, reported as a Problem whose path
// leads to the node at fault. null and the empty group decode as nil.
func Decode(data []byte) (Node, error) {
	raw := jsontext.Value(data)
	if !raw.IsValid() {
		return nil, Problem{Msg: i18n.Text("not valid JSON")}
	}
	return decodeRef(raw, 1)
}

// decodeRef decodes the node of a Ref slot at the given depth. An absent or null slot and the empty group both become
// nil.
func decodeRef(raw jsontext.Value, depth int) (Node, error) {
	if len(raw) == 0 || raw.Kind() == 'n' {
		return nil, nil
	}
	n, err := decodeNode(raw, depth)
	if err != nil || (Ref{Node: n}).IsZero() {
		return nil, err
	}
	return n, nil
}

// decodeNode decodes one node at the given depth.
func decodeNode(raw jsontext.Value, depth int) (Node, error) {
	switch {
	case raw.Kind() == 'n':
		return nil, problem(i18n.Text("missing node"))
	case raw.Kind() != '{':
		return nil, problem(i18n.Text("a node is not an object"))
	case depth > MaxDepth:
		return nil, problem(reasonTooDeep())
	}
	var head struct {
		Type *string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, decodeError(err)
	}
	if head.Type == nil {
		return nil, problem(i18n.Text(`missing "type"`))
	}
	var n interface {
		Node
		decode(raw jsontext.Value, depth int) error
	}
	switch *head.Type {
	case typeGroup:
		n = &Group{}
	case typeCheck:
		n = &Check{}
	case typeConst:
		n = &Const{}
	case typeCount:
		n = &Count{}
	case typeCompare:
		n = &Compare{}
	case typeScript:
		n = &Script{}
	default:
		return nil, problem(fmt.Sprintf(i18n.Text("unknown node type %q"), *head.Type))
	}
	if err := n.decode(raw, depth); err != nil {
		return nil, err
	}
	return n, nil
}

// unmarshalNode implements UnmarshalJSONFrom for a node, which it decodes as the root of a tree that must be a node of
// the same type.
func unmarshalNode[T any, P interface {
	*T
	Node
}](dec *jsontext.Decoder, p P) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	n, err := decodeNode(raw, 1)
	if err != nil {
		return err
	}
	other, ok := n.(P)
	if !ok {
		return problem(i18n.Text("a node of another type"))
	}
	*p = *other
	return nil
}

// decodeFields reads raw, a node at the given depth, into fields, the node's shadow.
func decodeFields[T any](raw jsontext.Value, depth int, fields *T) error {
	if err := json.Unmarshal(raw, &typedShadow[T]{Fields: fields}, decodeOptions(depth)); err != nil {
		return decodeError(err)
	}
	return nil
}

// decodeRequired is decodeFields for a node with a required enum member, which it first sets to unset, a value
// outside the enum that only a missing member leaves in place.
func decodeRequired[T any, E comparable](raw jsontext.Value, depth int, fields *T, member string, e *E, unset E) error {
	*e = unset
	if err := decodeFields(raw, depth, fields); err != nil {
		return err
	}
	if *e == unset {
		return problem(fmt.Sprintf(i18n.Text("missing %q"), member))
	}
	return nil
}

var optionsByDepth [MaxDepth + 1]json.Options

func init() {
	for depth := range optionsByDepth {
		optionsByDepth[depth] = newDecodeOptions(depth)
	}
}

// decodeOptions returns the options that read the members of a node or operand at the given depth.
func decodeOptions(depth int) json.Options {
	return optionsByDepth[min(depth, MaxDepth)]
}

// newDecodeOptions returns the options that read the members of a node or operand at the given depth: unknown members
// are rejected, and nested nodes, operands, criteria and enums are read by the strict decoders, a level deeper.
func newDecodeOptions(depth int) json.Options {
	return json.JoinOptions(json.RejectUnknownMembers(true), json.WithUnmarshalers(json.JoinUnmarshalers(
		json.UnmarshalFromFunc(func(dec *jsontext.Decoder, n *Node) error {
			raw, err := dec.ReadValue()
			if err == nil {
				*n, err = decodeNode(raw, depth+1)
			}
			return err
		}),
		json.UnmarshalFromFunc(func(dec *jsontext.Decoder, r *Ref) error {
			raw, err := dec.ReadValue()
			if err == nil {
				r.Node, err = decodeRef(raw, depth+1)
			}
			return err
		}),
		json.UnmarshalFromFunc(func(dec *jsontext.Decoder, o *Operand) error {
			raw, err := dec.ReadValue()
			if err == nil {
				err = o.decode(raw, depth+1)
			}
			return err
		}),
		json.UnmarshalFromFunc(func(dec *jsontext.Decoder, t *criteria.Text) error {
			cmp, q, err := decodeCriterion[string](dec, criteria.ExtractKnownStringComparison, false)
			*t = criteria.Text{Compare: cmp, Qualifier: q}
			return err
		}),
		json.UnmarshalFromFunc(func(dec *jsontext.Decoder, n *criteria.Number) error {
			cmp, q, err := decodeCriterion[fxp.Int](dec, criteria.ExtractKnownNumericComparison, false)
			*n = criteria.Number{Compare: cmp, Qualifier: q}
			return err
		}),
		json.UnmarshalFromFunc(func(dec *jsontext.Decoder, w *criteria.Weight) error {
			cmp, q, err := decodeCriterion[fxp.Weight](dec, criteria.ExtractKnownNumericComparison, false)
			*w = criteria.Weight{Compare: cmp, Qualifier: q}
			return err
		}),
		json.UnmarshalFromFunc(func(dec *jsontext.Decoder, p **criteria.Number) error {
			if dec.PeekKind() == 'n' {
				_, err := dec.ReadToken()
				*p = nil
				return err
			}
			cmp, q, err := decodeCriterion[fxp.Int](dec, criteria.ExtractKnownNumericComparison, true)
			*p = canonicalQuantity(&criteria.Number{Compare: cmp, Qualifier: q})
			return err
		}),
		enumDecoder(ExtractKnownGroupMode),
		enumDecoder(ExtractKnownCompareOp),
		enumDecoder(ExtractKnownAggregate),
	)))
}

// enumDecoder returns a decoder for an enum whose keys must be known.
func enumDecoder[E any](extract func(string) (E, bool)) *json.Unmarshalers {
	return json.UnmarshalFromFunc(func(dec *jsontext.Decoder, e *E) error {
		var key string
		if err := json.UnmarshalDecode(dec, &key); err != nil {
			return err
		}
		var known bool
		if *e, known = extract(key); !known {
			return memberProblem(func(member string) string {
				return fmt.Sprintf(i18n.Text("unknown %q value %q"), member, key)
			})
		}
		return nil
	})
}

// criterionWire is the raw form of a criterion: a comparison, which must be known, and a qualifier.
type criterionWire[Q any] struct {
	Compare   *string `json:"compare"`
	Qualifier *Q      `json:"qualifier"`
}

// decodeCriterion reads a criterion: null is none. That of a check must have a comparison other than "any", while a
// quantity may leave it out to mean "any", though never with a qualifier.
func decodeCriterion[Q any, C comparable](dec *jsontext.Decoder, extract func(string) (C, bool), quantity bool) (cmp C, q Q, err error) {
	if dec.PeekKind() == 'n' {
		_, err = dec.ReadToken()
		return cmp, q, err
	}
	var w criterionWire[Q]
	if err = json.UnmarshalDecode(dec, &w, json.RejectUnknownMembers(true)); err != nil {
		return cmp, q, err
	}
	if w.Compare == nil && !quantity {
		return cmp, q, memberProblem(func(member string) string {
			return fmt.Sprintf(i18n.Text("missing %q"), member+".compare")
		})
	}
	var known bool
	if w.Compare == nil {
		w.Compare = new("")
	}
	if cmp, known = extract(*w.Compare); !known {
		return cmp, q, memberProblem(func(member string) string {
			return fmt.Sprintf(i18n.Text("unknown %q value %q"), member+".compare", *w.Compare)
		})
	}
	var none C
	switch {
	case cmp == none && !quantity:
		return cmp, q, memberProblem(func(member string) string {
			return fmt.Sprintf(i18n.Text("%q compares nothing"), member)
		})
	case cmp == none && w.Qualifier != nil:
		return cmp, q, memberProblem(func(member string) string {
			return fmt.Sprintf(i18n.Text("%q has a qualifier but compares nothing"), member)
		})
	case w.Qualifier != nil:
		q = *w.Qualifier
	default:
	}
	return cmp, q, nil
}

// memberProblem is an error a decoder reports about the member it was reading, which only the node holding it can name.
type memberProblem func(member string) string

// Error implements error.
func (m memberProblem) Error() string {
	return m("?")
}

// decodeError returns the Problem a failed read of a node's or operand's members reports: a Problem from a nested
// node or operand gains the path to it, and a decoder's problem or json/v2's error is given the member at fault, but
// never repeats the underlying error, which may carry a stack trace.
func decodeError(err error) error {
	var path Path
	var member string
	if se := (*json.SemanticError)(nil); errors.As(err, &se) {
		path, member = splitJSONPointer(se.JSONPointer)
	}
	if p := (Problem{}); errors.As(err, &p) {
		p.Path = append(path, p.Path...)
		return p
	}
	if m := memberProblem(nil); errors.As(err, &m) {
		return Problem{Path: path, Msg: m(member)}
	}
	if errors.Is(err, json.ErrUnknownName) {
		return Problem{Path: path, Msg: fmt.Sprintf(i18n.Text("unknown member %q"), member)}
	}
	return Problem{Path: path, Msg: fmt.Sprintf(i18n.Text("invalid value for %q"), member)}
}

// splitJSONPointer splits a pointer into the slots it leads through and the member it then ends at, joined with dots.
func splitJSONPointer(ptr jsontext.Pointer) (path Path, member string) {
	tokens := slices.Collect(ptr.Tokens())
	for len(tokens) != 0 {
		kind, ok := slotKinds[tokens[0]]
		if !ok {
			break
		}
		s := Slot{Kind: kind}
		if kind == SlotChild || kind == SlotSum {
			if len(tokens) < 2 {
				break
			}
			var err error
			if s.Index, err = strconv.Atoi(tokens[1]); err != nil {
				break
			}
			tokens = tokens[1:]
		}
		path = append(path, s)
		tokens = tokens[1:]
	}
	return path, strings.Join(tokens, ".")
}

// slotKinds maps the members that hold nodes and operands to the kind of slot they are.
var slotKinds = map[string]SlotKind{
	"children":  SlotChild,
	"only_when": SlotOnlyWhen,
	"where":     SlotWhere,
	"left":      SlotLeft,
	"right":     SlotRight,
	"sum":       SlotSum,
}

func problem(msg string) error {
	return Problem{Msg: msg}
}

func reasonTooDeep() string {
	return fmt.Sprintf(i18n.Text("nested more than %d deep"), MaxDepth)
}

// typedShadow is a node's wire type ahead of the fields of its shadow.
type typedShadow[T any] struct {
	Type   string `json:"type"`
	Fields *T     `json:",embed"`
}

// encodeTyped writes a node through its shadow, with the wire type first.
func encodeTyped[T any](enc *jsontext.Encoder, typ string, fields *T) error {
	return json.MarshalEncode(enc, &typedShadow[T]{Type: typ, Fields: fields})
}
