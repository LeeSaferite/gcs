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

// PrereqList holds a prereq that contains a list of prerequisites. Not swaps met and unmet, making "all of" into "not
// all of" and "any of" into "none of".
type PrereqList struct {
	Parent  *PrereqList     `json:"-"`
	Type    prereq.Type     `json:"type"`
	All     bool            `json:"all"`
	Not     bool            `json:"not,omitzero"`
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
// joins more than one is parenthesized, a negated list says so, a tech level condition is noted at the end, and empty
// lists are left out.
func (p *PrereqList) Describe(entity *Entity, replacements map[string]string, em func(string) string) string {
	text, n, all := p.describeChildren(entity, replacements, em, p.Not)
	if n != 0 {
		text = p.describeNot(text, all)
	}
	return text + p.describeWhenTL()
}

// describeChildren joins the descriptions of the children, returning how many it joined and whether they are joined
// with "and". When lower is true, the first of them follows a joining word, as every other one does, and so begins in
// lowercase unless it is a name someone wrote. A negated list that holds only a nested list joining several items
// joins those items instead, in that list's mode, since negating one thing is the same whichever way it is combined.
func (p *PrereqList) describeChildren(entity *Entity, replacements map[string]string, em func(string) string, lower bool) (text string, count int, all bool) {
	parts := make([]string, 0, len(p.Prereqs))
	// bare is the text of a nested list that joins more than one item, without its parentheses, and bareAll its mode.
	var bare string
	var bareAll bool
	for _, one := range p.Prereqs {
		lower = lower || len(parts) != 0
		if list, isList := one.(*PrereqList); isList {
			var n int
			var listAll bool
			if text, n, listAll = list.describeChildren(entity, replacements, em, lower || list.Not); n == 0 {
				continue
			}
			switch {
			case list.Not:
				if text = list.describeNot(text, listAll); lower {
					text = lowerFirst(text)
				}
			case n > 1:
				if list.WhenTL.Compare == criteria.AnyNumber {
					bare, bareAll = text, listAll
				}
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
	if p.Not && len(parts) == 1 && bare != "" {
		return bare, 1, bareAll
	}
	joiner := i18n.Text(" or ")
	if p.All {
		joiner = i18n.Text(" and ")
	}
	return strings.Join(parts, joiner), len(parts), p.All
}

// describeNot returns the text of the children of a negated list, joined with "and" when all is set, as the list says
// them, or unchanged when it isn't one.
func (p *PrereqList) describeNot(text string, all bool) string {
	switch {
	case !p.Not:
		return text
	case all:
		return i18n.Text("Not all of (%s)", text)
	default:
		return i18n.Text("None of (%s)", text)
	}
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
	xhash.Bool(h, p.Not)
	p.WhenTL.Hash(h)
	hashList(h, p.Prereqs)
}
