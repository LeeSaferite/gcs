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
	"iter"

	"github.com/richardwilkes/gcs/v5/model/fxp"
)

// Hit is one thing a Met node held for.
type Hit struct {
	Node Node
	Item Item // the subject the node held for; nil for compares
}

// Result is the outcome of evaluating a node.
type Result struct {
	Node        Node
	State       State
	Have        fxp.Int // count, value, left operand, or Number/Weight check value
	HaveKind    ValueKind
	Need        fxp.Int // quantity or right operand, for "9 of 12"
	Reason      string
	Specificity int      // Collect only
	Truncated   bool     // Collect ran out of detail budget below here; State is still exact
	Children    []Result // Collect only
	hits        []Hit    // leaves, counts and compares only
	local       bool     // a Failed state is member-local rather than global
}

// MemberLocal returns true when the state is a member-local Failed: one caused by the subject itself, such as a
// missing field, rather than by something that could not be evaluated at all.
func (r *Result) MemberLocal() bool {
	return r.State == Failed && r.local
}

// Hits walks the Met branches of a Collect result and yields what each Met leaf, count and compare held for. Negated
// nodes give none. Nothing is copied up the tree.
func (r *Result) Hits() iter.Seq[Hit] {
	return func(yield func(Hit) bool) {
		r.yieldHits(yield)
	}
}

func (r *Result) yieldHits(yield func(Hit) bool) bool {
	if r.State != Met || negated(r.Node) {
		return true
	}
	for _, hit := range r.hits {
		if !yield(hit) {
			return false
		}
	}
	for i := range r.Children {
		if !r.Children[i].yieldHits(yield) {
			return false
		}
	}
	return true
}

// negated returns true for a node whose Not is set.
func negated(n Node) bool {
	switch t := n.(type) {
	case *Group:
		return t.Not
	case *Check:
		return t.Not
	case *Count:
		return t.Not
	case *Compare:
		return t.Not
	default:
		return false
	}
}

// failGlobal returns a global Failed result.
func failGlobal(reason string) Result {
	return Result{State: Failed, Reason: reason}
}

// metState returns Met for true and Unmet for false, swapped when not is set.
func metState(met, not bool) State {
	if met != not {
		return Met
	}
	return Unmet
}
