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
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/toolbox/v2/check"
)

// wideHost returns a host whose self has n children, each with its own n children, and so on.
func wideHost(n, depth int) (h *fakeHost, self *fakeItem) {
	var build func(name string, level int) *fakeItem
	build = func(name string, level int) *fakeItem {
		it := newItem(name, map[string]Value{"level": numberValue(int64(level))})
		if level < depth {
			kids := make([]*fakeItem, n)
			for i := range kids {
				kids[i] = build(name+"-", level+1)
			}
			it.with("kids", kids...)
		}
		return it
	}
	return newHost(), build("root", 0)
}

func kidsCount(where Node) *Count {
	return &Count{Of: "kids", Where: Ref{Node: where}}
}

// everyKid counts the kids with a quantity that only the tenth member decides, so that a count below the root, which
// YesNo stops visiting once its state is decided, still visits all ten.
func everyKid() *Count {
	return &Count{Of: "kids", Quantity: numberCritPtr(criteria.AtLeastNumber, 10)}
}

// TestStateBudget checks that running out of the state budget is a global Failed, that an any group can still be Met
// beside it, and that adversarial nested counts end promptly.
func TestStateBudget(t *testing.T) {
	c := check.New(t)
	h, self := wideHost(10, 1)
	r := Evaluate(kidsCount(nil), h, self, Options{Budget: 5})
	c.Equal(Failed, r.State)
	c.False(r.MemberLocal())
	c.Equal("too complex to decide", r.Reason)
	c.Equal(Met, Evaluate(kidsCount(nil), h, self, Options{Budget: 10}).State)
	anyGroup := &Group{Mode: GroupAny, Children: Nodes{kidsCount(nil), &Const{Value: true}}}
	c.Equal(Met, Evaluate(anyGroup, h, self, Options{Budget: 5}).State)
	all := &Group{Children: Nodes{everyKid(), &Const{Value: true}}}
	c.Equal(Failed, Evaluate(all, h, self, Options{Budget: 5}).State)
	// Once exhausted, every later visit fails at once.
	twice := &Group{Mode: GroupAny, Children: Nodes{everyKid(), &Compare{
		Left: Operand{Aggregate: AggregateCount, Members: Members{Of: "kids"}}, Op: OpIs, Right: value(10),
	}}}
	c.Equal(Failed, Evaluate(twice, h, self, Options{Budget: 5}).State)

	h, self = wideHost(60, 3)
	nested := kidsCount(kidsCount(kidsCount(&Const{})))
	for _, opt := range []Options{yesNo(), collect()} {
		r = Evaluate(nested, h, self, opt)
		c.Equal(Failed, r.State)
		c.Equal("too complex to decide", r.Reason)
	}
}

// TestDetailBudget checks that a tiny detail budget marks results Truncated and never changes a state.
func TestDetailBudget(t *testing.T) {
	c := check.New(t)
	h, self := wideHost(10, 2)
	level := &Check{Field: "level", Number: numberCrit(criteria.AtLeastNumber, 1)}
	tree := &Group{Children: Nodes{
		&Group{Mode: GroupAny, Children: Nodes{&Const{}, kidsCount(kidsCount(level))}},
		kidsCount(level),
		&Group{Mode: GroupAny, OnlyWhen: Ref{Node: kidsCount(nil)}, Children: Nodes{&Const{Value: true}}},
	}}
	full := Evaluate(tree, h, self, collect())
	c.Equal(Met, full.State)
	c.False(full.Truncated)
	c.Equal(2, full.Specificity)
	for _, detail := range []int{1, 2, 3, 5, 8, 13, 40, 100} {
		r := Evaluate(tree, h, self, Options{Mode: Collect, DetailBudget: detail})
		c.Equal(full.State, r.State, detail)
		c.True(r.Truncated, detail)
		c.True(r.Specificity <= full.Specificity, detail)
		var walk func(a, b *Result)
		walk = func(a, b *Result) {
			c.Equal(a.State, b.State, detail)
			if !b.Truncated {
				c.Equal(len(a.Children), len(b.Children), detail)
			}
			for i := range b.Children {
				walk(&a.Children[i], &b.Children[i])
			}
		}
		walk(&full, &r)
	}
}

// TestRootMismatch checks the rule for a detail pass whose root state differs from the YesNo state, which happens only
// when the YesNo pass ran out of budget: the root keeps the YesNo state, is Truncated, and carries nothing Met.
func TestRootMismatch(t *testing.T) {
	c := check.New(t)
	h, self := wideHost(10, 1)
	tree := &Group{Children: Nodes{&Const{Value: true}, everyKid()}}
	r := Evaluate(tree, h, self, Options{Mode: Collect, Budget: 5})
	c.Equal(Failed, r.State)
	c.True(r.Truncated)
	c.Equal(0, len(r.Children))
	c.Equal(0, r.Specificity)
	r = Evaluate(kidsCount(nil), h, self, Options{Mode: Collect, Budget: 5})
	c.Equal(Failed, r.State)
	for range r.Hits() {
		c.True(false, "a mismatched root carries no hits")
	}
}

// TestEvalDepth checks that a tree built in code deeper than MaxDepth is a global Failed at the too-deep node.
func TestEvalDepth(t *testing.T) {
	c := check.New(t)
	h := newHost()
	c.Equal(Met, Evaluate(nested(31, &Const{Value: true}), h, nil, yesNo()).State)
	r := Evaluate(nested(32, &Const{Value: true}), h, nil, yesNo())
	c.Equal(Failed, r.State)
	c.Equal("nested more than 32 deep", r.Reason)
	deep := nested(31, &Compare{Left: value(1), Right: value(1)})
	c.Equal(Failed, Evaluate(deep, h, nil, yesNo()).State)
	c.Equal(Met, Evaluate(&Group{Mode: GroupAny, Children: Nodes{nested(32, &Const{}), &Const{Value: true}}}, h, nil, yesNo()).State)
	c.Equal(Skipped, Evaluate(nil, h, nil, collect()).State)
}
