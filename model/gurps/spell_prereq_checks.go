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
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/spellcmp"
	"github.com/richardwilkes/toolbox/v2/xbytes"
)

type SpellPrereqChecks []SpellPrereqCheck

func (c *SpellPrereqChecks) Satisfied(entity *Entity, exclude any, tooltip *xbytes.InsertBuffer, prefix string, _ *bool) bool {

	for _, check := range c.([]SpellPrereqCheck) {
		switch check.Type {
		case spellcmp.Name:
			if p.QualifierCriteria.Matches(replacements, sp.NameWithReplacements()) {
				count++
			}
		case spellcmp.Tag:
			for _, one := range sp.Tags {
				if p.QualifierCriteria.Matches(replacements, one) {
					count++
					break
				}
			}
		case spellcmp.College:
			for _, one := range sp.CollegeWithReplacements() {
				if p.QualifierCriteria.Matches(replacements, one) {
					count++
					break
				}
			}
		case spellcmp.CollegeCount:
			for _, one := range sp.CollegeWithReplacements() {
				colleges[one] = true
			}
		case spellcmp.Any:
			count++
		}
	}
}

type SpellPrereqCheck interface {
	Type() spellcmp.Type
}

type spellPrereqCheckName struct {
	Compare   string `json:"compare,omitzero"`
	Qualifier string `json:"qualifier,omitzero"`
}

func (c *spellPrereqCheckName) Type() spellcmp.Type { return spellcmp.Name }

type spellPrereqCheckTag struct {
	Compare   string `json:"compare,omitzero"`
	Qualifier string `json:"qualifier,omitzero"`
}

func (c *spellPrereqCheckTag) Type() spellcmp.Type { return spellcmp.Tag }

type spellPrereqCheckCollege struct {
	Compare   string `json:"compare,omitzero"`
	Qualifier string `json:"qualifier,omitzero"`
}

func (c *spellPrereqCheckCollege) Type() spellcmp.Type { return spellcmp.College }

type spellPrereqCheckCollegeCount struct {
	Compare   string `json:"compare,omitzero"`
	Qualifier string `json:"qualifier,omitzero"`
}

func (c *spellPrereqCheckCollegeCount) Type() spellcmp.Type { return spellcmp.CollegeCount }
