// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

// outcome is what a YesNo evaluation needs of a node below the root: its state, the kind of a Failed state, and the
// reason. It is much smaller than a Result, which below the root nobody sees in YesNo mode.
type outcome struct {
	reason string
	state  State
	local  bool
}

func (r *Result) outcome() outcome {
	return outcome{reason: r.Reason, state: r.State, local: r.local}
}

func (o outcome) result() Result {
	return Result{State: o.state, Reason: o.reason, local: o.local}
}

// test evaluates a node in YesNo mode for a parent that needs only its outcome. It must only be called while not
// collecting. Groups, checks and consts have dedicated paths; other nodes are evaluated in full.
func (e *evaluator) test(n Node, f frame) outcome {
	var o outcome
	switch t := n.(type) {
	case nil:
		return outcome{state: Skipped}
	case *Group:
		o = t.test(e, f)
	case *Check:
		o = t.test(e, f)
	case *Const:
		if msg := e.problem(t, int(f.depth)); msg != "" {
			return outcome{reason: msg, state: Failed}
		}
		return outcome{state: metState(t.Value, false)}
	default:
		r := n.eval(e, f)
		o = r.outcome()
	}
	return o
}

// testRef tests the node of a slot; an empty one is Skipped.
func (e *evaluator) testRef(r Ref, f frame) outcome {
	if r.IsZero() {
		return outcome{state: Skipped}
	}
	return e.test(r.Node, f)
}

// test is the YesNo evaluation of a group. The guard comes first, then the children, stopping once nothing further
// can change the state (see decided): all at the first Unmet, any at the first Met, at_least once N are Met or the
// Unmet children alone put N out of reach. A group that ends Failed has evaluated every child, so its kind is exact.
func (g *Group) test(e *evaluator, f frame) outcome {
	if msg := e.problem(g, int(f.depth)); msg != "" {
		return outcome{reason: msg, state: Failed}
	}
	if !g.OnlyWhen.IsZero() {
		guard := e.testRef(g.OnlyWhen, f.next())
		switch guard.state {
		case Unmet:
			return outcome{state: Skipped}
		case Failed:
			return guard
		default:
		}
	}
	if g.Exclusive {
		r := e.exclusiveGroup(g, f)
		return r.outcome()
	}
	var t groupTally
	cf := f.next()
	for i, child := range g.Children {
		o := e.test(child, cf)
		t.add(o.state, o.local, o.reason)
		if g.decided(&t, len(g.Children)-i-1) {
			break
		}
	}
	return t.outcome(g)
}
