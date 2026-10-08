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

// legacyEvaluate is the legacy PrereqList.Evaluate, kept as the reference the engine is tested against.
func (p *PrereqList) legacyEvaluate(entity *Entity, exclude any, visit func(one Prereq, result PrereqResult, reason string)) PrereqResult {
	result, _ := p.legacyEval(&prereqEvaluation{entity: entity, exclude: exclude, visit: visit}, nil, nil, p.All)
	return result
}

// legacySatisfied is the legacy PrereqList.Satisfied, kept as the reference the engine is tested against.
func (p *PrereqList) legacySatisfied(entity *Entity, exclude any, buffer *xbytes.InsertBuffer, prefix string, hasEquipmentPenalty *bool) bool {
	result, _ := p.legacyEval(&prereqEvaluation{entity: entity, exclude: exclude, prefix: prefix}, buffer,
		hasEquipmentPenalty, p.All)
	return result == PrereqMet || result == PrereqSkipped
}

// prereqEvaluation holds what stays the same throughout an evaluation of a prerequisite list.
type prereqEvaluation struct {
	entity  *Entity
	exclude any
	visit   func(one Prereq, result PrereqResult, reason string)
	prefix  string
}

// legacyEval is legacyEvaluate, also writing the text and setting the equipment penalty that legacySatisfied
// describes, and returning how many items of text it wrote at the level of the prefix. flatten requests that the unmet
// items be written at that level rather than under a heading.
func (p *PrereqList) legacyEval(ev *prereqEvaluation, buffer *xbytes.InsertBuffer, hasEquipmentPenalty *bool, flatten bool) (result PrereqResult, items int) {
	if ev.entity == nil {
		return PrereqMet, 0
	}
	if !p.AppliesAt(ev.entity) {
		p.visitSkipped(ev.visit)
		return PrereqSkipped, 0
	}
	met, applicable, failed := 0, 0, false
	var local *xbytes.InsertBuffer
	if buffer != nil {
		local = &xbytes.InsertBuffer{}
	}
	eqpPenalty := false
	for _, one := range p.Prereqs {
		childResult := PrereqMet
		if list, ok := one.(*PrereqList); ok {
			var n int
			childResult, n = list.legacyEval(ev, local, &eqpPenalty, list.All == p.All)
			items += n
		} else {
			var reason string
			if script, isScript := one.(*ScriptPrereq); isScript {
				childResult, reason = script.legacyEvaluate(ev.entity, ev.exclude, local, ev.prefix)
			} else if !legacyLeaf(one).Satisfied(ev.entity, ev.exclude, local, ev.prefix, &eqpPenalty) {
				childResult = PrereqUnmet
			}
			if childResult != PrereqMet {
				items++
			}
			if ev.visit != nil {
				ev.visit(one, childResult, reason)
			}
		}
		switch childResult {
		case PrereqSkipped:
			continue
		case PrereqMet:
			met++
		case PrereqFailed:
			failed = true
		default:
		}
		applicable++
	}
	switch {
	case applicable == 0:
		result = PrereqSkipped
	case met == applicable || (!p.All && met > 0):
		result = PrereqMet
	case failed:
		result = PrereqFailed
	default:
		result = PrereqUnmet
	}
	if ev.visit != nil {
		ev.visit(p, result, "")
	}
	if result == PrereqMet || result == PrereqSkipped {
		return result, 0
	}
	if eqpPenalty && hasEquipmentPenalty != nil {
		*hasEquipmentPenalty = true
	}
	if buffer == nil {
		return result, 0
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
