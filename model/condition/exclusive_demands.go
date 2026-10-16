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
	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
)

// demand is a count that consumes members from a pool: it needs q of its candidates, or, when grouped, q of its values
// each holding p members. Every member counts toward at most one demand of a pool, and toward one value only.
type demand struct {
	count        *Count
	pos          map[int]int   // pool member to its index in cands
	memberValues map[int][]int // grouped: pool member to the values it has, in its own order
	cands        []int         // pool members whose where passed, in collection order
	wheres       []Result      // the where result of each candidate, in Collect mode
	values       []groupValue  // grouped: values in order of first appearance
	q            int
	p            int
	gap          int // members by which a plain "is" demand's full candidate count misses q
	// exact is set for an "is" quantity: a plain demand's full candidate count must equal it, and a grouped one never
	// opens a value in surplus.
	exact   bool
	grouped bool
	// Where the demand sits in its pool's network: the node and sink edge of a plain demand, or of the node a
	// per-group-1 demand's values feed; and for a demand needing p >= 2 members per value, each value's node and sink
	// edge.
	node       int
	sink       int
	valueNodes []int
	valueSinks []int
}

// groupValue is one value of a grouped demand and the candidates that have it.
type groupValue struct {
	members  []int
	eligible bool // per_group "is" P needs exactly P candidates with the value
}

// owner records where a pool member went: nowhere when d is nil, else to d, and for a grouped demand to one value.
type owner struct {
	d     *demand
	value int
}

// isDemand returns true for a count that consumes pool members: not negated, with a demanding quantity, and when
// grouped a demanding per-group too.
func isDemand(c *Count) bool {
	return !c.Not && demanding(c.Quantity) && (c.GroupBy == "" || demanding(c.PerGroup))
}

// ceilInt returns v rounded up to an integer.
func ceilInt(v fxp.Int) int {
	return v.Ceil().AsInteger[int]()
}

// need returns how many a quantity asks for (nil means 1) and whether it asks for exactly that many.
func need(q *criteria.Number) (n int, exact bool) {
	if q == nil {
		return 1, false
	}
	return max(ceilInt(q.Qualifier), 1), q.Compare == criteria.EqualsNumber
}

// newDemand returns a demand for c over the given candidates, items being the pool's members.
func newDemand(e *evaluator, c *Count, cands []int, wheres []Result, items []Item) *demand {
	d := &demand{count: c, cands: cands, wheres: wheres, pos: make(map[int]int, len(cands))}
	for i, m := range cands {
		d.pos[m] = i
	}
	d.q, d.exact = need(c.Quantity)
	if c.GroupBy == "" {
		if d.exact && fxp.FromInteger(len(cands)) != c.Quantity.Qualifier {
			d.gap = max(1, abs(len(cands)-d.q))
		}
		return d
	}
	d.grouped = true
	d.p, _ = need(c.PerGroup)
	d.memberValues = make(map[int][]int)
	index := make(map[string]int)
	for _, m := range cands {
		for _, name := range groupValues(e.host.Field(items[m], c.GroupBy)) {
			v, ok := index[name]
			if !ok {
				v = len(d.values)
				index[name] = v
				d.values = append(d.values, groupValue{})
			}
			d.values[v].members = append(d.values[v].members, m)
			d.memberValues[m] = append(d.memberValues[m], v)
		}
	}
	for i := range d.values {
		d.values[i].eligible = c.PerGroup == nil || c.PerGroup.Compare != criteria.EqualsNumber ||
			fxp.FromInteger(len(d.values[i].members)) == c.PerGroup.Qualifier
	}
	return d
}

// usable reports whether member m may be used: available is nil, meaning all, or marks it.
func usable(available []bool, m int) bool {
	return len(available) == 0 || available[m]
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// tally counts what owners give d: members for a plain demand, and for a grouped one the members of each value and the
// number of values holding at least p.
func (d *demand) tally(owners []owner) (members int, perValue []int, filled int) {
	if d.grouped {
		perValue = make([]int, len(d.values))
	}
	for _, m := range d.cands {
		if o := owners[m]; o.d == d {
			members++
			if d.grouped {
				perValue[o.value]++
			}
		}
	}
	for _, n := range perValue {
		if n >= d.p {
			filled++
		}
	}
	return members, perValue, filled
}

// satisfied reports whether owners meet the demand.
func (d *demand) satisfied(owners []owner) bool {
	members, _, filled := d.tally(owners)
	if d.grouped {
		return filled >= d.q
	}
	return members >= d.q && d.gap == 0
}

// accepts reports whether value v of a grouped demand takes one more member in surplus: it is eligible, below p when
// per_group is exact, and already opened when the quantity is exact.
func (d *demand) accepts(v int, perValue []int) bool {
	return d.values[v].eligible && (d.count.PerGroup == nil || d.count.PerGroup.Compare != criteria.EqualsNumber ||
		perValue[v] < d.p) && (!d.exact || perValue[v] > 0)
}

// running is a demand's tally, kept up to date while members are placed.
type running struct {
	perValue []int
	members  int
	filled   int
}

// runningTallies returns the tally owners give each demand.
func runningTallies(demands []*demand, owners []owner) []running {
	runs := make([]running, len(demands))
	for i, d := range demands {
		runs[i].members, runs[i].perValue, runs[i].filled = d.tally(owners)
	}
	return runs
}

// place puts member m into d if m is available and d can take it: any plain demand, or the first value of m that
// accepts it. When short is set, only a demand still short of its need takes it, as the greedy witness requires. r is
// d's running tally.
func (d *demand) place(m int, owners []owner, available []bool, short bool, r *running) bool {
	if _, ok := d.pos[m]; !ok || !usable(available, m) {
		return false
	}
	if !d.grouped {
		if short && r.members >= d.q {
			return false
		}
		owners[m] = owner{d: d, value: -1}
		r.members++
		return true
	}
	if short && r.filled >= d.q {
		return false
	}
	for _, v := range d.memberValues[m] {
		takes := d.accepts(v, r.perValue)
		if short {
			takes = d.values[v].eligible && r.perValue[v] < d.p
		}
		if !takes {
			continue
		}
		owners[m] = owner{d: d, value: v}
		r.members++
		r.perValue[v]++
		if r.perValue[v] == d.p {
			r.filled++
		}
		return true
	}
	return false
}

// surplus gives each unused available candidate to the first demand, in tree order, that takes it, so Have and hits
// show every qualifying member. It runs last and never changes a state.
func surplus(demands []*demand, owners []owner, available []bool) {
	runs := runningTallies(demands, owners)
	for m := range owners {
		if owners[m].d != nil {
			continue
		}
		for i, d := range demands {
			if d.place(m, owners, available, false, &runs[i]) {
				break
			}
		}
	}
}

// greedy assigns members in pool order, each to the first demand still short that takes it. It is the witness used
// when the budget runs out inside a solve: if it satisfies every demand, the demands can be met.
func greedy(demands []*demand, members int, available []bool) []owner {
	owners := make([]owner, members)
	runs := runningTallies(demands, owners)
	for m := range owners {
		for i, d := range demands {
			if d.place(m, owners, available, true, &runs[i]) {
				break
			}
		}
	}
	return owners
}
