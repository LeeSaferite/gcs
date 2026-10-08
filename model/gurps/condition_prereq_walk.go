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
	visit        func(one Prereq, result PrereqResult, reason string)
	prefix       string
	replacements map[string]string
}

func newPrereqWalk(entity *Entity, exclude any, visit func(one Prereq, result PrereqResult, reason string), prefix string) *prereqWalk {
	w := &prereqWalk{entity: entity, exclude: exclude, visit: visit, prefix: prefix}
	if na, ok := exclude.(nameable.Accesser); ok {
		w.replacements = na.NameableReplacements()
	}
	return w
}

// list returns the result of the list r stands for, visiting it and what it holds, and writes what is unmet to buffer,
// if not nil, returning how many items it wrote at the level of the prefix. flatten requests that the unmet items be
// written at that level rather than under a heading.
func (w *prereqWalk) list(p *PrereqList, r *condition.Result, buffer *xbytes.InsertBuffer, flatten bool) (result PrereqResult, items int) {
	if !p.AppliesAt(w.entity) {
		p.visitSkipped(w.visit)
		return PrereqSkipped, 0
	}
	var local *xbytes.InsertBuffer
	if buffer != nil {
		local = &xbytes.InsertBuffer{}
	}
	for i, one := range p.Prereqs {
		child := &r.Children[i]
		if list, ok := one.(*PrereqList); ok {
			_, n := w.list(list, child, local, list.All == p.All)
			items += n
			continue
		}
		if one == nil {
			continue
		}
		childResult := prereqResultOf(child.State)
		var reason string
		script, isScript := one.(*ScriptPrereq)
		if isScript {
			reason = strings.TrimSpace(child.Reason)
		}
		if childResult != PrereqMet {
			items++
			if local != nil {
				if isScript {
					script.writeUnmet(local, w.entity, w.replacements, w.prefix, childResult, reason)
				} else {
					local.WriteString(w.prefix)
					local.WriteString(w.unmetText(one))
				}
			}
		}
		if w.visit != nil {
			w.visit(one, childResult, reason)
		}
	}
	result = prereqResultOf(r.State)
	if w.visit != nil {
		w.visit(p, result, "")
	}
	if result == PrereqMet || result == PrereqSkipped || buffer == nil {
		return result, 0
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
