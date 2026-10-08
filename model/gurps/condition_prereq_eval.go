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
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/prereq"
	"github.com/richardwilkes/toolbox/v2/xbytes"
)

// Evaluate checks this list against the entity for the item given as exclude, returning its result. A list is skipped,
// along with everything in it, when it does not apply at the sheet's tech level, and is also skipped when it has
// nothing in it that isn't skipped. Its parent leaves it out, so an "all of" list is met when the rest are all met and
// an "any of" list when any of the rest is. A list that isn't met has failed when any of the rest has, and is otherwise
// unmet. visit, if not nil, is called with the result of each prerequisite in the list and then of the list itself,
// along with the reason a script gives, which is the error when it couldn't run. Each script runs at most once, and
// none in a skipped list does. Without an entity, the list is met and nothing is visited.
func (p *PrereqList) Evaluate(entity *Entity, exclude any, visit func(one Prereq, result PrereqResult, reason string)) PrereqResult {
	if entity == nil {
		return PrereqMet
	}
	r := prereqTreeFor(p, prereqOwnerOf(exclude)).evaluate(newConditionHost(entity), exclude, condition.Collect)
	result, _ := newPrereqWalk(entity, exclude, visit, "").list(p, &r, nil, p.All)
	return result
}

// Satisfied returns true when Evaluate finds this list met or skipped, as it always does without an entity.
// hasEquipmentPenalty, if not nil, is set to true when this list is unsatisfied and an unmet equipped-equipment
// prerequisite is among what made it so: one of its own, or one reached through nested lists that are each
// unsatisfied. An "all of" list that also fails for some other prerequisite sets it, while an unmet equipment
// prerequisite inside a satisfied or skipped nested list never does.
//
// The text written to buffer lists what is unmet, each item preceded by prefix. A skipped list writes nothing, a list
// with a single unmet item writes just that item, and a nested list with the same mode as its parent writes its items
// alongside its parent's. Any other list writes a heading with its items indented beneath it. This list is treated as
// though its caller were an "all of" list, so the items of an "all of" list are written without a heading.
func (p *PrereqList) Satisfied(entity *Entity, exclude any, buffer *xbytes.InsertBuffer, prefix string, hasEquipmentPenalty *bool) bool {
	if entity == nil {
		return true
	}
	return prereqsSatisfied(p, newConditionHost(entity), exclude, buffer, prefix, hasEquipmentPenalty)
}

// prereqsSatisfied is Satisfied with a host that may be shared by every list one recalculation pass checks. It is a
// variable so the differential tests can recalculate a sheet with the legacy evaluation in its place.
var prereqsSatisfied = (*PrereqList).satisfied

// satisfied is Satisfied with the host to evaluate with. A YesNo evaluation decides the common case, a satisfied list;
// an unsatisfied one is then explained, which runs only the detail pass, for its text and equipment penalty. A list
// holding scripts is evaluated in Collect mode from the start instead, so each script runs only once.
func (p *PrereqList) satisfied(h *conditionHost, exclude any, buffer *xbytes.InsertBuffer, prefix string, hasEquipmentPenalty *bool) bool {
	tree := prereqTreeFor(p, prereqOwnerOf(exclude))
	mode := condition.YesNo
	if tree.scripts {
		mode = condition.Collect
	}
	r := tree.evaluate(h, exclude, mode)
	if r.State.Passes() {
		return true
	}
	if mode != condition.Collect {
		r = condition.Explain(&r, h, exclude, condition.Options{})
	}
	if hasEquipmentPenalty != nil && prereqEquipmentPenalty(&r) {
		*hasEquipmentPenalty = true
	}
	if buffer != nil {
		newPrereqWalk(h.entity, exclude, nil, prefix).list(p, &r, buffer, p.All)
	}
	return false
}

// UnsatisfiedPrereqText returns what one prerequisite, a list or a single one, says when unsatisfied, each item
// preceded by prefix, or an empty string when it is satisfied. A single prerequisite is checked as the only item of an
// "all of" list, ignoring the tech level conditions of the lists holding it.
func UnsatisfiedPrereqText(one Prereq, entity *Entity, exclude any, prefix string) string {
	list, ok := one.(*PrereqList)
	if !ok {
		list = &PrereqList{Type: prereq.List, All: true}
		list.Prereqs = Prereqs{one.Clone(list)}
	}
	var buffer xbytes.InsertBuffer
	list.Satisfied(entity, exclude, &buffer, prefix, nil)
	return buffer.String()
}

// prereqResultOf returns the PrereqResult of an engine state.
func prereqResultOf(state condition.State) PrereqResult {
	switch state {
	case condition.Met:
		return PrereqMet
	case condition.Unmet:
		return PrereqUnmet
	case condition.Skipped:
		return PrereqSkipped
	default:
		return PrereqFailed
	}
}
