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
	"hash"
	"slices"

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
// v * Scale + Offset, with a Scale of 0 meaning 1. An Operand is not a node.
type Operand struct {
	Value     *fxp.Int       `json:"value,omitzero"`  // pointer so 0 is a real constant
	Weight    *fxp.Weight    `json:"weight,omitzero"` // in pounds; written like "10.79 lb"
	Attribute string         `json:"attribute,omitzero"`
	Entity    string         `json:"entity,omitzero"`
	Self      string         `json:"self,omitzero"`
	Aggregate Aggregate      `json:"aggregate,omitzero"`
	Members                  // with Aggregate; cleared otherwise
	Field     string         `json:"field,omitzero"`   // sum, max, min
	Default   *fxp.Int       `json:"default,omitzero"` // max/min of no members; nil means 0
	Sum       []Operand      `json:"sum,omitempty"`
	Scale     fxp.Int        `json:"scale,omitzero"` // 0 means 1
	Offset    fxp.Int        `json:"offset,omitzero"`
	Extra     jsontext.Value `json:",embed"`
}

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

// hasMembers reports whether the canonical view keeps Members, Field and Default.
func (o *Operand) hasMembers() bool {
	return o.Aggregate.EnsureValid() != AggregateNone
}

func (o *Operand) clone() Operand {
	c := *o
	c.Value = clonePtr(o.Value)
	c.Weight = clonePtr(o.Weight)
	c.Members = o.cloneMembers()
	c.Default = clonePtr(o.Default)
	if o.Sum != nil {
		c.Sum = make([]Operand, len(o.Sum))
		for i := range o.Sum {
			c.Sum[i] = o.Sum[i].clone()
		}
	}
	c.Extra = slices.Clone(o.Extra)
	return c
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func (o *Operand) hash(h hash.Hash) {
	c := o.canonical()
	hashIntPtr(h, c.Value)
	hashIntPtr(h, c.Weight)
	xhash.StringWithLen(h, c.Attribute)
	xhash.StringWithLen(h, c.Entity)
	xhash.StringWithLen(h, c.Self)
	xhash.StringWithLen(h, c.Aggregate.Key())
	c.hashMembers(h)
	xhash.StringWithLen(h, c.Field)
	hashIntPtr(h, c.Default)
	xhash.Num64(h, len(c.Sum))
	for i := range c.Sum {
		c.Sum[i].hash(h)
	}
	xhash.Num64(h, c.Scale)
	xhash.Num64(h, c.Offset)
	hashExtra(h, c.Extra)
}

func (o *Operand) fillWithNameableKeys(m, existing map[string]string) {
	if o.hasMembers() {
		o.Where.fillWithNameableKeys(m, existing)
	}
	for i := range o.Sum {
		o.Sum[i].fillWithNameableKeys(m, existing)
	}
}

func (o *Operand) validate(v *validator, depth int) {
	switch o.Source() {
	case SourceNone:
		v.report(i18n.Text("an operand has no value"))
	case SourceMany:
		v.report(i18n.Text("an operand has more than one value"))
	default:
	}
	switch o.Aggregate {
	case AggregateNone:
	case AggregateCount:
		if o.Default != nil {
			v.report(i18n.Text("a count has a default"))
		}
	case AggregateSum, AggregateMax, AggregateMin:
		if o.Field == "" {
			v.report(i18n.Text("an aggregate has no field"))
		}
	default:
		v.report(i18n.Text("unknown aggregate"))
	}
	if o.hasMembers() {
		if o.Of == "" {
			v.report(i18n.Text("an aggregate has no collection"))
		}
		v.ref(SlotWhere, o.Where, depth+1)
	}
	for i := range o.Sum {
		v.operand(Slot{Kind: SlotSum, Index: i}, &o.Sum[i], depth+1)
	}
}

func (o *Operand) walk(w *walker) {
	if o.hasMembers() {
		w.ref(SlotWhere, o.Where)
	}
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
