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

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Count{}

// Count counts the members of a collection that pass a where, optionally grouped by the values of a field. Wire type
// "count".
type Count struct {
	Members
	Quantity  *criteria.Number `json:"quantity"` // nil means at least 1; written exactly when non-nil, "any" as {}
	GroupBy   string           `json:"group_by,omitzero"`
	PerGroup  *criteria.Number `json:"per_group"` // nil means at least 1; written like Quantity
	Exclusive bool             `json:"exclusive,omitzero"`
	Not       bool             `json:"not,omitzero"`
	Extra     jsontext.Value   `json:",embed"`
}

// countFields is the field-by-field shadow Count is written through, with wrappers that write Quantity and PerGroup
// exactly when they are non-nil.
type countFields struct {
	Of        string         `json:"of,omitzero"`
	Where     Ref            `json:"where,omitzero"`
	Quantity  quantityRef    `json:"quantity,omitzero"`
	GroupBy   string         `json:"group_by,omitzero"`
	PerGroup  quantityRef    `json:"per_group,omitzero"`
	Exclusive bool           `json:"exclusive,omitzero"`
	Not       bool           `json:"not,omitzero"`
	Extra     jsontext.Value `json:",embed"`
}

// countWire is the raw form Count is read through.
type countWire struct {
	Of        string         `json:"of"`
	Where     jsontext.Value `json:"where"`
	Quantity  *numberWire    `json:"quantity"`
	GroupBy   string         `json:"group_by"`
	PerGroup  *numberWire    `json:"per_group"`
	Exclusive bool           `json:"exclusive"`
	Not       bool           `json:"not"`
	Type      string         `json:"type"`
	Extra     jsontext.Value `json:",embed"`
}

// Kind implements Node.
func (c *Count) Kind() NodeKind {
	return KindCount
}

// Clone implements Node.
func (c *Count) Clone() Node {
	other := *c
	other.Members = c.cloneMembers()
	if c.Quantity != nil {
		q := *c.Quantity
		other.Quantity = &q
	}
	if c.PerGroup != nil {
		p := *c.PerGroup
		other.PerGroup = &p
	}
	other.Extra = slices.Clone(c.Extra)
	return &other
}

// Hash implements Node.
func (c *Count) Hash(h hash.Hash) {
	xhash.Num8(h, KindCount)
	c.hashMembers(h)
	hashQuantity(h, c.Quantity)
	xhash.StringWithLen(h, c.GroupBy)
	hashQuantity(h, c.PerGroup)
	xhash.Bool(h, c.Exclusive)
	xhash.Bool(h, c.Not)
	hashExtra(h, c.Extra)
}

// FillWithNameableKeys implements Node.
func (c *Count) FillWithNameableKeys(m, existing map[string]string) {
	c.Where.fillWithNameableKeys(m, existing)
}

// Validate implements Node.
func (c *Count) Validate() []Problem {
	return validateNode(c)
}

func (c *Count) validate(v *validator, depth int) {
	if c.Of == "" {
		v.report(i18n.Text("no collection"))
	}
	if c.GroupBy == "" && c.PerGroup != nil {
		v.report(i18n.Text("a per-group quantity without a group-by field"))
	}
	if c.GroupBy == "" && c.Exclusive {
		v.report(i18n.Text("exclusive without a group-by field"))
	}
	v.ref(SlotWhere, c.Where, depth+1)
}

func (c *Count) walk(w *walker) {
	w.ref(SlotWhere, c.Where)
}

// MarshalJSONTo implements json.MarshalerTo.
func (c *Count) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encodeTyped(enc, typeCount, &countFields{
		Of:        c.Of,
		Where:     c.Where,
		Quantity:  quantityRef{p: canonicalQuantity(c.Quantity)},
		GroupBy:   c.GroupBy,
		PerGroup:  quantityRef{p: canonicalQuantity(c.PerGroup)},
		Exclusive: c.Exclusive,
		Not:       c.Not,
		Extra:     canonicalExtra(c.Extra),
	})
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (c *Count) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, c)
}

func (c *Count) decode(raw jsontext.Value, depth int) error {
	var w countWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	quantity, err := w.Quantity.quantity()
	if err != nil {
		return err
	}
	perGroup, err := w.PerGroup.quantity()
	if err != nil {
		return err
	}
	*c = Count{
		Of:        w.Of,
		Where:     Ref{Node: decodeRef(w.Where, depth+1)},
		Quantity:  quantity,
		GroupBy:   w.GroupBy,
		PerGroup:  perGroup,
		Exclusive: w.Exclusive,
		Not:       w.Not,
		Extra:     canonicalExtra(w.Extra),
	}
	return nil
}
