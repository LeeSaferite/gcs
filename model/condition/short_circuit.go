// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

import "strings"

// canFailGlobally reports whether evaluating n at depth could give a global Failed. It is a static property of the
// tree, decided without a host. A nil node cannot, and neither can a valid Const. A valid Check cannot unless its text
// qualifier holds a self token, which can name a field self lacks or expand past the alternative cap; a missing or
// unsuited field of the subject is only ever member-local. A valid group cannot when it is not exclusive (the solver
// can run out of budget), its guard cannot, and none of its children can. Everything else can: counts and compares (a
// collection the host lacks, an unresolved operand, the budget), scripts, Unknown nodes, and any node Validate
// reports, which includes one deeper than MaxDepth.
//
// It costs a walk of n's subtree, which is only ever done in place of evaluating that subtree.
func (e *evaluator) canFailGlobally(n Node, depth int) bool {
	switch t := n.(type) {
	case nil:
		return false
	case *Const:
		return e.problem(t, depth) != ""
	case *Check:
		return e.problem(t, depth) != "" ||
			(!t.Text.IsZero() && strings.Contains(t.Text.Qualifier, selfTokenPrefix))
	case *Group:
		if t.Exclusive || e.problem(t, depth) != "" || e.canFailGlobally(t.OnlyWhen.Node, depth+1) {
			return true
		}
		for _, child := range t.Children {
			if e.canFailGlobally(child, depth+1) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

// treeValid reports whether no node of the tree rooted at n, placed at depth, has a problem of its own; nil children
// don't count, since they are only ever skipped. When it holds, the evaluation needs no per-node validity checks.
func (e *evaluator) treeValid(n Node, depth int) bool {
	if n == nil {
		return true
	}
	if e.shallow.ownProblem(n, depth) != "" {
		return false
	}
	switch t := n.(type) {
	case *Group:
		if !t.OnlyWhen.IsZero() && !e.treeValid(t.OnlyWhen.Node, depth+1) {
			return false
		}
		for _, child := range t.Children {
			if !e.treeValid(child, depth+1) {
				return false
			}
		}
	case *Count:
		return t.Where.IsZero() || e.treeValid(t.Where.Node, depth+1)
	case *Compare:
		return e.operandValid(&t.Left, depth+1) && e.operandValid(&t.Right, depth+1)
	default:
	}
	return true
}

// operandValid reports whether the where nodes beneath an operand at depth are all valid; the operand itself was
// checked with its compare.
func (e *evaluator) operandValid(o *Operand, depth int) bool {
	if o.hasMembers() && !o.Where.IsZero() && !e.treeValid(o.Where.Node, depth+1) {
		return false
	}
	for i := range o.Sum {
		if !e.operandValid(&o.Sum[i], depth+1) {
			return false
		}
	}
	return true
}
