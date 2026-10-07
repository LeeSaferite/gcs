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
	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Check{}

// Check tests one field of the subject. At most one non-any criterion may be set; none means "any". Wire type
// "check".
type Check struct {
	Field        string          `json:"field"`
	Not          bool            `json:"not,omitzero"`
	Text         criteria.Text   `json:"text,omitzero"`
	Number       criteria.Number `json:"number,omitzero"`
	Weight       criteria.Weight `json:"weight,omitzero"`
	AbsentPasses bool            `json:"absent_passes,omitzero"`
	Extra        jsontext.Value  `json:",embed"`
}

// checkFields is the method-less shadow Check is written through.
type checkFields Check

// checkWire is the raw form Check is read through, so that unknown compare keys can be detected.
type checkWire struct {
	Field        string         `json:"field"`
	Not          bool           `json:"not"`
	Text         *textWire      `json:"text"`
	Number       *numberWire    `json:"number"`
	Weight       *weightWire    `json:"weight"`
	AbsentPasses bool           `json:"absent_passes"`
	Type         string         `json:"type"`
	Extra        jsontext.Value `json:",embed"`
}

// Kind implements Node.
func (c *Check) Kind() NodeKind {
	return KindCheck
}

// Clone implements Node.
func (c *Check) Clone() Node {
	other := *c
	other.Extra = slices.Clone(c.Extra)
	return &other
}

// Hash implements Node.
func (c *Check) Hash(h hash.Hash) {
	xhash.Num8(h, KindCheck)
	xhash.StringWithLen(h, c.Field)
	xhash.Bool(h, c.Not)
	c.Text.Hash(h)
	c.Number.Hash(h)
	c.Weight.Hash(h)
	xhash.Bool(h, c.AbsentPasses)
	hashExtra(h, c.Extra)
}

// FillWithNameableKeys implements Node.
func (c *Check) FillWithNameableKeys(m, existing map[string]string) {
	if !c.Text.IsZero() {
		nameable.Extract(m, existing, c.Text.Qualifier)
	}
}

// Validate implements Node.
func (c *Check) Validate() []Problem {
	return validateNode(c)
}

func (c *Check) validate(v *validator, _ int) {
	if c.Field == "" {
		v.report(i18n.Text("no field"))
	}
	set := 0
	for _, zero := range []bool{c.Text.IsZero(), c.Number.IsZero(), c.Weight.IsZero()} {
		if !zero {
			set++
		}
	}
	if set > 1 {
		v.report(i18n.Text("more than one criterion"))
	}
	if c.Text.Compare != c.Text.Compare.EnsureValid() || c.Number.Compare != c.Number.Compare.EnsureValid() ||
		c.Weight.Compare != c.Weight.Compare.EnsureValid() {
		v.report(i18n.Text("a criterion has an unknown comparison"))
	}
}

func (c *Check) walk(_ *walker) {}

// canonical returns a copy in canonical form.
func (c *Check) canonical() Check {
	other := *c
	other.Text = canonicalText(other.Text)
	other.Number = canonicalNumber(other.Number)
	other.Weight = canonicalWeight(other.Weight)
	other.Extra = canonicalExtra(other.Extra)
	return other
}

// MarshalJSONTo implements json.MarshalerTo.
func (c *Check) MarshalJSONTo(enc *jsontext.Encoder) error {
	other := c.canonical()
	return encodeTyped(enc, typeCheck, (*checkFields)(&other))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (c *Check) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, c)
}

func (c *Check) decode(raw jsontext.Value, _ int) error {
	var w checkWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	text, err := w.Text.criteria()
	if err != nil {
		return err
	}
	number, err := w.Number.criteria()
	if err != nil {
		return err
	}
	weight, err := w.Weight.criteria()
	if err != nil {
		return err
	}
	*c = Check{
		Field:        w.Field,
		Not:          w.Not,
		Text:         text,
		Number:       number,
		Weight:       weight,
		AbsentPasses: w.AbsentPasses,
		Extra:        canonicalExtra(w.Extra),
	}
	return nil
}
