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
	"github.com/richardwilkes/gcs/v5/model/nameable"
)

var _ condition.Host = &conditionHost{}

// conditionHost is the condition.Host for prerequisite trees evaluated against an entity, reproducing the searches
// and values the legacy prerequisites use. It caches the converted prerequisites of the spells the #737 guard
// inspects, so it is not safe for concurrent use, and it should not outlive a change to the entity.
type conditionHost struct {
	entity *Entity
	units  fxp.WeightUnit
	spells map[*Spell]condition.Node
}

func newConditionHost(entity *Entity) *conditionHost {
	return &conditionHost{
		entity: entity,
		units:  SheetSettingsFor(entity).DefaultWeightUnits,
		spells: make(map[*Spell]condition.Node),
	}
}

// Collection implements condition.Host. The entity collections leave self out; the item collections are relative to
// subject.
func (h *conditionHost) Collection(key string, self, subject condition.Item) ([]condition.Item, bool) {
	var list []condition.Item
	add := func(item condition.Item, include bool) {
		if include && item != self {
			list = append(list, item)
		}
	}
	switch key {
	case conditionTraits:
		Traverse(func(t *Trait) bool { add(t, true); return false }, true, false, h.entity.Traits...)
	case conditionSkills:
		Traverse(func(s *Skill) bool { add(s, s.AdjustedPoints(nil) != 0); return false }, false, true,
			h.entity.Skills...)
	case conditionSpells:
		Traverse(func(s *Spell) bool {
			add(s, s.AdjustedPoints(nil) != 0 && !h.requiresSpell(s, self))
			return false
		}, false, true, h.entity.Spells...)
	case conditionEquipped:
		Traverse(func(e *Equipment) bool { add(e, e.ReallyEquipped()); return false }, false, false,
			h.entity.CarriedEquipment...)
	case conditionCarried:
		Traverse(func(e *Equipment) bool { add(e, true); return false }, false, false, h.entity.CarriedEquipment...)
	case conditionOtherEquipment:
		Traverse(func(e *Equipment) bool { add(e, true); return false }, false, false, h.entity.OtherEquipment...)
	case conditionChildren, conditionContents, conditionModifiers:
		return relativeCollection(key, subject), true
	default:
		return nil, false
	}
	return list, true
}

// requiresSpell is the #737 circularity guard: true when the candidate's own prerequisites positively require the
// owning spell by name, so counting the candidate toward the owner's prerequisites would be circular. Owners other
// than spells have no guard.
func (h *conditionHost) requiresSpell(candidate *Spell, self condition.Item) bool {
	owner, ok := self.(*Spell)
	if !ok {
		return false
	}
	tree, ok := h.spells[candidate]
	if !ok {
		tree = conditionFromPrereqs(candidate.Prereq, candidate)
		h.spells[candidate] = tree
	}
	return condition.PositivelyRequires(tree, conditionSpells, filterFieldKeyName, h, candidate, owner)
}

// relativeCollection returns a collection relative to subject: its direct children, the contents of equipment (its
// children, with each group replaced by what it holds, as the contained quantity counts them), or its enabled
// modifiers.
func relativeCollection(key string, subject condition.Item) []condition.Item {
	var list []condition.Item
	switch t := subject.(type) {
	case *Trait:
		switch key {
		case conditionModifiers:
			Traverse(func(m *TraitModifier) bool { list = append(list, m); return false }, true, true, t.Modifiers...)
		case conditionChildren:
			list = appendItems(list, t.Children)
		default:
		}
	case *Skill:
		if key == conditionChildren {
			list = appendItems(list, t.Children)
		}
	case *Spell:
		if key == conditionChildren {
			list = appendItems(list, t.Children)
		}
	case *Equipment:
		switch key {
		case conditionModifiers:
			Traverse(func(m *EquipmentModifier) bool { list = append(list, m); return false }, true, true,
				t.Modifiers...)
		case conditionChildren:
			list = appendItems(list, t.Children)
		default:
			list = appendContents(list, t.Children)
		}
	}
	return list
}

func appendItems[T any](list []condition.Item, items []T) []condition.Item {
	for _, one := range items {
		list = append(list, one)
	}
	return list
}

func appendContents(list []condition.Item, children []*Equipment) []condition.Item {
	for _, child := range children {
		if child.IsGroup() {
			list = appendContents(list, child.Children)
		} else {
			list = append(list, child)
		}
	}
	return list
}

// Attribute implements condition.Host. Like the legacy prerequisites, it reads an unknown attribute as the minimum
// value.
func (h *conditionHost) Attribute(key string) (fxp.Int, bool) {
	return h.entity.ResolveAttributeCurrent(key), true
}

// Entity implements condition.Host. The tech level is read as the legacy when_tl reads it, never below 0.
func (h *conditionHost) Entity(key string) (fxp.Int, bool) {
	if key != filterFieldKeyTechLevel {
		return 0, false
	}
	tl, _, _ := ExtractTechLevel(h.entity.Profile.TechLevel)
	return max(tl, 0), true
}

// Replacements implements condition.Host with the owner's replacements.
func (h *conditionHost) Replacements(self, _ condition.Item) map[string]string {
	if na, ok := self.(nameable.Accesser); ok {
		return na.NameableReplacements()
	}
	return nil
}

// RunScript implements condition.Host by running the script as a legacy script prerequisite would.
func (h *conditionHost) RunScript(s *condition.Script, self, _ condition.Item) (state condition.State, reason string) {
	result, reason := (&ScriptPrereq{Name: s.Name, Script: s.Script}).Evaluate(h.entity, self)
	switch result {
	case PrereqMet:
		return condition.Met, reason
	case PrereqUnmet:
		return condition.Unmet, reason
	default:
		return condition.Failed, reason
	}
}

// prereqEquipmentPenalty returns true when a Collect result of a converted prerequisite tree earns a skill or spell the
// missing-equipment penalty: an Unmet count over equipped reached only through groups that don't pass, stopping at
// Not, as PrereqList.Satisfied sets it.
func prereqEquipmentPenalty(r *condition.Result) bool {
	if r.State.Passes() {
		return false
	}
	switch n := r.Node.(type) {
	case *condition.Count:
		return n.Of == conditionEquipped && !n.Not && r.State == condition.Unmet
	case *condition.Group:
		if !n.Not {
			for i := range r.Children {
				if prereqEquipmentPenalty(&r.Children[i]) {
					return true
				}
			}
		}
	default:
	}
	return false
}
