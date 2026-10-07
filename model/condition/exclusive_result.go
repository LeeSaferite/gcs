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
	"github.com/richardwilkes/gcs/v5/model/fxp"
)

// report is what the results of a reported combination are built from: where each member went (surplus included),
// which demands the assignment met, and the members it left unused for the unchosen children of choice points.
type report struct {
	owners   []owner
	met      map[*demand]bool
	residual []bool
}

// result returns the result of a pool node for the reported combination.
func (s *poolSolver) result(p *pnode, rep *report) Result {
	switch p.kind {
	case pnodeOther:
		return p.result
	case pnodeDemand:
		return s.demandResult(p.demand, rep.owners, rep.met[p.demand])
	default:
		return s.groupResult(p, rep)
	}
}

// groupResult combines the results of a pool group by the 6.5 rules over the combination: Met when every required
// child is Met, otherwise Failed when any applicable child is Failed, otherwise Unmet. Unchosen children of a choice
// point are evaluated against residual, the members the assignment left unused.
func (s *poolSolver) groupResult(p *pnode, rep *report) Result {
	r := Result{Node: p.node}
	if len(p.applicable) == 0 {
		r.State = Skipped
		if s.collect {
			for _, c := range p.children {
				r.Children = append(r.Children, s.result(c, rep))
			}
		}
		return r
	}
	results := make([]Result, len(p.children))
	required := make([]bool, len(p.children))
	allMet := true
	for _, i := range p.chosen {
		required[i] = true
		results[i] = s.result(p.children[i], rep)
		allMet = allMet && results[i].State == Met
	}
	var t groupTally
	for i, c := range p.children {
		if !required[i] && (s.collect || !allMet) {
			if c.isApplicable() {
				results[i] = s.residualResult(c, rep.residual)
			} else {
				results[i] = s.result(c, rep)
			}
		}
		if c.isApplicable() && (required[i] || s.collect || !allMet) {
			t.add(results[i].State, results[i].local, results[i].Reason)
		}
	}
	switch {
	case allMet:
		r.State = Met
		if s.collect {
			for _, i := range p.chosen {
				r.Specificity += results[i].Specificity
			}
		}
	case t.failed > 0:
		r.State = Failed
		r.Reason = t.reason
		r.local = !t.global
	default:
		r.State = Unmet
	}
	if s.collect {
		r.Children = results
	}
	return r
}

// residualResult evaluates an unchosen child of a choice point as a pool of its own over the residual members.
func (s *poolSolver) residualResult(c *pnode, residual []bool) Result {
	sub := &poolSolver{
		e: s.e, index: s.index, items: s.items, available: residual, demands: s.demands, net: s.network(),
		collect: s.collect,
	}
	if c.kind == pnodeDemand {
		owners := sub.solve([]*demand{c.demand}).owners
		if owners == nil {
			return failGlobal(reasonTooComplex())
		}
		surplus([]*demand{c.demand}, owners, residual)
		return sub.demandResult(c.demand, owners, c.demand.satisfied(owners))
	}
	return sub.solveRoot(c)
}

// demandResult reports a demand: the members assigned to it, or for a grouped one the values filled, as Have; each
// assigned member as a hit, with the result of its where as a child.
func (s *poolSolver) demandResult(d *demand, owners []owner, met bool) Result {
	members, _, filled := d.tally(owners)
	r := Result{Node: d.count, State: metState(met, false), HaveKind: ValueNumber, Need: fxp.One}
	if d.count.Quantity != nil {
		r.Need = d.count.Quantity.Qualifier
	}
	if d.grouped {
		r.Have = fxp.FromInteger(filled)
	} else {
		r.Have = fxp.FromInteger(members)
	}
	if s.collect {
		for i, m := range d.cands {
			if owners[m].d != d {
				continue
			}
			r.hits = append(r.hits, Hit{Node: d.count, Item: s.items[m]})
			if !d.count.Where.IsZero() && i < len(d.wheres) {
				r.Children = append(r.Children, d.wheres[i])
			}
		}
		if met {
			r.Specificity = 1
		}
	}
	return r
}
