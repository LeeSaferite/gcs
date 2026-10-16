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

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xbytes"
)

// prereqWalk walks a Collect result of a converted prerequisite list alongside the list, writing the unsatisfied text
// and reporting each prerequisite's result. The conversion keeps the list's shape, so each list's result has one child
// per prerequisite, in order: the legacy node a result stands for is found by position, and no map is kept.
type prereqWalk struct {
	entity       *Entity
	exclude      any
	visit        func(one Prereq, result CheckResult, reason string)
	prefix       string
	replacements map[string]string
}

func newPrereqWalk(entity *Entity, exclude any, visit func(one Prereq, result CheckResult, reason string), prefix string) *prereqWalk {
	w := &prereqWalk{entity: entity, exclude: exclude, visit: visit, prefix: prefix}
	if na, ok := exclude.(nameable.Accesser); ok {
		w.replacements = na.NameableReplacements()
	}
	return w
}

// list returns the result of the list r stands for, visiting it and what it holds, and writes what is unmet to buffer,
// if not nil, returning how many items it wrote at the level of the prefix. flatten requests that the unmet items of a
// list that isn't negated be written at that level rather than under a heading. A negated list that is unmet lists
// what is met instead, or what failed when it did.
func (w *prereqWalk) list(p *PrereqList, r *condition.Result, buffer *xbytes.InsertBuffer, flatten bool) (result CheckResult, items int) {
	if !p.AppliesAt(w.entity) {
		p.visitAll(w.visit, CheckSkipped, "")
		return CheckSkipped, 0
	}
	if len(r.Children) != len(p.Prereqs) {
		return w.whole(p, r, buffer)
	}
	var local *xbytes.InsertBuffer
	if buffer != nil {
		local = &xbytes.InsertBuffer{}
	}
	var met []Prereq
	for i, one := range p.Prereqs {
		if one == nil {
			continue
		}
		out := local
		if p.Not && local != nil {
			// What is unmet doesn't count against a negated list, so only what failed is kept.
			out = &xbytes.InsertBuffer{}
		}
		child := &r.Children[i]
		var childResult CheckResult
		if list, ok := one.(*PrereqList); ok {
			var n int
			childResult, n = w.list(list, child, out, list.All == p.All && !p.Not)
			items += n
		} else {
			childResult = w.leaf(one, child, out)
			if childResult != CheckMet {
				items++
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
	}
	result = checkResultOf(r.State)
	if w.visit != nil {
		w.visit(p, result, "")
	}
	if result == CheckMet || result == CheckSkipped || buffer == nil {
		return result, 0
	}
	if p.Not {
		buffer.WriteString(w.prefix)
		if p.All {
			buffer.WriteString(i18n.Text("Requires not all of:"))
		} else {
			buffer.WriteString(i18n.Text("Requires none of:"))
		}
		if result == CheckFailed {
			buffer.WriteString(strings.ReplaceAll(local.String(), "\n", "\n\t"))
			return result, 1
		}
		for _, one := range met {
			buffer.WriteString(strings.ReplaceAll(w.prefix+one.Describe(w.entity, w.replacements, plainText), "\n",
				"\n\t"))
		}
		return result, 1
	}
	if flatten || items == 1 {
		buffer.WriteString(local.String())
		return result, items
	}
	buffer.WriteString(w.prefix)
	if p.All {
		buffer.WriteString(i18n.Text("Requires all of:"))
	} else {
		buffer.WriteString(i18n.Text("Requires at least one of:"))
	}
	buffer.WriteString(strings.ReplaceAll(local.String(), "\n", "\n\t"))
	return result, 1
}

// leaf returns the result of a prerequisite other than a list, which r stands for, visiting it, and writes what it says
// to buffer, if not nil, when it isn't met.
func (w *prereqWalk) leaf(one Prereq, r *condition.Result, buffer *xbytes.InsertBuffer) CheckResult {
	result := checkResultOf(r.State)
	var reason string
	script, isScript := one.(*ScriptPrereq)
	if isScript {
		reason = strings.TrimSpace(r.Reason)
	}
	if result != CheckMet && buffer != nil {
		if isScript {
			script.writeUnmet(buffer, w.entity, w.replacements, w.prefix, result, reason)
		} else {
			buffer.WriteString(w.prefix)
			buffer.WriteString(w.unmetText(one))
		}
	}
	if w.visit != nil {
		w.visit(one, result, reason)
	}
	return result
}

// whole reports a list whose result has no details, since it couldn't be converted or was too complex to explain:
// each of its prerequisites gets the list's result, and the text is the list's reason.
func (w *prereqWalk) whole(p *PrereqList, r *condition.Result, buffer *xbytes.InsertBuffer) (result CheckResult, items int) {
	result = checkResultOf(r.State)
	p.visitAll(w.visit, result, r.Reason)
	if result == CheckMet || result == CheckSkipped || buffer == nil {
		return result, 0
	}
	reason := r.Reason
	if reason == "" {
		reason = i18n.Text("Too complex to explain")
	}
	buffer.WriteString(w.prefix)
	buffer.WriteString(reason)
	return result, 1
}

// unmetText returns what an unmet prerequisite other than a list or script says: its description, followed for a
// skill or spell owner by the rule that owner's type adds.
func (w *prereqWalk) unmetText(one Prereq) string {
	text := one.Describe(w.entity, w.replacements, plainText)
	switch p := one.(type) {
	case *SkillPrereq:
		if sk, ok := w.exclude.(*Skill); ok && sk.TechLevel != nil {
			text += i18n.Text(" with a matching tech level")
		}
	case *SpellPrereq:
		// Name the owning spell's resolved power source, so that one that is blank or spelled differently from the
		// spells it expects to match can be diagnosed from the tooltip alone.
		if sp, ok := w.exclude.(*Spell); ok && p.SamePowerSource {
			text += ` ("` + sp.PowerSourceWithReplacements() + `")`
		}
	default:
	}
	return text
}
