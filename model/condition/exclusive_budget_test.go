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
	"fmt"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/toolbox/v2/check"
)

type budgetCase struct {
	name  string
	tree  Node
	spell []*fakeItem
}

func budgetCases() []budgetCase {
	dispel := exclusive(spellsWhere(checkText("name", criteria.IsText, "Counterspell"), nil), spellsWhere(nil, atLeast(12)))
	earth := func(q *criteria.Number) *Count { return spellsWhere(inCollege("Earth"), q) }
	water := func() *Count { return spellsWhere(inCollege("Water"), nil) }
	grouped := func(q, per *criteria.Number, excl bool) *Count {
		return &Count{Of: "spells", GroupBy: "college", Quantity: q, PerGroup: per, Exclusive: excl}
	}
	mixed := []*fakeItem{
		spell("m1", "A"), spell("m2", "A", "B"), spell("m3", "A"), spell("x", "B"), spell("Mud Jet", "Earth", "Water"),
		spell("E1", "Earth"),
	}
	return []budgetCase{
		{"dispel", dispel, append([]*fakeItem{spell("Counterspell")}, numbered("Other", 12)...)},
		{"greedy counterexample", exclusive(earth(nil), water()), []*fakeItem{spell("Mud Jet", "Earth", "Water"), spell("Shape Earth", "Earth")}},
		{"choices", exclusive(&Group{Mode: GroupAny, Children: Nodes{earth(atLeast(3)), &Group{Children: Nodes{earth(nil), water()}}}}, earth(nil)), mixed},
		{"infeasible choices", exclusive(&Group{Mode: GroupAny, Children: Nodes{earth(atLeast(5)), water()}}, water()), mixed},
		{"grouped pool", exclusive(grouped(atLeast(2), atLeast(2), false), earth(nil)), mixed},
		{"grouped one", exclusive(grouped(atLeast(3), nil, false), water()), mixed},
		{"accelerate", grouped(atLeast(2), atLeast(2), true), mixed},
		{"accelerate is", grouped(exactly(2), atLeast(2), true), mixed},
		{"accelerate one", grouped(atLeast(4), nil, true), mixed},
	}
}

// TestSolverBudgetSweep evaluates each case under every small state and detail budget: the state is always the one an
// unlimited evaluation gives, or a global Failed "too complex to decide", and Collect always agrees with YesNo.
func TestSolverBudgetSweep(t *testing.T) {
	c := check.New(t)
	for _, tc := range budgetCases() {
		h, self := spellBook(tc.spell...)
		want := Evaluate(tc.tree, h, self, yesNo()).State
		c.Equal(want, Evaluate(tc.tree, h, self, collect()).State, tc.name)
		witnessed := false
		for budget := 1; budget <= 120; budget++ {
			r := Evaluate(tc.tree, h, self, Options{Budget: budget})
			desc := fmt.Sprintf("%s budget %d", tc.name, budget)
			if r.State != want {
				c.Equal(Failed, r.State, desc)
				c.Equal("too complex to decide", r.Reason, desc)
			}
			for _, detail := range []int{1, 2, 3, 5, 8, 13, 21} {
				full := Evaluate(tc.tree, h, self, Options{Mode: Collect, Budget: budget, DetailBudget: detail})
				c.Equal(r.State, full.State, fmt.Sprint(desc, " detail ", detail))
				c.True(full.Truncated || detail > 1, fmt.Sprint(desc, " detail ", detail))
			}
			witnessed = witnessed || r.State == Met
		}
		c.Equal(want == Met, witnessed, tc.name)
	}
}

// TestSolverWitness checks the budget rule inside a solve: a greedy assignment that meets every demand stands as a
// witness, and otherwise the result is a global Failed.
func TestSolverWitness(t *testing.T) {
	c := check.New(t)
	cases := budgetCases()
	h, self := spellBook(cases[0].spell...)
	// 26 member visits find the candidates; the solve then runs out.
	r := Evaluate(cases[0].tree, h, self, Options{Budget: 27})
	c.Equal(Met, r.State)
	h, self = spellBook(cases[1].spell...)
	r = Evaluate(cases[1].tree, h, self, Options{Budget: 5})
	c.Equal(Failed, r.State)
	c.Equal("too complex to decide", r.Reason)
	h, self = spellBook(cases[6].spell...)
	c.Equal(Met, Evaluate(cases[6].tree, h, self, Options{Budget: 7}).State)
	c.Equal(Failed, Evaluate(cases[7].tree, h, self, Options{Budget: 7}).State)
}

// TestExclusiveEdges covers pool nodes that settle before solving: nil children, invalid groups and counts, and a
// demand whose collection the host lacks.
func TestExclusiveEdges(t *testing.T) {
	c := check.New(t)
	h, self := spellBook(spell("E1", "Earth"), spell("Mud Jet", "Earth", "Water"))
	earth := spellsWhere(inCollege("Earth"), nil)
	r := evaluateBoth(c, exclusive(nil, earth), h, self)
	c.Equal(Met, r.State)
	c.Equal(Skipped, r.Children[0].State)
	r = evaluateBoth(c, exclusive(&Group{OnlyWhen: Ref{Node: &Const{Value: true}}, Children: Nodes{earth}}), h, self)
	c.Equal(Met, r.State)
	r = evaluateBoth(c, exclusive(&Group{Mode: GroupAtLeast, N: 3, Children: Nodes{earth}}, earth), h, self)
	c.Equal(Failed, r.State)
	r = evaluateBoth(c, exclusive(&Count{}, earth), h, self)
	c.Equal(Failed, r.State)
	r = evaluateBoth(c, exclusive(&Count{Of: "zz"}, earth), h, self)
	c.Equal(Failed, r.State)
	c.Equal(`unknown collection "zz"`, r.Reason)
	// Outside a pool, per-group 1: Mud Jet fills one value only.
	count := &Count{Of: "spells", GroupBy: "college", Quantity: atLeast(2), Exclusive: true}
	r = evaluateBoth(c, count, h, self)
	c.Equal(Met, r.State)
	h, self = spellBook(spell("Mud Jet", "Earth", "Water"))
	c.Equal(Unmet, evaluateBoth(c, count, h, self).State)
}
