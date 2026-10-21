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
	"errors"
	"hash"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Count{}

// Members selects the members of a collection that pass a where. It is embedded (plain Go embedding) in Count and
// Operand. The owner (self) is left out of the collection unless IncludeSelf is set.
type Members struct {
	Of          string `json:"of,omitzero"`
	IncludeSelf bool   `json:"include_self,omitzero"`
	Where       Ref    `json:"where,omitzero"` // nil matches every member
}

func (m *Members) cloneMembers() Members {
	return Members{Of: m.Of, IncludeSelf: m.IncludeSelf, Where: m.Where.clone()}
}

func (m *Members) hashMembers(h hash.Hash) {
	xhash.StringWithLen(h, m.Of)
	xhash.Bool(h, m.IncludeSelf)
	m.Where.hash(h)
}

// Count counts the members of a collection that pass a where, optionally grouped by the values of a field. An
// exclusive grouped count counts each member toward one of its values only. Wire type "count".
type Count struct {
	Members
	Quantity  *criteria.Number `json:"quantity,omitzero"` // nil means at least 1; see canonicalQuantity
	GroupBy   string           `json:"group_by,omitzero"`
	PerGroup  *criteria.Number `json:"per_group,omitzero"` // as Quantity, of each group's members
	Exclusive bool             `json:"exclusive,omitzero"`
	Not       bool             `json:"not,omitzero"`
}

// countFields is the shadow Count is written and read through; see codec.go.
type countFields Count

// Clone implements Node.
func (c *Count) Clone() Node {
	other := *c
	other.Members = c.cloneMembers()
	other.Quantity = clonePtr(c.Quantity)
	other.PerGroup = clonePtr(c.PerGroup)
	return &other
}

// Hash implements Node.
func (c *Count) Hash(h hash.Hash) {
	xhash.Num8(h, hashCount)
	c.hashMembers(h)
	hashQuantity(h, c.Quantity)
	xhash.StringWithLen(h, c.GroupBy)
	hashQuantity(h, c.PerGroup)
	xhash.Bool(h, c.Exclusive)
	xhash.Bool(h, c.Not)
}

// FillWithNameableKeys implements Node.
func (c *Count) FillWithNameableKeys(m, existing map[string]string) {
	c.Where.fillWithNameableKeys(m, existing)
}

func (c *Count) validate(v *validator) {
	if c.Of == "" {
		v.report(i18n.Text("no collection"))
	}
	if !quantityKnown(c.Quantity) || !quantityKnown(c.PerGroup) {
		v.report(i18n.Text("a quantity has an unknown comparison"))
	}
	if c.GroupBy == "" && canonicalQuantity(c.PerGroup) != nil {
		v.report(i18n.Text("a per-group quantity without a group-by field"))
	}
	if c.Exclusive {
		switch {
		case c.GroupBy == "":
			v.report(i18n.Text("exclusive without a group-by field"))
		case !atLeastWhole(c.PerGroup):
			v.report(i18n.Text(`an exclusive count's per-group quantity is not "at least" a whole number`))
		default:
		}
	}
	v.ref(SlotWhere, c.Where)
}

func (c *Count) walk(w *walker) {
	w.ref(SlotWhere, c.Where)
}

// MarshalJSONTo implements json.MarshalerTo.
func (c *Count) MarshalJSONTo(enc *jsontext.Encoder) error {
	if !quantityKnown(c.Quantity) || !quantityKnown(c.PerGroup) {
		return errors.New(i18n.Text("a quantity has an unknown comparison"))
	}
	other := *c
	other.Quantity = canonicalQuantity(c.Quantity)
	other.PerGroup = canonicalQuantity(c.PerGroup)
	return encodeTyped(enc, typeCount, (*countFields)(&other))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (c *Count) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, c)
}

func (c *Count) decode(raw jsontext.Value, depth int) error {
	return decodeFields(raw, depth, (*countFields)(c))
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
