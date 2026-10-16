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
	"slices"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/eqcontainer"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
)

// newConditionTestEntity returns an entity at the tech level holding a spread of items for the prerequisites to find,
// and the owners to evaluate them for: every item on the entity plus detached ones covering the owner-type rules.
func newConditionTestEntity(tl string) (e *Entity, owners []any) {
	e = NewEntity()
	e.Profile.TechLevel = tl
	magery := NewTrait(e, nil, false)
	magery.Name, magery.CanLevel, magery.Levels = "Magery", true, fxp.Two
	magery.LocalNotes, magery.Tags = "one college (Air)", []string{"Mental"}
	mod := NewTraitModifier(e, nil, false)
	mod.Name, mod.LocalNotes = "Air", "Air only"
	magery.Modifiers = []*TraitModifier{mod}
	luck := NewTrait(e, nil, false)
	luck.Name, luck.Disabled = "Luck", true
	group := NewTrait(e, nil, true)
	group.Name = "Advantages"
	fit := NewTrait(e, group, false)
	fit.Name = "Fit"
	group.Children = []*Trait{fit}
	e.Traits = []*Trait{magery, luck, group}

	addTestSkill(e, "Guns", "Pistol", "3", fxp.One)
	addTestSkill(e, "Guns", "Rifle", "", fxp.Two).TechLevel = new("")
	addTestSkill(e, "Broadsword", "", "", fxp.Four)
	addTestSkill(e, "Zero", "", "", 0)
	technique := NewTechnique(e, nil, "Broadsword")
	technique.Name = "Feint"
	e.Skills = append(e.Skills, technique)

	spell := func(name, powerSource string, tl *string, colleges ...string) *Spell {
		s := addTestSpell(e, name, fxp.One)
		s.College, s.PowerSource, s.TechLevel = colleges, powerSource, tl
		return s
	}
	spell("Fireball", "Arcane", nil, "Fire")
	spell("Shape Fire", "arcane", nil, "Fire").Tags = []string{"Fire Spell"}
	spell("Ignite Fire", "Arcane", new("3"), "Fire", "Air")
	spell("Purify Air", "Divine", new(""), "Air")
	spell("Zero", "Arcane", nil, "Fire").Points = 0
	addSpellNamePrereq(spell("Create Fire", "Arcane", nil, "Fire"), "Ignite Fire")
	suspend := spell("Suspend Magic", "Arcane", nil, "Meta")
	suspend.Prereq = listOf(spellPrereq(spellcmp.Name, criteria.Text{}, numberCriteria(criteria.AtLeastNumber,
		fxp.Two), true))

	equipment := func(parent *Equipment, container bool, name string, equipped bool, qty fxp.Int,
		weight string,
	) *Equipment {
		eqp := NewEquipment(e, parent, container)
		eqp.Name, eqp.Equipped, eqp.Quantity, eqp.BaseWeight = name, equipped, qty, weight
		if parent != nil {
			parent.Children = append(parent.Children, eqp)
		} else {
			e.CarriedEquipment = append(e.CarriedEquipment, eqp)
		}
		return eqp
	}
	equipment(nil, false, "Sword", true, fxp.One, "3 lb").Tags = []string{"Weapon"}
	pack := equipment(nil, true, "Backpack", true, fxp.One, "2 lb")
	equipment(pack, false, "Rope", true, fxp.Two, "1 lb")
	pouch := equipment(pack, true, "Pouch", true, fxp.One, "")
	pouch.ContainerType = eqcontainer.Group
	equipment(pouch, false, "Torch", true, fxp.Three, "1 lb")
	equipment(nil, false, "Lockpicks", false, fxp.One, "")
	equipment(nil, false, "Ghost", true, 0, "")
	e.Recalculate()

	Traverse(func(t *Trait) bool { owners = append(owners, t); return false }, false, false, e.Traits...)
	Traverse(func(s *Skill) bool { owners = append(owners, s); return false }, false, false, e.Skills...)
	Traverse(func(s *Spell) bool { owners = append(owners, s); return false }, false, false, e.Spells...)
	Traverse(func(eqp *Equipment) bool { owners = append(owners, eqp); return false }, false, false,
		e.CarriedEquipment...)
	for _, one := range []*string{nil, new(""), new("3"), new("4")} {
		skill := NewSkill(nil, nil, false)
		skill.TechLevel = one
		owners = append(owners, skill)
		for _, source := range []string{"Arcane", "Divine", ""} {
			s := NewSpell(nil, nil, false)
			s.Name, s.TechLevel, s.PowerSource = "Ignite Fire", one, source
			owners = append(owners, s)
		}
	}
	box := NewEquipment(nil, nil, true)
	nested := NewEquipment(nil, box, false)
	nested.Quantity, nested.BaseWeight = fxp.Three, "4 lb"
	box.Children = []*Equipment{nested}
	owners = append(owners, NewTrait(nil, nil, false), NewEquipment(nil, nil, false), box, NewEquipment(nil, nil, true))
	return e, owners
}

// conditionTestLists returns the prerequisite lists the constructed differential evaluates: the unit cases plus lists
// aimed at the constructed entity's items.
func conditionTestLists() []*PrereqList {
	var lists []*PrereqList
	for _, tc := range prereqCases() {
		if tc.name != "nil child" { // legacy can't evaluate a nil prerequisite
			lists = append(lists, tc.list)
		}
	}
	atLeast := func(n int) criteria.Number { return numberCriteria(criteria.AtLeastNumber, fxp.FromInteger(n)) }
	magery := traitPrereq("Magery", true)
	magery.LevelCriteria = atLeast(2)
	magery.NotesCriteria = textCriteria(criteria.ContainsText, "Air only")
	tooHigh := traitPrereq("magery", true)
	tooHigh.LevelCriteria = atLeast(3)
	rifle := skillPrereq("guns", true)
	rifle.SpecializationCriteria = textCriteria(criteria.IsText, "Rifle")
	pistol := skillPrereq("Guns", true)
	pistol.SpecializationCriteria = textCriteria(criteria.IsText, "Pistol")
	pistol.LevelCriteria = atLeast(8)
	same := spellPrereq(spellcmp.College, textCriteria(criteria.IsText, "Fire"), atLeast(2), true)
	same.SamePowerSource = true
	lockpicks := equippedPrereq(textCriteria(criteria.IsText, "Lockpicks"), criteria.Text{})
	sword := equippedPrereq(criteria.Text{}, textCriteria(criteria.IsText, "weapon"))
	whenTL := func(compare criteria.NumericComparison, tl int, prereqs ...Prereq) *PrereqList {
		list := listOf(prereqs...)
		list.WhenTL = numberCriteria(compare, fxp.FromInteger(tl))
		return list
	}
	unknown := NewUnknownPrereq("future", []byte(`{"type":"future"}`))
	lists = append(lists,
		listOf(magery), listOf(tooHigh), listOf(traitPrereq("Luck", true)), listOf(traitPrereq("Fit", true)),
		listOf(traitPrereq("Advantages", false)), listOf(rifle), listOf(pistol), listOf(skillPrereq("Zero", true)),
		listOf(skillPrereq("Feint", true)),
		listOf(spellPrereq(spellcmp.Name, textCriteria(criteria.IsText, "Ignite Fire"), atLeast(1), true)),
		listOf(spellPrereq(spellcmp.Name, textCriteria(criteria.StartsWithText, "Purify"), atLeast(1), true)),
		listOf(spellPrereq(spellcmp.College, textCriteria(criteria.ContainsText, "fire"), atLeast(3), true)),
		listOf(spellPrereq(spellcmp.CollegeCount, criteria.Text{}, atLeast(3), true)),
		listOf(spellPrereq(spellcmp.Tag, textCriteria(criteria.ContainsText, "Fire"), atLeast(1), true)),
		listOf(spellPrereq(spellcmp.Any, criteria.Text{}, atLeast(5), true)),
		listOf(spellPrereq(spellcmp.Name, textCriteria(criteria.IsText, "Zero"), atLeast(1), false)),
		listOf(spellPrereq(spellcmp.Name, textCriteria(criteria.IsText, "Create Fire"), atLeast(1), true)),
		listOf(spellPrereq(spellcmp.Name, textCriteria(criteria.IsText, "Suspend Magic"), atLeast(1), true)),
		listOf(same),
		listOf(attributePrereq("zz", "", numberCriteria(criteria.AtMostNumber, 0), true)),
		listOf(attributePrereq("dx", "iq", atLeast(20), true)),
		listOf(lockpicks), listOf(sword), anyOf(lockpicks, magery), listOf(anyOf(lockpicks, tooHigh)),
		listOf(listOf(lockpicks), magery), anyOf(listOf(lockpicks, sword), tooHigh),
		listOf(whenTL(criteria.AtMostNumber, 2, lockpicks)), listOf(equippedPrereq(textCriteria(criteria.IsText,
			"Ghost"), criteria.Text{})), listOf(equippedPrereq(textCriteria(criteria.IsText, "Torch"), criteria.Text{})),
		listOf(whenTL(criteria.AtLeastNumber, 3, magery), tooHigh), anyOf(whenTL(criteria.EqualsNumber, 0, tooHigh)),
		listOf(whenTL(criteria.NotEqualsNumber, 3, tooHigh, magery)),
		listOf(scriptPrereq("", "true")), listOf(scriptPrereq("", "false")), listOf(scriptPrereq("A", `"why"`)),
		listOf(scriptPrereq("", `throw new Error("boom")`), tooHigh), anyOf(scriptPrereq("", `throw "x"`), magery),
		anyOf(scriptPrereq("", `throw "x"`), tooHigh), listOf(scriptPrereq("", "self.name == 'Magery'")),
		listOf(unknown, magery), anyOf(unknown, magery), anyOf(unknown, tooHigh),
		listOf(weightPrereq(criteria.AtMostNumber, 5, true)), listOf(weightPrereq(criteria.AtLeastNumber, 12, true)),
		listOf(quantityPrereq(criteria.EqualsNumber, fxp.Three, true)),
		listOf(quantityPrereq(criteria.AtMostNumber, fxp.Four, false)),
	)
	return lists
}

// TestConditionDifferentialConstructed compares legacy evaluation with the engine's for every list in
// conditionTestLists, every owner newConditionTestEntity makes, and several sheet tech levels.
func TestConditionDifferentialConstructed(t *testing.T) {
	lists := conditionTestLists()
	var d diffTally
	for _, tl := range []string{"", "3", "TL5", "-1"} {
		e, owners := newConditionTestEntity(tl)
		h := newConditionHost(e)
		for _, owner := range owners {
			for _, list := range lists {
				d.compare(t, h, e, list, owner, "TL "+tl)
			}
		}
	}
	t.Log(d.String())
	if slices.Contains(d.states[:], 0) || d.penalties == 0 {
		t.Errorf("the lists should reach every state and the penalty: %s", d.String())
	}
}

// TestConditionAcceptedDivergences pins the divergences from legacy that the engine design accepts.
func TestConditionAcceptedDivergences(t *testing.T) {
	e := NewEntity()
	spell := addTestSpell(e, "Fireball", fxp.One)
	spell.College, spell.TechLevel = CollegeList{"", "Fire"}, new("3a")
	h := newConditionHost(e)
	owner := NewSpell(nil, nil, false)
	owner.TechLevel = new("3A")
	for _, tc := range []struct {
		name string
		list *PrereqList
	}{
		{"the tech level rule folds case", listOf(spellPrereq(spellcmp.Name, textCriteria(criteria.IsText,
			"Fireball"), numberCriteria(criteria.AtLeastNumber, fxp.One), true))},
		{"a college count leaves out empty colleges", listOf(spellPrereq(spellcmp.CollegeCount, criteria.Text{},
			numberCriteria(criteria.AtLeastNumber, fxp.Two), true))},
		{"a negated college compare uses list semantics", listOf(spellPrereq(spellcmp.College,
			textCriteria(criteria.IsNotText, "Fire"), numberCriteria(criteria.AtLeastNumber, fxp.One), true))},
	} {
		var self any = owner
		if tc.name != "the tech level rule folds case" {
			self = NewSpell(nil, nil, false)
		}
		if want, got := legacyVerdict(e, tc.list, self), engineVerdict(h, tc.list, self); want.satisfied == got.satisfied {
			t.Errorf("%s: expected the legacy %v and the engine %v to differ", tc.name, want, got)
		}
	}
}

// TestConditionSpellGuard checks the #737 guard through the host: a spell that requires the owner by name, any name
// included, is left out of the owner's spells, while other owners still see it.
func TestConditionSpellGuard(t *testing.T) {
	e, _ := newConditionTestEntity("3")
	h := newConditionHost(e)
	var ignite, suspend *Spell
	for _, s := range e.Spells {
		switch s.Name {
		case "Ignite Fire":
			ignite = s
		case "Suspend Magic":
			suspend = s
		}
	}
	names := func(self condition.Item) map[string]bool {
		items, ok := h.Collection(conditionSpells, self, self)
		if !ok {
			t.Fatal("the host should have the spells collection")
		}
		m := make(map[string]bool)
		for _, one := range items {
			if s, isSpell := one.(*Spell); isSpell {
				m[s.Name] = true
			}
		}
		return m
	}
	forIgnite, forTrait := names(ignite), names(NewTrait(nil, nil, false))
	if forIgnite["Create Fire"] || forIgnite["Suspend Magic"] || forIgnite["Ignite Fire"] || forIgnite["Zero"] {
		t.Errorf("guard, self or zero-point spell counted for Ignite Fire: %v", forIgnite)
	}
	if !forTrait["Create Fire"] || !forTrait["Suspend Magic"] || !forTrait["Ignite Fire"] {
		t.Errorf("a trait owner should see every spell with points: %v", forTrait)
	}
	if names(suspend)["Suspend Magic"] {
		t.Error("a spell should not count itself")
	}
}
