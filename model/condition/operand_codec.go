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

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

// operandFields is the method-less shadow Operand is written through.
type operandFields Operand

// operandWire is the raw form Operand is read through.
type operandWire struct {
	Value     *fxp.Int         `json:"value"`
	Weight    *fxp.Weight      `json:"weight"`
	Attribute string           `json:"attribute"`
	Entity    string           `json:"entity"`
	Self      string           `json:"self"`
	Aggregate string           `json:"aggregate"`
	Of        string           `json:"of"`
	Where     jsontext.Value   `json:"where"`
	Field     string           `json:"field"`
	Default   *fxp.Int         `json:"default"`
	Sum       []jsontext.Value `json:"sum"`
	Scale     fxp.Int          `json:"scale"`
	Offset    fxp.Int          `json:"offset"`
	Extra     jsontext.Value   `json:",embed"`
}

// canonical returns a shallow copy in canonical form.
func (o *Operand) canonical() Operand {
	c := *o
	c.Aggregate = c.Aggregate.EnsureValid()
	if c.Aggregate == AggregateNone {
		c.Members = Members{}
		c.Field = ""
		c.Default = nil
	}
	if c.Scale == fxp.One {
		c.Scale = 0
	}
	c.Extra = canonicalExtra(c.Extra)
	return c
}

// MarshalJSONTo implements json.MarshalerTo.
func (o *Operand) MarshalJSONTo(enc *jsontext.Encoder) error {
	c := o.canonical()
	return json.MarshalEncode(enc, (*operandFields)(&c))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom. A where beneath it decodes like any other Ref, but a malformed
// operand is an error, since an operand is not a node and so can't become an Unknown one.
func (o *Operand) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	return o.decode(compactJSON(raw), 1)
}

func (o *Operand) decode(raw jsontext.Value, depth int) error {
	if depth > MaxDepth {
		return errors.New(reasonTooDeep())
	}
	if raw.Kind() != '{' {
		return errors.New(i18n.Text("an operand is not an object"))
	}
	var w operandWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	agg, err := knownKey(w.Aggregate, "aggregate", ExtractKnownAggregate)
	if err != nil {
		return err
	}
	var sum []Operand
	if w.Sum != nil {
		sum = make([]Operand, len(w.Sum))
		for i, one := range w.Sum {
			if err = sum[i].decode(one, depth+1); err != nil {
				return err
			}
		}
	}
	*o = Operand{
		Value:     w.Value,
		Weight:    w.Weight,
		Attribute: w.Attribute,
		Entity:    w.Entity,
		Self:      w.Self,
		Aggregate: agg,
		Of:        w.Of,
		Where:     Ref{Node: decodeRef(w.Where, depth+1)},
		Field:     w.Field,
		Default:   w.Default,
		Sum:       sum,
		Scale:     w.Scale,
		Offset:    w.Offset,
		Extra:     canonicalExtra(w.Extra),
	}
	*o = o.canonical()
	return nil
}

// decodeSide decodes one side of a compare. An absent side decodes like {}: an operand with no source, which Validate
// reports.
func (o *Operand) decodeSide(raw jsontext.Value, depth int) error {
	if len(raw) == 0 {
		raw = jsontext.Value("{}")
	}
	return o.decode(raw, depth)
}
