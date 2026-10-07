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

// Members selects the members of a collection that pass a where. It is embedded (plain Go embedding) in Count and
// Operand.
type Members struct {
	Of    string `json:"of,omitzero"`
	Where Ref    `json:"where,omitzero"` // nil matches every member
}

func (m *Members) cloneMembers() Members {
	return Members{Of: m.Of, Where: m.Where.clone()}
}

func (m *Members) hashMembers(h hash.Hash) {
	xhash.StringWithLen(h, m.Of)
	m.Where.hash(h)
}

// Selector is the standalone persisted form of Members, for gurps selectors. It has no "type" member. Keep it a named
// field in gurps structs, never Go-embedded, or its Extra would become their fallback.
type Selector struct {
	Members
	Extra jsontext.Value `json:",embed"`
}

// selectorFields is the method-less shadow Selector is written through.
type selectorFields Selector

// selectorWire is the raw form Selector is read through.
type selectorWire struct {
	Of    string         `json:"of"`
	Where jsontext.Value `json:"where"`
	Extra jsontext.Value `json:",embed"`
}

// Clone returns a deep copy.
func (s *Selector) Clone() Selector {
	return Selector{Members: s.cloneMembers(), Extra: slices.Clone(s.Extra)}
}

// Hash writes the canonical view into the hasher.
func (s *Selector) Hash(h hash.Hash) {
	s.hashMembers(h)
	hashExtra(h, s.Extra)
}

// FillWithNameableKeys adds the nameable keys of every qualifier and script beneath the where.
func (s *Selector) FillWithNameableKeys(m, existing map[string]string) {
	s.Where.fillWithNameableKeys(m, existing)
}

// Validate returns the problems found, in pre-order, with paths relative to the selector.
func (s *Selector) Validate() []Problem {
	var v validator
	if s.Of == "" {
		v.report(i18n.Text("no collection"))
	}
	v.ref(SlotWhere, s.Where, 1)
	return v.problems
}

// Walk calls Walk for the where, with paths starting at it.
func (s *Selector) Walk(fn func(path Path, n Node) bool) {
	(&walker{fn: fn}).ref(SlotWhere, s.Where)
}

// MarshalJSONTo implements json.MarshalerTo.
func (s Selector) MarshalJSONTo(enc *jsontext.Encoder) error {
	s.Extra = canonicalExtra(s.Extra)
	return json.MarshalEncode(enc, (*selectorFields)(&s))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom. The where decodes like any other Ref, so a bad node beneath it
// becomes an Unknown node rather than an error.
func (s *Selector) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	var w selectorWire
	if err := json.UnmarshalDecode(dec, &w); err != nil {
		return err
	}
	*s = Selector{
		Of:    w.Of,
		Where: Ref{Node: decodeRef(w.Where, 1)},
		Extra: canonicalExtra(w.Extra),
	}
	return nil
}
