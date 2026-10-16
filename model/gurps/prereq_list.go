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
	"hash"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/prereq"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
	"github.com/richardwilkes/toolbox/v2/xreflect"
)

var _ Prereq = &PrereqList{}

// PrereqList holds a prereq that contains a list of prerequisites.
type PrereqList struct {
	Parent  *PrereqList     `json:"-"`
	Type    prereq.Type     `json:"type"`
	All     bool            `json:"all"`
	WhenTL  criteria.Number `json:"when_tl,omitzero"`
	Prereqs Prereqs         `json:"prereqs,omitempty"`
}

// NewPrereqList creates a new PrereqList.
func NewPrereqList() *PrereqList {
	return &PrereqList{
		Type: prereq.List,
		All:  true,
	}
}

// IsZero reports whether json's omitzero option should omit this value.
func (p *PrereqList) IsZero() bool {
	return p == nil || len(p.Prereqs) == 0
}

// PrereqType implements Prereq.
func (p *PrereqList) PrereqType() prereq.Type {
	return p.Type
}

// ParentList implements Prereq.
func (p *PrereqList) ParentList() *PrereqList {
	if p == nil {
		return nil
	}
	return p.Parent
}

// SetParentList implements Prereq.
func (p *PrereqList) SetParentList(list *PrereqList) {
	p.Parent = list
}

// Clone implements Prereq.
func (p *PrereqList) Clone(parent *PrereqList) Prereq {
	return p.CloneAsPrereqList(parent)
}

// CloneAsPrereqList clones this prereq list.
func (p *PrereqList) CloneAsPrereqList(parent *PrereqList) *PrereqList {
	clone := *p
	clone.Parent = parent
	clone.Prereqs = make(Prereqs, len(p.Prereqs))
	for i := range p.Prereqs {
		clone.Prereqs[i] = p.Prereqs[i].Clone(&clone)
	}
	return &clone
}

// CloneResolvingEmpty clones this prereq list. A nil list becomes a new, empty one unless isContainer is true, and an
// empty list becomes nil if pruneIfEmpty is true.
func (p *PrereqList) CloneResolvingEmpty(isContainer, pruneIfEmpty bool) *PrereqList {
	if p != nil {
		if pruneIfEmpty && p.IsZero() {
			return nil
		}
		return p.CloneAsPrereqList(nil)
	}
	if isContainer {
		return nil
	}
	return NewPrereqList()
}

// FillWithNameableKeys implements Prereq.
func (p *PrereqList) FillWithNameableKeys(m, existing map[string]string) {
	for _, one := range p.Prereqs {
		one.FillWithNameableKeys(m, existing)
	}
}

// HasNothingToCheck returns true if this list, which may be nil, holds no prerequisite other than nil entries and lists
// that themselves have nothing to check.
func (p *PrereqList) HasNothingToCheck() bool {
	if p != nil {
		for _, one := range p.Prereqs {
			if xreflect.IsNil(one) {
				continue
			}
			if list, ok := one.(*PrereqList); !ok || !list.HasNothingToCheck() {
				return false
			}
		}
	}
	return true
}

// AppliesAt returns true if this list applies at the tech level of the entity, which may be nil. A list with no tech
// level condition always applies, as does any list when there is no entity.
func (p *PrereqList) AppliesAt(entity *Entity) bool {
	if entity == nil || p.WhenTL.Compare == criteria.AnyNumber {
		return true
	}
	tl, _, _ := ExtractTechLevel(entity.Profile.TechLevel)
	return p.WhenTL.Compare.Matches(p.WhenTL.Qualifier, max(tl, 0))
}

// AppliesWithParentsAt returns true if this list and the lists holding it apply at the tech level of the entity, which
// may be nil.
func (p *PrereqList) AppliesWithParentsAt(entity *Entity) bool {
	for ; p != nil; p = p.Parent {
		if !p.AppliesAt(entity) {
			return false
		}
	}
	return true
}

// visitAll calls visit, if not nil, with each prerequisite in this list and then the list itself, all with the given
// result and reason.
func (p *PrereqList) visitAll(visit func(one Prereq, result CheckResult, reason string), result CheckResult, reason string) {
	if visit == nil {
		return
	}
	for _, one := range p.Prereqs {
		if list, ok := one.(*PrereqList); ok {
			list.visitAll(visit, result, reason)
		} else {
			visit(one, result, reason)
		}
	}
	visit(p, result, reason)
}

// Describe implements Prereq. The children are joined with "and" or "or" to match the list's mode, a nested list that
// joins more than one is parenthesized, a tech level condition is noted at the end, and empty lists are left out.
func (p *PrereqList) Describe(entity *Entity, replacements map[string]string, em func(string) string) string {
	text, _ := p.describeChildren(entity, replacements, em, false)
	return text + p.describeWhenTL()
}

// describeChildren joins the descriptions of the children, returning how many it joined. When lower is true, the first
// of them follows a joining word, as every other one does, and so begins in lowercase unless it is a name someone
// wrote.
func (p *PrereqList) describeChildren(entity *Entity, replacements map[string]string, em func(string) string, lower bool) (text string, count int) {
	parts := make([]string, 0, len(p.Prereqs))
	for _, one := range p.Prereqs {
		lower = lower || len(parts) != 0
		if list, isList := one.(*PrereqList); isList {
			var n int
			if text, n = list.describeChildren(entity, replacements, em, lower); n == 0 {
				continue
			}
			if n > 1 {
				text = "(" + text + ")"
			}
			text += list.describeWhenTL()
		} else {
			text = one.Describe(entity, replacements, em)
			if script, isScript := one.(*ScriptPrereq); lower && (!isScript || script.ResolvedName(replacements) == "") {
				text = lowerFirst(text)
			}
		}
		parts = append(parts, text)
	}
	joiner := i18n.Text(" or ")
	if p.All {
		joiner = i18n.Text(" and ")
	}
	return strings.Join(parts, joiner), len(parts)
}

// describeWhenTL returns the note that ends a description of this list when it has a tech level condition, or an empty
// string when it has none.
func (p *PrereqList) describeWhenTL() string {
	if p.WhenTL.Compare == criteria.AnyNumber {
		return ""
	}
	return i18n.Text(" (only when TL %s)", p.WhenTL.AltString())
}

// lowerFirst lowercases the first letter of text when it begins a word that continues in lowercase or is a single
// letter, so a description can follow a joining word without lowering a name or an acronym.
func lowerFirst(text string) string {
	r, size := utf8.DecodeRuneInString(text)
	if next, _ := utf8.DecodeRuneInString(text[size:]); unicode.IsUpper(r) && (unicode.IsLower(next) || next == '\'' || next == ' ') {
		return string(unicode.ToLower(r)) + text[size:]
	}
	return text
}

// Hash writes this object's contents into the hasher.
func (p *PrereqList) Hash(h hash.Hash) {
	if p.IsZero() {
		xhash.Num8(h, uint8(255))
		return
	}
	xhash.Num8(h, p.Type)
	xhash.Bool(h, p.All)
	p.WhenTL.Hash(h)
	hashList(h, p.Prereqs)
}
