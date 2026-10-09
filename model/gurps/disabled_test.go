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

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/stlimit"
	"github.com/richardwilkes/toolbox/v2/check"
)

func TestDisabledItemsAreAsIfAbsent(t *testing.T) {
	c := check.New(t)
	e := NewEntity()
	_, skill, spell, eqp := newSwitchableItemSet(e, NewAttributeBonus(StrengthID))
	skill.Points = fxp.Four
	spell.Points = fxp.Four
	eqp.BaseWeight = "2 lb"
	eqp.BaseValue = "10"
	needsSkill := addTraitWithFeatures(e, "Needs Brawling")
	needsSkill.Prereq = NewPrereqList()
	addSkillPrereq(needsSkill.Prereq, "Brawling", fxp.One)
	e.Recalculate()
	c.Equal(fxp.Four, e.AttributeBonusFor(StrengthID, stlimit.None, nil))
	c.Equal(fxp.Four, e.PointsBreakdown().Skills)
	c.Equal(fxp.Four, e.PointsBreakdown().Spells)
	c.True(skill.LevelData.Level > 0)
	c.True(spell.LevelData.Level > 0)
	c.True(e.WeightCarried(false) > 0)
	c.Equal(fxp.Ten, e.WealthCarried())
	c.Equal(1, len(e.SkillNamed("Brawling", "", false, nil)))
	c.Equal("", needsSkill.UnsatisfiedReason)

	skill.Disabled = true
	spell.Disabled = true
	eqp.Disabled = true
	e.Recalculate()
	c.Equal(fxp.One, e.AttributeBonusFor(StrengthID, stlimit.None, nil), "only the trait's bonus remains")
	c.Equal(fxp.Int(0), e.PointsBreakdown().Skills)
	c.Equal(fxp.Int(0), e.PointsBreakdown().Spells)
	c.Equal(fxp.Int(0), skill.LevelData.Level)
	c.Equal(fxp.Int(0), spell.LevelData.Level)
	c.Equal(fxp.Weight(0), e.WeightCarried(false))
	c.Equal(fxp.Int(0), e.WealthCarried())
	c.Equal(0, len(e.SkillNamed("Brawling", "", false, nil)))
	c.NotEqual("", needsSkill.UnsatisfiedReason)
}

func TestDisabledContainerDisablesItsContents(t *testing.T) {
	c := check.New(t)
	e := NewEntity()
	box := NewEquipment(e, nil, true)
	item := NewEquipment(e, box, false)
	item.BaseWeight = "3 lb"
	box.Children = []*Equipment{item}
	e.CarriedEquipment = append(e.CarriedEquipment, box)
	e.Recalculate()
	c.True(e.WeightCarried(false) > 0)

	item.Disabled = true
	c.Equal(fxp.Weight(0), e.WeightCarried(false), "a disabled item adds nothing to its container")

	item.Disabled = false
	box.Disabled = true
	c.False(item.Enabled())
	c.False(item.ReallyEquipped())
	c.Equal(fxp.Weight(0), e.WeightCarried(false))
}

func TestDisabledIsClearedOffASheet(t *testing.T) {
	c := check.New(t)
	e := NewEntity()
	skill := NewSkill(e, nil, false)
	skill.Disabled = true
	spell := NewSpell(e, nil, false)
	spell.Disabled = true
	eqp := NewEquipment(e, nil, false)
	eqp.Disabled = true
	c.True(skill.Clone(LibraryFile{}, e, nil, Copy).Disabled)
	c.True(spell.Clone(LibraryFile{}, e, nil, Copy).Disabled)
	c.True(eqp.Clone(LibraryFile{}, e, nil, Copy).Disabled)
	c.False(skill.Clone(LibraryFile{}, nil, nil, Copy).Disabled)
	c.False(spell.Clone(LibraryFile{}, nil, nil, Copy).Disabled)
	c.False(eqp.Clone(LibraryFile{}, nil, nil, Copy).Disabled)
}
