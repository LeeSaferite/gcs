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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

var allStates = []State{Met, Unmet, Skipped, Failed}

// stateSequences returns every sequence of states of length 0 to max.
func stateSequences(maxLen int) [][]State {
	result := make([][]State, 1, 128)
	frontier := [][]State{nil}
	for range maxLen {
		next := make([][]State, 0, len(allStates)*len(frontier))
		for _, seq := range frontier {
			for _, s := range allStates {
				next = append(next, append(append([]State(nil), seq...), s))
			}
		}
		result = append(result, next...)
		frontier = next
	}
	return result
}

// expectedGroupState is the reference model of section 6.5, written independently of the engine: the legacy rule of
// PR #1182, where a child that decides the group does so even when others failed.
func expectedGroupState(mode GroupMode, n int, not bool, guard *State, states []State) State {
	if guard != nil {
		switch *guard {
		case Unmet:
			return Skipped
		case Failed:
			return Failed
		default:
		}
	}
	applicable, met, unmet, failed := 0, 0, 0, 0
	for _, s := range states {
		switch s {
		case Skipped:
			continue
		case Met:
			met++
		case Unmet:
			unmet++
		default:
			failed++
		}
		applicable++
	}
	if applicable == 0 {
		return Skipped
	}
	need := applicable
	switch mode {
	case GroupAny:
		need = 1
	case GroupAtLeast:
		need = min(n, applicable)
	default:
	}
	met = min(met, need)
	switch {
	case met == need:
		if not {
			return Unmet
		}
		return Met
	case applicable-unmet >= need && failed > 0: // the threshold is still reachable, so the Failed ones decide
		return Failed
	case not:
		return Met
	default:
		return Unmet
	}
}

// TestGroupTruthTables checks all, any and at_least(N) over every sequence of up to three child states, with and
// without Not and each kind of guard, in both modes.
func TestGroupTruthTables(t *testing.T) {
	c := check.New(t)
	h := newHost()
	self := newItem("self", nil)
	guards := []*State{nil, new(Met), new(Unmet), new(Skipped), new(Failed)}
	for _, states := range stateSequences(3) {
		children := make(Nodes, len(states))
		for i, s := range states {
			children[i] = leafFor(s)
		}
		for _, mode := range GroupModes {
			ns := []int{0}
			if mode == GroupAtLeast {
				ns = nil
				for n := 1; n <= len(states); n++ {
					ns = append(ns, n)
				}
			}
			for _, n := range ns {
				for _, not := range []bool{false, true} {
					for _, guard := range guards {
						g := &Group{Mode: mode, N: n, Not: not, Children: children}
						if guard != nil {
							g.OnlyWhen = Ref{Node: leafFor(*guard)}
						}
						want := expectedGroupState(mode, n, not, guard, states)
						desc := fmt.Sprintf("%v n=%d not=%v guard=%v states=%v", mode, n, not, guardName(guard), states)
						yes := Evaluate(g, h, self, Options{})
						c.Equal(want, yes.State, desc)
						full := Evaluate(g, h, self, Options{Mode: Collect})
						c.Equal(want, full.State, desc+" (collect)")
						wantChildren := len(states)
						if guard != nil && *guard == Failed {
							wantChildren = 0
						}
						c.Equal(wantChildren, len(full.Children), desc+" (children)")
					}
				}
			}
		}
	}
}

// TestFailedKinds checks that member-local and global Failed propagate as section 6.1 says, independent of child
// order, and that a global Failed inside a where can never let a count become Met.
func TestFailedKinds(t *testing.T) {
	c := check.New(t)
	h := newHost()
	member := newItem("member", nil)
	self := newItem("self", nil).with("children", member)
	global := &Unknown{Reason: "nope"}
	for _, tc := range []struct {
		name  string
		node  Node
		state State
		local bool
	}{
		{"all[local, global]", &Group{Children: Nodes{localFailure(), global}}, Failed, false},
		{"all[global, local]", &Group{Children: Nodes{global, localFailure()}}, Failed, false},
		{"all[local, unmet]", &Group{Children: Nodes{localFailure(), &Const{}}}, Unmet, false},
		{"all[unmet, local]", &Group{Children: Nodes{&Const{}, localFailure()}}, Unmet, false},
		{"all[global, unmet]", &Group{Children: Nodes{global, &Const{}}}, Unmet, false},
		{"any[global, unmet]", &Group{Mode: GroupAny, Children: Nodes{global, &Const{}}}, Failed, false},
		{"any[local, unmet]", &Group{Mode: GroupAny, Children: Nodes{&Const{}, localFailure()}}, Failed, true},
		{"at_least 2 of [local, unmet, unmet]", &Group{Mode: GroupAtLeast, N: 2, Children: Nodes{
			localFailure(), &Const{},
			&Const{},
		}}, Unmet, false},
		{"at_least 2 of [local, unmet, met]", &Group{Mode: GroupAtLeast, N: 2, Children: Nodes{
			localFailure(), &Const{},
			&Const{Value: true},
		}}, Failed, true},
		{"any[global script, met]", &Group{Mode: GroupAny, Children: Nodes{&Script{Script: "failed"}, &Const{Value: true}}}, Met, false},
		{"any[met, global script]", &Group{Mode: GroupAny, Children: Nodes{&Const{Value: true}, &Script{Script: "failed"}}}, Met, false},
		{"not all[local]", &Group{Not: true, Children: Nodes{localFailure()}}, Failed, true},
		{"guard local", &Group{OnlyWhen: Ref{Node: localFailure()}, Children: Nodes{&Const{}}}, Failed, true},
		{"guard global", &Group{OnlyWhen: Ref{Node: global}, Children: Nodes{&Const{}}}, Failed, false},
		{"not count where unknown", &Count{Of: "children", Where: Ref{Node: global}, Not: true}, Failed, false},
		{"count is 0 where unknown", &Count{Of: "children", Where: Ref{Node: global}, Quantity: numberCritPtr(1, 0)}, Failed, false},
		{"count at_most 0 where unknown", &Count{Of: "children", Where: Ref{Node: global}, Quantity: numberCritPtr(4, 0)}, Failed, false},
		{"count at_most 0 where local", &Count{Of: "children", Where: Ref{Node: localFailure()}, Quantity: numberCritPtr(4, 0)}, Met, false},
		{"not count where local", &Count{Of: "children", Where: Ref{Node: localFailure()}, Not: true}, Met, false},
		{"not check local", &Check{Field: "missing", Not: true}, Failed, true},
	} {
		for _, opt := range []Options{yesNo(), collect()} {
			r := Evaluate(tc.node, h, self, opt)
			c.Equal(tc.state, r.State, tc.name)
			c.Equal(tc.local, r.MemberLocal(), tc.name)
		}
	}
}

func guardName(guard *State) string {
	if guard == nil {
		return "none"
	}
	return guard.String()
}
