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
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
)

// Divergence classes the engine design accepts (engine-design.md section 10).
const (
	divergenceTLCase       = "the tech level rule folds case"
	divergenceEmptyCollege = "a college count leaves out empty colleges"
	divergenceListCompare  = "spell college and tag criteria use list semantics"
)

// diffTally counts the outcome of a differential run.
type diffTally struct {
	agree     int
	diverged  map[string]int
	states    [4]int // legacy results by condition.State
	penalties int
}

func (d *diffTally) String() string {
	return fmt.Sprintf("agree %d (met %d, unmet %d, skipped %d, failed %d, penalties %d), accepted divergences %v",
		d.agree, d.states[condition.Met], d.states[condition.Unmet], d.states[condition.Skipped],
		d.states[condition.Failed], d.penalties, d.diverged)
}

// compare evaluates list for owner both ways on e, failing the test for a divergence that isn't an accepted one.
func (d *diffTally) compare(t *testing.T, h *conditionHost, e *Entity, list *PrereqList, owner any, label string) {
	t.Helper()
	want, got := legacyVerdict(e, list, owner), engineVerdict(h, list, owner)
	d.states[want.state]++
	if want.penalty {
		d.penalties++
	}
	if want == got {
		d.agree++
		return
	}
	class := acceptedDivergence(e, list, owner)
	if class == "" {
		t.Errorf("%s: legacy %v, engine %v for %s", label, want, got, legacyJSONString(t, list))
		class = "unexplained"
	} else {
		t.Logf("accepted (%s): %s: legacy %v, engine %v", class, label, want, got)
	}
	if d.diverged == nil {
		d.diverged = make(map[string]int)
	}
	d.diverged[class]++
}

// acceptedDivergence returns the accepted divergence class that can explain a difference between legacy and engine
// results for the list, or "" when none can.
func acceptedDivergence(e *Entity, list *PrereqList, owner any) string {
	var spells []*Spell
	Traverse(func(s *Spell) bool { spells = append(spells, s); return false }, false, false, e.Spells...)
	ownerTL := ownerTechLevel(owner)
	var class string
	var walk func(p *PrereqList)
	walk = func(p *PrereqList) {
		for _, one := range p.Prereqs {
			switch x := one.(type) {
			case *PrereqList:
				walk(x)
			case *SpellPrereq:
				q := x.QualifierCriteria
				switch {
				case (x.SubType == spellcmp.Tag || x.SubType == spellcmp.College) &&
					(q.Compare == criteria.AnyText || q.Compare.IsNotType() || q.Qualifier == "" ||
						strings.Contains(q.Qualifier, ",")):
					class = divergenceListCompare
				case x.SubType == spellcmp.CollegeCount && slices.ContainsFunc(spells, func(s *Spell) bool {
					return slices.Contains(s.CollegeWithReplacements(), "")
				}):
					class = divergenceEmptyCollege
				case ownerTL != nil && slices.ContainsFunc(spells, func(s *Spell) bool {
					return s.TechLevel != nil && *s.TechLevel != *ownerTL && strings.EqualFold(*s.TechLevel, *ownerTL)
				}):
					class = divergenceTLCase
				}
			case *SkillPrereq:
				if _, isSkill := owner.(*Skill); isSkill && ownerTL != nil {
					Traverse(func(s *Skill) bool {
						if s.TechLevel != nil && *s.TechLevel != *ownerTL && strings.EqualFold(*s.TechLevel, *ownerTL) {
							class = divergenceTLCase
						}
						return false
					}, false, false, e.Skills...)
				}
			}
		}
	}
	walk(list)
	return class
}

func ownerTechLevel(owner any) *string {
	switch x := owner.(type) {
	case *Skill:
		return x.TechLevel
	case *Spell:
		return x.TechLevel
	default:
		return nil
	}
}

// entityPrereqOwners returns every item on the entity with prerequisites.
func entityPrereqOwners(e *Entity) []prereqOwnerItem {
	var list []prereqOwnerItem
	add := func(owner any, name string, p *PrereqList) {
		if p != nil {
			list = append(list, prereqOwnerItem{owner: owner, name: name, list: p})
		}
	}
	Traverse(func(t *Trait) bool { add(t, t.Name, t.Prereq); return false }, false, false, e.Traits...)
	Traverse(func(s *Skill) bool { add(s, s.Name, s.Prereq); return false }, false, false, e.Skills...)
	Traverse(func(s *Spell) bool { add(s, s.Name, s.Prereq); return false }, false, false, e.Spells...)
	equipment := func(eqp *Equipment) bool { add(eqp, eqp.Name, eqp.Prereq); return false }
	Traverse(equipment, false, false, e.CarriedEquipment...)
	Traverse(equipment, false, false, e.OtherEquipment...)
	return list
}

type prereqOwnerItem struct {
	owner any
	name  string
	list  *PrereqList
}

// TestConditionDifferentialTestdata compares legacy and engine results for the items of the checked-in sheets, and for
// the constructed lists placed on them with each of their items as the owner.
func TestConditionDifferentialTestdata(t *testing.T) {
	var d diffTally
	lists := conditionTestLists()
	for _, name := range []string{"issue767.gcs", "saved_calc.gcs"} {
		e, err := NewEntityFromFile(os.DirFS("testdata"), name)
		if err != nil {
			t.Fatal(err)
		}
		h := newConditionHost(e)
		for _, item := range entityPrereqOwners(e) {
			d.compare(t, h, e, item.list, item.owner, name+": "+item.name)
		}
		var owners []any
		Traverse(func(s *Skill) bool { owners = append(owners, s); return false }, false, false, e.Skills...)
		Traverse(func(s *Spell) bool { owners = append(owners, s); return false }, false, false, e.Spells...)
		Traverse(func(t *Trait) bool { owners = append(owners, t); return false }, false, false, e.Traits...)
		for _, owner := range owners {
			for _, list := range lists {
				d.compare(t, h, e, list, owner, name)
			}
		}
	}
	t.Log(d.String())
}
