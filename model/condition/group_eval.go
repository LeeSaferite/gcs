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

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
)

// eval implements Node. The guard comes first: Unmet makes the group Skipped without evaluating its children, Failed
// makes it Failed, and Met or Skipped means the group applies.
func (g *Group) eval(e *evaluator, f frame) Result {
	if !e.collect {
		return g.test(e, f).result()
	}
	if !g.OnlyWhen.IsZero() {
		guard := e.evalYesNo(g.OnlyWhen, f)
		switch guard.State {
		case Unmet:
			r := Result{State: Skipped, Children: make([]Result, len(g.Children))}
			for i, child := range g.Children {
				r.Children[i] = Result{Node: child, State: Skipped}
			}
			return r
		case Failed:
			return failure(guard.Reason)
		default:
		}
	}
	if g.Exclusive {
		return e.exclusiveGroup(g, f)
	}
	var t groupTally
	children := make([]Result, len(g.Children))
	for i, child := range g.Children {
		children[i] = e.eval(child, f)
		t.add(children[i].State, children[i].Reason)
	}
	r := t.outcome(g).result()
	r.Children = children
	if r.State == Met && !g.Not {
		r.Specificity = g.specificity(children)
	}
	return r
}

// groupTally counts the states of a group's applicable children.
type groupTally struct {
	reason     string // of the first Failed child
	met        int
	unmet      int
	applicable int
	failed     int
}

func (t *groupTally) add(state State, reason string) {
	switch state {
	case Skipped:
		return
	case Met:
		t.met++
	case Unmet:
		t.unmet++
	default:
		t.failed++
		if t.reason == "" {
			t.reason = reason
		}
	}
	t.applicable++
}

// outcome returns the state the tally gives a group with this mode and Not: Skipped when no child applies, Met when
// the group holds however its Failed children would have come out, Unmet when it holds for none of those ways, so that
// no Failed child could have changed it, and otherwise Failed. Not then swaps Met and Unmet.
func (t *groupTally) outcome(g *Group) outcome {
	if t.applicable == 0 {
		return outcome{state: Skipped}
	}
	always, never := g.holdsOver(t.met, t.met+t.failed, t.applicable)
	switch {
	case always:
		return outcome{state: metState(true, g.Not)}
	case never:
		return outcome{state: metState(false, g.Not)}
	default:
		return outcome{reason: t.reason, state: Failed}
	}
}

// holdsOver reports whether the group holds for every number of Met children from low to high, out of applicable,
// and whether it holds for none of them.
func (g *Group) holdsOver(low, high, applicable int) (always, never bool) {
	always, never = true, true
	for met := low; met <= high; met++ {
		if g.holds(met, applicable) {
			never = false
		} else {
			always = false
		}
	}
	return always, never
}

// holds reports whether the group holds when met of its applicable children are Met.
func (g *Group) holds(met, applicable int) bool {
	switch g.Mode {
	case GroupAny:
		return met >= 1
	case GroupCount:
		if g.Quantity == nil {
			return met >= 1
		}
		return g.Quantity.Matches(fxp.FromInteger(met))
	default:
		return met >= applicable
	}
}

// decided reports whether a YesNo evaluation can stop with remaining children still to go, because no way the rest
// could come out would change the state: all at the first Unmet, any at the first Met, and count once some child
// applies and the count holds, or fails to, for every number of Met children the rest could bring.
func (g *Group) decided(t *groupTally, remaining int) bool {
	switch g.Mode {
	case GroupAny:
		return t.met != 0
	case GroupCount:
		if t.applicable == 0 {
			return false
		}
		always, never := g.holdsOver(t.met, t.met+t.failed+remaining, 0)
		return always || never
	default:
		return t.unmet != 0
	}
}

// specificity returns the specificity of a Met group: the sum over applicable children for all, the maximum over Met
// children for any, and for count the sum of the best N Met children when the quantity is at least N, otherwise the
// sum over Met children.
func (g *Group) specificity(children []Result) int {
	var specs []int
	for i := range children {
		if children[i].State == Met {
			specs = append(specs, children[i].Specificity)
		}
	}
	switch g.Mode {
	case GroupAny:
		return slices.Max(specs)
	case GroupCount:
		if q := canonicalQuantity(g.Quantity); q == nil || q.Compare == criteria.AtLeastNumber {
			best := 1
			if q != nil {
				best = max(q.Qualifier.Ceil().AsInteger[int](), 0)
			}
			slices.SortFunc(specs, func(a, b int) int { return cmp.Compare(b, a) })
			specs = specs[:min(best, len(specs))]
		}
	default:
	}
	total := 0
	for _, one := range specs {
		total += one
	}
	return total
}
