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
	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Check{}

// Check tests one field of the subject. At most one criterion may be set; none means the subject has the field, or for
// a Bool field that it is true. Wire type "check".
type Check struct {
	Field  string          `json:"field"`
	Not    bool            `json:"not,omitzero"`
	Text   criteria.Text   `json:"text,omitzero"`
	Number criteria.Number `json:"number,omitzero"`
	Weight criteria.Weight `json:"weight,omitzero"`
}

// checkFields is the shadow Check is written and read through; see codec.go.
type checkFields Check

// Clone implements Node.
func (c *Check) Clone() Node {
	other := *c
	return &other
}

// Hash implements Node.
func (c *Check) Hash(h hash.Hash) {
	xhash.Num8(h, hashCheck)
	xhash.StringWithLen(h, c.Field)
	xhash.Bool(h, c.Not)
	c.Text.Hash(h)
	c.Number.Hash(h)
	c.Weight.Hash(h)
}

// FillWithNameableKeys implements Node.
func (c *Check) FillWithNameableKeys(m, existing map[string]string) {
	if !c.Text.IsZero() {
		nameable.Extract(m, existing, literalRuns(c.Text.Qualifier)...)
	}
}

func (c *Check) validate(v *validator) {
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
	if !c.comparisonsKnown() {
		v.report(i18n.Text("a criterion has an unknown comparison"))
	}
	if !c.Text.IsZero() {
		if msg := tokenProblem(c.Text.Qualifier); msg != "" {
			v.report(msg)
		}
	}
}

func (c *Check) walk(_ *walker) {}

func (c *Check) comparisonsKnown() bool {
	return c.Text.Compare == c.Text.Compare.EnsureValid() && c.Number.Compare == c.Number.Compare.EnsureValid() &&
		c.Weight.Compare == c.Weight.Compare.EnsureValid()
}

// MarshalJSONTo implements json.MarshalerTo.
func (c *Check) MarshalJSONTo(enc *jsontext.Encoder) error {
	if !c.comparisonsKnown() {
		return errors.New(i18n.Text("a criterion has an unknown comparison"))
	}
	other := *c
	other.Text = canonicalText(other.Text)
	other.Number = canonicalNumber(other.Number)
	other.Weight = canonicalWeight(other.Weight)
	return encodeTyped(enc, typeCheck, (*checkFields)(&other))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (c *Check) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, c)
}

func (c *Check) decode(raw jsontext.Value, depth int) error {
	return decodeFields(raw, depth, (*checkFields)(c))
}
