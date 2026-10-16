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
	"errors"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
)

// ownerKind is the type of item a prerequisite list belongs to.
type ownerKind byte

const (
	ownerOther ownerKind = iota
	ownerSkill
	ownerSpell
	ownerEquipment
)

// prereqOwner is what about a prerequisite list's owner decides the rules legacy prerequisites apply implicitly: its
// type, and for a skill or spell whether it has a tech level.
type prereqOwner struct {
	kind      ownerKind
	techLevel bool
}

// prereqOwnerOf returns the prereqOwner of owner. Techniques are skills.
func prereqOwnerOf(owner any) prereqOwner {
	switch t := owner.(type) {
	case *Skill:
		return prereqOwner{kind: ownerSkill, techLevel: t.TechLevel != nil}
	case *Spell:
		return prereqOwner{kind: ownerSpell, techLevel: t.TechLevel != nil}
	case *Equipment:
		return prereqOwner{kind: ownerEquipment}
	default:
		return prereqOwner{}
	}
}

// containsAnything stands in for a spell name criteria of "any": it matches every name too, but is a real criterion,
// so the #737 guard (positivelyRequires) sees the requirement just as prereqListRequiresSpell does.
var containsAnything = criteria.Text{Compare: criteria.ContainsText}

// convertPrereqList converts a legacy prerequisite list belonging to owner into a condition tree that evaluates
// exactly as the list does. The rules legacy prerequisites apply because of the owner's type become visible nodes. A
// prerequisite this version of GCS doesn't know can't be converted, which fails the whole list.
func convertPrereqList(p *PrereqList, owner prereqOwner) (*condition.Group, error) {
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
			child, err := convertPrereq(one, owner)
			if err != nil {
				return nil, err
			}
			g.Children[i] = child
		}
	}
	return g, nil
}

func convertPrereq(one Prereq, owner prereqOwner) (condition.Node, error) {
	switch p := one.(type) {
	case *PrereqList:
		return convertPrereqList(p, owner)
	case *TraitPrereq:
		return &condition.Count{Of: conditionTraits, Where: whereOf(
			textCheck(filterFieldKeyName, p.NameCriteria),
			numberCheck(conditionLevel, p.LevelCriteria),
			textCheck(filterFieldKeyNotes, p.NotesCriteria),
		), Not: !p.Has}, nil
	case *SkillPrereq:
		var tl condition.Node
		if owner.kind == ownerSkill && owner.techLevel {
			tl = techLevelRule()
		}
		return &condition.Count{Of: conditionSkills, Where: whereOf(
			textCheck(filterFieldKeyName, p.NameCriteria),
			textCheck(conditionSpecialization, p.SpecializationCriteria),
			textCheck(conditionOptionalSpecialty, p.OptionalSpecializationCriteria),
			numberCheck(conditionLevel, p.LevelCriteria),
			tl,
		), Not: !p.Has}, nil
	case *SpellPrereq:
		return convertSpellPrereq(p, owner), nil
	case *AttributePrereq:
		return convertAttributePrereq(p), nil
	case *ContainedWeightPrereq:
		w := p.WeightCriteria.Qualifier
		return convertContained(p.Has, p.WeightCriteria.Compare, owner, conditionContainedWeight,
			&condition.Operand{Weight: &w}), nil
	case *ContainedQuantityPrereq:
		q := p.QualifierCriteria.Qualifier
		return convertContained(p.Has, p.QualifierCriteria.Compare, owner, conditionContainedQuantity,
			&condition.Operand{Value: &q}), nil
	case *EquippedEquipmentPrereq:
		return &condition.Count{Of: conditionEquipped, Where: whereOf(
			textCheck(filterFieldKeyName, p.NameCriteria),
			textCheck(filterFieldKeyTags, p.TagsCriteria),
		)}, nil
	case *ScriptPrereq:
		return &condition.Script{Name: p.Name, Script: p.Script}, nil
	case *UnknownPrereq:
		return nil, errors.New(p.Describe(nil, nil, plainText))
	default:
		return nil, nil
	}
}

// convertSpellPrereq converts a spell prerequisite. The sub type's check is always present, even when it accepts
// anything, so the tree still tells the sub types apart; the qualifier of the college count and any sub types means
// nothing and is dropped, as is the power source criteria when the same power source is required. Same power source
// compares with the owning spell's, and matches nothing for other owners. A spell without colleges or tags never
// matches a college or tag criteria, negative ones included.
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
		subType = presentCheck(&condition.Check{Field: filterFieldKeyTags, Text: escapeText(p.QualifierCriteria)})
	case spellcmp.College:
		subType = presentCheck(&condition.Check{Field: conditionCollege, Text: escapeText(p.QualifierCriteria)})
	case spellcmp.CollegeCount:
		c.GroupBy = conditionCollege
	default:
	}
	switch {
	case p.SamePowerSource && owner.kind == ownerSpell:
		powerSource = selfCheck(conditionPowerSource)
	case p.SamePowerSource:
		powerSource = &condition.Const{}
	default:
		powerSource = textCheck(conditionPowerSource, p.PowerSourceCriteria)
	}
	if owner.kind == ownerSpell && owner.techLevel {
		tl = techLevelRule()
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
		return &condition.Const{Value: p.Has}
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
func convertContained(has bool, cmp criteria.NumericComparison, owner prereqOwner, self string, right *condition.Operand) condition.Node {
	switch {
	case owner.kind != ownerEquipment:
		return &condition.Const{Value: !has}
	case cmp == criteria.AnyNumber:
		return &condition.Const{Value: has}
	default:
		return &condition.Group{Mode: condition.GroupAny, Not: !has, Children: condition.Nodes{
			&condition.Check{Field: filterFieldKeyContainer, Not: true},
			numberCompare(&condition.Operand{Self: self}, cmp, right, false),
		}}
	}
}
