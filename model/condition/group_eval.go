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
	"cmp"
	"slices"
)

// eval implements Node. The guard comes first: Unmet makes the group Skipped without evaluating its children, Failed
// makes it Failed with the guard's kind, and Met or Skipped means the group applies.
func (g *Group) eval(e *evaluator, f frame) Result {
	if !e.collect {
		return g.test(e, f).result()
	}
	if r, bad := e.invalid(g, f); bad {
		return r
	}
	if !g.OnlyWhen.IsZero() {
		guard := e.evalYesNo(g.OnlyWhen, f.next())
		switch guard.State {
		case Unmet:
			return g.skipped(true)
		case Failed:
			return Result{State: Failed, Reason: guard.Reason, local: guard.local}
		default:
		}
	}
	if g.Exclusive {
		return e.exclusiveGroup(g, f)
	}
	return g.combine(e, f)
}

// skipped returns the result of a group whose guard is unmet. Collect reports each child Skipped, unevaluated.
func (g *Group) skipped(collect bool) Result {
	r := Result{State: Skipped}
	if collect {
		r.Children = make([]Result, len(g.Children))
		for i, child := range g.Children {
			r.Children[i] = Result{Node: child, State: Skipped}
		}
	}
	return r
}

// groupTally counts the states of a group's applicable children.
type groupTally struct {
	reason     string
	met        int
	unmet      int
	applicable int
	failed     int
	global     bool // some applicable Failed child is global
}

func (t *groupTally) add(state State, local bool, reason string) {
	switch state {
	case Skipped:
		return
	case Met:
		t.met++
	case Unmet:
		t.unmet++
	case Failed:
		t.failed++
		if !local {
			t.global = true
		}
		if t.reason == "" {
			t.reason = reason
		}
	default:
	}
	t.applicable++
}

// outcome returns the state the tally gives a group with this mode and Not, as 6.5 has it: Skipped when no child
// applies, Met when enough are Met, and otherwise as notMet has it.
func (t *groupTally) outcome(g *Group) outcome {
	switch {
	case t.applicable == 0:
		return outcome{state: Skipped}
	case t.met >= g.required(t.applicable):
		return outcome{state: metState(true, g.Not)}
	default:
		return t.notMet(g, g.Not)
	}
}

// notMet returns the state of a group that isn't Met: Unmet when its Unmet children alone leave too few to reach the
// number required, so that no Failed child could have changed it, otherwise Failed when any applicable child is
// Failed, global when any of those is, and Unmet when none is. not then swaps Unmet for Met.
func (t *groupTally) notMet(g *Group, not bool) outcome {
	if t.failed > 0 && t.unmet <= t.applicable-g.required(t.applicable) {
		return outcome{state: Failed, reason: t.reason, local: !t.global}
	}
	return outcome{state: metState(false, not)}
}

// combine evaluates every child of a group that applies, in Collect mode, and combines their states by the group's
// mode.
func (g *Group) combine(e *evaluator, f frame) Result {
	var t groupTally
	children := make([]Result, 0, len(g.Children))
	cf := f.next()
	for _, child := range g.Children {
		r := e.eval(child, cf)
		t.add(r.State, r.local, r.Reason)
		children = append(children, r)
	}
	r := t.outcome(g).result()
	r.Children = children
	if r.State == Met && !g.Not {
		r.Specificity = g.specificity(children, t.applicable)
	}
	return r
}

// required returns how many applicable children must be Met.
func (g *Group) required(applicable int) int {
	switch g.Mode {
	case GroupAny:
		return 1
	case GroupAtLeast:
		return min(g.N, applicable)
	default:
		return applicable
	}
}

// decided reports whether a YesNo evaluation can stop with remaining children still to go: once enough are Met
// whatever the rest are (any at one, at_least at N; all only at the end), or once the Unmet ones alone put the number
// required out of reach even if every remaining child applies and is Met, which makes the group Unmet whatever the
// rest are, Failed ones included.
func (g *Group) decided(t *groupTally, remaining int) bool {
	possible := t.applicable + remaining
	if t.unmet > possible-g.required(possible) {
		return true
	}
	switch g.Mode {
	case GroupAny:
		return t.met >= 1
	case GroupAtLeast:
		return t.met >= g.N
	default:
		return false
	}
}

// specificity returns the specificity of a Met group: the sum over applicable children for all, the maximum over Met
// children for any, and the sum of the best min(N, applicable) Met children for at_least.
func (g *Group) specificity(children []Result, applicable int) int {
	var specs []int
	for i := range children {
		if children[i].State == Met {
			specs = append(specs, children[i].Specificity)
		}
	}
	switch g.Mode {
	case GroupAny:
		return slices.Max(specs)
	case GroupAtLeast:
		slices.SortFunc(specs, func(a, b int) int { return cmp.Compare(b, a) })
		specs = specs[:min(g.N, applicable)]
	default:
	}
	total := 0
	for _, one := range specs {
		total += one
	}
	return total
}
