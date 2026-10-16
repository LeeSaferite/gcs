// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package gurps

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/xbytes"
)

// withLegacyPrereqs runs fn with recalculation evaluating prerequisites the legacy way. Tests using it must not call
// t.Parallel.
func withLegacyPrereqs(fn func()) {
	saved := prereqsSatisfied
	prereqsSatisfied = func(p *PrereqList, h *conditionHost, exclude any, buffer *xbytes.InsertBuffer, prefix string,
		hasEquipmentPenalty *bool,
	) bool {
		return p.legacySatisfied(h.entity, exclude, buffer, prefix, hasEquipmentPenalty)
	}
	defer func() { prereqsSatisfied = saved }()
	fn()
}

// recalcSnapshot returns, one line per item, everything recalculation decides from prerequisites: the unsatisfied
// reasons and the trait status shown for them, whether each trait is enabled, disabled by its prerequisites or caught
// in a contradiction, and each skill's and spell's equipment penalty and level.
func recalcSnapshot(e *Entity) []string {
	var lines []string
	Traverse(func(t *Trait) bool {
		reason, contradiction := t.PrereqStatus()
		lines = append(lines, fmt.Sprintf("trait %q %q %q enabled=%v disabled=%v contradicted=%v level=%v", t.Name,
			reason, contradiction, t.Enabled(), t.DisabledByPrereqs(), t.ContradictedPrereqs(), t.CurrentLevel()))
		return false
	}, false, false, e.Traits...)
	Traverse(func(s *Skill) bool {
		lines = append(lines, fmt.Sprintf("skill %q %q penalty=%v level=%v", s.Name, s.UnsatisfiedReason,
			s.takesEquipmentPenalty, s.LevelData.Level))
		return false
	}, false, false, e.Skills...)
	Traverse(func(s *Spell) bool {
		lines = append(lines, fmt.Sprintf("spell %q %q penalty=%v level=%v", s.Name, s.UnsatisfiedReason,
			s.takesEquipmentPenalty, s.LevelData.Level))
		return false
	}, false, false, e.Spells...)
	equipment := func(eqp *Equipment) bool {
		lines = append(lines, fmt.Sprintf("equipment %q %q", eqp.Name, eqp.UnsatisfiedReason))
		return false
	}
	Traverse(equipment, false, false, e.CarriedEquipment...)
	Traverse(equipment, false, false, e.OtherEquipment...)
	return lines
}

// recalcBothWays recalculates the entity with the legacy evaluation and then with the engine, with the sheet's own
// trait enforcement and with it forced on, and returns the snapshots of each, in that order.
func recalcBothWays(e *Entity) (legacy, engine []string) {
	enforce := e.SheetSettings.EnforceTraitPrereqs
	for _, on := range []bool{enforce, true} {
		e.SheetSettings.EnforceTraitPrereqs = on
		withLegacyPrereqs(e.Recalculate)
		legacy = append(legacy, recalcSnapshot(e)...)
		e.Recalculate()
		engine = append(engine, recalcSnapshot(e)...)
	}
	e.SheetSettings.EnforceTraitPrereqs = enforce
	return legacy, engine
}

// recalcTally counts the item states two recalculations agreed on, and how many of them were unsatisfied or disabled.
type recalcTally struct {
	agree       int
	unsatisfied int
	disabled    int
}

func (r *recalcTally) String() string {
	return fmt.Sprintf("%d item states agree (%d with unsatisfied prerequisites, %d traits disabled by them)", r.agree,
		r.unsatisfied, r.disabled)
}

// compare fails the test for every line that differs.
func (r *recalcTally) compare(t *testing.T, label string, legacy, engine []string) {
	t.Helper()
	if len(legacy) != len(engine) {
		t.Errorf("%s: %d legacy lines, %d engine lines", label, len(legacy), len(engine))
		return
	}
	for i := range legacy {
		if legacy[i] != engine[i] {
			t.Errorf("%s:\nlegacy %s\nengine %s", label, legacy[i], engine[i])
			continue
		}
		r.agree++
		if strings.Contains(legacy[i], "Prerequisites have not been met") {
			r.unsatisfied++
		}
		if strings.Contains(legacy[i], "disabled=true") {
			r.disabled++
		}
	}
}

// TestConditionRecalc recalculates the checked-in sheets and the constructed entity both ways and compares what each
// decided for every item.
func TestConditionRecalc(t *testing.T) {
	var r recalcTally
	for _, name := range []string{"issue767.gcs", "saved_calc.gcs"} {
		e, err := NewEntityFromFile(os.DirFS("testdata"), name)
		if err != nil {
			t.Fatal(err)
		}
		legacy, engine := recalcBothWays(e)
		r.compare(t, name, legacy, engine)
	}
	e, _ := newConditionTestEntity("3")
	// Give the constructed items the constructed lists, so recalculation has something to decide.
	lists := conditionTestLists()
	i := 0
	next := func() *PrereqList { i++; return lists[i%len(lists)].CloneAsPrereqList(nil) }
	Traverse(func(x *Trait) bool { x.Prereq = next(); return false }, false, false, e.Traits...)
	Traverse(func(x *Skill) bool { x.Prereq = next(); return false }, false, false, e.Skills...)
	Traverse(func(x *Spell) bool { x.Prereq = next(); return false }, false, false, e.Spells...)
	Traverse(func(x *Equipment) bool { x.Prereq = next(); return false }, false, false, e.CarriedEquipment...)
	legacy, engine := recalcBothWays(e)
	r.compare(t, "constructed", legacy, engine)
	// Two traits that each require the other's absence contradict one another once the sheet enforces them.
	e = NewEntity()
	for _, pair := range [][2]string{{"A", "B"}, {"B", "A"}} {
		trait := NewTrait(e, nil, false)
		trait.Name, trait.Prereq = pair[0], listOf(traitPrereq(pair[1], false))
		e.Traits = append(e.Traits, trait)
	}
	legacy, engine = recalcBothWays(e)
	r.compare(t, "contradiction", legacy, engine)
	t.Log(r.String())
	if r.unsatisfied == 0 || r.disabled == 0 || !slices.ContainsFunc(engine, func(line string) bool {
		return strings.Contains(line, "contradicted=true")
	}) {
		t.Error("the comparison should cover unsatisfied prerequisites, traits disabled by them and a contradiction")
	}
}

// TestConditionRecalcLibrary recalculates every Master Library sheet both ways and compares what each decided for
// every item.
func TestConditionRecalcLibrary(t *testing.T) {
	root := masterLibrary(t)
	fsys := os.DirFS(root)
	var names []string
	if err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Ext(p) == SheetExt {
			names = append(names, p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	var r recalcTally
	for _, name := range names {
		e, err := NewEntityFromFile(fsys, name)
		if err != nil {
			t.Fatal(err)
		}
		legacy, engine := recalcBothWays(e)
		r.compare(t, name, legacy, engine)
	}
	t.Logf("%d sheets: %s", len(names), r.String())
}

// BenchmarkConditionRecalc recalculates every Master Library sheet with the legacy evaluation and with the engine, and
// logs the sheets whose recalculation the engine slows the most, and the Druid sheet that used to be the slowest.
func BenchmarkConditionRecalc(b *testing.B) {
	root := masterLibrary(b)
	fsys := os.DirFS(root)
	var names []string
	var sheets []*Entity
	if err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Ext(p) == SheetExt {
			e, loadErr := NewEntityFromFile(fsys, p)
			if loadErr != nil {
				return loadErr
			}
			names = append(names, p)
			sheets = append(sheets, e)
		}
		return err
	}); err != nil {
		b.Fatal(err)
	}
	recalcAll := func() {
		for _, e := range sheets {
			e.Recalculate()
		}
	}
	b.Run("legacy", func(b *testing.B) {
		for b.Loop() {
			withLegacyPrereqs(recalcAll)
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(sheets)), "ns/sheet")
	})
	b.Run("engine", func(b *testing.B) {
		for b.Loop() {
			recalcAll()
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(sheets)), "ns/sheet")
	})
	type ratio struct {
		name           string
		legacy, engine time.Duration
	}
	ratios := make([]ratio, len(sheets))
	for i, e := range sheets {
		// The fastest of several interleaved rounds, so a collection or another process doesn't decide the ratio.
		timeIt := func(fn func()) time.Duration {
			start := time.Now()
			for range 10 {
				fn()
			}
			return time.Since(start) / 10
		}
		one := ratio{name: names[i], legacy: time.Hour, engine: time.Hour}
		for range 5 {
			one.legacy = min(one.legacy, timeIt(func() { withLegacyPrereqs(e.Recalculate) }))
			one.engine = min(one.engine, timeIt(e.Recalculate))
		}
		ratios[i] = one
	}
	slices.SortFunc(ratios, func(x, y ratio) int {
		return cmp.Compare(float64(y.engine)/float64(y.legacy), float64(x.engine)/float64(x.legacy))
	})
	for _, one := range ratios {
		if strings.Contains(one.name, "Master of Elements - Druid") {
			b.Logf("Druid: %.2fx %v -> %v", float64(one.engine)/float64(one.legacy), one.legacy, one.engine)
		}
	}
	for _, one := range ratios[:min(8, len(ratios))] {
		b.Logf("%.2fx %v -> %v %s", float64(one.engine)/float64(one.legacy), one.legacy, one.engine, one.name)
	}
}
