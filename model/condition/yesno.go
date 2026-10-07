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
	if o.state == Failed && e.opt.FailedIsUnmet {
		switch n.(type) {
		case *Check, *Unknown:
			o.state = Unmet
		default:
		}
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
// can change the state: all at the first global Failed, any at the first Met, at_least once N are Met.
//
// An all group also skips the children that can no longer matter. Once a child is a member-local Failed, the group can
// only end up a member-local Failed, unless a later child is a global Failed; so only the children that can fail
// globally (see canFailGlobally) are still evaluated. Once a child is Unmet, the same holds when the frame is lenient,
// or when FailedIsUnmet is set, since a later member-local Failed would then make no difference anyone can see: the
// group would be "not met and not a global Failed" either way. The states are therefore the ones evaluating every child
// gives, except an Unmet where a member-local Failed would have been, in exactly the places where the two are
// indistinguishable.
func (g *Group) test(e *evaluator, f frame) outcome {
	if msg := e.problem(g, int(f.depth)); msg != "" {
		return outcome{reason: msg, state: Failed}
	}
	if !g.OnlyWhen.IsZero() {
		guard := e.testRef(g.OnlyWhen, f.strict())
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
	if g.Not {
		cf = f.strict()
	}
	lenient := cf.lenient || e.opt.FailedIsUnmet
	settled := false
	for _, child := range g.Children {
		if settled && !e.canFailGlobally(child, int(cf.depth)) {
			continue
		}
		o := e.test(child, cf)
		t.add(o.state, o.local, o.reason)
		if g.decided(&t, o) {
			break
		}
		settled = settled || (g.Mode == GroupAll && ((o.state == Failed && o.local) || (o.state == Unmet && lenient)))
	}
	return t.outcome(g)
}
