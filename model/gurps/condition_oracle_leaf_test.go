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
	"strings"

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xbytes"
)

// This file holds the legacy evaluation of each prerequisite type, which production code no longer uses: the
// differential tests compare the condition engine against it, and the older unit tests exercise it directly.

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against.
func (p *TraitPrereq) Satisfied(entity *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {
	if entity == nil {
		return true
	}
	var replacements map[string]string
	if na, ok := exclude.(nameable.Accesser); ok {
		replacements = na.NameableReplacements()
	}
	satisfied := false
	Traverse(func(t *Trait) bool {
		if exclude == t || !p.NameCriteria.Matches(replacements, t.NameWithReplacements()) {
			return false
		}
		notes := t.Notes()
		if modNotes := t.ModifierNotes(); modNotes != "" {
			notes += "\n" + modNotes
		}
		if !p.NotesCriteria.Matches(replacements, notes) {
			return false
		}
		var levels fxp.Int
		if t.IsLeveled() {
			levels = t.CurrentLevel()
		}
		satisfied = p.LevelCriteria.Matches(levels)
		return satisfied
	}, true, false, entity.Traits...)
	if !p.Has {
		satisfied = !satisfied
	}
	if !satisfied && tooltip != nil {
		tooltip.WriteString(prefix)
		tooltip.WriteString(p.Describe(entity, replacements, plainText))
	}
	return satisfied
}

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against.
func (p *SkillPrereq) Satisfied(entity *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {
	if entity == nil {
		return true
	}
	var replacements map[string]string
	if na, ok := exclude.(nameable.Accesser); ok {
		replacements = na.NameableReplacements()
	}
	satisfied := false
	var techLevel *string
	if sk, ok := exclude.(*Skill); ok {
		techLevel = sk.TechLevel
	}
	Traverse(func(sk *Skill) bool {
		// A skill with no points spent on it doesn't count, even if it has a level from a default.
		if exclude == sk || sk.AdjustedPoints(nil) == 0 ||
			!p.NameCriteria.Matches(replacements, sk.NameWithReplacements()) ||
			!p.SpecializationCriteria.Matches(replacements, sk.SpecializationWithReplacements()) ||
			!p.OptionalSpecializationCriteria.Matches(replacements, sk.OptionalSpecializationWithReplacements()) {
			return false
		}
		satisfied = p.LevelCriteria.Matches(sk.LevelData.Level)
		if satisfied && techLevel != nil {
			satisfied = sk.TechLevel == nil || *techLevel == *sk.TechLevel
		}
		return satisfied
	}, false, true, entity.Skills...)
	if !p.Has {
		satisfied = !satisfied
	}
	if !satisfied && tooltip != nil {
		tooltip.WriteString(prefix)
		tooltip.WriteString(p.Describe(entity, replacements, plainText))
		if techLevel != nil {
			tooltip.WriteString(i18n.Text(" with a matching tech level"))
		}
	}
	return satisfied
}

// powerSourceMatches returns true if a candidate spell's power source passes this prerequisite's filter. For the "same
// as this spell" form, ownerPowerSource is the owning spell's resolved power source; nil (the prerequisite does not
// belong to a spell) matches nothing.
func (p *SpellPrereq) powerSourceMatches(replacements map[string]string, candidate string,
	ownerPowerSource *string,
) bool {
	if p.SamePowerSource {
		return ownerPowerSource != nil && strings.EqualFold(candidate, *ownerPowerSource)
	}
	return p.PowerSourceCriteria.Matches(replacements, candidate)
}

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against.
func (p *SpellPrereq) Satisfied(entity *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {
	if entity == nil {
		return true
	}
	var replacements map[string]string
	if na, ok := exclude.(nameable.Accesser); ok {
		replacements = na.NameableReplacements()
	}
	var techLevel *string
	var ownerName string
	var ownerPowerSource *string
	excludeSpell, isSpell := exclude.(*Spell)
	if isSpell {
		techLevel = excludeSpell.TechLevel
		// Resolve the owning spell's name and power source once rather than once per candidate spell, since a nameable
		// marker in either would otherwise be re-applied for every spell on the sheet.
		ownerName = excludeSpell.NameWithReplacements()
		resolved := excludeSpell.PowerSourceWithReplacements()
		ownerPowerSource = &resolved
	}
	filterPowerSource := p.hasPowerSourceFilter()
	count := 0
	colleges := make(map[string]bool)
	Traverse(func(sp *Spell) bool {
		if exclude == sp || sp.AdjustedPoints(nil) == 0 {
			return false
		}
		// Don't count a spell that, in turn, directly requires the spell being checked, since doing so would create a
		// circular prerequisite relationship (see GitHub issue #737).
		if excludeSpell != nil && spellDirectlyRequiresNamed(sp, ownerName, *ownerPowerSource) {
			return false
		}
		if techLevel != nil && sp.TechLevel != nil && *techLevel != *sp.TechLevel {
			return false
		}
		if filterPowerSource && !p.powerSourceMatches(replacements, sp.PowerSourceWithReplacements(), ownerPowerSource) {
			return false
		}
		switch p.SubType {
		case spellcmp.Name:
			if p.QualifierCriteria.Matches(replacements, sp.NameWithReplacements()) {
				count++
			}
		case spellcmp.Tag:
			for _, one := range sp.Tags {
				if p.QualifierCriteria.Matches(replacements, one) {
					count++
					break
				}
			}
		case spellcmp.College:
			for _, one := range sp.CollegeWithReplacements() {
				if p.QualifierCriteria.Matches(replacements, one) {
					count++
					break
				}
			}
		case spellcmp.CollegeCount:
			for _, one := range sp.CollegeWithReplacements() {
				colleges[one] = true
			}
		case spellcmp.Any:
			count++
		}
		return false
	}, false, true, entity.Spells...)
	if p.SubType == spellcmp.CollegeCount {
		count = len(colleges)
	}
	satisfied := p.QuantityCriteria.Matches(fxp.FromInteger(count))
	if !p.Has {
		satisfied = !satisfied
	}
	if !satisfied && tooltip != nil {
		tooltip.WriteString(prefix)
		tooltip.WriteString(p.Describe(entity, replacements, plainText))
		// Name the owning spell's resolved power source, so that one that is blank or spelled differently from the
		// spells it expects to match can be diagnosed from the tooltip alone.
		if p.SamePowerSource && ownerPowerSource != nil {
			tooltip.WriteString(` ("` + *ownerPowerSource + `")`)
		}
	}
	return satisfied
}

// spellDirectlyRequires returns true if the candidate spell directly lists a spell-by-name prerequisite that matches
// the target spell. It keeps a spell from being counted as a prerequisite for a spell that it, in turn, requires, which
// would create a circular prerequisite relationship.
func spellDirectlyRequires(candidate, target *Spell) bool {
	if candidate == nil || target == nil {
		return false
	}
	return spellDirectlyRequiresNamed(candidate, target.NameWithReplacements(), target.PowerSourceWithReplacements())
}

// spellDirectlyRequiresNamed is spellDirectlyRequires with the target spell's name and power source already resolved,
// for callers that check many candidates against the same target.
func spellDirectlyRequiresNamed(candidate *Spell, targetName, targetPowerSource string) bool {
	if candidate == nil || candidate.Prereq == nil {
		return false
	}
	candidatePowerSource := candidate.PowerSourceWithReplacements()
	return prereqListRequiresSpell(candidate.Prereq, candidate.NameableReplacements(), &candidatePowerSource, targetName,
		targetPowerSource)
}

// prereqListRequiresSpell returns true if the given prereq list holds a spell-by-name prerequisite (at any nesting
// depth) that is required (Has), matches the target spell's name, and whose power source filter, if any, the target
// spell's power source passes. The replacements and owner power source are those of the spell the prereq list belongs
// to: its qualifiers resolve against the former, and a "same power source as this spell" filter compares to the latter.
func prereqListRequiresSpell(list *PrereqList, replacements map[string]string, ownerPowerSource *string, targetName,
	targetPowerSource string,
) bool {
	if list == nil {
		return false
	}
	for _, one := range list.Prereqs {
		switch p := one.(type) {
		case *PrereqList:
			if prereqListRequiresSpell(p, replacements, ownerPowerSource, targetName, targetPowerSource) {
				return true
			}
		case *SpellPrereq:
			if p.Has && p.SubType == spellcmp.Name && p.QualifierCriteria.Matches(replacements, targetName) &&
				p.powerSourceMatches(replacements, targetPowerSource, ownerPowerSource) {
				return true
			}
		}
	}
	return false
}

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against.
func (p *AttributePrereq) Satisfied(entity *Entity, _ any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {
	if entity == nil {
		return true
	}
	value := entity.ResolveAttributeCurrent(p.Which)
	if p.CombinedWith != "" {
		value += entity.ResolveAttributeCurrent(p.CombinedWith)
	}
	satisfied := p.QualifierCriteria.Matches(value)
	if !p.Has {
		satisfied = !satisfied
	}
	if !satisfied && tooltip != nil {
		tooltip.WriteString(prefix)
		tooltip.WriteString(p.Describe(entity, nil, plainText))
	}
	return satisfied
}

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against.
func (p *ContainedQuantityPrereq) Satisfied(_ *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {
	satisfied := false
	if eqp, ok := exclude.(*Equipment); ok {
		if satisfied = !eqp.Container(); !satisfied {
			satisfied = p.QualifierCriteria.Matches(containedQuantity(eqp.Children))
		}
	}
	if !p.Has {
		satisfied = !satisfied
	}
	if !satisfied && tooltip != nil {
		tooltip.WriteString(prefix)
		tooltip.WriteString(p.Describe(nil, nil, plainText))
	}
	return satisfied
}

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against.
func (p *ContainedWeightPrereq) Satisfied(entity *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {
	satisfied := false
	if eqp, ok := exclude.(*Equipment); ok {
		if satisfied = !eqp.Container(); !satisfied {
			satisfied = p.WeightCriteria.Matches(eqp.ContainedWeight(false, SheetSettingsFor(entity).DefaultWeightUnits))
		}
	}
	if !p.Has {
		satisfied = !satisfied
	}
	if !satisfied && tooltip != nil {
		tooltip.WriteString(prefix)
		tooltip.WriteString(p.Describe(entity, nil, plainText))
	}
	return satisfied
}

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against.
func (p *EquippedEquipmentPrereq) Satisfied(entity *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string, hasEquipmentPenalty *bool) bool {
	if entity == nil {
		return true
	}
	var replacements map[string]string
	if na, ok := exclude.(nameable.Accesser); ok {
		replacements = na.NameableReplacements()
	}
	satisfied := false
	Traverse(func(eqp *Equipment) bool {
		satisfied = exclude != eqp && eqp.ReallyEquipped() &&
			p.NameCriteria.Matches(replacements, eqp.NameWithReplacements()) &&
			p.TagsCriteria.MatchesList(replacements, eqp.Tags...)
		return satisfied
	}, false, false, entity.CarriedEquipment...)
	if !satisfied {
		if hasEquipmentPenalty != nil {
			*hasEquipmentPenalty = true
		}
		if tooltip != nil {
			tooltip.WriteString(prefix)
			tooltip.WriteString(p.Describe(entity, replacements, plainText))
		}
	}
	return satisfied
}

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against. An
// unknown prerequisite is never satisfied.
func (p *UnknownPrereq) Satisfied(_ *Entity, _ any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {
	if tooltip != nil {
		tooltip.WriteString(prefix)
		tooltip.WriteString(p.Describe(nil, nil, plainText))
	}
	return false
}

// Satisfied is the legacy evaluation of the prerequisite, kept as the reference the engine is tested against. An unmet
// prerequisite writes its description, followed by the reason it gives, if any. A reason of more than one line goes
// on the lines below it, one level deeper.
func (s *ScriptPrereq) Satisfied(entity *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {
	result, _ := s.legacyEvaluate(entity, exclude, tooltip, prefix)
	return result == PrereqMet
}

// legacyEvaluate is Satisfied, returning the script's result and the reason it gives.
func (s *ScriptPrereq) legacyEvaluate(entity *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string) (result PrereqResult, reason string) {
	result, reason = s.Evaluate(entity, exclude)
	reason = strings.TrimSpace(reason)
	if result != PrereqMet && tooltip != nil {
		var replacements map[string]string
		if na, ok := exclude.(nameable.Accesser); ok {
			replacements = na.NameableReplacements()
		}
		s.writeUnmet(tooltip, entity, replacements, prefix, result, reason)
	}
	return result, reason
}
