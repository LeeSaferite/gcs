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

var _ Node = &Compare{}

// Compare compares two operands. Wire type "compare".
type Compare struct {
	Left  Operand        `json:"left"`
	Op    CompareOp      `json:"op"` // required and known, else the node is Unknown
	Right Operand        `json:"right"`
	Not   bool           `json:"not,omitzero"`
	Extra jsontext.Value `json:",embed"`
}

// compareFields is the method-less shadow Compare is written through.
type compareFields Compare

// compareWire is the raw form Compare is read through.
type compareWire struct {
	Left  jsontext.Value `json:"left"`
	Op    *string        `json:"op"`
	Right jsontext.Value `json:"right"`
	Not   bool           `json:"not"`
	Type  string         `json:"type"`
	Extra jsontext.Value `json:",embed"`
}

// Kind implements Node.
func (c *Compare) Kind() NodeKind {
	return KindCompare
}

// Clone implements Node.
func (c *Compare) Clone() Node {
	return &Compare{
		Left:  c.Left.clone(),
		Op:    c.Op,
		Right: c.Right.clone(),
		Not:   c.Not,
		Extra: slices.Clone(c.Extra),
	}
}

// Hash implements Node.
func (c *Compare) Hash(h hash.Hash) {
	xhash.Num8(h, KindCompare)
	c.Left.hash(h)
	xhash.StringWithLen(h, c.Op.Key())
	c.Right.hash(h)
	xhash.Bool(h, c.Not)
	hashExtra(h, c.Extra)
}

// FillWithNameableKeys implements Node.
func (c *Compare) FillWithNameableKeys(m, existing map[string]string) {
	c.Left.fillWithNameableKeys(m, existing)
	c.Right.fillWithNameableKeys(m, existing)
}

// Validate implements Node.
func (c *Compare) Validate() []Problem {
	return validateNode(c)
}

func (c *Compare) validate(v *validator, depth int) {
	if c.Op != c.Op.EnsureValid() {
		v.report(i18n.Text("unknown operator"))
	}
	leftNumber, leftWeight := c.Left.constants()
	rightNumber, rightWeight := c.Right.constants()
	if (leftNumber || rightNumber) && (leftWeight || rightWeight) {
		v.report(i18n.Text("number and weight constants are mixed"))
	}
	v.operand(Slot{Kind: SlotLeft}, &c.Left, depth+1)
	v.operand(Slot{Kind: SlotRight}, &c.Right, depth+1)
}

func (c *Compare) walk(w *walker) {
	w.operand(Slot{Kind: SlotLeft}, &c.Left)
	w.operand(Slot{Kind: SlotRight}, &c.Right)
}

// MarshalJSONTo implements json.MarshalerTo.
func (c *Compare) MarshalJSONTo(enc *jsontext.Encoder) error {
	other := *c
	other.Op = other.Op.EnsureValid()
	other.Extra = canonicalExtra(other.Extra)
	return encodeTyped(enc, typeCompare, (*compareFields)(&other))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (c *Compare) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, c)
}

func (c *Compare) decode(raw jsontext.Value, depth int) error {
	var w compareWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	op, err := requiredKey(w.Op, "op", ExtractKnownCompareOp)
	if err != nil {
		return err
	}
	var left, right Operand
	if err = left.decodeSide(w.Left, depth+1); err != nil {
		return err
	}
	if err = right.decodeSide(w.Right, depth+1); err != nil {
		return err
	}
	*c = Compare{Left: left, Op: op, Right: right, Not: w.Not, Extra: canonicalExtra(w.Extra)}
	return nil
}

// eval implements Node. Both operands must resolve, and a Number may not be compared with a Weight.
func (c *Compare) eval(e *evaluator, f frame) Result {
	if r, bad := e.invalid(c, f); bad {
		return r
	}
	oe := operandEval{e: e, collect: e.collect}
	left, fail := oe.resolve(&c.Left, f.next())
	if fail != nil {
		return *fail
	}
	right, fail := oe.resolve(&c.Right, f.next())
	if fail != nil {
		return *fail
	}
	kind, fail := combineKinds(left.kind, right.kind)
	if fail != nil {
		return *fail
	}
	if kind == ValueAbsent {
		kind = ValueNumber
	}
	r := Result{
		State:    metState(c.Op.Compare(left.value, right.value), c.Not),
		Have:     left.value,
		HaveKind: kind,
		Need:     right.value,
	}
	if oe.collect {
		r.hits = append(make([]Hit, 0, len(oe.contrib)+1), Hit{Node: c})
		for i := range oe.contrib {
			one := &oe.contrib[i]
			r.hits = append(r.hits, Hit{Node: c, Item: one.item})
			if one.where.Node != nil {
				r.Children = append(r.Children, one.where)
			}
		}
		if r.State == Met && !c.Not {
			r.Specificity = 1
		}
	}
	return r
}
