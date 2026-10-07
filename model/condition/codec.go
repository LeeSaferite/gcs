// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

// typedShadow writes a node's wire type ahead of the fields of its method-less shadow type.
type typedShadow[T any] struct {
	Type   string `json:"type"`
	Fields *T     `json:",embed"`
}

// encodeTyped writes a node through its shadow type, with the wire type first.
func encodeTyped[T any](enc *jsontext.Encoder, typ string, fields *T) error {
	return json.MarshalEncode(enc, &typedShadow[T]{Type: typ, Fields: fields})
}

// unmarshalNode reads one value from dec into the node dst, which must end up as the node type dst points to.
func unmarshalNode[T any, P interface {
	*T
	Node
}](dec *jsontext.Decoder, dst P) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	n := decodeNode(raw, 1)
	if typed, ok := n.(P); ok {
		*dst = *typed
		return nil
	}
	if u, ok := n.(*Unknown); ok {
		return errors.New(u.Reason)
	}
	return fmt.Errorf(i18n.Text("unexpected node type %q"), kindTypes[n.Kind()])
}

// requiredKey resolves a persisted enum key that must be present and known.
func requiredKey[E any](key *string, member string, extract func(string) (E, bool)) (E, error) {
	if key == nil {
		var zero E
		return zero, fmt.Errorf(i18n.Text("missing %s"), member)
	}
	return knownKey(*key, member, extract)
}

// knownKey resolves a persisted enum key that must be known.
func knownKey[E any](key, member string, extract func(string) (E, bool)) (E, error) {
	v, ok := extract(key)
	if !ok {
		return v, fmt.Errorf(i18n.Text("unknown %s %q"), member, key)
	}
	return v, nil
}

// textWire is the raw form of criteria.Text, decoded so that an unknown compare key can be detected rather than
// silently becoming "any".
type textWire struct {
	Compare   string `json:"compare"`
	Qualifier string `json:"qualifier"`
}

func (w *textWire) criteria() (criteria.Text, error) {
	var t criteria.Text
	if w == nil {
		return t, nil
	}
	var err error
	if t.Compare, err = knownKey(w.Compare, "text comparison", criteria.ExtractKnownStringComparison); err != nil {
		return t, err
	}
	t.Qualifier = w.Qualifier
	return canonicalText(t), nil
}

// numberWire is the raw form of criteria.Number.
type numberWire struct {
	Compare   string  `json:"compare"`
	Qualifier fxp.Int `json:"qualifier"`
}

func (w *numberWire) criteria() (criteria.Number, error) {
	var n criteria.Number
	if w == nil {
		return n, nil
	}
	var err error
	if n.Compare, err = knownKey(w.Compare, "numeric comparison", criteria.ExtractKnownNumericComparison); err != nil {
		return n, err
	}
	n.Qualifier = w.Qualifier
	return canonicalNumber(n), nil
}

// quantity returns the canonical quantity this wire form describes: nil when absent.
func (w *numberWire) quantity() (*criteria.Number, error) {
	if w == nil {
		return nil, nil
	}
	n, err := w.criteria()
	if err != nil {
		return nil, err
	}
	return canonicalQuantity(&n), nil
}

// weightWire is the raw form of criteria.Weight.
type weightWire struct {
	Compare   string     `json:"compare"`
	Qualifier fxp.Weight `json:"qualifier"`
}

func (w *weightWire) criteria() (criteria.Weight, error) {
	var c criteria.Weight
	if w == nil {
		return c, nil
	}
	var err error
	if c.Compare, err = knownKey(w.Compare, "weight comparison", criteria.ExtractKnownNumericComparison); err != nil {
		return c, err
	}
	c.Qualifier = w.Qualifier
	return canonicalWeight(c), nil
}

// quantityRef writes a Count quantity exactly when it is non-nil, "any" as {}. json/v2's omitzero on the pointer
// itself would drop a non-nil "any", since criteria.Number reports itself as zero then.
type quantityRef struct {
	p *criteria.Number
}

// IsZero implements json's omitzero check.
func (q quantityRef) IsZero() bool {
	return q.p == nil
}

// MarshalJSONTo implements json.MarshalerTo.
func (q quantityRef) MarshalJSONTo(enc *jsontext.Encoder) error {
	return json.MarshalEncode(enc, &q.p.NumberData)
}
