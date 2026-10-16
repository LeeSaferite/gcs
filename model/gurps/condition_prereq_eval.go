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
// unmet. A negated list then swaps met and unmet, leaving skipped and failed alone. visit, if not nil, is called with the result of each prerequisite in the list and then of the list itself,
// along with the reason a script gives, which is the error when it couldn't run. Each script runs at most once, and
// none in a skipped list does. Without an entity, the list is met and nothing is visited.
func (p *PrereqList) Evaluate(entity *Entity, exclude any, visit func(one Prereq, result CheckResult, reason string)) CheckResult {
	if entity == nil {
		return CheckMet
	}
	r := prereqTreeFor(p, prereqOwnerOf(exclude)).evaluate(newConditionHost(entity), exclude, condition.Collect)
	result, _ := newPrereqWalk(entity, exclude, visit, "").list(p, &r, nil, p.All)
	return result
}

// Satisfied returns true when Evaluate finds this list met or skipped, as it always does without an entity.
// hasEquipmentPenalty, if not nil, is set to true when this list is unsatisfied partly because equipment it requires to
// be equipped isn't: an unmet prerequisite to have it equipped, or, inside a negated list, a met prerequisite not to,
// reached through nested lists that each count against the one holding them. An "all of" list that also fails for some
// other prerequisite sets it, while one inside a nested list that doesn't count against its parent never does.
//
// The text written to buffer lists what is unmet, each item preceded by prefix. A skipped list writes nothing, a list
// with a single unmet item writes just that item, and a nested list with the same mode as its parent writes its items
// alongside its parent's. Any other list writes a heading with its items indented beneath it, which for a negated list
// are those that are met, or those that failed when the list did. This list is treated as though its caller were an
// "all of" list, so the items of an "all of" list are written without a heading.
func (p *PrereqList) Satisfied(entity *Entity, exclude any, buffer *xbytes.InsertBuffer, prefix string, hasEquipmentPenalty *bool) bool {
	if entity == nil {
		return true
	}
	return prereqsSatisfied(p, newConditionHost(entity), exclude, buffer, prefix, hasEquipmentPenalty)
}

// prereqsSatisfied is Satisfied with a host that may be shared by every list one recalculation pass checks. It is a
// variable so the differential tests can recalculate a sheet with the legacy evaluation in its place.
var prereqsSatisfied = (*PrereqList).satisfied

// satisfied is Satisfied with the host to evaluate with. The state is decided first, and an unsatisfied list is then
// explained, for its text and equipment penalty.
func (p *PrereqList) satisfied(h *conditionHost, exclude any, buffer *xbytes.InsertBuffer, prefix string, hasEquipmentPenalty *bool) bool {
	r := prereqTreeFor(p, prereqOwnerOf(exclude)).evaluate(h, exclude, condition.Explain)
	if r.State.Passes() {
		return true
	}
	if hasEquipmentPenalty != nil && prereqEquipmentPenalty(p, &r, false) {
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

// checkResultOf returns the CheckResult of an engine state.
func checkResultOf(state condition.State) CheckResult {
	switch state {
	case condition.Met:
		return CheckMet
	case condition.Unmet:
		return CheckUnmet
	case condition.Skipped:
		return CheckSkipped
	default:
		return CheckFailed
	}
}

// prereqEquipmentPenalty returns true when list, whose Collect result is r, earns a skill or spell the missing-equipment
// penalty, as PrereqList.Satisfied sets it. negated is set when the lists holding it are negated an odd number of
// times. Equipment counts when a prerequisite, turned around by the negations it sits under, requires it to be equipped
// and that requirement isn't met. A list passes that on when it counts against the lists holding it: when it has
// failed, which no negation changes, or when it is unmet and they don't negate it, or met and they do.
func prereqEquipmentPenalty(list *PrereqList, r *condition.Result, negated bool) bool {
	if r.State == condition.Skipped || len(r.Children) != len(list.Prereqs) {
		return false
	}
	inner := negated != list.Not
	found := false
	for i, one := range list.Prereqs {
		child := &r.Children[i]
		switch p := one.(type) {
		case *PrereqList:
			found = prereqEquipmentPenalty(p, child, inner) || found
		case *EquippedEquipmentPrereq:
			want := condition.Unmet
			if inner {
				want = condition.Met
			}
			found = found || (p.Type == prereq.EquippedEquipment && p.Not == inner && child.State == want)
		default:
		}
	}
	return found && (r.State == condition.Failed || (r.State == condition.Met) == negated)
}
