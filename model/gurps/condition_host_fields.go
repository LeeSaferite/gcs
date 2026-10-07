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
	"github.com/richardwilkes/gcs/v5/model/fxp"
)

// Field implements condition.Host with the values the legacy prerequisites read.
func (h *conditionHost) Field(item condition.Item, key string) (condition.Value, bool) {
	switch t := item.(type) {
	case *Trait:
		return traitField(t, key)
	case *Skill:
		return skillField(t, key)
	case *Spell:
		return spellField(t, key)
	case *Equipment:
		return h.equipmentField(t, key)
	case *TraitModifier:
		return modifierField(key, t.NameWithReplacements(), t.Tags, t.Container())
	case *EquipmentModifier:
		return modifierField(key, t.NameWithReplacements(), t.Tags, t.Container())
	default:
		return condition.Value{}, false
	}
}

// traitField returns a trait's field. Its notes are the trait's own followed by those of its modifiers, and its level
// is 0 unless it is leveled, as the trait prerequisite reads them.
func traitField(t *Trait, key string) (condition.Value, bool) {
	switch key {
	case filterFieldKeyNotes:
		notes := t.Notes()
		if modNotes := t.ModifierNotes(); modNotes != "" {
			notes += "\n" + modNotes
		}
		return textValue(notes), true
	case conditionLevel:
		var level fxp.Int
		if t.IsLeveled() {
			level = t.CurrentLevel()
		}
		return numberValue(level), true
	default:
		return commonField(key, t.NameWithReplacements(), t.Tags, t.AdjustedPoints(nil), t.Container())
	}
}

func skillField(s *Skill, key string) (condition.Value, bool) {
	switch key {
	case conditionSpecialization:
		return textValue(s.SpecializationWithReplacements()), true
	case conditionOptionalSpecialty:
		return textValue(s.OptionalSpecializationWithReplacements()), true
	case conditionLevel:
		return numberValue(s.LevelData.Level), true
	case filterFieldKeyTechLevel:
		return techLevelValue(s.TechLevel), true
	case filterFieldKeyNotes:
		return textValue(s.Notes()), true
	default:
		return commonField(key, s.NameWithReplacements(), s.Tags, s.AdjustedPoints(nil), s.Container())
	}
}

func spellField(s *Spell, key string) (condition.Value, bool) {
	switch key {
	case conditionCollege:
		return condition.Value{Kind: condition.ValueList, List: s.CollegeWithReplacements()}, true
	case conditionPowerSource:
		return textValue(s.PowerSourceWithReplacements()), true
	case conditionLevel:
		return numberValue(s.LevelData.Level), true
	case filterFieldKeyTechLevel:
		return techLevelValue(s.TechLevel), true
	case filterFieldKeyNotes:
		return textValue(s.Notes()), true
	default:
		return commonField(key, s.NameWithReplacements(), s.Tags, s.AdjustedPoints(nil), s.Container())
	}
}

// equipmentField returns a piece of equipment's field. What a container holds is measured as the contained weight
// and quantity prerequisites measure it.
func (h *conditionHost) equipmentField(e *Equipment, key string) (condition.Value, bool) {
	switch key {
	case filterFieldKeyName:
		return textValue(e.NameWithReplacements()), true
	case filterFieldKeyNotes:
		return textValue(e.Notes()), true
	case filterFieldKeyTags:
		return condition.Value{Kind: condition.ValueList, List: e.Tags}, true
	case filterFieldKeyTechLevel:
		return textValue(e.TechLevel), true
	case filterFieldKeyContainer:
		return boolValue(e.Container()), true
	case conditionContainedWeight:
		return condition.Value{Kind: condition.ValueWeight, Number: fxp.Int(e.ContainedWeight(false, h.units))}, true
	case conditionContainedQuantity:
		return numberValue(containedQuantity(e.Children)), true
	default:
		return condition.Value{}, false
	}
}

func commonField(key, name string, tags []string, points fxp.Int, container bool) (condition.Value, bool) {
	if key == filterFieldKeyPoints {
		return numberValue(points), true
	}
	return modifierField(key, name, tags, container)
}

func modifierField(key, name string, tags []string, container bool) (condition.Value, bool) {
	switch key {
	case filterFieldKeyName:
		return textValue(name), true
	case filterFieldKeyTags:
		return condition.Value{Kind: condition.ValueList, List: tags}, true
	case filterFieldKeyContainer:
		return boolValue(container), true
	default:
		return condition.Value{}, false
	}
}

func textValue(s string) condition.Value {
	return condition.Value{Kind: condition.ValueText, Text: s}
}

func numberValue(n fxp.Int) condition.Value {
	return condition.Value{Kind: condition.ValueNumber, Number: n}
}

func boolValue(b bool) condition.Value {
	return condition.Value{Kind: condition.ValueBool, Bool: b}
}

// techLevelValue returns a skill or spell's tech level: Absent when it has none, which the tech level rule passes.
func techLevelValue(tl *string) condition.Value {
	if tl == nil {
		return condition.Value{Kind: condition.ValueAbsent}
	}
	return textValue(*tl)
}
