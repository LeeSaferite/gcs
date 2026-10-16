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
	"fmt"

	"github.com/richardwilkes/toolbox/v2/i18n"
)

// MaxDepth is the deepest a node may sit, counting the root as 1 and every step through children, where, only_when,
// operands and sums. Decode turns a subtree that starts deeper into an Unknown node; Validate reports one built in
// code.
const MaxDepth = 32

// nodeDecoder is implemented by every node type that has a typed body.
type nodeDecoder interface {
	Node
	decode(raw jsontext.Value, depth int) error
}

// MarshalJSONTo implements json.MarshalerTo. A zero Ref is written as null.
func (r Ref) MarshalJSONTo(enc *jsontext.Encoder) error {
	if r.IsZero() {
		return enc.WriteToken(jsontext.Null)
	}
	return json.MarshalEncode(enc, r.Node)
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom. Anything that can't be decoded becomes an Unknown node rather
// than an error, so one bad node never fails the file.
func (r *Ref) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	r.Node = decodeRef(raw, 1)
	return nil
}

// MarshalJSONTo implements json.MarshalerTo. A nil element is written as null.
func (n Nodes) MarshalJSONTo(enc *jsontext.Encoder) error {
	if err := enc.WriteToken(jsontext.BeginArray); err != nil {
		return err
	}
	for _, one := range n {
		var err error
		if one == nil {
			err = enc.WriteToken(jsontext.Null)
		} else {
			err = json.MarshalEncode(enc, one)
		}
		if err != nil {
			return err
		}
	}
	return enc.WriteToken(jsontext.EndArray)
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom. Elements that can't be decoded become Unknown nodes; a null
// element becomes an Unknown node with an empty Type.
func (n *Nodes) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	var list []jsontext.Value
	if err := json.UnmarshalDecode(dec, &list); err != nil {
		return err
	}
	*n = decodeNodes(list, 1)
	return nil
}

// decodeRef decodes the node of a Ref slot at the given depth. An absent or null slot and the canonical empty group
// both become nil.
func decodeRef(raw jsontext.Value, depth int) Node {
	if len(raw) == 0 || raw.Kind() == 'n' {
		return nil
	}
	n := decodeNode(raw, depth)
	if (Ref{Node: n}).IsZero() {
		return nil
	}
	return n
}

// decodeNodes decodes a list of nodes, each at the given depth.
func decodeNodes(list []jsontext.Value, depth int) Nodes {
	if list == nil {
		return nil
	}
	nodes := make(Nodes, len(list))
	for i, raw := range list {
		nodes[i] = decodeNode(raw, depth)
	}
	return nodes
}

// decodeNodeRaw decodes one node at the given depth by parsing its raw bytes into the node's wire form. It never
// fails: anything it can't turn into a typed node becomes an Unknown node holding the compacted raw bytes and the
// reason. decodeNode streams instead and comes here only for what it doesn't handle itself.
func decodeNodeRaw(raw jsontext.Value, depth int) Node {
	raw = compactJSON(raw)
	if raw.Kind() == 'n' {
		return &Unknown{Data: raw, Reason: i18n.Text("missing node")}
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return &Unknown{Data: raw, Reason: err.Error()}
	}
	if depth > MaxDepth {
		return &Unknown{Type: head.Type, Data: raw, Reason: reasonTooDeep()}
	}
	var n nodeDecoder
	switch head.Type {
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
		return &Unknown{Type: head.Type, Data: raw, Reason: fmt.Sprintf(i18n.Text("unknown node type %q"), head.Type)}
	}
	if err := n.decode(raw, depth); err != nil {
		return &Unknown{Type: head.Type, Data: raw, Reason: err.Error()}
	}
	return n
}

func reasonTooDeep() string {
	return fmt.Sprintf(i18n.Text("nested more than %d deep"), MaxDepth)
}
