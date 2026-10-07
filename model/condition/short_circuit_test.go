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
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/check"
)

// TestShortCircuitMatchesFullEvaluation checks the YesNo short cuts against evaluating everything: for random where
// trees, a count's YesNo state must be the one the members' full (Collect) states give, which never skip a child or a
// member. The count sits below the root, so its member loop may stop early too.
func TestShortCircuitMatchesFullEvaluation(t *testing.T) {
	c := check.New(t)
	r := rand.New(rand.NewPCG(13, 14)) //nolint:gosec // Not for security.
	quantities := []*criteria.Number{
		nil, atLeast(2), exactly(1), numberCritPtr(criteria.AtMostNumber, 1),
		numberCritPtr(criteria.NotEqualsNumber, 2),
		{},
	}
	for i := range 3000 {
		h, self := randomHost(r)
		where := randomNode(r, 2)
		count := &Count{Of: "kids", Where: Ref{Node: where}, Quantity: quantities[r.IntN(len(quantities))]}
		failedIsUnmet := r.IntN(2) == 0
		opt := Options{FailedIsUnmet: failedIsUnmet}
		have, global := 0, false
		kids, _ := h.Collection("kids", self, self)
		for _, kid := range kids {
			full := EvaluateFor(where, h, self, kid, Options{Mode: Collect, FailedIsUnmet: failedIsUnmet})
			switch {
			case full.State.Passes():
				have++
			case full.State == Failed && !full.local:
				global = true
			default:
			}
		}
		want := Failed
		if !global {
			want = Unmet
			if have >= 1 && count.Quantity == nil || count.Quantity != nil && count.Quantity.Matches(fxpInt(have)) {
				want = Met
			}
		}
		desc := fmt.Sprint("case ", i)
		c.Equal(want, Evaluate(count, h, self, opt).State, desc)
		c.Equal(want, Evaluate(&Group{Children: Nodes{count}}, h, self, opt).State, desc+" (nested)")
		c.Equal(want, Evaluate(&Group{Mode: GroupAny, Children: Nodes{&Const{}, count}}, h, self, opt).State, desc)
	}
}

// TestShortCircuitStillFindsGlobalFailures checks that the children an all group skips are only ever ones that
// cannot fail globally: a later sibling that can is still evaluated.
func TestShortCircuitStillFindsGlobalFailures(t *testing.T) {
	c := check.New(t)
	h := newHost()
	member := newItem("member", map[string]Value{"level": numberValue(1)})
	self := newItem("self", nil).with("kids", member)
	unmet := checkText("name", criteria.IsText, "other")
	local := localFailure()
	for _, tc := range []struct {
		name string
		node Node
		opt  Options
	}{
		{"unmet then unknown in a where", &Count{Of: "kids", Where: Ref{Node: &Group{
			Children: Nodes{unmet, &Const{}, &Unknown{Reason: "later"}},
		}}}, Options{}},
		{"unmet then script in a where", &Count{Of: "kids", Where: Ref{Node: &Group{
			Children: Nodes{unmet, &Script{Script: "failed"}},
		}}}, Options{}},
		{"local then count at the root", &Group{Children: Nodes{local, &Count{Of: "zz"}}}, Options{}},
		{"unmet then invalid check with FailedIsUnmet", &Group{Children: Nodes{
			checkText("name", criteria.IsText, "other"), &Group{N: 2, Children: Nodes{&Const{}}},
		}}, Options{FailedIsUnmet: true}},
		{"unmet then self token", &Count{Of: "kids", Where: Ref{Node: &Group{
			Children: Nodes{unmet, checkText("name", criteria.IsText, "{self.missing}")},
		}}}, Options{}},
		{"unmet then exclusive group", &Count{Of: "kids", Where: Ref{Node: &Group{
			Children: Nodes{unmet, &Group{Exclusive: true, Children: Nodes{&Count{Of: "zz"}}}},
		}}}, Options{}},
	} {
		r := Evaluate(tc.node, h, self, tc.opt)
		c.Equal(Failed, r.State, tc.name)
		c.False(r.MemberLocal(), tc.name)
	}
	// Below the root of a strict context, an Unmet then a member-local Failed stays a member-local Failed.
	r := Evaluate(&Group{Children: Nodes{unmet, local}}, h, member, Options{})
	c.Equal(Failed, r.State)
	c.True(r.MemberLocal())
	// Under Not inside a where, an Unmet followed by a member-local Failed must not turn into Met.
	notGroup := &Count{Of: "kids", Where: Ref{Node: &Group{Not: true, Children: Nodes{
		&Group{Children: Nodes{unmet, local}},
	}}}}
	c.Equal(Unmet, Evaluate(notGroup, h, self, Options{}).State)
}

// TestCountStopsEarly checks that a count below the root stops visiting members once its state is decided, unless
// its where could fail globally, and that a count at the root still visits every member for its Have.
func TestCountStopsEarly(t *testing.T) {
	c := check.New(t)
	h, self := wideHost(10, 1)
	nested := &Group{Children: Nodes{kidsCount(nil)}}
	c.Equal(Met, Evaluate(nested, h, self, Options{Budget: 2}).State)
	risky := &Group{Children: Nodes{kidsCount(&Script{Script: "met"})}}
	c.Equal(Failed, Evaluate(risky, h, self, Options{Budget: 2}).State)
	c.Equal(Failed, Evaluate(kidsCount(nil), h, self, Options{Budget: 2}).State)
	r := Evaluate(kidsCount(nil), h, self, Options{})
	c.Equal("10", r.Have.String())
	for _, q := range []*criteria.Number{
		exactly(1), numberCritPtr(criteria.AtMostNumber, 0),
		numberCritPtr(criteria.NotEqualsNumber, 1),
		{},
	} {
		count := kidsCount(nil)
		count.Quantity = q
		c.Equal(Evaluate(&Group{Children: Nodes{count}}, h, self, Options{}).State,
			Evaluate(&Group{Children: Nodes{count}}, h, self, Options{Budget: 3}).State, q)
	}
}

func fxpInt(n int) fxp.Int {
	return fxp.FromInteger(n)
}
