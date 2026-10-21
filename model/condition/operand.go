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
	"hash"

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

// OperandSource identifies where an Operand takes its value from. It is never persisted.
type OperandSource byte

// Possible OperandSource values.
const (
	SourceNone OperandSource = iota
	SourceValue
	SourceWeight
	SourceAttribute
	SourceEntity
	SourceSelf
	SourceAggregate
	SourceSum
	SourceMany
)

// Operand is one side of a Compare. Exactly one source should be set; the resolved value v then becomes
// v * Scale + Offset, with a nil Scale meaning 1. An Operand is not a node.
type Operand struct {
	Value     *fxp.Int    `json:"value,omitzero"`  // pointer so 0 is a real constant
	Weight    *fxp.Weight `json:"weight,omitzero"` // in pounds; written like "10.79 lb"
	Attribute string      `json:"attribute,omitzero"`
	Entity    string      `json:"entity,omitzero"`
	Self      string      `json:"self,omitzero"`
	Aggregate Aggregate   `json:"aggregate,omitzero"`
	Members               // with Aggregate only
	Field     string      `json:"field,omitzero"` // sum, max and min only
	Sum       []Operand   `json:"sum,omitempty"`
	Scale     *fxp.Int    `json:"scale,omitzero"`
	Offset    fxp.Int     `json:"offset,omitzero"`
}

// operandFields is the shadow Operand is written and read through; see codec.go.
type operandFields Operand

// Source returns where the operand takes its value from: SourceNone when nothing is set, SourceMany when more than
// one source is.
func (o *Operand) Source() OperandSource {
	src := SourceNone
	count := 0
	for _, one := range []struct {
		set bool
		src OperandSource
	}{
		{o.Value != nil, SourceValue},
		{o.Weight != nil, SourceWeight},
		{o.Attribute != "", SourceAttribute},
		{o.Entity != "", SourceEntity},
		{o.Self != "", SourceSelf},
		{o.Aggregate != AggregateNone, SourceAggregate},
		{len(o.Sum) != 0, SourceSum},
	} {
		if one.set {
			src = one.src
			count++
		}
	}
	if count > 1 {
		return SourceMany
	}
	return src
}

func (o *Operand) clone() Operand {
	c := *o
	c.Value = clonePtr(o.Value)
	c.Weight = clonePtr(o.Weight)
	c.Members = o.cloneMembers()
	if o.Sum != nil {
		c.Sum = make([]Operand, len(o.Sum))
		for i := range o.Sum {
			c.Sum[i] = o.Sum[i].clone()
		}
	}
	c.Scale = clonePtr(o.Scale)
	return c
}

func (o *Operand) hash(h hash.Hash) {
	hashIntPtr(h, o.Value)
	hashIntPtr(h, o.Weight)
	xhash.StringWithLen(h, o.Attribute)
	xhash.StringWithLen(h, o.Entity)
	xhash.StringWithLen(h, o.Self)
	xhash.StringWithLen(h, o.Aggregate.Key())
	o.hashMembers(h)
	xhash.StringWithLen(h, o.Field)
	xhash.Num64(h, len(o.Sum))
	for i := range o.Sum {
		o.Sum[i].hash(h)
	}
	hashIntPtr(h, o.Scale)
	xhash.Num64(h, o.Offset)
}

func (o *Operand) fillWithNameableKeys(m, existing map[string]string) {
	o.Where.fillWithNameableKeys(m, existing)
	for i := range o.Sum {
		o.Sum[i].fillWithNameableKeys(m, existing)
	}
}

func (o *Operand) validate(v *validator) {
	switch o.Source() {
	case SourceNone:
		v.report(i18n.Text("an operand has no value"))
	case SourceMany:
		v.report(i18n.Text("an operand has more than one value"))
	default:
	}
	switch o.Aggregate {
	case AggregateNone:
		if o.Of != "" || o.IncludeSelf || !o.Where.IsZero() || o.Field != "" {
			v.report(i18n.Text("an operand has a collection or field but no aggregate"))
		}
	case AggregateCount:
		if o.Field != "" {
			v.report(i18n.Text("a count has a field"))
		}
	case AggregateSum, AggregateMax, AggregateMin:
		if o.Field == "" {
			v.report(i18n.Text("an aggregate has no field"))
		}
	default:
		v.report(i18n.Text("unknown aggregate"))
	}
	if o.Aggregate != AggregateNone && o.Of == "" {
		v.report(i18n.Text("an aggregate has no collection"))
	}
	v.ref(SlotWhere, o.Where)
	for i := range o.Sum {
		v.operand(Slot{Kind: SlotSum, Index: i}, &o.Sum[i])
	}
}

func (o *Operand) walk(w *walker) {
	w.ref(SlotWhere, o.Where)
	for i := range o.Sum {
		w.operand(Slot{Kind: SlotSum, Index: i}, &o.Sum[i])
	}
}

// constants reports whether the operand, or any operand in its sum, is a Number or a Weight constant.
func (o *Operand) constants() (number, weight bool) {
	number = o.Value != nil
	weight = o.Weight != nil
	for i := range o.Sum {
		n, w := o.Sum[i].constants()
		number = number || n
		weight = weight || w
	}
	return number, weight
}

// MarshalJSONTo implements json.MarshalerTo.
func (o *Operand) MarshalJSONTo(enc *jsontext.Encoder) error {
	if o.Aggregate != o.Aggregate.EnsureValid() {
		return errors.New(i18n.Text("unknown aggregate"))
	}
	return json.MarshalEncode(enc, (*operandFields)(o))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (o *Operand) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	return o.decode(raw, 1)
}

func (o *Operand) decode(raw jsontext.Value, depth int) error {
	switch {
	case depth > MaxDepth:
		return problem(reasonTooDeep())
	case raw.Kind() != '{':
		return problem(i18n.Text("an operand is not an object"))
	}
	*o = Operand{}
	if err := json.Unmarshal(raw, (*operandFields)(o), decodeOptions(depth)); err != nil {
		return decodeError(err)
	}
	return nil
}
