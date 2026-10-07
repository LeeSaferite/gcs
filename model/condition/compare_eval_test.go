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
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/check"
)

// compareFixture returns a host whose self has a mixed "children" collection, plus attributes and entity values.
func compareFixture() (h *fakeHost, self *fakeItem, kids []*fakeItem) {
	h = newHost()
	h.attributes["st"] = fxp.FromInteger(10)
	h.attributes["iq"] = fxp.FromInteger(12)
	h.entities["tech_level"] = fxp.FromInteger(3)
	kids = []*fakeItem{
		newItem("a", map[string]Value{"level": numberValue(2), "tag": textValue("x")}),
		newItem("b", map[string]Value{"level": numberValue(5), "tag": textValue("x")}),
		newItem("c", map[string]Value{"level": weightValue(9), "tag": textValue("x")}),
		newItem("d", map[string]Value{"level": numberValue(5), "tag": textValue("y")}),
		newItem("e", map[string]Value{"level": textValue("high"), "tag": textValue("x")}),
		newItem("f", map[string]Value{"level": absentValue(), "tag": textValue("x")}),
		newItem("g", map[string]Value{"tag": textValue("x")}),
	}
	self = newItem("self", map[string]Value{"level": numberValue(4), "weight": weightValue(7), "name": textValue("me")}).
		with("children", kids...)
	return h, self, kids
}

func agg(kind Aggregate, field string, where Node) Operand {
	return Operand{Aggregate: kind, Of: "children", Where: Ref{Node: where}, Field: field}
}

func cmpNode(left Operand, op CompareOp, right Operand) *Compare { //nolint:gocritic // Values read best in the tables.
	return &Compare{Left: left, Op: op, Right: right}
}

func value(n int64) Operand {
	return Operand{Value: intPtr(n)}
}

// TestCompareOperands checks each operand source, aggregates over a heterogeneous collection (members with the wrong
// kind, Absent or missing fields are skipped), scale and offset, and the Number against Weight rule.
func TestCompareOperands(t *testing.T) {
	h, self, _ := compareFixture()
	x := checkText("tag", criteria.IsText, "x")
	scaled := Operand{Attribute: "st", Scale: fxp.FromInteger(2), Offset: fxp.FromInteger(-1)}
	for _, tc := range []struct {
		name  string
		node  *Compare
		state State
		have  int64
		kind  ValueKind
	}{
		{"value", cmpNode(value(3), OpIs, value(3)), Met, 3, ValueNumber},
		{"attribute", cmpNode(Operand{Attribute: "st"}, OpMoreThan, Operand{Attribute: "iq"}), Unmet, 10, ValueNumber},
		{"entity", cmpNode(Operand{Entity: "tech_level"}, OpAtLeast, value(3)), Met, 3, ValueNumber},
		{"self number", cmpNode(Operand{Self: "level"}, OpLessThan, value(5)), Met, 4, ValueNumber},
		{"self weight", cmpNode(Operand{Self: "weight"}, OpAtMost, Operand{Weight: weightPtr(7)}), Met, 7, ValueWeight},
		{"scale and offset", cmpNode(scaled, OpIs, value(19)), Met, 19, ValueNumber},
		{"count", cmpNode(agg(AggregateCount, "", x), OpIs, value(6)), Met, 6, ValueNumber},
		{"count all", cmpNode(agg(AggregateCount, "", nil), OpIs, value(7)), Met, 7, ValueNumber},
		{"sum skips other kinds", cmpNode(agg(AggregateSum, "level", x), OpIs, value(7)), Met, 7, ValueNumber},
		{"max", cmpNode(agg(AggregateMax, "level", nil), OpIs, value(5)), Met, 5, ValueNumber},
		{"min", cmpNode(agg(AggregateMin, "level", nil), OpIs, value(2)), Met, 2, ValueNumber},
		{"max of none", cmpNode(agg(AggregateMax, "level", &Const{}), OpIs, value(0)), Met, 0, ValueNumber},
		{"max of none default", cmpNode(Operand{Aggregate: AggregateMax, Of: "children", Where: Ref{Node: &Const{}}, Field: "level", Default: intPtr(-1)}, OpIs, value(-1)), Met, -1, ValueNumber},
		{"sum of none with weight", cmpNode(agg(AggregateSum, "level", &Const{}), OpIs, Operand{Weight: weightPtr(0)}), Met, 0, ValueWeight},
		{"untyped both", cmpNode(agg(AggregateSum, "level", &Const{}), OpIs, agg(AggregateMin, "level", &Const{})), Met, 0, ValueNumber},
		{"sum", cmpNode(Operand{Sum: []Operand{{Attribute: "st"}, {Attribute: "iq"}, value(-2)}}, OpIs, value(20)), Met, 20, ValueNumber},
		{"not", &Compare{Left: value(1), Op: OpIsNot, Right: value(1), Not: true}, Met, 1, ValueNumber},
		{"unknown attribute", cmpNode(Operand{Attribute: "zz"}, OpIs, value(1)), Failed, 0, ValueAbsent},
		{"unknown entity", cmpNode(value(1), OpIs, Operand{Entity: "zz"}), Failed, 0, ValueAbsent},
		{"self text", cmpNode(Operand{Self: "name"}, OpIs, value(1)), Failed, 0, ValueAbsent},
		{"self missing", cmpNode(Operand{Self: "zz"}, OpIs, value(1)), Failed, 0, ValueAbsent},
		{"number against weight", cmpNode(Operand{Self: "weight"}, OpIs, value(1)), Failed, 0, ValueAbsent},
		{"sum mixing", cmpNode(Operand{Sum: []Operand{value(1), {Self: "weight"}}}, OpIs, value(1)), Failed, 0, ValueAbsent},
		{"sum failing", cmpNode(Operand{Sum: []Operand{value(1), {Attribute: "zz"}}}, OpIs, value(1)), Failed, 0, ValueAbsent},
		{"unknown collection", cmpNode(Operand{Aggregate: AggregateCount, Of: "zz"}, OpIs, value(1)), Failed, 0, ValueAbsent},
		{"global failed where", cmpNode(agg(AggregateCount, "", &Unknown{}), OpIs, value(0)), Failed, 0, ValueAbsent},
		{"local failed where", cmpNode(agg(AggregateCount, "", localFailure()), OpIs, value(0)), Met, 0, ValueNumber},
		{"invalid", cmpNode(Operand{}, OpIs, value(1)), Failed, 0, ValueAbsent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			for _, opt := range []Options{yesNo(), collect()} {
				r := Evaluate(tc.node, h, self, opt)
				c.Equal(tc.state, r.State, r.Reason)
				if tc.state != Failed {
					c.Equal(fxp.FromInteger(tc.have), r.Have)
					c.Equal(tc.kind, r.HaveKind)
				} else {
					c.False(r.MemberLocal())
				}
			}
		})
	}
}

func TestCompareOps(t *testing.T) {
	c := check.New(t)
	one, two := fxp.One, fxp.Two
	for op, want := range map[CompareOp][3]bool{
		OpIs:       {false, true, false},
		OpIsNot:    {true, false, true},
		OpAtLeast:  {false, true, true},
		OpAtMost:   {true, true, false},
		OpMoreThan: {false, false, true},
		OpLessThan: {true, false, false},
	} {
		c.Equal(want, [3]bool{op.Compare(one, two), op.Compare(one, one), op.Compare(two, one)}, op.Key())
	}
	c.True(CompareOp(99).Compare(one, one))
}

// TestCompareHits checks that a Met compare cites itself and the members its aggregates drew on: every counted or
// summed member, and for max and min only the members at the extreme, ties included.
func TestCompareHits(t *testing.T) {
	c := check.New(t)
	h, self, kids := compareFixture()
	x := checkText("tag", criteria.IsText, "x")
	node := cmpNode(agg(AggregateMax, "level", nil), OpAtLeast, agg(AggregateCount, "", x))
	r := Evaluate(node, h, self, collect())
	c.Equal(Unmet, r.State)
	node = cmpNode(agg(AggregateMax, "level", nil), OpAtLeast, value(5))
	r = Evaluate(node, h, self, collect())
	c.Equal(Met, r.State)
	c.Equal(1, r.Specificity)
	var items []Item
	for hit := range r.Hits() {
		c.Equal(Node(node), hit.Node)
		items = append(items, hit.Item)
	}
	c.Equal([]Item{nil, kids[1], kids[3]}, items)
	c.Equal(0, len(r.Children))

	node = cmpNode(agg(AggregateSum, "level", x), OpIs, value(7))
	r = Evaluate(node, h, self, collect())
	items = nil
	for hit := range r.Hits() {
		items = append(items, hit.Item)
	}
	// Only the members the sum drew on: c's weight, e's text and f's and g's missing values are skipped.
	c.Equal([]Item{nil, kids[0], kids[1], kids[0], kids[1]}, items)
	c.Equal(2, len(r.Children))
	hitCount := 0
	for hit := range r.Hits() {
		if hit.Node == Node(x) {
			hitCount++
		}
	}
	c.Equal(2, hitCount)

	negated := &Compare{Left: value(1), Op: OpIs, Right: value(2), Not: true}
	r = Evaluate(negated, h, self, collect())
	c.Equal(Met, r.State)
	c.Equal(0, r.Specificity)
	for range r.Hits() {
		c.True(false, "a negated compare gives no hits")
	}
}
