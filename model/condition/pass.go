// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

// frame is the context a node is evaluated in. The subject becomes each member inside a where, and of then points at
// the key of that where's collection. Self never changes during a call, so the evaluator holds it. A frame is copied
// for every node, so it is kept small.
type frame struct {
	subject Item
	of      *string
	depth   int32
	// lenient is set where nothing above can tell an Unmet from a member-local Failed: inside a where, until a Not
	// group or a guard. A where only asks whether its member passes, and groups carry either state up as "not met
	// and not a global Failed". See (*Group).test for what this allows.
	lenient bool
}

// collection returns the key of the collection whose where the frame is in, or "" for the root subject.
func (f frame) collection() string {
	if f.of == nil {
		return ""
	}
	return *f.of
}

// next returns the frame for a node one level down, with the same subject.
func (f frame) next() frame {
	f.depth++
	return f
}

// strict returns the frame for a node one level down where Unmet and member-local Failed must stay apart: below a Not
// group, or in a guard.
func (f frame) strict() frame {
	f.depth++
	f.lenient = false
	return f
}

// member returns the frame for the where of a collection, with the given member as its subject.
func (f frame) member(of *string, item Item) frame {
	return frame{subject: item, of: of, depth: f.depth + 1, lenient: true}
}

func (e *evaluator) startPass(detail bool) {
	e.budget = e.opt.Budget
	e.detail = e.opt.DetailBudget
	e.collect = detail
	e.detailPass = detail
	e.exhausted = false
}

// spend charges one member visit. While collecting, visits are charged to the detail budget, and running out of it
// stops collecting; otherwise they are charged to the state budget. It returns false once the state budget is spent.
func (e *evaluator) spend() bool {
	if e.collect {
		e.spendDetail()
		return true
	}
	if e.exhausted {
		return false
	}
	e.budget--
	if e.budget < 0 {
		e.exhausted = true
		return false
	}
	return true
}

func (e *evaluator) spendDetail() {
	e.detail--
	if e.detail <= 0 {
		e.collect = false
	}
}
