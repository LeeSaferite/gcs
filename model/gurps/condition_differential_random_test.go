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
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
)

// newCriteriaTestEntity returns the constructed entity of newConditionTestEntity, with what the newer prerequisite
// criteria read added: trait points, tags and modifiers (disabled ones, and those of a container, included), skill
// tags, equipment modifiers, equipment that is carried but not equipped, and spells that require "Ignite Fire" by name
// through negated lists.
func newCriteriaTestEntity(tl string) (e *Entity, owners []any) {
	e, owners = newConditionTestEntity(tl)
	traitMod := func(name, notes string, disabled bool, tags ...string) *TraitModifier {
		m := NewTraitModifier(e, nil, false)
		m.Name, m.LocalNotes, m.Disabled, m.Tags = name, notes, disabled, tags
		return m
	}
	masters := NewTrait(e, nil, true)
	masters.Name = "Weapon Masters"
	masters.Modifiers = []*TraitModifier{traitMod("Fine", "", false, "Quality")}
	master := NewTrait(e, masters, false)
	master.Name, master.BasePoints, master.Tags = "Sword Master", fxp.Ten, []string{"Combat", "Physical"}
	master.Modifiers = []*TraitModifier{traitMod("Cheap", "never sharpened", true, "Quality")}
	masters.Children = []*Trait{master}
	e.Traits = append(e.Traits, masters)
	e.Traits[0].Modifiers = append(e.Traits[0].Modifiers, traitMod("Water", "Water only", true))
	Traverse(func(s *Skill) bool {
		if s.Name == "Broadsword" {
			s.Tags = []string{"Combat", "Melee"}
		}
		return false
	}, false, false, e.Skills...)

	eqpMod := func(name, notes string, disabled bool, tags ...string) *EquipmentModifier {
		m := NewEquipmentModifier(e, nil, false)
		m.Name, m.LocalNotes, m.Disabled, m.Tags = name, notes, disabled, tags
		return m
	}
	group := NewEquipmentModifier(e, nil, true)
	group.Name = "Upgrades"
	inner := eqpMod("Balanced", "for throwing", false, "Quality")
	group.Children = []*EquipmentModifier{inner}
	e.CarriedEquipment[0].Modifiers = []*EquipmentModifier{
		eqpMod("Fine", "", false, "Quality"),
		eqpMod("Cheap", "", true, "Quality"), group,
	}
	add := func(parent *Equipment, container bool, name string, equipped bool, qty fxp.Int) *Equipment {
		eqp := NewEquipment(e, parent, container)
		eqp.Name, eqp.Equipped, eqp.Quantity = name, equipped, qty
		if parent != nil {
			parent.Children = append(parent.Children, eqp)
		} else {
			e.CarriedEquipment = append(e.CarriedEquipment, eqp)
		}
		return eqp
	}
	kit := add(nil, false, "Kit", false, fxp.One)
	kit.Modifiers = []*EquipmentModifier{eqpMod("Fine", "", false, "Quality")}
	add(nil, false, "Shield", false, fxp.One).Disabled = true
	empty := add(nil, true, "Empty Sack", false, 0)
	add(empty, false, "Rations", false, fxp.Two)

	requires := func(name string, outer, inner, has bool) {
		s := addTestSpell(e, name, fxp.One)
		s.College, s.PowerSource = CollegeList{"Fire"}, "Arcane"
		p := spellPrereq(spellcmp.Name, textCriteria(criteria.IsText, "Ignite Fire"), numberCriteria(
			criteria.AtLeastNumber, fxp.One,
		), has)
		list := listOf(p)
		list.Not = inner
		s.Prereq = listOf(list)
		s.Prereq.Not = outer
	}
	requires("Flame Jet", true, false, false)
	requires("Smoke", true, false, true)
	requires("Rain of Fire", true, true, true)
	e.Recalculate()

	owners = append(owners, master, masters, kit)
	return e, owners
}

// criteriaTestLeaves returns prerequisites, other than lists, that exercise the newer criteria and Carried Equipment
// against newCriteriaTestEntity, along with some older ones.
func criteriaTestLeaves() []Prereq {
	atLeast := func(n int) criteria.Number { return numberCriteria(criteria.AtLeastNumber, fxp.FromInteger(n)) }
	is := func(s string) criteria.Text { return textCriteria(criteria.IsText, s) }
	isNot := func(s string) criteria.Text { return textCriteria(criteria.IsNotText, s) }
	var leaves []Prereq
	trait := func(name string, has bool, set func(p *TraitPrereq)) {
		p := traitPrereq(name, has)
		if name == "" {
			p.NameCriteria = criteria.Text{}
		}
		set(p)
		leaves = append(leaves, p)
	}
	trait("Sword Master", true, func(p *TraitPrereq) { p.PointsCriteria = atLeast(10) })
	trait("Sword Master", true, func(p *TraitPrereq) { p.PointsCriteria = atLeast(11) })
	trait("", true, func(p *TraitPrereq) { p.TagsCriteria = is("Combat") })
	trait("", false, func(p *TraitPrereq) { p.TagsCriteria = isNot("Mental") })
	trait("", true, func(p *TraitPrereq) { p.TagsCriteria = is("Physical, Mental") })
	trait("", true, func(p *TraitPrereq) { p.ModifierNameCriteria = is("Fine") })
	trait("", true, func(p *TraitPrereq) { p.ModifierNameCriteria = is("Cheap") })
	trait("", true, func(p *TraitPrereq) {
		p.ModifierNameCriteria = is("Air")
		p.ModifierNotesCriteria = textCriteria(criteria.ContainsText, "only")
	})
	trait("", true, func(p *TraitPrereq) {
		p.ModifierNameCriteria = is("Water")
		p.ModifierNotesCriteria = textCriteria(criteria.ContainsText, "only")
	})
	trait("Magery", true, func(p *TraitPrereq) { p.ModifierTagsCriteria = is("Quality") })
	trait("Sword Master", true, func(p *TraitPrereq) { p.ModifierTagsCriteria = isNot("Quality") })

	skill := func(name string, has bool, set func(p *SkillPrereq)) {
		p := skillPrereq(name, has)
		set(p)
		leaves = append(leaves, p)
	}
	skill("Broadsword", true, func(p *SkillPrereq) { p.PointsCriteria = atLeast(4) })
	skill("Guns", true, func(p *SkillPrereq) { p.PointsCriteria = atLeast(2) })
	skill("Broadsword", true, func(p *SkillPrereq) { p.TagsCriteria = is("Melee") })
	skill("Guns", true, func(p *SkillPrereq) { p.TagsCriteria = isNot("Melee") })
	skill("Feint", false, func(p *SkillPrereq) { p.TagsCriteria = is("Combat") })

	spell := func(subType spellcmp.Type, qualifier criteria.Text, quantity int, level criteria.Number, has bool) {
		p := spellPrereq(subType, qualifier, atLeast(quantity), has)
		p.LevelCriteria = level
		leaves = append(leaves, p)
	}
	spell(spellcmp.College, is("Fire"), 2, atLeast(1), true)
	spell(spellcmp.College, is("Fire"), 2, atLeast(30), true)
	spell(spellcmp.College, textCriteria(criteria.ContainsText, "Fire"), 4, criteria.Number{}, true)
	spell(spellcmp.CollegeCount, criteria.Text{}, 2, numberCriteria(criteria.AtMostNumber, fxp.FromInteger(30)), true)
	spell(spellcmp.Name, is("Ignite Fire"), 1, atLeast(1), false)
	spell(spellcmp.Any, criteria.Text{}, 8, criteria.Number{}, true)
	for _, name := range []string{"Flame Jet", "Smoke", "Rain of Fire"} {
		spell(spellcmp.Name, is(name), 1, criteria.Number{}, true)
	}

	equipped := func(carried bool, name criteria.Text, not bool, set func(p *EquippedEquipmentPrereq)) {
		p := equippedPrereq(name, criteria.Text{})
		if carried {
			p = NewCarriedEquipmentPrereq()
			p.NameCriteria = name
		}
		p.Not = not
		set(p)
		leaves = append(leaves, p)
	}
	none := func(*EquippedEquipmentPrereq) {}
	equipped(false, is("Sword"), false, func(p *EquippedEquipmentPrereq) { p.ModifierNameCriteria = is("Fine") })
	equipped(false, is("Sword"), false, func(p *EquippedEquipmentPrereq) { p.ModifierNameCriteria = is("Cheap") })
	equipped(false, is("Sword"), false, func(p *EquippedEquipmentPrereq) {
		p.ModifierNameCriteria = is("Balanced")
		p.ModifierNotesCriteria = textCriteria(criteria.ContainsText, "throw")
	})
	equipped(false, criteria.Text{}, false, func(p *EquippedEquipmentPrereq) { p.ModifierTagsCriteria = is("Quality") })
	equipped(false, is("Longarm"), false, none)
	equipped(false, is("Longarm"), true, none)
	equipped(false, is("Sword"), true, none)
	equipped(true, is("Kit"), false, none)
	equipped(true, is("Kit"), true, none)
	equipped(true, is("Sword"), false, none)
	equipped(true, is("Lockpicks"), false, none)
	equipped(true, is("Rations"), false, none)
	equipped(true, is("Shield"), false, none)
	equipped(true, is("Ghost"), false, none)
	equipped(true, criteria.Text{}, false, func(p *EquippedEquipmentPrereq) { p.ModifierNameCriteria = is("Fine") })

	leaves = append(leaves, attributePrereq("st", "", atLeast(10), true), scriptPrereq("", "true"),
		scriptPrereq("", "false"))
	return leaves
}

// randomPrereqList returns a list of up to four prerequisites drawn from leaves and lists nested up to depth deep,
// each list "all of" or "any of", sometimes negated, and sometimes applying only from TL 3.
func randomPrereqList(r *rand.Rand, leaves []Prereq, depth int) *PrereqList {
	var children []Prereq
	for range 1 + r.IntN(4) {
		if depth > 0 && r.IntN(3) == 0 {
			children = append(children, randomPrereqList(r, leaves, depth-1))
		} else {
			children = append(children, leaves[r.IntN(len(leaves))].Clone(nil))
		}
	}
	list := listOf(children...)
	list.All = r.IntN(2) == 0
	list.Not = r.IntN(3) == 0
	if r.IntN(8) == 0 {
		list.WhenTL = numberCriteria(criteria.AtLeastNumber, fxp.Three)
	}
	return list
}

// TestConditionDifferentialCriteria compares legacy evaluation with the engine's for each of the newer criteria alone
// and under negated lists, and for thousands of random lists of them, for every owner newCriteriaTestEntity makes.
func TestConditionDifferentialCriteria(t *testing.T) {
	leaves := criteriaTestLeaves()
	const random = 2000
	lists := make([]*PrereqList, 0, 3*len(leaves)+random)
	for _, one := range leaves {
		lists = append(lists, listOf(one.Clone(nil)))
		for _, all := range []bool{true, false} {
			negated := listOf(one.Clone(nil))
			negated.All, negated.Not = all, true
			lists = append(lists, listOf(negated))
		}
	}
	r := rand.New(rand.NewPCG(11, 12)) //nolint:gosec // Not for security.
	for range random {
		lists = append(lists, randomPrereqList(r, leaves, 2))
	}
	var d diffTally
	for _, tl := range []string{"", "3"} {
		e, owners := newCriteriaTestEntity(tl)
		h := newConditionHost(e)
		for i, list := range lists {
			// Every owner for the single lists, and a few for each random one, keeps the run short.
			if i < 3*len(leaves) {
				for _, owner := range owners {
					d.compare(t, h, e, list, owner, "TL "+tl)
				}
				continue
			}
			for range 3 {
				d.compare(t, h, e, list, owners[r.IntN(len(owners))], "TL "+tl)
			}
		}
	}
	t.Log(d.String())
	if slices.Contains(d.states[:CheckFailed], 0) || d.penalties == 0 || d.texts == 0 {
		t.Errorf("the lists should reach met, unmet and skipped, the penalty and texts: %s", d.String())
	}
}
