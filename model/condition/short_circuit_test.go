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
		var opt Options
		have, global := 0, false
		kids, _ := h.Collection("kids", self, self)
		for _, kid := range kids {
			full := EvaluateFor(where, h, self, kid, Options{Mode: Collect})
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

// TestShortCircuitDecides checks that YesNo stops a group once a child decides it, so a later child that would fail,
// even globally, changes nothing: all at the first Unmet, any at the first Met, and at_least once N are Met or the
// Unmet ones alone put N out of reach. A group that ends Failed still evaluates every child, so its kind is exact.
func TestShortCircuitDecides(t *testing.T) {
	c := check.New(t)
	h := newHost()
	member := newItem("member", map[string]Value{"level": numberValue(1)})
	self := newItem("self", nil).with("kids", member)
	unmet := checkText("name", criteria.IsText, "other")
	failed := &Script{Script: "failed"}
	for _, tc := range []struct {
		name  string
		node  Node
		state State
		local bool
	}{
		{"all: unmet then unknown", &Group{Children: Nodes{unmet, &Unknown{Reason: "later"}}}, Unmet, false},
		{"all: failed then unmet", &Group{Children: Nodes{failed, &Const{}}}, Unmet, false},
		{"all: unmet then unknown in a where", &Count{Of: "kids", Where: Ref{Node: &Group{
			Children: Nodes{unmet, &Unknown{Reason: "later"}},
		}}}, Unmet, false},
		{"any: met then unknown", &Group{Mode: GroupAny, Children: Nodes{&Const{Value: true}, &Unknown{}}}, Met, false},
		{"any: failed then met", &Group{Mode: GroupAny, Children: Nodes{failed, &Const{Value: true}}}, Met, false},
		{"at_least: out of reach then unknown", &Group{Mode: GroupAtLeast, N: 2, Children: Nodes{
			&Const{}, &Const{},
			&Unknown{},
		}}, Unmet, false},
		{"at_least: met then unknown", &Group{Mode: GroupAtLeast, N: 1, Children: Nodes{
			&Const{Value: true},
			&Unknown{},
		}}, Met, false},
		{"all: local then global", &Group{Children: Nodes{localFailure(), &Count{Of: "zz"}}}, Failed, false},
		{"all: global then local", &Group{Children: Nodes{&Count{Of: "zz"}, localFailure()}}, Failed, false},
		{"all: local then met", &Group{Children: Nodes{localFailure(), &Const{Value: true}}}, Failed, true},
		{"not: unmet then unknown", &Group{Not: true, Children: Nodes{unmet, &Unknown{}}}, Met, false},
	} {
		for _, opt := range []Options{yesNo(), collect()} {
			r := Evaluate(tc.node, h, member, opt)
			if count, ok := tc.node.(*Count); ok {
				r = Evaluate(count, h, self, opt)
			}
			c.Equal(tc.state, r.State, tc.name)
			c.Equal(tc.local, r.MemberLocal(), tc.name)
		}
	}
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
