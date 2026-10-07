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

func spellsHost() (h *fakeHost, self *fakeItem, spells []*fakeItem) {
	h = newHost()
	spells = []*fakeItem{
		newItem("Fireball", map[string]Value{"college": textValue("Fire")}),
		newItem("Flame Jet", map[string]Value{"college": listValue("Fire", "Fire")}),
		newItem("Haste", map[string]Value{"college": listValue("Movement", "Air")}),
		newItem("Shape Air", map[string]Value{"college": textValue("Air")}),
		newItem("Odd", map[string]Value{"college": textValue("")}),
		newItem("Absent", map[string]Value{"college": absentValue()}),
		newItem("Missing", nil),
		newItem("Lower", map[string]Value{"college": textValue("fire")}),
		newItem("Level", map[string]Value{"college": numberValue(3)}),
		newItem("Flag", map[string]Value{"college": boolValue(true)}),
	}
	self = newItem("self", nil)
	h.entity["spells"] = append(append(make([]*fakeItem, 0, len(spells)+1), spells...), self)
	return h, self, spells
}

func spellCount(where Node) *Count {
	return &Count{Of: "spells", Where: Ref{Node: where}}
}

// TestCount checks quantities, grouping by exact values (empty, missing and Absent values join no group; a List
// value puts a member under each distinct element), PerGroup equality, and the self exclusion of entity collections.
func TestCount(t *testing.T) {
	h, self, _ := spellsHost()
	fire := checkText("college", criteria.IsText, "Fire")
	grouped := func(q, per *criteria.Number) *Count {
		return &Count{Of: "spells", GroupBy: "college", Quantity: q, PerGroup: per}
	}
	for _, tc := range []struct {
		name  string
		node  *Count
		state State
		have  int64
		need  int64
	}{
		{"all members, self excluded", spellCount(nil), Met, 10, 1},
		{"where", spellCount(fire), Met, 3, 1},
		{"none", spellCount(&Const{}), Unmet, 0, 1},
		{"quantity", &Count{Of: "spells", Where: Ref{Node: fire}, Quantity: numberCritPtr(criteria.AtLeastNumber, 4)}, Unmet, 3, 4},
		{"quantity any", &Count{Of: "spells", Where: Ref{Node: &Const{}}, Quantity: &criteria.Number{}}, Met, 0, 0},
		{"not", &Count{Of: "spells", Where: Ref{Node: fire}, Not: true}, Unmet, 3, 1},
		{"grouped", grouped(nil, nil), Met, 6, 1},
		{"grouped quantity", grouped(numberCritPtr(criteria.EqualsNumber, 6), nil), Met, 6, 6},
		{"per group is 2", grouped(nil, numberCritPtr(criteria.EqualsNumber, 2)), Met, 2, 1},
		{"per group at least 3", grouped(nil, numberCritPtr(criteria.AtLeastNumber, 3)), Unmet, 0, 1},
		{"unknown collection", &Count{Of: "zz"}, Failed, 0, 0},
		{"invalid", &Count{}, Failed, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			for _, opt := range []Options{yesNo(), collect()} {
				r := Evaluate(tc.node, h, self, opt)
				c.Equal(tc.state, r.State, r.Reason)
				if r.State != Failed {
					c.Equal(fxp.FromInteger(tc.have), r.Have)
					c.Equal(ValueNumber, r.HaveKind)
					c.Equal(fxp.FromInteger(tc.need), r.Need)
				}
			}
		})
	}
}

// TestCountHits checks that a Met count cites each counted member and the hits of its where, and that a grouped
// count cites only the members of the values it counted.
func TestCountHits(t *testing.T) {
	c := check.New(t)
	h, self, spells := spellsHost()
	fire := checkText("college", criteria.IsText, "Fire")
	node := spellCount(fire)
	r := Evaluate(node, h, self, collect())
	c.Equal(1, r.Specificity)
	c.Equal(3, len(r.Children))
	var counted, checked []Item
	for hit := range r.Hits() {
		if hit.Node == Node(node) {
			counted = append(counted, hit.Item)
		} else {
			checked = append(checked, hit.Item)
		}
	}
	c.Equal([]Item{spells[0], spells[1], spells[7]}, counted)
	c.Equal(counted, checked)

	per := &Count{Of: "spells", GroupBy: "college", PerGroup: numberCritPtr(criteria.EqualsNumber, 2)}
	r = Evaluate(per, h, self, collect())
	counted = nil
	for hit := range r.Hits() {
		counted = append(counted, hit.Item)
	}
	c.Equal([]Item{spells[0], spells[1], spells[2], spells[3]}, counted)
	c.Equal(0, len(r.Children))

	r = Evaluate(&Count{Members: node.Members, Not: true, Quantity: numberCritPtr(criteria.AtLeastNumber, 9)}, h, self, collect())
	c.Equal(Met, r.State)
	c.Equal(0, r.Specificity)
	for range r.Hits() {
		c.True(false, "a negated count gives no hits")
	}
}

// TestGroupHitsAndSpecificity checks the specificity of all, any and at_least groups and Not, and that a group
// exposes the hits of its Met children only.
func TestGroupHitsAndSpecificity(t *testing.T) {
	c := check.New(t)
	h := newHost()
	subject := newItem("Fireball", map[string]Value{"college": textValue("Fire"), "level": numberValue(3)})
	name := checkText("name", criteria.IsText, "Fireball")
	college := checkText("college", criteria.IsText, "Fire")
	level := &Check{Field: "level", Number: numberCrit(criteria.AtLeastNumber, 2)}
	miss := checkText("name", criteria.IsText, "Water")
	pair := &Group{Children: Nodes{college, level}}
	for _, tc := range []struct {
		name string
		node Node
		spec int
		hits int
	}{
		{"all", &Group{Children: Nodes{name, college, &Const{Value: true}, &Check{Field: "name"}}}, 2, 2},
		{"any", &Group{Mode: GroupAny, Children: Nodes{name, pair, miss}}, 2, 3},
		{"at_least", &Group{Mode: GroupAtLeast, N: 2, Children: Nodes{name, pair, &Const{Value: true}, miss}}, 3, 3},
		{"at_least skipped", &Group{Mode: GroupAtLeast, N: 3, Children: Nodes{name, &Script{Script: "skipped"}, nil}}, 1, 1},
		{"not", &Group{Not: true, Children: Nodes{miss}}, 0, 0},
		{"script", &Script{Script: "met"}, 0, 0},
		{"const", &Const{Value: true}, 0, 0},
	} {
		r := EvaluateFor(tc.node, h, nil, subject, collect())
		c.Equal(Met, r.State, tc.name)
		c.Equal(tc.spec, r.Specificity, tc.name)
		hits := 0
		for range r.Hits() {
			hits++
		}
		c.Equal(tc.hits, hits, tc.name)
	}
	// Stopping early in the iterator.
	r := EvaluateFor(&Group{Children: Nodes{name, pair}}, h, nil, subject, collect())
	n := 0
	for range r.Hits() {
		n++
		break
	}
	c.Equal(1, n)
	for range r.Hits() {
		n++
		if n == 3 {
			break
		}
	}
	c.Equal(3, n)
}

// TestGuardInCollect checks that an unmet guard reports every child Skipped without evaluating it.
func TestGuardInCollect(t *testing.T) {
	c := check.New(t)
	h := newHost()
	g := &Group{OnlyWhen: Ref{Node: &Const{}}, Children: Nodes{&Script{Script: "met"}, nil}}
	r := Evaluate(g, h, nil, collect())
	c.Equal(Skipped, r.State)
	c.Equal(2, len(r.Children))
	for _, child := range r.Children {
		c.Equal(Skipped, child.State)
	}
	c.Equal(int64(0), h.runs.Load())
}

// TestScriptRunsOncePerCall checks that a script runs at most once per (node, self, subject) in one call, across both
// passes, and that a script in another language never runs.
func TestScriptRunsOncePerCall(t *testing.T) {
	c := check.New(t)
	h := newHost()
	a, b := newItem("a", nil), newItem("b", nil)
	self := newItem("self", nil).with("kids", a, b)
	script := &Script{Script: "met"}
	tree := &Group{Children: Nodes{
		script,
		&Count{Of: "kids", Where: Ref{Node: script}, Quantity: numberCritPtr(criteria.EqualsNumber, 2)},
	}}
	c.Equal(Met, Evaluate(tree, h, self, collect()).State)
	c.Equal(int64(3), h.runs.Load())
	c.Equal(Met, Evaluate(tree, h, self, yesNo()).State)
	c.Equal(int64(6), h.runs.Load())
	r := Evaluate(&Script{Script: "met", Lang: "cel"}, h, self, collect())
	c.Equal(Failed, r.State)
	c.Equal(int64(6), h.runs.Load())
	r = Evaluate(&Script{Script: "failed"}, h, self, yesNo())
	c.Equal("script failed", r.Reason)
	c.False(r.MemberLocal())
}
