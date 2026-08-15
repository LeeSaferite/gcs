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
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xbytes"
)

// addTestCollegeSpell creates a non-container spell owned by the entity with the given name and college, one point, then
// appends it to the entity's spell list.
func addTestCollegeSpell(e *Entity, name, college string) *Spell {
	s := addTestSpell(e, name, fxp.One)
	s.College = CollegeList{college}
	return s
}

// addSpellNamePrereq attaches a "requires a spell named target" prerequisite to the supplied spell.
func addSpellNamePrereq(s *Spell, target string) {
	p := NewSpellPrereq()
	p.SubType = spellcmp.Name
	p.QualifierCriteria.Compare = criteria.IsText
	p.QualifierCriteria.Qualifier = target
	s.Prereq = NewPrereqList()
	s.Prereq.Prereqs = append(s.Prereq.Prereqs, p)
	p.Parent = s.Prereq
}

// TestSpellPrereqCircularNotCounted verifies that a spell which itself requires the spell being checked is not counted
// toward that spell's own college prerequisite, which would otherwise create a circular prerequisite relationship.
// See GitHub issue #737.
func TestSpellPrereqCircularNotCounted(t *testing.T) {
	c := check.New(t)

	e := NewEntity()

	// "Wisdom" requires at least 5 spells whose college contains "Mind Control".
	wisdom := addTestCollegeSpell(e, "Wisdom", "Mind Control")
	collegeReq := NewSpellPrereq()
	collegeReq.SubType = spellcmp.College
	collegeReq.QualifierCriteria.Compare = criteria.ContainsText
	collegeReq.QualifierCriteria.Qualifier = "Mind Control"
	collegeReq.QuantityCriteria.Compare = criteria.AtLeastNumber
	collegeReq.QuantityCriteria.Qualifier = fxp.FromInteger(5)
	wisdom.Prereq = NewPrereqList()
	wisdom.Prereq.Prereqs = append(wisdom.Prereq.Prereqs, collegeReq)
	collegeReq.Parent = wisdom.Prereq

	// "Boost Intelligence" is a Mind Control spell that directly requires "Wisdom".
	boost := addTestCollegeSpell(e, "Boost Intelligence", "Mind Control")
	addSpellNamePrereq(boost, "Wisdom")

	// Four additional, non-circular Mind Control spells.
	addTestCollegeSpell(e, "Mind A", "Mind Control")
	addTestCollegeSpell(e, "Mind B", "Mind Control")
	addTestCollegeSpell(e, "Mind C", "Mind Control")
	addTestCollegeSpell(e, "Mind D", "Mind Control")

	// The direct-prerequisite helper must recognize the circular relationship.
	c.True(spellDirectlyRequires(boost, wisdom), "Boost Intelligence directly requires Wisdom")
	c.False(spellDirectlyRequires(e.Spells[2], wisdom), "Mind A does not require Wisdom")

	// There are five Mind Control spells besides Wisdom (Boost Intelligence + four plain ones), but Boost Intelligence
	// must not be counted because it requires Wisdom. That leaves only four, so the "at least 5" requirement is not met.
	c.False(collegeReq.Satisfied(e, wisdom, nil, "", nil),
		"Wisdom's college prerequisite must not be satisfied once the circular spell is excluded")

	// Adding a fifth non-circular Mind Control spell brings the count back up to five and satisfies the requirement.
	addTestCollegeSpell(e, "Mind E", "Mind Control")
	c.True(collegeReq.Satisfied(e, wisdom, nil, "", nil),
		"Wisdom's college prerequisite must be satisfied once a fifth non-circular spell is present")
}

// TestSpellPrereqNestedCircularNotCounted verifies that a circular relationship expressed inside a nested prereq list is
// still detected.
func TestSpellPrereqNestedCircularNotCounted(t *testing.T) {
	c := check.New(t)

	e := NewEntity()
	wisdom := addTestCollegeSpell(e, "Wisdom", "Mind Control")
	boost := addTestCollegeSpell(e, "Boost Intelligence", "Mind Control")

	// Nest the spell-name prerequisite one level down inside an "any of" sub-list.
	inner := NewPrereqList()
	inner.All = false
	namePrereq := NewSpellPrereq()
	namePrereq.SubType = spellcmp.Name
	namePrereq.QualifierCriteria.Compare = criteria.IsText
	namePrereq.QualifierCriteria.Qualifier = "Wisdom"
	inner.Prereqs = append(inner.Prereqs, namePrereq)
	namePrereq.Parent = inner
	boost.Prereq = NewPrereqList()
	boost.Prereq.Prereqs = append(boost.Prereq.Prereqs, inner)
	inner.Parent = boost.Prereq

	c.True(spellDirectlyRequires(boost, wisdom), "nested spell-name prerequisite must be detected")
}

// TestSpellPrereqNilEntity verifies that a nil entity is treated as satisfied rather than panicking, matching every
// other prereq implementation.
func TestSpellPrereqNilEntity(t *testing.T) {
	c := check.New(t)
	for _, subType := range []spellcmp.Type{
		spellcmp.Name,
		spellcmp.Tag,
		spellcmp.PowerSource,
		spellcmp.Class,
		spellcmp.College,
		spellcmp.CollegeCount,
		spellcmp.Any,
	} {
		p := NewSpellPrereq()
		p.SubType = subType
		p.QualifierCriteria.Compare = criteria.IsText
		p.QualifierCriteria.Qualifier = "Wisdom"
		var tooltip xbytes.InsertBuffer
		c.NotPanics(func() {
			c.True(p.Satisfied(nil, nil, &tooltip, "", nil), "%v: a nil entity must be treated as satisfied", subType)
		}, "%v: a nil entity must not panic", subType)
		c.Equal("", tooltip.String(), "%v: no tooltip should be written for a nil entity", subType)
	}
}

// TestSpellPrereqPowerSource verifies that a Power Source prerequisite counts only spells whose power source matches
// the qualifier, and that it distinguishes between spells that share the same name but have different power sources.
func TestSpellPrereqPowerSource(t *testing.T) {
	c := check.New(t)

	e := NewEntity()

	// "Detect Magic" exists in all three power sources, so a name-based match alone can't tell them apart.
	manaDetectMagic := addTestSpell(e, "Detect Magic", fxp.One)
	manaDetectMagic.PowerSource = "Mana"
	sanctityDetectMagic := addTestSpell(e, "Detect Magic", fxp.One)
	sanctityDetectMagic.PowerSource = "Sanctity"
	natureDetectMagic := addTestSpell(e, "Detect Magic", fxp.One)
	natureDetectMagic.PowerSource = "Nature's Strength"

	// "Shape Earth" exists in only two power sources
	manaShapeEarth := addTestSpell(e, "Shape Earth", fxp.One)
	manaShapeEarth.PowerSource = "Mana"
	natureShapeEarth := addTestSpell(e, "Shape Earth", fxp.One)
	natureShapeEarth.PowerSource = "Nature's Strength"

	req := NewSpellPrereq()
	req.SubType = spellcmp.PowerSource
	req.QualifierCriteria.Compare = criteria.IsText
	req.QualifierCriteria.Qualifier = "Mana"

	// Exactly two Mana spells exist and using "exactly 2" rather than "at least 2" catches the non-Mana spells being accidentally included
	req.QuantityCriteria.Compare = criteria.EqualsNumber
	req.QuantityCriteria.Qualifier = fxp.FromInteger(2)
	c.True(
		req.Satisfied(e, nil, nil, "", nil),
		"the two Mana spells should satisfy an exactly-2 Mana requirement",
	)

	// Requiring more than two Mana spells must fail, since only two exist.
	req.QuantityCriteria.Compare = criteria.AtLeastNumber
	req.QuantityCriteria.Qualifier = fxp.FromInteger(3)
	var tooltip xbytes.InsertBuffer
	c.False(
		req.Satisfied(e, nil, &tooltip, "", nil),
		"only two Mana spells exist, so a more-than-2 requirement must fail",
	)
	c.Contains(
		tooltip.String(),
		"power source",
		"the tooltip should mention 'power source'",
	)
}

// TestSpellPrereqClass verifies that a class prerequisite counts only spells whose class matches the qualifier.
func TestSpellPrereqClass(t *testing.T) {
	c := check.New(t)

	e := NewEntity()

	// "Reflect" exists as both a Regular and a Blocking spell, so a name-based match alone can't tell them apart.
	regularReflect := addTestSpell(e, "Reflect", fxp.One)
	regularReflect.Class = "Regular"
	blockingReflect := addTestSpell(e, "Reflect", fxp.One)
	blockingReflect.Class = "Blocking"

	// "Fireball" exists as only a Regular spell.
	regularFireball := addTestSpell(e, "Fireball", fxp.One)
	regularFireball.Class = "Regular"

	req := NewSpellPrereq()
	req.SubType = spellcmp.Class
	req.QualifierCriteria.Compare = criteria.IsText
	req.QualifierCriteria.Qualifier = "Regular"

	// Exactly two Regular spells exist and using "exactly 2" rather than "at least 2" catches the non-Regular spells being accidentally included
	req.QuantityCriteria.Compare = criteria.EqualsNumber
	req.QuantityCriteria.Qualifier = fxp.FromInteger(2)
	c.True(
		req.Satisfied(e, nil, nil, "", nil),
		"the two Regular spells should satisfy an exactly-2 Regular requirement",
	)

	// Requiring more than two Regular spells must fail, since only two exist.
	req.QuantityCriteria.Compare = criteria.AtLeastNumber
	req.QuantityCriteria.Qualifier = fxp.FromInteger(3)
	var tooltip xbytes.InsertBuffer
	c.False(
		req.Satisfied(e, nil, &tooltip, "", nil),
		"only two Regular spells exist, so a more-than-2 requirement must fail",
	)
	c.Contains(
		tooltip.String(),
		"class",
		"the tooltip should mention 'class'",
	)
}
