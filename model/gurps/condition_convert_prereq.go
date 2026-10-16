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
	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
)

// prereqOwner is the type of item a prerequisite list belongs to, which decides the rules legacy prerequisites apply
// implicitly.
type prereqOwner byte

const (
	ownerOther prereqOwner = iota
	ownerSkill
	ownerSpell
	ownerEquipment
)

// prereqOwnerOf returns the prereqOwner of owner. Techniques are skills.
func prereqOwnerOf(owner any) prereqOwner {
	switch owner.(type) {
	case *Skill:
		return ownerSkill
	case *Spell:
		return ownerSpell
	case *Equipment:
		return ownerEquipment
	default:
		return ownerOther
	}
}

// containsAnything stands in for a spell name criteria of "any": it matches every name too, but is a real criterion,
// so the #737 guard (condition.PositivelyRequires) sees the requirement just as prereqListRequiresSpell does.
var containsAnything = criteria.Text{Compare: criteria.ContainsText}

// conditionFromPrereqs converts a legacy prerequisite list, which may be nil, belonging to owner into a condition tree
// that evaluates exactly as the list does. The rules legacy prerequisites apply because of the owner's type become
// visible nodes.
func conditionFromPrereqs(list *PrereqList, owner any) condition.Node {
	if list == nil {
		return nil
	}
	return convertPrereqList(list, prereqOwnerOf(owner))
}

func convertPrereqList(p *PrereqList, owner prereqOwner) *condition.Group {
	g := &condition.Group{Mode: condition.GroupAny}
	if p.All {
		g.Mode = condition.GroupAll
	}
	if p.WhenTL.Compare != criteria.AnyNumber {
		tl := p.WhenTL.Qualifier
		g.OnlyWhen.Node = numberCompare(&condition.Operand{Entity: filterFieldKeyTechLevel}, p.WhenTL.Compare,
			&condition.Operand{Value: &tl}, false)
	}
	if len(p.Prereqs) != 0 {
		g.Children = make(condition.Nodes, len(p.Prereqs))
		for i, one := range p.Prereqs {
			g.Children[i] = convertPrereq(one, owner)
		}
	}
	return g
}

func convertPrereq(one Prereq, owner prereqOwner) condition.Node {
	switch p := one.(type) {
	case *PrereqList:
		return convertPrereqList(p, owner)
	case *TraitPrereq:
		return &condition.Count{Of: conditionTraits, Where: whereOf(
			textCheck(filterFieldKeyName, p.NameCriteria),
			numberCheck(conditionLevel, p.LevelCriteria),
			textCheck(filterFieldKeyNotes, p.NotesCriteria),
		), Not: !p.Has}
	case *SkillPrereq:
		var tl condition.Node
		if owner == ownerSkill {
			tl = selfCheck(filterFieldKeyTechLevel, true)
		}
		return &condition.Count{Of: conditionSkills, Where: whereOf(
			textCheck(filterFieldKeyName, p.NameCriteria),
			textCheck(conditionSpecialization, p.SpecializationCriteria),
			textCheck(conditionOptionalSpecialty, p.OptionalSpecializationCriteria),
			numberCheck(conditionLevel, p.LevelCriteria),
			tl,
		), Not: !p.Has}
	case *SpellPrereq:
		return convertSpellPrereq(p, owner)
	case *AttributePrereq:
		return convertAttributePrereq(p)
	case *ContainedWeightPrereq:
		w := p.WeightCriteria.Qualifier
		return convertContained(p, p.Has, p.WeightCriteria.Compare, owner, conditionContainedWeight,
			&condition.Operand{Weight: &w})
	case *ContainedQuantityPrereq:
		q := p.QualifierCriteria.Qualifier
		return convertContained(p, p.Has, p.QualifierCriteria.Compare, owner, conditionContainedQuantity,
			&condition.Operand{Value: &q})
	case *EquippedEquipmentPrereq:
		return &condition.Count{Of: conditionEquipped, Where: whereOf(
			textCheck(filterFieldKeyName, p.NameCriteria),
			textCheck(filterFieldKeyTags, p.TagsCriteria),
		)}
	case *ScriptPrereq:
		return &condition.Script{Name: p.Name, Script: p.Script}
	case *UnknownPrereq:
		return legacyConst(false, p.Data)
	default:
		return nil
	}
}

// convertSpellPrereq converts a spell prerequisite. The sub type's check is always present, even when it accepts
// anything, so the tree still tells the sub types apart; the qualifier of the college count and any sub types means
// nothing and is dropped, as is the power source criteria when the same power source is required. Same power source
// compares with the owning spell's, and matches nothing for other owners.
func convertSpellPrereq(p *SpellPrereq, owner prereqOwner) condition.Node {
	c := &condition.Count{Of: conditionSpells, Not: !p.Has}
	var subType, powerSource, tl condition.Node
	switch p.SubType {
	case spellcmp.Name:
		t := p.QualifierCriteria
		if t.Compare == criteria.AnyText {
			t = containsAnything
		}
		subType = &condition.Check{Field: filterFieldKeyName, Text: escapeText(t)}
	case spellcmp.Tag:
		subType = &condition.Check{Field: filterFieldKeyTags, Text: escapeText(p.QualifierCriteria)}
	case spellcmp.College:
		subType = &condition.Check{Field: conditionCollege, Text: escapeText(p.QualifierCriteria)}
	case spellcmp.CollegeCount:
		c.GroupBy = conditionCollege
	default:
	}
	switch {
	case p.SamePowerSource && owner == ownerSpell:
		powerSource = selfCheck(conditionPowerSource, false)
	case p.SamePowerSource:
		powerSource = &condition.Const{}
	default:
		powerSource = textCheck(conditionPowerSource, p.PowerSourceCriteria)
	}
	if owner == ownerSpell {
		tl = selfCheck(filterFieldKeyTechLevel, true)
	}
	c.Where = whereOf(subType, powerSource, tl)
	if q := p.QuantityCriteria; q.Compare != criteria.AtLeastNumber || q.Qualifier != fxp.One {
		if q.Compare == criteria.AnyNumber {
			q.Qualifier = 0
		}
		c.Quantity = &q
	}
	return c
}

// convertAttributePrereq converts an attribute prerequisite. Legacy reads an empty attribute ID as the minimum value,
// which the engine can't name as an attribute, so it becomes that constant.
func convertAttributePrereq(p *AttributePrereq) condition.Node {
	if p.QualifierCriteria.Compare == criteria.AnyNumber {
		return legacyConst(p.Has, legacyJSON(p))
	}
	left := attributeOperand(p.Which)
	if p.CombinedWith != "" {
		left = condition.Operand{Sum: []condition.Operand{left, attributeOperand(p.CombinedWith)}}
	}
	q := p.QualifierCriteria.Qualifier
	return numberCompare(&left, p.QualifierCriteria.Compare, &condition.Operand{Value: &q}, !p.Has)
}

func attributeOperand(id string) condition.Operand {
	if id == "" {
		minimum := fxp.Min
		return condition.Operand{Value: &minimum}
	}
	return condition.Operand{Attribute: id}
}

// convertContained converts a contained weight or quantity prerequisite: met when the owner isn't a container or what
// it holds passes the criteria. Only equipment can hold anything, and an "any" criteria always passes.
func convertContained(p Prereq, has bool, cmp criteria.NumericComparison, owner prereqOwner, self string, right *condition.Operand) condition.Node {
	switch {
	case owner != ownerEquipment:
		return legacyConst(!has, legacyJSON(p))
	case cmp == criteria.AnyNumber:
		return legacyConst(has, legacyJSON(p))
	default:
		return &condition.Group{Mode: condition.GroupAny, Not: !has, Children: condition.Nodes{
			&condition.Check{Field: filterFieldKeyContainer, Not: true},
			numberCompare(&condition.Operand{Self: self}, cmp, right, false),
		}}
	}
}
