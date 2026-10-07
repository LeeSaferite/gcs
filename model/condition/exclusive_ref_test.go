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
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/check"
)

var refColleges = []string{"A", "B", "C"}

// refInstance is a small random pool problem: members with tags and colleges, and a tree of groups, consts, skipped
// scripts and counts over them.
type refInstance struct {
	members []*fakeItem
	root    *Group
	demands int
}

func randomQuantity(r *rand.Rand, maxQ int64) *criteria.Number {
	switch r.IntN(3) {
	case 0:
		return nil
	case 1:
		return atLeast(1 + r.Int64N(maxQ))
	default:
		return exactly(1 + r.Int64N(maxQ))
	}
}

func randomInstance(r *rand.Rand) *refInstance {
	inst := &refInstance{}
	for i := range 1 + r.IntN(8) {
		var tags, colleges []string
		for d := range 3 {
			if r.IntN(2) == 0 {
				tags = append(tags, fmt.Sprintf("d%d", d))
			}
		}
		for _, one := range refColleges {
			if r.IntN(3) == 0 && len(colleges) < 2 {
				colleges = append(colleges, one)
			}
		}
		inst.members = append(inst.members, newItem(fmt.Sprintf("m%d", i), map[string]Value{
			"tags": listValue(tags...), "college": listValue(colleges...),
		}))
	}
	var node func(depth int) Node
	node = func(depth int) Node {
		switch k := r.IntN(10); {
		case k < 2 && depth < 2:
			g := &Group{Mode: GroupModes[r.IntN(len(GroupModes))]}
			for range 1 + r.IntN(3) {
				g.Children = append(g.Children, node(depth+1))
			}
			if g.Mode == GroupAtLeast {
				g.N = 1 + r.IntN(len(g.Children))
			}
			return g
		case k < 3:
			return &Const{Value: r.IntN(4) != 0}
		case k < 4:
			return &Script{Script: "skipped"}
		default:
			if inst.demands >= 4 {
				return &Const{Value: true}
			}
			inst.demands++
			count := &Count{Of: "kids", Where: Ref{Node: checkText("tags", criteria.IsText,
				fmt.Sprintf("d%d", r.IntN(3)))}, Quantity: randomQuantity(r, 3)}
			if r.IntN(3) == 0 {
				count.GroupBy = "college"
				count.Quantity = randomQuantity(r, 2)
				count.PerGroup = randomQuantity(r, 2)
			}
			return count
		}
	}
	inst.root = &Group{Exclusive: true, Mode: GroupModes[r.IntN(len(GroupModes))]}
	for range 1 + r.IntN(3) {
		inst.root.Children = append(inst.root.Children, node(1))
	}
	if inst.root.Mode == GroupAtLeast {
		inst.root.N = 1 + r.IntN(len(inst.root.Children))
	}
	return inst
}

// refDemand is a count of an instance as the brute force sees it.
type refDemand struct {
	count  *Count
	cands  []int
	values map[string][]int
}

// refState decides an instance by brute force: Met when some assignment of members (each to at most one count, and
// for a grouped count to one of its values) satisfies the tree, Skipped when nothing applies, otherwise Unmet.
func refState(inst *refInstance, h *fakeHost) State {
	var demands []*refDemand
	byCount := make(map[*Count]*refDemand)
	Walk(inst.root, func(_ Path, n Node) bool {
		c, ok := n.(*Count)
		if !ok {
			return true
		}
		d := &refDemand{count: c, values: make(map[string][]int)}
		tag := c.Where.Node.(*Check).Text.Qualifier //nolint:errcheck // Built that way.
		for i, m := range inst.members {
			v, _ := h.Field(m, "tags")
			if !slices.Contains(v.List, tag) {
				continue
			}
			d.cands = append(d.cands, i)
			if c.GroupBy != "" {
				colleges, _ := h.Field(m, "college")
				for _, one := range colleges.List {
					d.values[one] = append(d.values[one], i)
				}
			}
		}
		demands = append(demands, d)
		byCount[c] = d
		return false
	})
	type choice struct {
		d     *refDemand
		value string
	}
	assigned := make([]choice, len(inst.members))
	satisfied := func(d *refDemand) bool {
		q, exact := need(d.count.Quantity)
		if d.count.GroupBy == "" {
			n := 0
			for _, a := range assigned {
				if a.d == d {
					n++
				}
			}
			return n >= q && (!exact || fxp.FromInteger(len(d.cands)) == d.count.Quantity.Qualifier)
		}
		p, _ := need(d.count.PerGroup)
		filled := 0
		for value, members := range d.values {
			if d.count.PerGroup != nil && d.count.PerGroup.Compare == criteria.EqualsNumber &&
				fxp.FromInteger(len(members)) != d.count.PerGroup.Qualifier {
				continue
			}
			n := 0
			for _, a := range assigned {
				if a.d == d && a.value == value {
					n++
				}
			}
			if n >= p {
				filled++
			}
		}
		return filled >= q
	}
	var holds func(n Node) (applicable, met bool)
	holds = func(n Node) (applicable, met bool) {
		switch t := n.(type) {
		case *Group:
			app, metCount := 0, 0
			for _, child := range t.Children {
				a, m := holds(child)
				if a {
					app++
					if m {
						metCount++
					}
				}
			}
			if app == 0 {
				return false, false
			}
			return true, metCount >= t.required(app)
		case *Const:
			return true, t.Value
		case *Script:
			return false, false
		default:
			return true, satisfied(byCount[t.(*Count)]) //nolint:errcheck // Only counts remain.
		}
	}
	if app, _ := holds(inst.root); !app {
		return Skipped
	}
	var search func(i int) bool
	search = func(i int) bool {
		if i == len(inst.members) {
			_, met := holds(inst.root)
			return met
		}
		assigned[i] = choice{}
		if search(i + 1) {
			return true
		}
		for _, d := range demands {
			if !slices.Contains(d.cands, i) {
				continue
			}
			if d.count.GroupBy == "" {
				assigned[i] = choice{d: d}
				if search(i + 1) {
					return true
				}
				continue
			}
			for value, members := range d.values {
				if slices.Contains(members, i) {
					assigned[i] = choice{d: d, value: value}
					if search(i + 1) {
						return true
					}
				}
			}
		}
		assigned[i] = choice{}
		return false
	}
	if search(0) {
		return Met
	}
	return Unmet
}

// TestSolverAgainstBruteForce compares the solver's state with a brute force over thousands of small random pools,
// in both modes.
func TestSolverAgainstBruteForce(t *testing.T) {
	c := check.New(t)
	r := rand.New(rand.NewPCG(5, 6)) //nolint:gosec // Not for security.
	met := 0
	for i := range 4000 {
		inst := randomInstance(r)
		h := newHost()
		self := newItem("self", nil).with("kids", inst.members...)
		want := refState(inst, h)
		got := evaluateBoth(c, inst.root, h, self)
		c.Equal(want, got.State, fmt.Sprintf("case %d", i))
		if want == Met {
			met++
		}
	}
	c.True(met > 600 && met < 3400, fmt.Sprint("met ", met))
}
