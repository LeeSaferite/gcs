// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

import "slices"

// combination is a recorded combination of choices and its assignment.
type combination struct {
	chosen   [][]int
	solved   assignment
	notMet   int // required non-demand nodes not Met
	spec     int
	feasible bool
}

// better reports whether an infeasible combination ranks above c: fewer required non-demand nodes not Met, then less
// shortfall. Ties keep the first enumerated.
func (c *combination) better(notMet, deficit int) bool {
	return notMet < c.notMet || (notMet == c.notMet && deficit < c.solved.deficit)
}

// solveRoot solves the pool rooted at p. Every combination of choices is tried in order: choice points in pre-order,
// options in child order, subsets in lexicographic order. YesNo stops at the first feasible one; Collect keeps the
// feasible one with the highest specificity, the first on ties. With none feasible, the best infeasible one is
// reported.
func (s *poolSolver) solveRoot(p *pnode) Result {
	if p.kind != pnodeGroup || !p.isApplicable() {
		return s.result(p, nil)
	}
	choices := choicePoints(p, nil)
	var best *combination
	var lost bool
	record := func(c *combination) {
		c.chosen = make([][]int, len(choices))
		for i, one := range choices {
			c.chosen[i] = slices.Clone(one.chosen)
		}
		best = c
	}
	s.enumerate([]*pnode{p}, func() bool {
		if best != nil && best.feasible && !s.e.collect {
			return false
		}
		demands, others := s.required(p)
		notMet := 0
		for _, o := range others {
			if o.result.State != Met {
				notMet++
			}
		}
		if !s.e.spend() {
			lost = true
			return s.witness(demands, notMet, record)
		}
		if best != nil && notMet > 0 && (best.feasible || notMet > best.notMet) {
			return true
		}
		solved := s.solve(demands)
		if !solved.ok {
			lost = true
			return s.witness(demands, notMet, record)
		}
		if notMet == 0 && solved.deficit == 0 {
			spec := 0
			if s.collect {
				spec = s.specificity(p)
			}
			if best == nil || !best.feasible || spec > best.spec {
				record(&combination{solved: solved, spec: spec, feasible: true})
			}
			return true
		}
		if best == nil || (!best.feasible && best.better(notMet, solved.deficit)) {
			record(&combination{solved: solved, notMet: notMet})
		}
		return true
	})
	if lost && (best == nil || !best.feasible) {
		return failGlobal(reasonTooComplex())
	}
	for i, one := range choices {
		one.chosen = best.chosen[i]
	}
	return s.finish(p, best.solved.owners)
}

// witness handles a budget that ran out inside a solve, which can only happen once no feasible combination has been
// found, since Collect stops at the first feasible one when its detail budget runs out. A greedy assignment that meets
// every demand of the current combination, with every other required node Met, is a valid witness. It returns false
// to stop the enumeration.
func (s *poolSolver) witness(demands []*demand, notMet int, record func(*combination)) bool {
	owners := greedy(demands, len(s.items), s.available)
	for _, d := range demands {
		if !d.satisfied(owners) {
			return false
		}
	}
	if notMet == 0 {
		record(&combination{solved: assignment{owners: owners, ok: true}, feasible: true})
	}
	return false
}

// finish reports the chosen combination: unchosen children of choice points are evaluated against the members the
// assignment left unused, and then the surplus goes out.
func (s *poolSolver) finish(p *pnode, owners []owner) Result {
	owners = slices.Clone(owners)
	demands, _ := s.required(p)
	met := make(map[*demand]bool, len(demands))
	for _, d := range demands {
		met[d] = d.satisfied(owners)
	}
	residual := make([]bool, len(s.items))
	for m := range residual {
		residual[m] = owners[m].d == nil && usable(s.available, m)
	}
	surplus(demands, owners, s.available)
	return s.groupResult(p, &report{owners: owners, met: met, residual: residual})
}

// choicePoints appends the choice points of the pool rooted at p, in pre-order.
func choicePoints(p *pnode, list []*pnode) []*pnode {
	if p.kind != pnodeGroup {
		return list
	}
	if p.k != 0 {
		list = append(list, p)
	}
	for _, c := range p.children {
		list = choicePoints(c, list)
	}
	return list
}

// enumerate calls visit for every combination of the pending nodes' choices, in pre-order, until visit returns false.
func (s *poolSolver) enumerate(pending []*pnode, visit func() bool) bool {
	if len(pending) == 0 {
		return visit()
	}
	p, rest := pending[0], pending[1:]
	if p.kind != pnodeGroup {
		return s.enumerate(rest, visit)
	}
	expand := func(chosen []int) bool {
		next := make([]*pnode, 0, len(chosen)+len(rest))
		for _, i := range chosen {
			next = append(next, p.children[i])
		}
		return s.enumerate(append(next, rest...), visit)
	}
	if p.k == 0 {
		return expand(p.applicable)
	}
	return eachSubset(len(p.applicable), p.k, func(subset []int) bool {
		p.chosen = make([]int, len(subset))
		for i, j := range subset {
			p.chosen[i] = p.applicable[j]
		}
		return expand(p.chosen)
	})
}

// eachSubset calls fn with every k-subset of 0..n-1, in lexicographic order, until fn returns false.
func eachSubset(n, k int, fn func([]int) bool) bool {
	subset := make([]int, k)
	for i := range subset {
		subset[i] = i
	}
	for {
		if !fn(subset) {
			return false
		}
		i := k - 1
		for i >= 0 && subset[i] == n-k+i {
			i--
		}
		if i < 0 {
			return true
		}
		subset[i]++
		for j := i + 1; j < k; j++ {
			subset[j] = subset[j-1] + 1
		}
	}
}

// required returns the demands and the other nodes the current combination requires, in pre-order.
func (s *poolSolver) required(p *pnode) (demands []*demand, others []*pnode) {
	var walk func(p *pnode)
	walk = func(p *pnode) {
		switch p.kind {
		case pnodeDemand:
			demands = append(demands, p.demand)
		case pnodeOther:
			others = append(others, p)
		default:
			for _, i := range p.chosen {
				walk(p.children[i])
			}
		}
	}
	walk(p)
	return demands, others
}

// specificity returns the specificity of the current combination when feasible: 1 per demand, a non-demand node's
// own, and for a group the sum over its required children.
func (s *poolSolver) specificity(p *pnode) int {
	switch p.kind {
	case pnodeDemand:
		return 1
	case pnodeOther:
		return p.result.Specificity
	default:
		total := 0
		for _, i := range p.chosen {
			total += s.specificity(p.children[i])
		}
		return total
	}
}
