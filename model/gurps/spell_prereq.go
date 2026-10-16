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

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/prereq"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Prereq = &SpellPrereq{}

// SpellPrereq holds a prerequisite for a spell.
type SpellPrereq struct {
	Parent              *PrereqList     `json:"-"`
	Type                prereq.Type     `json:"type"`
	SubType             spellcmp.Type   `json:"sub_type"`
	Has                 bool            `json:"has"`
	QualifierCriteria   criteria.Text   `json:"qualifier,omitzero"`
	QuantityCriteria    criteria.Number `json:"quantity,omitzero"`
	SamePowerSource     bool            `json:"same_power_source,omitzero"`
	PowerSourceCriteria criteria.Text   `json:"power_source,omitzero"`
}

// NewSpellPrereq creates a new SpellPrereq.
func NewSpellPrereq() *SpellPrereq {
	var p SpellPrereq
	p.Type = prereq.Spell
	p.SubType = spellcmp.Name
	p.QualifierCriteria.Compare = criteria.IsText
	p.QuantityCriteria.Compare = criteria.AtLeastNumber
	p.QuantityCriteria.Qualifier = fxp.One
	p.SamePowerSource = true
	p.PowerSourceCriteria.Compare = criteria.AnyText
	p.Has = true
	return &p
}

// PrereqType implements Prereq.
func (p *SpellPrereq) PrereqType() prereq.Type {
	return p.Type
}

// ParentList implements Prereq.
func (p *SpellPrereq) ParentList() *PrereqList {
	return p.Parent
}

// SetParentList implements Prereq.
func (p *SpellPrereq) SetParentList(list *PrereqList) {
	p.Parent = list
}

// Clone implements Prereq.
func (p *SpellPrereq) Clone(parent *PrereqList) Prereq {
	clone := *p
	clone.Parent = parent
	return &clone
}

// FillWithNameableKeys implements Prereq.
func (p *SpellPrereq) FillWithNameableKeys(m, existing map[string]string) {
	if p.SubType.UsesStringCriteria() {
		nameable.Extract(m, existing, p.QualifierCriteria.Qualifier)
	}
	if !p.SamePowerSource {
		nameable.Extract(m, existing, p.PowerSourceCriteria.Qualifier)
	}
}

func (p *SpellPrereq) hasPowerSourceFilter() bool {
	return p.SamePowerSource || p.PowerSourceCriteria.Compare != criteria.AnyText
}

// Describe implements Prereq.
func (p *SpellPrereq) Describe(_ *Entity, replacements map[string]string, em func(string) string) string {
	text := i18n.Text("Knows")
	if !p.Has {
		text = i18n.Text("Does not know")
	}
	var quantity string
	if p.QuantityCriteria.Compare != criteria.AnyNumber {
		quantity = " " + p.QuantityCriteria.AltString()
	}
	one, many := i18n.Text(" spell"), i18n.Text(" spells")
	if p.SubType == spellcmp.CollegeCount {
		text += i18n.Text(" spells from")
		one, many = i18n.Text(" college"), i18n.Text(" colleges")
	}
	text += quantity
	if p.QuantityCriteria.Qualifier == fxp.One {
		text += one
	} else {
		text += many
	}
	whose := p.SubType.UsesStringCriteria() && p.QualifierCriteria.Compare != criteria.AnyText
	if whose {
		switch p.SubType {
		case spellcmp.Tag:
			text += i18n.Text(" whose tag ")
		case spellcmp.College:
			text += i18n.Text(" whose college ")
		default:
			text += i18n.Text(" whose name ")
		}
		text += describeText(p.QualifierCriteria, replacements, em)
	}
	if p.hasPowerSourceFilter() {
		if whose {
			text += i18n.Text(" and whose power source ")
		} else {
			text += i18n.Text(" whose power source ")
		}
		if p.SamePowerSource {
			text += i18n.Text("is the same as this spell's")
		} else {
			text += describeText(p.PowerSourceCriteria, replacements, em)
		}
	}
	return text
}

// Hash writes this object's contents into the hasher.
func (p *SpellPrereq) Hash(h hash.Hash) {
	if p == nil {
		xhash.Num8(h, uint8(255))
		return
	}
	xhash.Num8(h, p.Type)
	xhash.Num8(h, p.SubType)
	xhash.Bool(h, p.Has)
	p.QualifierCriteria.Hash(h)
	p.QuantityCriteria.Hash(h)
	xhash.Bool(h, p.SamePowerSource)
	if !p.SamePowerSource {
		p.PowerSourceCriteria.Hash(h)
	}
}
