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
	"encoding/json/v2"
	"strconv"
	"sync"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/toolbox/v2/check"
)

// matcherRows returns n rows for a check-only filter.
func matcherRows(n int) []*fakeItem {
	rows := make([]*fakeItem, n)
	for i := range rows {
		name := "Row " + strconv.Itoa(i)
		if i%3 == 0 {
			name = "Fire " + strconv.Itoa(i)
		}
		rows[i] = newItem(name, map[string]Value{
			"level": numberValue(int64(i % 5)),
			"tags":  listValue("Spell", "Tag "+strconv.Itoa(i%7)),
		})
	}
	return rows
}

// checkOnlyTree is a filter of checks only, as list filters are.
func checkOnlyTree() Node {
	return &Group{Children: Nodes{
		checkText("name", criteria.StartsWithText, "fire"),
		&Check{Field: "level", Number: numberCrit(criteria.AtLeastNumber, 2)},
		&Group{Mode: GroupAny, Children: Nodes{
			checkText("tags", criteria.IsText, "Tag 3"),
			checkText("tags", criteria.ContainsText, "tag 1"),
		}},
	}}
}

// TestConcurrentEvaluation runs Evaluate and Matchers over one tree from several goroutines at once. It is listed in
// the TestRace wrapper.
func TestConcurrentEvaluation(t *testing.T) {
	c := check.New(t)
	h := &resolvingHost{fakeHost: newHost()}
	rows := matcherRows(300)
	tree := checkOnlyTree()
	big := &Group{Mode: GroupAny, Children: Nodes{tree, &Script{Script: "met"}}}
	want := make([]bool, len(rows))
	for i, row := range rows {
		want[i] = EvaluateFor(tree, h, row, row, Options{}).State.Passes()
	}
	var wg sync.WaitGroup
	results := make([][]bool, 8)
	for g := range results {
		wg.Go(func() {
			m := NewMatcher(tree, h, Options{FailedIsUnmet: true})
			got := make([]bool, len(rows))
			for i, row := range rows {
				got[i] = m.Matches(row)
				Evaluate(big, h, row, Options{Mode: Collect})
			}
			results[g] = got
		})
	}
	wg.Wait()
	for _, got := range results {
		c.Equal(want, got)
	}
}

// BenchmarkMatcher matches 10,000 rows against check-only trees and reports the allocations per row. The first tree
// uses comparisons whose criteria allocate on their own (case folding and comma splitting); the second uses only
// comparisons that don't, so every allocation it reports would be the engine's.
func BenchmarkMatcher(b *testing.B) {
	h := &resolvingHost{fakeHost: newHost()}
	rows := matcherRows(10_000)
	plain := &Group{Children: Nodes{
		&Check{Field: "level", Number: numberCrit(criteria.AtLeastNumber, 2)},
		&Group{Mode: GroupAny, Children: Nodes{
			checkText("name", criteria.IsText, "fire 3"),
			&Check{Field: "level", Number: numberCrit(criteria.EqualsNumber, 4)},
		}},
	}}
	for _, tc := range []struct {
		name string
		tree Node
	}{
		{"criteria", checkOnlyTree()},
		{"plain", plain},
	} {
		b.Run(tc.name, func(b *testing.B) {
			m := NewMatcher(tc.tree, h, Options{FailedIsUnmet: true})
			matched := 0
			b.ReportAllocs()
			for b.Loop() {
				for _, row := range rows {
					if m.Matches(row) {
						matched++
					}
				}
			}
			b.ReportMetric(testing.AllocsPerRun(1, func() {
				for _, row := range rows {
					m.Matches(row)
				}
			})/float64(len(rows)), "allocs/row")
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(rows)), "ns/row")
			if matched == 0 {
				b.Fatal("nothing matched")
			}
		})
	}
}

// TestConcurrentDecode decodes the wire files from several goroutines at once, which share the pooled streams. It is
// listed in the TestRace wrapper.
func TestConcurrentDecode(t *testing.T) {
	c := check.New(t)
	files := wireFiles(t)
	want := make(map[string]string, len(files))
	for name, data := range files {
		want[name] = string(encodeCompact(t, decodeRefBytes(t, data)))
	}
	var wg sync.WaitGroup
	got := make([]map[string]string, 8)
	for g := range got {
		wg.Go(func() {
			out := make(map[string]string, len(files))
			for range 20 {
				for name, data := range files {
					var r Ref
					if json.Unmarshal(data, &r) == nil {
						if enc, err := json.Marshal(r); err == nil {
							out[name] = string(enc)
						}
					}
				}
			}
			got[g] = out
		})
	}
	wg.Wait()
	for _, out := range got {
		c.Equal(want, out)
	}
}
