// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

import "math"

// assignment is the outcome of solving one set of demands.
type assignment struct {
	owners  []owner
	deficit int  // members short, counting a grouped demand short by k values as k * p
	ok      bool // the budget held
}

// network returns the pool's flow network, building it on first use.
func (s *poolSolver) network() *poolNet {
	if s.net == nil {
		s.net = newPoolNet(len(s.items), s.demands)
	}
	return s.net
}

// solve assigns pool members to the demands, in tree order. Demands needing p >= 2 members per value branch on their
// values.
func (s *poolSolver) solve(demands []*demand) assignment {
	net := s.network()
	net.reset(s.available)
	var base []flowTarget
	var bigs []*demand
	gap := 0
	for _, d := range demands {
		gap += d.gap
		if d.grouped && d.p > 1 {
			bigs = append(bigs, d)
		} else {
			base = append(base, net.open(d.node, d.sink, d.q))
		}
	}
	if !net.maxFlow(s.e, base) {
		return assignment{}
	}
	deficit := net.deficit(base) + gap
	if len(bigs) == 0 || deficit > gap {
		for _, d := range bigs {
			deficit += d.q * d.p
		}
		return assignment{owners: net.assigned(), deficit: deficit, ok: true}
	}
	return s.searchValues(net, bigs, gap)
}

// probes returns the targets of the values of d from index from on that are eligible and have at least p candidates
// in the whole pool, for measuring how much more flow they could take.
func probes(d *demand, from int) (targets []flowTarget, first int) {
	first = -1
	for v := from; v < len(d.values); v++ {
		if d.values[v].eligible && len(d.values[v].members) >= d.p {
			if first < 0 {
				first = v
			}
			targets = append(targets, flowTarget{node: d.valueNodes[v], sinkEdge: d.valueSinks[v]})
		}
	}
	return targets, first
}

// searchValues branches on the values of each grouped demand needing p >= 2 members per value, in tree order and each
// demand's values in order of first appearance, including a value before excluding it. The other demands are already
// saturated. Including a value opens its sink and augments from the current flow, which never takes flow off a
// saturated sink, so a value that can't be filled shows the include is infeasible. Each node prunes with the relaxation
// bound: the values still undecided can add at most min(their number, F / p), where F is the flow they could still
// take. The assignment returned is the first that meets every demand, or else the one found with the least deficit.
func (s *poolSolver) searchValues(net *poolNet, bigs []*demand, gap int) assignment {
	included := make([]int, len(bigs))
	pos := make([]int, len(bigs))
	best := assignment{deficit: math.MaxInt, ok: true}
	consider := func(short int) {
		deficit := short + gap
		for i, d := range bigs {
			deficit += (d.q - included[i]) * d.p
		}
		if deficit < best.deficit {
			best = assignment{owners: net.assigned(), deficit: deficit, ok: true}
		}
	}
	var search func() bool
	search = func() bool {
		gi := -1
		for i, d := range bigs {
			if included[i] < d.q {
				gi = i
				break
			}
		}
		if gi < 0 {
			consider(0)
			return true
		}
		d := bigs[gi]
		targets, v := probes(d, pos[gi])
		flow, ok := net.probe(s.e, targets, d.p, d.p*(d.q-included[gi]))
		if !ok {
			best.ok = false
			return true
		}
		consider(0)
		if included[gi]+min(len(targets), flow/d.p) < d.q {
			return false
		}
		saved := pos[gi]
		pos[gi] = v + 1
		net.save()
		target := net.open(d.valueNodes[v], d.valueSinks[v], d.p)
		if !net.maxFlow(s.e, []flowTarget{target}) {
			best.ok = false
			return true
		}
		included[gi]++
		if short := net.cap[target.sinkEdge]; short != 0 {
			consider(short)
		} else if search() {
			return true
		}
		included[gi]--
		net.restore()
		if search() {
			return true
		}
		pos[gi] = saved
		return false
	}
	search()
	return best
}

// maximize fills as many values of a grouped demand as it can, each member counting toward one value only: a max flow
// when each value needs one member, otherwise a branch and bound over the values, including a value before excluding
// it, with the incumbent as the bound.
func (s *poolSolver) maximize(d *demand) assignment {
	net := s.network()
	net.reset(nil)
	if d.p == 1 {
		target := net.open(d.node, d.sink, len(d.values))
		if !net.maxFlow(s.e, []flowTarget{target}) {
			return assignment{}
		}
		return assignment{owners: net.assigned(), ok: true}
	}
	best := assignment{ok: true}
	bestCount := -1
	reachable, _ := probes(d, 0)
	var search func(from, count int)
	search = func(from, count int) {
		if !best.ok || bestCount == len(reachable) {
			return
		}
		if count > bestCount {
			best, bestCount = assignment{owners: net.assigned(), ok: true}, count
		}
		targets, v := probes(d, from)
		flow, ok := net.probe(s.e, targets, d.p, d.p*(bestCount-count+1))
		if !ok {
			best.ok = false
			return
		}
		if len(targets) == 0 || count+min(len(targets), flow/d.p) <= bestCount {
			return
		}
		net.save()
		target := net.open(d.valueNodes[v], d.valueSinks[v], d.p)
		if !net.maxFlow(s.e, []flowTarget{target}) {
			best.ok = false
			return
		}
		if net.cap[target.sinkEdge] == 0 {
			search(v+1, count+1)
		}
		net.restore()
		search(v+1, count)
	}
	search(0, 0)
	return best
}
