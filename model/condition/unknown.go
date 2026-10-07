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
	"hash"
	"slices"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Unknown{}

// Unknown holds a node that could not be decoded: an unknown type, a malformed known node, an unknown enum or compare
// key, or a subtree nested too deeply. Its data is kept compacted and written back unchanged, so loading and saving a
// file never destroys it. It evaluates as a global Failed.
type Unknown struct {
	Type   string         // the "type" member, if one could be read
	Data   jsontext.Value // the node exactly as read, compacted
	Reason string         // why the node could not be decoded; not hashed
}

// Kind implements Node.
func (u *Unknown) Kind() NodeKind {
	return KindUnknown
}

// Clone implements Node.
func (u *Unknown) Clone() Node {
	return &Unknown{Type: u.Type, Data: slices.Clone(u.Data), Reason: u.Reason}
}

// Hash implements Node. The reason is not hashed.
func (u *Unknown) Hash(h hash.Hash) {
	xhash.Num8(h, KindUnknown)
	xhash.StringWithLen(h, u.Type)
	xhash.BytesWithLen(h, compactJSON(u.Data))
}

// FillWithNameableKeys implements Node. An unknown node has nothing to fill.
func (u *Unknown) FillWithNameableKeys(_, _ map[string]string) {}

// Validate implements Node.
func (u *Unknown) Validate() []Problem {
	return validateNode(u)
}

func (u *Unknown) validate(v *validator, _ int) {
	v.report(u.reason())
}

func (u *Unknown) reason() string {
	if u.Reason != "" {
		return u.Reason
	}
	return i18n.Text("unknown node")
}

func (u *Unknown) walk(_ *walker) {}

// MarshalJSONTo implements json.MarshalerTo. The data is written back as it was read. An Unknown built in code with no
// data is written as an object holding just its type, or as null when it has no type either.
func (u *Unknown) MarshalJSONTo(enc *jsontext.Encoder) error {
	switch {
	case len(u.Data) != 0:
		return enc.WriteValue(u.Data)
	case u.Type != "":
		return json.MarshalEncode(enc, &struct {
			Type string `json:"type"`
		}{Type: u.Type})
	default:
		return enc.WriteToken(jsontext.Null)
	}
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom. Any value is accepted and kept as the data.
func (u *Unknown) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	raw = compactJSON(raw)
	var head struct {
		Type string `json:"type"`
	}
	if raw.Kind() != '{' || json.Unmarshal(raw, &head) != nil {
		head.Type = ""
	}
	*u = Unknown{Type: head.Type, Data: raw}
	return nil
}

// eval implements Node. An unknown node is always a global Failed.
func (u *Unknown) eval(_ *evaluator, _ frame) Result {
	return failGlobal(u.reason())
}
