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
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestExplainMatchesCollect checks, over random trees and hosts with unlimited budgets, that explaining a YesNo result
// gives exactly the Collect result, in every field and throughout the tree.
func TestExplainMatchesCollect(t *testing.T) {
	c := check.New(t)
	r := rand.New(rand.NewPCG(5, 6)) //nolint:gosec // Not for security.
	for i := range 2000 {
		h, self := randomHost(r)
		tree := randomNode(r, 0)
		opt := Options{FailedIsUnmet: r.IntN(2) == 0}
		yes := Evaluate(tree, h, self, opt)
		got := Explain(&yes, h, self, opt)
		opt.Mode = Collect
		want := Evaluate(tree, h, self, opt)
		c.True(reflect.DeepEqual(want, got), i)
	}
}

// TestExplainBudgets checks that explaining matches Collect when the state pass ran out of budget, so the root keeps
// the YesNo state, and when the detail pass does.
func TestExplainBudgets(t *testing.T) {
	c := check.New(t)
	h, self := wideHost(10, 1)
	tree := &Group{Children: Nodes{&Const{Value: true}, everyKid()}}
	for _, opt := range []Options{{Budget: 5}, {DetailBudget: 3}} {
		yes := Evaluate(tree, h, self, opt)
		got := ExplainFor(&yes, h, self, self, opt)
		opt.Mode = Collect
		want := Evaluate(tree, h, self, opt)
		c.True(want.Truncated)
		c.True(reflect.DeepEqual(want, got), opt)
	}
}

// TestExplainFallback checks that a result without a node or a real state is evaluated in full.
func TestExplainFallback(t *testing.T) {
	c := check.New(t)
	h, self := wideHost(3, 1)
	c.Equal(Skipped, Explain(nil, h, self, Options{}).State)
	c.Equal(Skipped, Explain(&Result{}, h, self, Options{}).State)
	tree := kidsCount(nil)
	got := Explain(&Result{Node: tree, State: Skipped + 1}, h, self, Options{})
	c.True(reflect.DeepEqual(Evaluate(tree, h, self, collect()), got))
}
