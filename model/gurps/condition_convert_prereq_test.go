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
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
)

// prereqCase is one legacy prerequisite list, its owner's kind and the canonical JSON it converts to.
type prereqCase struct {
	name  string
	owner string
	list  *PrereqList
	want  string
}

// listOf returns an "all of" list holding the prerequisites.
func listOf(prereqs ...Prereq) *PrereqList {
	list := NewPrereqList()
	for _, one := range prereqs {
		if one != nil {
			one.SetParentList(list)
		}
		list.Prereqs = append(list.Prereqs, one)
	}
	return list
}

func anyOf(prereqs ...Prereq) *PrereqList {
	list := listOf(prereqs...)
	list.All = false
	return list
}

func traitPrereq(name string, has bool) *TraitPrereq {
	p := NewTraitPrereq()
	p.NameCriteria.Qualifier = name
	p.Has = has
	return p
}

func skillPrereq(name string, has bool) *SkillPrereq {
	p := NewSkillPrereq()
	p.NameCriteria.Qualifier = name
	p.Has = has
	return p
}

func spellPrereq(subType spellcmp.Type, qualifier criteria.Text, quantity criteria.Number, has bool) *SpellPrereq {
	p := NewSpellPrereq()
	p.SubType = subType
	p.QualifierCriteria = qualifier
	p.QuantityCriteria = quantity
	p.SamePowerSource = false
	p.Has = has
	return p
}

func attributePrereq(which, combined string, qualifier criteria.Number, has bool) *AttributePrereq {
	p := NewAttributePrereq(nil)
	p.Which = which
	p.CombinedWith = combined
	p.QualifierCriteria = qualifier
	p.Has = has
	return p
}

func weightPrereq(compare criteria.NumericComparison, pounds int, has bool) *ContainedWeightPrereq {
	p := NewContainedWeightPrereq(nil)
	p.WeightCriteria.Compare = compare
	p.WeightCriteria.Qualifier = fxp.WeightFromInteger(pounds, fxp.Pound)
	p.Has = has
	return p
}

func quantityPrereq(compare criteria.NumericComparison, qty fxp.Int, has bool) *ContainedQuantityPrereq {
	p := NewContainedQuantityPrereq()
	p.QualifierCriteria = numberCriteria(compare, qty)
	p.Has = has
	return p
}

func equippedPrereq(name, tags criteria.Text) *EquippedEquipmentPrereq {
	p := NewEquippedEquipmentPrereq()
	p.NameCriteria = name
	p.TagsCriteria = tags
	return p
}

func scriptPrereq(name, script string) *ScriptPrereq {
	p := NewScriptPrereq()
	p.Name = name
	p.Script = script
	return p
}

// prereqCases returns the unit cases: every legacy type, has=false on each type that has it, when_tl, nesting, empty
// lists, unknown prerequisites, contained prerequisites on equipment and other owners, and the owner-type rules.
func prereqCases() []prereqCase {
	atLeast1 := numberCriteria(criteria.AtLeastNumber, fxp.One)
	isFire := textCriteria(criteria.IsText, "Fire")
	trait := traitPrereq("Magery", true)
	trait.LevelCriteria = numberCriteria(criteria.AtLeastNumber, fxp.Two)
	trait.NotesCriteria = textCriteria(criteria.ContainsText, "one college ({Air})")
	skill := skillPrereq("Guns", false)
	skill.SpecializationCriteria = textCriteria(criteria.IsText, "Pistol")
	skill.OptionalSpecializationCriteria = textCriteria(criteria.StartsWithText, "Re")
	skill.LevelCriteria = numberCriteria(criteria.AtLeastNumber, fxp.Twelve)
	same := spellPrereq(spellcmp.Name, isFire, atLeast1, true)
	same.SamePowerSource = true
	source := spellPrereq(spellcmp.Any, criteria.Text{}, numberCriteria(criteria.AtLeastNumber, fxp.Three), true)
	source.PowerSourceCriteria = textCriteria(criteria.IsText, "Clerical")
	whenTL := listOf(traitPrereq("High TL", true))
	whenTL.WhenTL = numberCriteria(criteria.AtLeastNumber, fxp.Four)
	return []prereqCase{
		{"empty list", "trait", NewPrereqList(), `null`},
		{"empty any list", "trait", anyOf(), `null`},
		{"trait", "trait", listOf(trait), `{"type":"group","mode":"all","children":[{"type":"count","of":"traits","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Magery"}},{"type":"check","field":"level","number":{"compare":"at_least","qualifier":2}},{"type":"check","field":"notes","text":{"compare":"contains","qualifier":"one college ({{Air})"}}]}}]}`},
		{"trait has=false", "trait", listOf(traitPrereq("Magery", false)), `{"type":"group","mode":"all","children":[{"type":"count","of":"traits","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Magery"}},{"type":"check","field":"level","number":{"compare":"at_least"}}]},"not":true}]}`},
		{"trait any name", "trait", listOf(&TraitPrereq{Type: traitPrereq("", true).Type, Has: true}), `{"type":"group","mode":"all","children":[{"type":"count","of":"traits"}]}`},
		{"skill on a trait", "trait", listOf(skill), `{"type":"group","mode":"all","children":[{"type":"count","of":"skills","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Guns"}},{"type":"check","field":"specialization","text":{"compare":"is","qualifier":"Pistol"}},{"type":"check","field":"optional_specialization","text":{"compare":"starts_with","qualifier":"Re"}},{"type":"check","field":"level","number":{"compare":"at_least","qualifier":12}}]},"not":true}]}`},
		{"skill on a skill", "skill_tl", listOf(skillPrereq("Guns", true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"skills","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Guns"}},{"type":"check","field":"level","number":{"compare":"at_least"}},{"type":"group","mode":"any","children":[{"type":"check","field":"tech_level","not":true},{"type":"check","field":"tech_level","text":{"compare":"is","qualifier":"{self.tech_level}"}}]}]}}]}`},
		{"skill on a technique", "technique", listOf(skillPrereq("Karate", true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"skills","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Karate"}},{"type":"check","field":"level","number":{"compare":"at_least"}}]}}]}`},
		{"skill on a spell", "spell", listOf(skillPrereq("Thaumatology", true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"skills","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Thaumatology"}},{"type":"check","field":"level","number":{"compare":"at_least"}}]}}]}`},
		{"spell name on a spell", "spell_tl", listOf(spellPrereq(spellcmp.Name, isFire, atLeast1, true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Fire"}},{"type":"group","mode":"any","children":[{"type":"check","field":"tech_level","not":true},{"type":"check","field":"tech_level","text":{"compare":"is","qualifier":"{self.tech_level}"}}]}]}}]}`},
		{"spell name on a trait", "trait", listOf(spellPrereq(spellcmp.Name, isFire, atLeast1, true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"check","field":"name","text":{"compare":"is","qualifier":"Fire"}}}]}`},
		{"spell name has=false", "spell", listOf(spellPrereq(spellcmp.Name, isFire, atLeast1, false)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"check","field":"name","text":{"compare":"is","qualifier":"Fire"}},"not":true}]}`},
		{"spell any name", "spell", listOf(spellPrereq(spellcmp.Name, criteria.Text{}, numberCriteria(criteria.AtLeastNumber, fxp.Eight), true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"check","field":"name","text":{"compare":"contains"}},"quantity":{"compare":"at_least","qualifier":8}}]}`},
		{"spell tag", "spell", listOf(spellPrereq(spellcmp.Tag, textCriteria(criteria.ContainsText, "Fire"), atLeast1, true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"check","field":"tags","text":{"compare":"contains","qualifier":"Fire"}}}]}`},
		{"spell any tag", "trait", listOf(spellPrereq(spellcmp.Tag, criteria.Text{}, atLeast1, true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"check","field":"tags"}}]}`},
		{"spell college", "spell", listOf(spellPrereq(spellcmp.College, textCriteria(criteria.ContainsText, "Air"), numberCriteria(criteria.AtLeastNumber, fxp.Five), true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"check","field":"college","text":{"compare":"contains","qualifier":"Air"}},"quantity":{"compare":"at_least","qualifier":5}}]}`},
		{"spell college is not", "spell", listOf(spellPrereq(spellcmp.College, textCriteria(criteria.IsNotText, "Fire"), atLeast1, true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"group","mode":"all","children":[{"type":"check","field":"college"},{"type":"check","field":"college","text":{"compare":"is_not","qualifier":"Fire"}}]}}]}`},
		{"spell college count", "spell", listOf(spellPrereq(spellcmp.CollegeCount, isFire, numberCriteria(criteria.AtLeastNumber, fxp.Ten), true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","quantity":{"compare":"at_least","qualifier":10},"group_by":"college"}]}`},
		{"spell any", "trait", listOf(spellPrereq(spellcmp.Any, isFire, numberCriteria(criteria.EqualsNumber, fxp.Two), true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","quantity":{"compare":"is","qualifier":2}}]}`},
		{"spell quantity any", "spell", listOf(spellPrereq(spellcmp.Name, isFire, criteria.Number{}, true)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"check","field":"name","text":{"compare":"is","qualifier":"Fire"}},"quantity":{}}]}`},
		{"spell same power source on a spell", "spell", listOf(same), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Fire"}},{"type":"check","field":"power_source","text":{"compare":"is","qualifier":"{self.power_source}"}}]}}]}`},
		{"spell same power source on a trait", "trait", listOf(same.Clone(nil)), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Fire"}},{"type":"const","value":false}]}}]}`},
		{"spell power source", "trait", listOf(source), `{"type":"group","mode":"all","children":[{"type":"count","of":"spells","where":{"type":"check","field":"power_source","text":{"compare":"is","qualifier":"Clerical"}},"quantity":{"compare":"at_least","qualifier":3}}]}`},
		{"attribute", "trait", listOf(attributePrereq("dx", "", numberCriteria(criteria.AtLeastNumber, fxp.Twelve), true)), `{"type":"group","mode":"all","children":[{"type":"compare","left":{"attribute":"dx"},"op":"at_least","right":{"value":12}}]}`},
		{"attribute has=false", "trait", listOf(attributePrereq("iq", "", numberCriteria(criteria.AtMostNumber, fxp.Ten), false)), `{"type":"group","mode":"all","children":[{"type":"compare","left":{"attribute":"iq"},"op":"more_than","right":{"value":10}}]}`},
		{"attribute combined", "spell", listOf(attributePrereq("iq", "dx", numberCriteria(criteria.AtLeastNumber, fxp.FromInteger(30)), true)), `{"type":"group","mode":"all","children":[{"type":"compare","left":{"sum":[{"attribute":"iq"},{"attribute":"dx"}]},"op":"at_least","right":{"value":30}}]}`},
		{"attribute empty id", "trait", listOf(attributePrereq("", "ht", numberCriteria(criteria.EqualsNumber, fxp.One), true)), `{"type":"group","mode":"all","children":[{"type":"compare","left":{"sum":[{"value":-922337203685477.5808},{"attribute":"ht"}]},"op":"is","right":{"value":1}}]}`},
		{"attribute any", "trait", listOf(attributePrereq("st", "", criteria.Number{}, true)), `{"type":"group","mode":"all","children":[{"type":"const","value":true}]}`},
		{"attribute any has=false", "trait", listOf(attributePrereq("st", "", criteria.Number{}, false)), `{"type":"group","mode":"all","children":[{"type":"const","value":false}]}`},
		{"contained weight on equipment", "equipment", listOf(weightPrereq(criteria.AtMostNumber, 10, true)), `{"type":"group","mode":"all","children":[{"type":"group","mode":"any","children":[{"type":"check","field":"container","not":true},{"type":"compare","left":{"self":"contained_weight"},"op":"at_most","right":{"weight":"10 lb"}}]}]}`},
		{"contained weight has=false", "equipment_container", listOf(weightPrereq(criteria.AtLeastNumber, 3, false)), `{"type":"group","mode":"all","children":[{"type":"group","mode":"any","not":true,"children":[{"type":"check","field":"container","not":true},{"type":"compare","left":{"self":"contained_weight"},"op":"at_least","right":{"weight":"3 lb"}}]}]}`},
		{"contained weight any", "equipment", listOf(weightPrereq(criteria.AnyNumber, 0, true)), `{"type":"group","mode":"all","children":[{"type":"const","value":true}]}`},
		{"contained weight on a trait", "trait", listOf(weightPrereq(criteria.AtMostNumber, 10, true)), `{"type":"group","mode":"all","children":[{"type":"const","value":false}]}`},
		{"contained weight on a trait has=false", "trait", listOf(weightPrereq(criteria.AtMostNumber, 10, false)), `{"type":"group","mode":"all","children":[{"type":"const","value":true}]}`},
		{"contained quantity", "equipment", listOf(quantityPrereq(criteria.AtMostNumber, fxp.Five, true)), `{"type":"group","mode":"all","children":[{"type":"group","mode":"any","children":[{"type":"check","field":"container","not":true},{"type":"compare","left":{"self":"contained_quantity"},"op":"at_most","right":{"value":5}}]}]}`},
		{"contained quantity has=false", "equipment", listOf(quantityPrereq(criteria.EqualsNumber, fxp.Two, false)), `{"type":"group","mode":"all","children":[{"type":"group","mode":"any","not":true,"children":[{"type":"check","field":"container","not":true},{"type":"compare","left":{"self":"contained_quantity"},"op":"is","right":{"value":2}}]}]}`},
		{"contained quantity on a skill", "skill", listOf(quantityPrereq(criteria.AtMostNumber, fxp.Five, true)), `{"type":"group","mode":"all","children":[{"type":"const","value":false}]}`},
		{"equipped", "skill", listOf(equippedPrereq(textCriteria(criteria.IsText, "Lockpicks"), textCriteria(criteria.ContainsText, "Tool"))), `{"type":"group","mode":"all","children":[{"type":"count","of":"equipped","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"Lockpicks"}},{"type":"check","field":"tags","text":{"compare":"contains","qualifier":"Tool"}}]}}]}`},
		{"equipped any", "spell", listOf(equippedPrereq(criteria.Text{}, criteria.Text{})), `{"type":"group","mode":"all","children":[{"type":"count","of":"equipped"}]}`},
		{"script", "trait", listOf(scriptPrereq("Custom", "self.level > 1")), `{"type":"group","mode":"all","children":[{"type":"script","name":"Custom","script":"self.level > 1"}]}`},
		{"when_tl", "trait", listOf(whenTL), `{"type":"group","mode":"all","children":[{"type":"group","mode":"all","only_when":{"type":"compare","left":{"entity":"tech_level"},"op":"at_least","right":{"value":4}},"children":[{"type":"count","of":"traits","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"High TL"}},{"type":"check","field":"level","number":{"compare":"at_least"}}]}}]}]}`},
		{"when_tl at most", "skill", anyOf(&PrereqList{Type: whenTL.Type, WhenTL: numberCriteria(criteria.AtMostNumber, fxp.Three)}), `{"type":"group","mode":"any","children":[{"type":"group","mode":"any","only_when":{"type":"compare","left":{"entity":"tech_level"},"op":"at_most","right":{"value":3}}}]}`},
		{"nested", "spell", anyOf(listOf(traitPrereq("A", true), anyOf(traitPrereq("B", false), skillPrereq("C", true))), NewPrereqList()), `{"type":"group","mode":"any","children":[{"type":"group","mode":"all","children":[{"type":"count","of":"traits","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"A"}},{"type":"check","field":"level","number":{"compare":"at_least"}}]}},{"type":"group","mode":"any","children":[{"type":"count","of":"traits","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"B"}},{"type":"check","field":"level","number":{"compare":"at_least"}}]},"not":true},{"type":"count","of":"skills","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"C"}},{"type":"check","field":"level","number":{"compare":"at_least"}}]}}]}]},{"type":"group","mode":"all"}]}`},
		{"nil child", "trait", listOf(nil, traitPrereq("A", true)), `{"type":"group","mode":"all","children":[null,{"type":"count","of":"traits","where":{"type":"group","mode":"all","children":[{"type":"check","field":"name","text":{"compare":"is","qualifier":"A"}},{"type":"check","field":"level","number":{"compare":"at_least"}}]}}]}`},
	}
}

func TestConditionFromPrereqs(t *testing.T) {
	for _, tc := range prereqCases() {
		if got := treeJSON(t, convertedTree(t, tc.list, newOwner(tc.owner))); got != tc.want {
			t.Errorf("%s:\nwant %s\n got %s", tc.name, tc.want, got)
		}
	}
	if convertedTree(t, nil, nil) != nil {
		t.Error("a nil list should convert to a nil tree")
	}
}
