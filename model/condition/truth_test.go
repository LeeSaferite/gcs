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

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
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

// expectedGroupState is the reference model, written independently of the engine: the legacy rule of PR #1182, where
// a child that decides the group does so even when others failed. A count group holds when its quantity matches the
// number of Met children, and is decided when it would be whichever way each Failed child had come out.
func expectedGroupState(mode GroupMode, q *criteria.Number, not bool, guard *State, states []State) State {
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
	result := func(holds bool) State {
		if holds != not {
			return Met
		}
		return Unmet
	}
	if mode == GroupCount {
		var outcomes [2]bool
		for ways := range 1 << failed {
			total := met
			for bit := range failed {
				total += (ways >> bit) & 1
			}
			holds := total >= 1
			if q != nil {
				holds = q.Matches(fxp.FromInteger(total))
			}
			if holds {
				outcomes[1] = true
			} else {
				outcomes[0] = true
			}
		}
		if outcomes[0] && outcomes[1] {
			return Failed
		}
		return result(outcomes[1])
	}
	need := applicable
	if mode == GroupAny {
		need = 1
	}
	met = min(met, need)
	switch {
	case met == need:
		return result(true)
	case applicable-unmet >= need && failed > 0: // the threshold is still reachable, so the Failed ones decide
		return Failed
	default:
		return result(false)
	}
}

// groupQuantities returns the quantities a group of the mode is tested with: none but for count, which takes nil and
// each comparison of up to n children.
func groupQuantities(mode GroupMode, n int) []*criteria.Number {
	if mode != GroupCount {
		return []*criteria.Number{nil}
	}
	list := []*criteria.Number{nil}
	for _, cmp := range criteria.NumericComparisons {
		for k := range n + 2 {
			list = append(list, &criteria.Number{Compare: cmp, Qualifier: fxp.FromInteger(k)})
		}
	}
	return list
}

// TestGroupTruthTables checks all, any and count over every sequence of up to three child states, with and without
// Not and each kind of guard, in every mode and, for count, with every comparison.
func TestGroupTruthTables(t *testing.T) {
	c := check.New(t)
	h := newHost()
	guards := []*State{nil, new(Met), new(Unmet), new(Skipped), new(Failed)}
	for _, states := range stateSequences(3) {
		children := make(Nodes, len(states))
		for i, s := range states {
			children[i] = leafFor(s)
		}
		for _, mode := range GroupModes {
			for _, q := range groupQuantities(mode, len(states)) {
				for _, not := range []bool{false, true} {
					for _, guard := range guards {
						g := &Group{Mode: mode, Quantity: q, Not: not, Children: children}
						if guard != nil {
							g.OnlyWhen = Ref{Node: leafFor(*guard)}
						}
						want := expectedGroupState(mode, q, not, guard, states)
						desc := fmt.Sprintf("%v q=%v not=%v guard=%v states=%v", mode, q, not, guardName(guard), states)
						full := evalAll(t, g, h, nil)
						c.Equal(want, full.State, desc)
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

// TestFailuresReachCounts checks that a where that ends Failed for any member fails its count whatever the
// quantity or Not, since the count can't be known, and that a where with no member to fail on doesn't.
func TestFailuresReachCounts(t *testing.T) {
	c := check.New(t)
	h := newHost()
	self := newItem("self", nil).with("kids", newItem("kid", nil)).with("none")
	failing := Ref{Node: &Script{Script: "failed"}}
	for _, tc := range []struct {
		node  Node
		state State
	}{
		{&Count{Of: "kids", Where: failing}, Failed},
		{&Count{Of: "kids", Where: failing, Not: true}, Failed},
		{&Count{Of: "kids", Where: failing, Quantity: &criteria.Number{}}, Failed},
		{&Count{Of: "kids", Where: Ref{Node: &Check{Field: "level"}}}, Failed},
		{&Count{Of: "none", Where: failing, Not: true}, Met},
		{&Group{Mode: GroupAny, Children: Nodes{&Count{Of: "kids", Where: failing}, &Const{Value: true}}}, Met},
	} {
		r := evalAll(t, tc.node, h, self)
		c.Equal(tc.state, r.State, encode(t, tc.node))
	}
}

func guardName(guard *State) string {
	if guard == nil {
		return "none"
	}
	return guard.String()
}
