// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

import "github.com/richardwilkes/gcs/v5/model/criteria"

// poolSolver solves one pool: the counts beneath an exclusive group, which share its members so that each member
// counts toward one of them at most. Members are keyed by identity across all the pool's collections.
type poolSolver struct {
	e         *evaluator
	index     map[Item]int
	items     []Item
	available []bool // members a residual solve may use; nil means all
	demands   []*demand
	net       *poolNet // shared with residual solves
	collect   bool
}

// pnodeKind classifies a node of a pool.
type pnodeKind byte

const (
	// pnodeGroup is an all, any or at_least group without Not; any and at_least are choice points.
	pnodeGroup pnodeKind = iota
	// pnodeDemand is a count that consumes members.
	pnodeDemand
	// pnodeOther is evaluated normally and consumes nothing: a boundary, a non-demand node, or a group its guard or
	// validity settled.
	pnodeOther
)

// pnode is a node of a pool, prepared for solving.
type pnode struct {
	node       Node
	group      *Group
	demand     *demand
	children   []*pnode
	applicable []int // indexes of the applicable children
	chosen     []int // indexes of the children the current combination requires
	result     Result
	k          int // subset size of a choice point; 0 when every applicable child is required
	kind       pnodeKind
}

func (p *pnode) isApplicable() bool {
	switch p.kind {
	case pnodeGroup:
		return len(p.applicable) != 0
	case pnodeDemand:
		return true
	default:
		return p.result.State != Skipped
	}
}

func newPoolSolver(e *evaluator) *poolSolver {
	return &poolSolver{e: e, index: make(map[Item]int), collect: e.collect}
}

// member returns the pool index of item, adding it if new.
func (s *poolSolver) member(item Item) int {
	m, ok := s.index[item]
	if !ok {
		m = len(s.items)
		s.index[item] = m
		s.items = append(s.items, item)
	}
	return m
}

// exclusiveGroup evaluates a group whose Exclusive flag starts a pool, after its guard has let it apply. A Not on the
// group applies to the outcome.
func (e *evaluator) exclusiveGroup(g *Group, f frame) Result {
	s := newPoolSolver(e)
	r := s.solveRoot(s.buildGroup(g, f))
	if g.Not && (r.State == Met || r.State == Unmet) {
		r.State = metState(r.State == Met, true)
		r.Specificity = 0
	}
	return r
}

// exclusiveGroupedCount groups the matched members of an exclusive grouped count outside a pool, each member counting
// toward one value only, choosing the assignment that fills the most values. Surplus members then go to the first of
// their values that accepts them. It returns the number of values filled and which members they hold. When the budget
// runs out, a greedy assignment stands in if it already meets a nil or at_least quantity.
func (e *evaluator) exclusiveGroupedCount(c *Count, members []Item) (have int, used []bool, fail *Result) {
	s := newPoolSolver(e)
	cands := make([]int, len(members))
	for i, item := range members {
		cands[i] = s.member(item)
	}
	d := newDemand(e, c, cands, nil, s.items)
	s.demands = append(s.demands, d)
	out := s.maximize(d)
	owners := out.owners
	if !out.ok {
		owners = greedy([]*demand{d}, len(s.items), nil)
		if (c.Quantity != nil && c.Quantity.Compare != criteria.AtLeastNumber) || !d.satisfied(owners) {
			r := failGlobal(reasonTooComplex())
			return 0, nil, &r
		}
	}
	surplus([]*demand{d}, owners, nil)
	_, _, have = d.tally(owners)
	used = make([]bool, len(members))
	for i, m := range cands {
		used[i] = owners[m].d == d
	}
	return have, used, nil
}

// build prepares a node of the pool. The solver descends through groups without Not; their guards are evaluated
// outside the pool first. Everything else is evaluated normally, except a demanding count, whose candidates are found
// now.
func (s *poolSolver) build(n Node, f frame) *pnode {
	switch t := n.(type) {
	case nil:
		return &pnode{kind: pnodeOther, result: Result{State: Skipped}}
	case *Group:
		if t.Not {
			break
		}
		if r, bad := s.e.invalid(t, f); bad {
			r.Node = t
			return &pnode{kind: pnodeOther, node: t, result: r}
		}
		if !t.OnlyWhen.IsZero() {
			guard := s.e.evalYesNo(t.OnlyWhen, f.next())
			switch guard.State {
			case Unmet:
				r := t.skipped(s.collect)
				r.Node = t
				return &pnode{kind: pnodeOther, node: t, result: r}
			case Failed:
				r := Result{Node: t, State: Failed, Reason: guard.Reason, local: guard.local}
				return &pnode{kind: pnodeOther, node: t, result: r}
			default:
			}
		}
		return s.buildGroup(t, f)
	case *Count:
		if isDemand(t) {
			return s.buildDemand(t, f)
		}
	default:
	}
	return &pnode{kind: pnodeOther, node: n, result: s.e.eval(n, f)}
}

func (s *poolSolver) buildGroup(g *Group, f frame) *pnode {
	p := &pnode{kind: pnodeGroup, node: g, group: g}
	for i, child := range g.Children {
		c := s.build(child, f.next())
		p.children = append(p.children, c)
		if c.isApplicable() {
			p.applicable = append(p.applicable, i)
		}
	}
	switch g.Mode {
	case GroupAny:
		p.k = 1
	case GroupAtLeast:
		p.k = min(g.N, len(p.applicable))
	default:
	}
	if p.k == len(p.applicable) {
		p.k = 0
	}
	p.chosen = p.applicable
	return p
}

// buildDemand finds the candidates of a demanding count. A count whose collection the host lacks, or whose where ends
// in a global Failed, is a required node that is Failed.
func (s *poolSolver) buildDemand(c *Count, f frame) *pnode {
	if r, bad := s.e.invalid(c, f); bad {
		r.Node = c
		return &pnode{kind: pnodeOther, node: c, result: r}
	}
	var cands []int
	var wheres []Result
	if fail := s.e.eachMember(&c.Members, f, func(item Item, where *Result) bool {
		cands = append(cands, s.member(item))
		if s.collect {
			wheres = append(wheres, whereResult(where))
		}
		return true
	}); fail != nil {
		r := *fail
		r.Node = c
		return &pnode{kind: pnodeOther, node: c, result: r}
	}
	d := newDemand(s.e, c, cands, wheres, s.items)
	s.demands = append(s.demands, d)
	return &pnode{kind: pnodeDemand, node: c, demand: d}
}
