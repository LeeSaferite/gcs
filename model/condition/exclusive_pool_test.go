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

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/check"
)

// TestPoolBoundaries checks that a nested exclusive group shares the pool, while one below Not starts its own, and
// that guards inside a pool are evaluated outside it.
func TestPoolBoundaries(t *testing.T) {
	c := check.New(t)
	h, self := spellBook(spell("Earth 1", "Earth"))
	earth := func() *Count { return spellsWhere(inCollege("Earth"), nil) }
	c.Equal(Unmet, evaluateBoth(c, exclusive(earth(), exclusive(earth())), h, self).State)
	c.Equal(Unmet, evaluateBoth(c, exclusive(earth(), &Group{Children: Nodes{earth()}}), h, self).State)
	notGroup := &Group{Not: true, Exclusive: true, Children: Nodes{spellsWhere(inCollege("Earth"), atLeast(2))}}
	c.Equal(Met, evaluateBoth(c, exclusive(earth(), notGroup), h, self).State)
	unmetGuard := &Group{OnlyWhen: Ref{Node: &Const{}}, Children: Nodes{earth()}}
	r := evaluateBoth(c, exclusive(earth(), unmetGuard), h, self)
	c.Equal(Met, r.State)
	c.Equal(Skipped, r.Children[1].State)
	failedGuard := &Group{OnlyWhen: Ref{Node: &Unknown{Reason: "x"}}, Children: Nodes{earth()}}
	r = evaluateBoth(c, exclusive(earth(), failedGuard), h, self)
	c.Equal(Failed, r.State)
	c.False(r.MemberLocal())
	localGuard := &Group{OnlyWhen: Ref{Node: localFailure()}, Children: Nodes{earth()}}
	r = evaluateBoth(c, exclusive(earth(), localGuard), h, self)
	c.Equal(Failed, r.State)
	c.True(r.MemberLocal())
	// Not on the exclusive group itself applies to the outcome.
	negated := exclusive(earth(), earth())
	negated.Not = true
	r = evaluateBoth(c, negated, h, self)
	c.Equal(Met, r.State)
	c.Equal(0, r.Specificity)
}

// TestExactDemands checks "is" demands: the full candidate count must equal the quantity.
func TestExactDemands(t *testing.T) {
	c := check.New(t)
	tree := exclusive(spellsWhere(inCollege("Earth"), exactly(2)), spellsWhere(inCollege("Water"), nil))
	h, self := spellBook(spell("E1", "Earth"), spell("E2", "Earth"), spell("Mud Jet", "Earth", "Water"))
	c.Equal(Unmet, evaluateBoth(c, tree, h, self).State)
	h, self = spellBook(spell("E1", "Earth"), spell("E2", "Earth"), spell("W1", "Water"))
	c.Equal(Met, evaluateBoth(c, tree, h, self).State)
	// A grouped "is" quantity never opens a value in surplus, and a per-group "is" only takes values with exactly P.
	grouped := &Count{Of: "spells", GroupBy: "college", Quantity: exactly(1), PerGroup: exactly(2)}
	h, self = spellBook(spell("E1", "Earth"), spell("E2", "Earth"), spell("W1", "Water"), spell("W2", "Water"),
		spell("F1", "Fire"))
	r := evaluateBoth(c, exclusive(grouped), h, self)
	c.Equal(Met, r.State)
	c.Equal(fxp.FromInteger(1), r.Children[0].Have)
	alone := *grouped
	alone.Exclusive = true
	r = evaluateBoth(c, &alone, h, self)
	c.Equal(Unmet, r.State)
	c.Equal(fxp.FromInteger(2), r.Have)
}

// TestChoicePoints checks any and at_least groups in a pool, including Skipped children, and that Collect reports the
// combination with the highest specificity, unchosen children evaluated against the unused members.
func TestChoicePoints(t *testing.T) {
	c := check.New(t)
	earth := func() *Count { return spellsWhere(inCollege("Earth"), nil) }
	water := func() *Count { return spellsWhere(inCollege("Water"), nil) }
	skipped := &Script{Script: "skipped"}
	h, self := spellBook(spell("Mud Jet", "Earth", "Water"))
	atLeastTwo := &Group{Mode: GroupAtLeast, N: 2, Children: Nodes{skipped, earth(), water()}}
	c.Equal(Unmet, evaluateBoth(c, exclusive(atLeastTwo), h, self).State)
	anyOf := &Group{Mode: GroupAny, Children: Nodes{skipped, earth(), water()}}
	r := evaluateBoth(c, exclusive(anyOf), h, self)
	c.Equal(Met, r.State)
	c.Equal(Met, r.Children[0].Children[1].State)
	c.Equal(Unmet, r.Children[0].Children[2].State)
	c.Equal(Skipped, r.Children[0].Children[0].State)
	h, self = spellBook(spell("Mud Jet", "Earth", "Water"), spell("Earth 1", "Earth"))
	c.Equal(Met, evaluateBoth(c, exclusive(atLeastTwo), h, self).State)
	best := exclusive(&Group{Mode: GroupAny, Children: Nodes{earth(), &Group{Children: Nodes{earth(), water()}}}})
	r = evaluateBoth(c, best, h, self)
	c.Equal(2, r.Specificity)
	c.Equal(Met, r.Children[0].Children[1].State)
	allSkipped := exclusive(&Group{Mode: GroupAny, Children: Nodes{skipped}}, earth())
	c.Equal(Met, evaluateBoth(c, allSkipped, h, self).State)
	c.Equal(Skipped, evaluateBoth(c, exclusive(skipped), h, self).State)
	c.Equal(Skipped, evaluateBoth(c, exclusive(), h, self).State)
}

// TestSurplusAndDeterminism checks that the surplus shows every qualifying member and that repeated evaluations give
// the same hits and Have.
func TestSurplusAndDeterminism(t *testing.T) {
	c := check.New(t)
	h, self := spellBook(spell("E1", "Earth"), spell("Mud Jet", "Earth", "Water"), spell("E2", "Earth"),
		spell("W1", "Water"), spell("Odd", "Air"))
	tree := exclusive(spellsWhere(inCollege("Earth"), nil), spellsWhere(inCollege("Water"), nil))
	first := evaluateBoth(c, tree, h, self)
	// The solution gives E1 to Earth and Mud Jet to Water; the surplus then gives E2 to Earth and W1 to Water.
	c.Equal(fxp.FromInteger(2), first.Children[0].Have)
	c.Equal(fxp.FromInteger(2), first.Children[1].Have)
	var hits []Hit
	for hit := range first.Hits() {
		hits = append(hits, hit)
	}
	c.Equal(8, len(hits))
	for range 5 {
		again := Evaluate(tree, h, self, collect())
		var other []Hit
		for hit := range again.Hits() {
			other = append(other, hit)
		}
		c.Equal(hits, other)
	}
}

// TestInfeasibleRanking checks which combination an infeasible pool reports: fewest required non-demand nodes not
// Met, then least shortfall, then the first.
func TestInfeasibleRanking(t *testing.T) {
	c := check.New(t)
	h, self := spellBook(spell("F1", "Fire"), spell("F2", "Fire"), spell("E1", "Earth"))
	fire := spellsWhere(inCollege("Fire"), atLeast(3))
	earth := spellsWhere(inCollege("Earth"), atLeast(3))
	r := evaluateBoth(c, exclusive(&Group{Mode: GroupAny, Children: Nodes{fire, earth}}), h, self)
	c.Equal(Unmet, r.State)
	c.Equal(fxp.FromInteger(2), r.Children[0].Children[0].Have)
	c.Equal(fxp.FromInteger(3), r.Children[0].Children[0].Need)
	fire5 := spellsWhere(inCollege("Fire"), atLeast(5))
	r = evaluateBoth(c, exclusive(&Group{Mode: GroupAny, Children: Nodes{fire5, earth}}), h, self)
	c.Equal(Unmet, r.State)
	c.Equal(fxp.FromInteger(1), r.Children[0].Children[1].Have)
	withConst := &Group{Mode: GroupAny, Children: Nodes{
		&Group{Children: Nodes{&Const{}, spellsWhere(inCollege("Fire"), nil)}},
		spellsWhere(inCollege("Fire"), atLeast(9)),
	}}
	r = evaluateBoth(c, exclusive(withConst), h, self)
	c.Equal(Unmet, r.State)
	c.Equal(fxp.FromInteger(2), r.Children[0].Children[1].Have)
	failing := &Group{Mode: GroupAny, Children: Nodes{fire5, &Script{Script: "failed"}}}
	r = evaluateBoth(c, exclusive(failing), h, self)
	c.Equal(Failed, r.State)
	c.False(r.MemberLocal())
}
