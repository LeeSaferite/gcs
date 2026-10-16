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
	"context"
	"strings"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/nameable"
)

var _ condition.Host = &conditionHost{}

// conditionHost is the condition.Host for prerequisite trees evaluated against an entity, reproducing the searches
// and values the legacy prerequisites use. It caches the entity collections it builds, the #737 guards of the spells it
// inspects and the costly fields it reads, some of which run scripts, so it is not safe for concurrent use, and it must
// not outlive a change to the entity: one recalculation pass uses one while it judges prerequisites, which changes
// nothing they read.
type conditionHost struct {
	entity      *Entity
	units       fxp.WeightUnit
	guards      map[*Spell][]spellGuard
	pointed     []*Spell // the spells with points, once found
	collections map[collectionKey][]condition.Item
	fields      map[collectionKey]fieldValue
}

type fieldValue struct {
	v  condition.Value
	ok bool
}

type collectionKey struct {
	self condition.Item
	key  string
}

func newConditionHost(entity *Entity) *conditionHost {
	return &conditionHost{
		entity:      entity,
		units:       SheetSettingsFor(entity).DefaultWeightUnits,
		guards:      make(map[*Spell][]spellGuard),
		collections: make(map[collectionKey][]condition.Item),
		fields:      make(map[collectionKey]fieldValue),
	}
}

// Collection implements condition.Host. The item collections are relative to subject. The entity collections are
// built once each, except the spells, which the #737 guard makes depend on self.
func (h *conditionHost) Collection(key string, self, subject condition.Item) ([]condition.Item, bool) {
	switch key {
	case conditionChildren, conditionContents, conditionModifiers:
		return relativeCollection(key, subject), true
	default:
	}
	ck := collectionKey{key: key}
	if key == conditionSpells {
		ck.self = self
	}
	if list, ok := h.collections[ck]; ok {
		return list, true
	}
	list, ok := h.entityCollection(key, self)
	if ok {
		h.collections[ck] = list
	}
	return list, ok
}

// entityCollection returns one of the entity's collections.
func (h *conditionHost) entityCollection(key string, self condition.Item) ([]condition.Item, bool) {
	list := make([]condition.Item, 0)
	add := func(item condition.Item, include bool) {
		if include {
			list = append(list, item)
		}
	}
	switch key {
	case conditionTraits:
		Traverse(func(t *Trait) bool { add(t, true); return false }, true, false, h.entity.Traits...)
	case conditionSkills:
		Traverse(func(s *Skill) bool { add(s, s.AdjustedPoints(nil) != 0); return false }, true, true,
			h.entity.Skills...)
	case conditionSpells:
		if h.pointed == nil {
			h.pointed = make([]*Spell, 0)
			Traverse(func(s *Spell) bool {
				if s.AdjustedPoints(nil) != 0 {
					h.pointed = append(h.pointed, s)
				}
				return false
			}, true, true, h.entity.Spells...)
		}
		for _, s := range h.pointed {
			add(s, !h.requiresSpell(s, self))
		}
	case conditionEquipped:
		Traverse(func(e *Equipment) bool { add(e, e.ReallyEquipped()); return false }, false, false,
			h.entity.CarriedEquipment...)
	case conditionCarried:
		Traverse(func(e *Equipment) bool { add(e, true); return false }, true, false, h.entity.CarriedEquipment...)
	case conditionCarriedUnequipped:
		Traverse(func(e *Equipment) bool { add(e, carriedUnequipped(e)); return false }, false, false,
			h.entity.CarriedEquipment...)
	case conditionOtherEquipment:
		Traverse(func(e *Equipment) bool { add(e, true); return false }, true, false, h.entity.OtherEquipment...)
	default:
		return nil, false
	}
	return list, true
}

// carriedUnequipped returns true for equipment that is carried, enabled and not used up, nor inside a container that
// is, but not equipped.
func carriedUnequipped(e *Equipment) bool {
	if e.ReallyEquipped() || !e.Enabled() {
		return false
	}
	for one := e; one != nil; one = one.parent {
		if one.Quantity <= 0 {
			return false
		}
	}
	return true
}

// requiresSpell is the #737 circularity guard: true when the candidate's own prerequisites positively require the
// owning spell by name, so counting the candidate toward the owner's prerequisites would be circular. Owners other
// than spells have no guard.
func (h *conditionHost) requiresSpell(candidate *Spell, self condition.Item) bool {
	owner, ok := self.(*Spell)
	if !ok {
		return false
	}
	guards, ok := h.guards[candidate]
	if !ok {
		for _, t := range prereqTreeFor(candidate.Prereq, prereqOwnerOf(candidate)).guards {
			guard := spellGuard{matcher: condition.NewMatcher(t.node, h, condition.Options{})}
			if !strings.ContainsAny(t.name.Text.Qualifier, "{@") {
				guard.name = &t.name.Text
			}
			guards = append(guards, guard)
		}
		h.guards[candidate] = guards
	}
	for _, g := range guards {
		if g.name != nil && !g.name.Matches(nil, owner.NameWithReplacements()) {
			continue
		}
		if state, err := g.matcher.StateFor(context.Background(), candidate, owner); err == nil && state == condition.Met {
			return true
		}
	}
	return false
}

// spellGuard is one of the #737 guards of a spell, ready to check other spells against. name, when set, is the
// criterion of its first name check, which holds neither self tokens nor nameable markers, so it can be tried on its
// own first, exactly as the check would try it.
type spellGuard struct {
	name    *criteria.Text
	matcher *condition.Matcher
}

// spellGuardTree is one #737 guard of converted prerequisites: an all group of the checks a spell must meet for them
// to require it, the first being name, a name check.
type spellGuardTree struct {
	node *condition.Group
	name *condition.Check
}

// spellGuards appends the #737 guards of n, converted prerequisites, to guards, as the legacy guard has it. Each count
// over spells that n requires, being negated as often as the groups holding it are, along with negated, and whose
// where holds a non-negated name check gives a guard of the checks in that where, name checks first. Anything else in
// the where, the spell level and the tech level rule included, is ignored. Only checks are evaluated and no collection
// is enumerated, so the guard cannot recurse.
func spellGuards(n condition.Node, negated bool, guards []spellGuardTree) []spellGuardTree {
	switch t := n.(type) {
	case *condition.Group:
		negated = negated != t.Not
		for _, child := range t.Children {
			guards = spellGuards(child, negated, guards)
		}
	case *condition.Count:
		if t.Not != negated || t.Of != conditionSpells {
			return guards
		}
		checks := condition.Nodes{t.Where.Node}
		if g, ok := t.Where.Node.(*condition.Group); ok && g.Mode == condition.GroupAll && !g.Not && g.OnlyWhen.IsZero() {
			checks = g.Children
		}
		var names []*condition.Check
		var others condition.Nodes
		for _, one := range checks {
			c, ok := one.(*condition.Check)
			switch {
			case !ok || c.Field == conditionLevel:
			case c.Field == filterFieldKeyName && !c.Not && !c.Text.IsZero():
				names = append(names, c)
			default:
				others = append(others, c)
			}
		}
		if len(names) != 0 {
			children := make(condition.Nodes, 0, len(names)+len(others))
			for _, c := range names {
				children = append(children, c)
			}
			guards = append(guards, spellGuardTree{
				node: &condition.Group{Children: append(children, others...)},
				name: names[0],
			})
		}
	default:
	}
	return guards
}

// relativeCollection returns a collection relative to subject: its direct children, the contents of equipment (its
// children, with each group replaced by what it holds and each disabled item left out, as the contained quantity counts
// them), or its enabled modifiers.
func relativeCollection(key string, subject condition.Item) []condition.Item {
	var list []condition.Item
	switch t := subject.(type) {
	case *Trait:
		switch key {
		case conditionModifiers: // a container's modifiers apply to the traits it holds
			Traverse(func(m *TraitModifier) bool { list = append(list, m); return false }, true, true,
				t.AllModifiers()...)
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
		switch {
		case child.IsDisabled():
		case child.IsGroup():
			list = appendContents(list, child.Children)
		default:
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

// RunScript implements condition.Host by running the script as a legacy script prerequisite would, which bounds its
// time on its own.
func (h *conditionHost) RunScript(_ context.Context, s *condition.Script, self, _ condition.Item) (state condition.State, reason string) {
	result, reason := (&ScriptPrereq{Name: s.Name, Script: s.Script}).Evaluate(h.entity, self)
	switch result {
	case CheckMet:
		return condition.Met, reason
	case CheckUnmet:
		return condition.Unmet, reason
	default:
		return condition.Failed, reason
	}
}
