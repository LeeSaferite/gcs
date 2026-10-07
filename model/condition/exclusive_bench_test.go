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
)

// syntheticSheet returns 300 spells over 25 colleges, about 15% of them in two colleges, with Counterspell among them.
func syntheticSheet() (h *fakeHost, self *fakeItem) {
	r := rand.New(rand.NewPCG(9, 10)) //nolint:gosec // Not for security.
	colleges := []string{"Air", "Earth", "Fire", "Water"}
	for i := len(colleges); i < 25; i++ {
		colleges = append(colleges, fmt.Sprintf("College %d", i))
	}
	spells := make([]*fakeItem, 300)
	for i := range spells {
		list := []string{colleges[r.IntN(len(colleges))]}
		if r.IntN(100) < 15 {
			if other := colleges[r.IntN(len(colleges))]; other != list[0] {
				list = append(list, other)
			}
		}
		spells[i] = spell(fmt.Sprintf("Spell %d", i), list...)
	}
	spells[17] = spell("Counterspell", "Meta")
	return spellBook(spells...)
}

// BenchmarkSolver evaluates library shapes and adversarial pools on a 300-spell sheet, reporting whether the state
// budget ran out.
func BenchmarkSolver(b *testing.B) {
	h, self := syntheticSheet()
	inCollegeAtLeast := func(college string, n int64) Node { return spellsWhere(inCollege(college), atLeast(n)) }
	choices := &Group{Mode: GroupAtLeast, N: 5}
	for i := range 12 {
		choices.Children = append(choices.Children, inCollegeAtLeast(fmt.Sprintf("College %d", 4+i), 13))
	}
	scarce := exclusive()
	for range 16 {
		scarce.Children = append(scarce.Children, &Group{Mode: GroupAny, Children: Nodes{
			spellsWhere(checkText("name", criteria.IsText, "Counterspell"), nil),
			spellsWhere(checkText("name", criteria.IsText, "Spell 3"), nil),
		}})
	}
	for _, tc := range []struct {
		name string
		tree Node
	}{
		{"dispel magic", exclusive(spellsWhere(checkText("name", criteria.IsText, "Counterspell"), nil),
			spellsWhere(nil, atLeast(12)))},
		{"force dome", exclusive(inCollegeAtLeast("Air", 2), inCollegeAtLeast("Earth", 2), inCollegeAtLeast("Fire", 2),
			inCollegeAtLeast("Water", 2))},
		{"accelerate time", &Count{
			Of: "spells", GroupBy: "college", PerGroup: atLeast(2),
			Quantity: atLeast(10), Exclusive: true,
		}},
		{"adversarial values", exclusive(&Count{
			Of: "spells", GroupBy: "college",
			PerGroup: atLeast(13), Quantity: atLeast(25),
		})},
		{"adversarial choices", exclusive(choices)},
		{"adversarial combinations", scarce},
	} {
		for _, mode := range []Mode{YesNo, Collect} {
			b.Run(fmt.Sprintf("%s/%v", tc.name, map[Mode]string{YesNo: "yesno", Collect: "collect"}[mode]), func(b *testing.B) {
				var r Result
				b.ReportAllocs()
				for b.Loop() {
					r = Evaluate(tc.tree, h, self, Options{Mode: mode})
				}
				budgetHit := 0.0
				if r.State == Failed {
					budgetHit = 1
				}
				b.ReportMetric(budgetHit, "budget-hit")
				b.ReportMetric(float64(r.State), "state")
			})
		}
	}
}
