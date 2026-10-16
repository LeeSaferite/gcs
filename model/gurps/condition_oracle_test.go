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
	"fmt"
	"strings"

	"github.com/richardwilkes/gcs/v5/model/gurps/enums/prereq"
	"github.com/richardwilkes/gcs/v5/model/nameable"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xbytes"
)

// legacySatisfier is a prerequisite type with a legacy evaluation (see condition_oracle_leaf_test.go).
type legacySatisfier interface {
	Satisfied(entity *Entity, exclude any, buffer *xbytes.InsertBuffer, prefix string, hasEquipmentPenalty *bool) bool
}

// legacyLeaf returns the legacy evaluation of a prerequisite other than a list.
func legacyLeaf(one Prereq) legacySatisfier {
	leaf, ok := one.(legacySatisfier)
	if !ok {
		panic(fmt.Sprintf("%T has no legacy evaluation", one))
	}
	return leaf
}

// legacyEvaluate is the legacy PrereqList.Evaluate, kept as the reference the engine is tested against. It checks this
// list against the entity for the item given as exclude, returning its result. A list is skipped, along with everything
// in it, when it does not apply at the sheet's tech level, and is also skipped when it has nothing in it that isn't
// skipped. Its parent leaves it out. Of the rest, one unmet item makes an "all of" list unmet, and one met item makes
// an "any of" list met, even when others failed. Otherwise the list has failed when any of the rest has, and is met or
// unmet as they all are. A negated list then swaps met and unmet, leaving skipped and failed alone. A prerequisite of a
// type this version of GCS doesn't know has failed, so that negating it can't satisfy it. visit, if not nil, is called
// with the result of each prerequisite in the list and then of the list itself, along with the reason a script gives,
// which is the error when it couldn't run. Each script runs at most once, and none in a skipped list does. Without an
// entity, the list is met and nothing is visited.
func (p *PrereqList) legacyEvaluate(entity *Entity, exclude any, visit func(one Prereq, result CheckResult, reason string)) CheckResult {
	result, _ := p.legacyEval(&prereqEvaluation{entity: entity, exclude: exclude, visit: visit}, nil, nil, p.All, false)
	return result
}

// legacySatisfied is the legacy PrereqList.Satisfied, kept as the reference the engine is tested against. A list is
// satisfied when legacyEvaluate finds it met or skipped. hasEquipmentPenalty, if not nil, is set to true when this list
// is unsatisfied partly because equipment it requires to be equipped isn't: an unmet prerequisite to have it equipped,
// or, inside a negated list, a met prerequisite not to, reached through nested lists that each count against the one
// holding them. An "all of" list that also fails for some other prerequisite sets it, while one inside a nested list
// that doesn't count against its parent never does.
//
// The text written to buffer lists what is unmet. A skipped list writes nothing, a list with a single unmet item writes
// just that item, and a nested list with the same mode as its parent writes its items alongside its parent's. Any other
// list writes a heading with its items indented beneath it, which for a negated list are those that are met, or those
// that failed when the list did. This list is treated as though its caller were an "all of" list, so the items of an
// "all of" list are written without a heading.
func (p *PrereqList) legacySatisfied(entity *Entity, exclude any, buffer *xbytes.InsertBuffer, prefix string, hasEquipmentPenalty *bool) bool {
	result, _ := p.legacyEval(&prereqEvaluation{entity: entity, exclude: exclude, prefix: prefix}, buffer,
		hasEquipmentPenalty, p.All, false)
	return result == CheckMet || result == CheckSkipped
}

// prereqEvaluation holds what stays the same throughout an evaluation of a prerequisite list.
type prereqEvaluation struct {
	entity  *Entity
	exclude any
	visit   func(one Prereq, result CheckResult, reason string)
	prefix  string
}

// legacyEval is legacyEvaluate, also writing the text and setting the equipment penalty that legacySatisfied describes, and returning
// how many items of text it wrote at the level of the prefix. flatten requests that the unmet items of a list that
// isn't negated be written at that level rather than under a heading. negated is set when the lists holding this one
// are negated an odd number of times, which turns what each prerequisite asks for around: a met one then counts
// against the outermost list, and one that doesn't have equipment equipped asks that it be equipped.
func (p *PrereqList) legacyEval(ev *prereqEvaluation, buffer *xbytes.InsertBuffer, hasEquipmentPenalty *bool, flatten, negated bool) (result CheckResult, items int) {
	if ev.entity == nil {
		return CheckMet, 0
	}
	if !p.AppliesAt(ev.entity) {
		p.visitAll(ev.visit, CheckSkipped, "")
		return CheckSkipped, 0
	}
	var tally checkTally
	var local *xbytes.InsertBuffer
	if buffer != nil {
		local = &xbytes.InsertBuffer{}
	}
	var met []Prereq
	eqpPenalty := false
	inner := negated != p.Not
	for _, one := range p.Prereqs {
		out := local
		if p.Not && local != nil {
			// What is unmet doesn't count against a negated list, so only what failed is kept.
			out = &xbytes.InsertBuffer{}
		}
		childResult := CheckMet
		if list, ok := one.(*PrereqList); ok {
			var n int
			childResult, n = list.legacyEval(ev, out, &eqpPenalty, list.All == p.All && !p.Not, inner)
			items += n
		} else {
			var reason string
			penalty := &eqpPenalty
			if inner {
				penalty = nil
			}
			switch typed := one.(type) {
			case *ScriptPrereq:
				childResult, reason = typed.legacyEvaluate(ev.entity, ev.exclude, out, ev.prefix)
			case *UnknownPrereq:
				typed.Satisfied(ev.entity, ev.exclude, out, ev.prefix, nil)
				childResult = CheckFailed
			default:
				if !legacyLeaf(one).Satisfied(ev.entity, ev.exclude, out, ev.prefix, penalty) {
					childResult = CheckUnmet
				}
			}
			// Negated, one that doesn't have equipment equipped asks that it be, and is unmet when it is met.
			if eqp, isEqp := one.(*EquippedEquipmentPrereq); isEqp && inner && childResult == CheckMet && eqp.Not &&
				eqp.Type == prereq.EquippedEquipment {
				eqpPenalty = true
			}
			if childResult != CheckMet {
				items++
			}
			if ev.visit != nil {
				ev.visit(one, childResult, reason)
			}
		}
		if p.Not && local != nil {
			switch childResult {
			case CheckMet:
				met = append(met, one)
			case CheckFailed:
				local.WriteString(out.String())
			default:
			}
		}
		tally.add(childResult)
	}
	result = tally.combine(p.All).negate(p.Not)
	if ev.visit != nil {
		ev.visit(p, result, "")
	}
	// The penalty goes up when this list counts against the lists holding it: when it has failed, which no negation
	// changes, when it is unmet and they don't negate it, or when it is met and they do.
	if eqpPenalty && hasEquipmentPenalty != nil && result != CheckSkipped &&
		(result == CheckFailed || (result == CheckMet) == negated) {
		*hasEquipmentPenalty = true
	}
	if result == CheckMet || result == CheckSkipped {
		return result, 0
	}
	if buffer == nil {
		return result, 0
	}
	if p.Not {
		buffer.WriteString(ev.prefix)
		if p.All {
			buffer.WriteString(i18n.Text("Requires not all of:"))
		} else {
			buffer.WriteString(i18n.Text("Requires none of:"))
		}
		if result == CheckFailed {
			buffer.WriteString(strings.ReplaceAll(local.String(), "\n", "\n\t"))
			return result, 1
		}
		var replacements map[string]string
		if na, ok := ev.exclude.(nameable.Accesser); ok {
			replacements = na.NameableReplacements()
		}
		for _, one := range met {
			buffer.WriteString(strings.ReplaceAll(ev.prefix+one.Describe(ev.entity, replacements, plainText), "\n",
				"\n\t"))
		}
		return result, 1
	}
	if flatten || items == 1 {
		buffer.WriteString(local.String())
		return result, items
	}
	buffer.WriteString(ev.prefix)
	if p.All {
		buffer.WriteString(i18n.Text("Requires all of:"))
	} else {
		buffer.WriteString(i18n.Text("Requires at least one of:"))
	}
	buffer.WriteString(strings.ReplaceAll(local.String(), "\n", "\n\t"))
	return result, 1
}
