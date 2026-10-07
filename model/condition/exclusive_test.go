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
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/check"
)

func spell(name string, colleges ...string) *fakeItem {
	return newItem(name, map[string]Value{"college": listValue(colleges...), "tags": listValue()})
}

func spellBook(spells ...*fakeItem) (h *fakeHost, self *fakeItem) {
	h = newHost()
	h.entity["spells"] = spells
	return h, newItem("Owner", nil)
}

func atLeast(n int64) *criteria.Number {
	return numberCritPtr(criteria.AtLeastNumber, n)
}

func exactly(n int64) *criteria.Number {
	return numberCritPtr(criteria.EqualsNumber, n)
}

func spellsWhere(where Node, q *criteria.Number) *Count {
	return &Count{Of: "spells", Where: Ref{Node: where}, Quantity: q}
}

func inCollege(c string) Node {
	return checkText("college", criteria.IsText, c)
}

func exclusive(children ...Node) *Group {
	return &Group{Exclusive: true, Children: children}
}

func numbered(prefix string, n int, colleges ...string) []*fakeItem {
	list := make([]*fakeItem, n)
	for i := range list {
		list[i] = spell(fmt.Sprintf("%s %d", prefix, i), colleges...)
	}
	return list
}

// evaluateBoth evaluates in both modes, checks they agree, and returns the Collect result.
func evaluateBoth(c check.Checker, n Node, h Host, self Item) Result {
	yes := Evaluate(n, h, self, yesNo())
	full := Evaluate(n, h, self, collect())
	c.Equal(yes.State, full.State)
	return full
}

func TestDispelMagic(t *testing.T) {
	c := check.New(t)
	tree := exclusive(spellsWhere(checkText("name", criteria.IsText, "Counterspell"), nil), spellsWhere(nil, atLeast(12)))
	h, self := spellBook(append([]*fakeItem{spell("Counterspell")}, numbered("Other", 12)...)...)
	r := evaluateBoth(c, tree, h, self)
	c.Equal(Met, r.State)
	c.Equal(2, r.Specificity)
	c.Equal(fxp.FromInteger(12), r.Children[1].Have)
	h, self = spellBook(append([]*fakeItem{spell("Counterspell")}, numbered("Other", 11)...)...)
	r = evaluateBoth(c, tree, h, self)
	c.Equal(Unmet, r.State)
	c.Equal(Met, r.Children[0].State)
	c.Equal(Unmet, r.Children[1].State)
	c.Equal(fxp.FromInteger(11), r.Children[1].Have)
	c.Equal(fxp.FromInteger(12), r.Children[1].Need)
	nonExclusive := &Group{Children: tree.Children}
	c.Equal(Met, Evaluate(nonExclusive, h, self, yesNo()).State)
}

func TestForceDome(t *testing.T) {
	c := check.New(t)
	tree := exclusive(
		spellsWhere(inCollege("Air"), atLeast(2)),
		spellsWhere(inCollege("Earth"), atLeast(2)),
		spellsWhere(inCollege("Fire"), atLeast(2)),
		spellsWhere(inCollege("Water"), atLeast(2)),
	)
	spells := []*fakeItem{
		spell("Air 1", "Air"), spell("Air 2", "Air"), spell("Earth 1", "Earth"), spell("Mud Jet", "Earth", "Water"),
		spell("Water 1", "Water"), spell("Fire 1", "Fire"), spell("Fire 2", "Fire"),
	}
	h, self := spellBook(spells...)
	r := evaluateBoth(c, tree, h, self)
	c.Equal(Unmet, r.State)
	c.Equal(Met, Evaluate(&Group{Children: tree.Children}, h, self, yesNo()).State)
	h, self = spellBook(append(spells, spell("Water 2", "Water"))...)
	c.Equal(Met, evaluateBoth(c, tree, h, self).State)
}

func TestGreatShapeshift(t *testing.T) {
	c := check.New(t)
	shapeshifting := checkText("tags", criteria.IsText, "Shapeshifting")
	tree := exclusive(
		spellsWhere(checkText("name", criteria.IsText, "Alter Body"), nil),
		spellsWhere(shapeshifting, atLeast(4)),
		spellsWhere(nil, atLeast(15)),
	)
	shifts := numbered("Shift", 4)
	for _, one := range shifts {
		one.fields["tags"] = listValue("Shapeshifting")
	}
	base := append([]*fakeItem{spell("Alter Body")}, shifts...)
	h, self := spellBook(append(base, numbered("Other", 15)...)...)
	c.Equal(Met, evaluateBoth(c, tree, h, self).State)
	h, self = spellBook(append(base, numbered("Other", 14)...)...)
	r := evaluateBoth(c, tree, h, self)
	c.Equal(Unmet, r.State)
	c.Equal(fxp.FromInteger(14), r.Children[2].Have)
}

// TestAccelerateTime checks an exclusive grouped count outside a pool against the non-exclusive one: a spell in two
// colleges fills only one of them.
func TestAccelerateTime(t *testing.T) {
	c := check.New(t)
	count := &Count{
		Of:        "spells",
		GroupBy:   "college",
		PerGroup:  atLeast(2),
		Quantity:  atLeast(3),
		Exclusive: true,
	}
	spells := []*fakeItem{
		spell("A1", "A"), spell("AB", "A", "B"), spell("B1", "B"), spell("C1", "C"), spell("C2", "C"),
	}
	h, self := spellBook(spells...)
	r := evaluateBoth(c, count, h, self)
	c.Equal(Unmet, r.State)
	c.Equal(fxp.FromInteger(2), r.Have)
	plain := *count
	plain.Exclusive = false
	r = evaluateBoth(c, &plain, h, self)
	c.Equal(Met, r.State)
	c.Equal(fxp.FromInteger(3), r.Have)
	h, self = spellBook(append(spells, spell("B2", "B"))...)
	r = evaluateBoth(c, count, h, self)
	c.Equal(Met, r.State)
	c.Equal(fxp.FromInteger(3), r.Have)
	hits := 0
	for range r.Hits() {
		hits++
	}
	c.Equal(6, hits)
}

// TestFireSwarmExclusive contrasts the non-exclusive Fire Swarm shape with an exclusive one, where the named spells
// no longer count toward the college total.
func TestFireSwarmExclusive(t *testing.T) {
	c := check.New(t)
	children := Nodes{
		spellsWhere(checkText("name", criteria.IsText, "Fireball"), nil),
		spellsWhere(checkText("name", criteria.IsText, "Flame Jet"), nil),
		spellsWhere(inCollege("Fire"), atLeast(10)),
	}
	h, self := spellBook(append([]*fakeItem{spell("Fireball", "Fire"), spell("Flame Jet", "Fire")},
		numbered("Fire", 8, "Fire")...)...)
	c.Equal(Met, evaluateBoth(c, &Group{Children: children}, h, self).State)
	c.Equal(Unmet, evaluateBoth(c, exclusive(children...), h, self).State)
}

// TestGreedyCounterexample needs the flow solver: a greedy pass gives Mud Jet to Earth and leaves Water short.
func TestGreedyCounterexample(t *testing.T) {
	c := check.New(t)
	h, self := spellBook(spell("Mud Jet", "Earth", "Water"), spell("Shape Earth", "Earth"))
	tree := exclusive(spellsWhere(inCollege("Earth"), nil), spellsWhere(inCollege("Water"), nil))
	r := evaluateBoth(c, tree, h, self)
	c.Equal(Met, r.State)
	c.Equal(Item(h.entity["spells"][1]), r.Children[0].hits[0].Item)
	c.Equal(Item(h.entity["spells"][0]), r.Children[1].hits[0].Item)
}

// TestPerGroupRerouting needs the branch and bound to reroute: with P = 2 and Q = 2, value A {m1, m2, m3} and value B
// {m2, x} are both filled only when m2 goes to B.
func TestPerGroupRerouting(t *testing.T) {
	c := check.New(t)
	h, self := spellBook(spell("m1", "A"), spell("m2", "A", "B"), spell("m3", "A"), spell("x", "B"))
	grouped := &Count{Of: "spells", GroupBy: "college", PerGroup: atLeast(2), Quantity: atLeast(2)}
	r := evaluateBoth(c, exclusive(grouped), h, self)
	c.Equal(Met, r.State)
	c.Equal(fxp.FromInteger(2), r.Children[0].Have)
	alone := *grouped
	alone.Exclusive = true
	c.Equal(Met, evaluateBoth(c, &alone, h, self).State)
	// A third value can't be filled.
	tooMany := *grouped
	tooMany.Quantity = atLeast(3)
	c.Equal(Unmet, evaluateBoth(c, exclusive(&tooMany), h, self).State)
}
