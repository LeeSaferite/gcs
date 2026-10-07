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
	"slices"

	"github.com/richardwilkes/toolbox/v2/i18n"
)

// Problem is one thing Validate found wrong with a tree. Validate never changes data: invalid nodes stay, evaluate as
// a global Failed, and save unchanged.
type Problem struct {
	Path Path
	Msg  string
}

// validator collects problems. A shallow validator, which evaluation uses to decide whether a node is invalid, keeps
// only the first problem of the node itself and its operands: it never descends into child, only_when or where nodes,
// and never allocates.
type validator struct {
	first    string
	path     Path
	problems []Problem
	shallow  bool
}

// validateNode returns the problems found in the tree rooted at n.
func validateNode(n Node) []Problem {
	var v validator
	v.node(n, 1)
	return v.problems
}

func (v *validator) report(msg string) {
	if v.shallow {
		if v.first == "" {
			v.first = msg
		}
		return
	}
	v.problems = append(v.problems, Problem{Path: slices.Clone(v.path), Msg: msg})
}

func (v *validator) node(n Node, depth int) {
	if depth > MaxDepth {
		v.report(reasonTooDeep())
		return
	}
	n.validate(v, depth)
}

func (v *validator) child(s Slot, n Node, depth int) {
	if v.shallow {
		return
	}
	v.path = append(v.path, s)
	if n == nil {
		v.report(i18n.Text("missing child"))
	} else {
		v.node(n, depth)
	}
	v.path = v.path[:len(v.path)-1]
}

func (v *validator) ref(kind SlotKind, r Ref, depth int) {
	if !v.shallow && !r.IsZero() {
		v.path = append(v.path, Slot{Kind: kind})
		v.node(r.Node, depth)
		v.path = v.path[:len(v.path)-1]
	}
}

func (v *validator) operand(s Slot, o *Operand, depth int) {
	if !v.shallow {
		v.path = append(v.path, s)
		defer func() { v.path = v.path[:len(v.path)-1] }()
	}
	if depth > MaxDepth {
		v.report(reasonTooDeep())
	} else {
		o.validate(v, depth)
	}
}

// ownProblem returns the first problem of n itself, including its operands but not the nodes beneath it, or "" when
// there is none.
func (v *validator) ownProblem(n Node, depth int) string {
	v.first = ""
	v.node(n, depth)
	return v.first
}
