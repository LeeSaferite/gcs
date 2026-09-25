// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build smoke

package ux

import (
	"slices"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps"
)

// TestSmokeOpenLibraryFile opens the fixture master library's traits by double-clicking them in the navigator and
// checks that they come up in a library editor.
func TestSmokeOpenLibraryFile(t *testing.T) {
	s := startSmoke(t)
	n := s.find("row", "Dungeon Fantasy RPG Traits")
	s.screen.DoubleClick(s.nodeCenter(n))
	d := s.activeDockable()
	var title string
	s.screen.Do(func() { title = d.Title() })
	s.c.Equal("Dungeon Fantasy RPG Traits", title, "the traits should be in front")
	s.expectScreenshot("traits", nil)
}

// answerKnightAdvantages answers the first picker the Knight template poses, the one that spends 60 points on advantages
// and attribute increases.
func answerKnightAdvantages(s *smokeSession) {
	s.answerPicker("60 Points From", "Increased Strength 1 [10 points]", "Increased Dexterity 1 [20 points]",
		"Extra Attack 1 [25 points]", "Fit [5 points]")
}

// TestSmokeNewSheetFromTemplate opens the fixture Knight template from the navigator and creates a character from it,
// answering each of its pickers, including one revealed by an earlier choice, the substitution its Fast-Draw skill
// asks for and the modifier its Sense of Duty offers. It checks that the character ends up with exactly what was
// chosen, on top of what the template always grants.
func TestSmokeNewSheetFromTemplate(t *testing.T) {
	s := startSmoke(t)
	s.screen.DoubleClick(s.nodeCenter(s.find("row", "Knight")))
	s.menu("Edit", "New Character Sheet from Template")
	s.expectGUI("first_picker", s.screen.FocusedWindow().Content())
	s.expectScreenshot("first_picker", nil)
	answerKnightAdvantages(s)
	s.answerPicker("-15 points from here or the other group", "Honesty [-10 points]", "Sense of Duty [-5 points]")
	s.answerPicker("-20 points from", "Code of Honor (Chivalry) [-15 points]", "Wounded [-5 points]")
	s.answerPicker("One Of the Following Melee Packages", "Choose only one")
	s.answerPicker("Choose only one", "Broadsword")
	s.answerPicker("One Ranged Skill", "Crossbow")
	s.answerPicker("Shield Option", "Shield (Shield)")
	s.answerPicker("One Unarmed Striking Skill", "Brawling")
	s.answerPicker("One Armory Specialty", "Armory (Melee Weapons)")
	s.answerPicker("One Grappling Skill", "Wrestling")
	s.answerPicker("Choose Five", "First Aid", "Heraldry", "Hiking", "Observation", "Riding (Horse)")

	// The modifier the Sense of Duty picked above was preconfigured with is already chosen.
	s.c.Equal("Select Modifiers for:", s.dialogPrompt(), "the Sense of Duty should offer its modifiers")
	s.click("button", "OK")

	s.c.Equal("Provide substitutions:", s.dialogPrompt(), "the Fast-Draw skill should ask what it is for")
	s.click("combo-box", "any")
	s.screen.Type("Broadsword")
	s.click("button", "OK")

	sheet := s.sheet()
	var traits, skills []string
	var st, dx, ht, unspent fxp.Int
	s.screen.Do(func() {
		e := sheet.Entity()
		gurps.Traverse(func(one *gurps.Trait) bool {
			traits = append(traits, one.String())
			return false
		}, true, true, e.Traits...)
		gurps.Traverse(func(one *gurps.Skill) bool {
			skills = append(skills, one.String())
			return false
		}, true, true, e.Skills...)
		st = e.Attributes.Current("st")
		dx = e.Attributes.Current("dx")
		ht = e.Attributes.Current("ht")
		unspent = e.UnspentPoints()
	})
	s.c.Equal([]string{
		"Natural Attacks", "Increased Strength 4", "Increased Dexterity 4", "Increased Health 3",
		"Decreased Basic Speed 3", "Born War Leader 2", "Combat Reflexes", "High Pain Threshold",
		"Increased Strength 1", "Increased Dexterity 1", "Extra Attack 1", "Fit", "Honesty", "Sense of Duty",
		"Code of Honor (Chivalry)", "Wounded",
	}, traits, "traits")
	s.c.Equal([]string{
		"Broadsword", "Crossbow", "Shield (Shield)", "Connoisseur (Weapons)", "Knife", "Fast-Draw (Broadsword)",
		"Leadership", "Strategy", "Tactics", "Brawling", "Armory (Melee Weapons)", "Wrestling", "First Aid",
		"Heraldry", "Hiking", "Observation", "Riding (Horse)",
	}, skills, "skills")
	s.c.Equal(fxp.FromInteger(15), st, "ST")
	s.c.Equal(fxp.FromInteger(15), dx, "DX")
	s.c.Equal(fxp.FromInteger(13), ht, "HT")
	// A new character starts with 150 points and the Knight template spends 250.
	s.c.Equal(fxp.FromInteger(-100), unspent, "unspent points")
	s.expectGUI("knight", sheet)
	s.expectScreenshot("knight", nil)
}

// TestSmokeNewSheetFromTemplateCanceled starts creating a character from the fixture Knight template, then cancels at
// the second picker, and checks that the new character is abandoned along with the template, leaving only the template
// open.
func TestSmokeNewSheetFromTemplateCanceled(t *testing.T) {
	s := startSmoke(t)
	s.screen.DoubleClick(s.nodeCenter(s.find("row", "Knight")))
	s.menu("Edit", "New Character Sheet from Template")
	answerKnightAdvantages(s)
	s.c.Equal("-15 points from here or the other group", s.dialogPrompt(), "the second picker should be up")
	s.click("button", "Cancel")
	var open []string
	s.screen.Do(func() {
		for _, d := range AllDockables() {
			open = append(open, d.Title())
		}
	})
	s.c.True(s.screen.FocusedWindow() == s.wnd, "canceling should leave no dialog up")
	s.c.Equal([]string{"Knight"}, open, "canceling should close the new character, leaving only the template")
}

// TestSmokeApplyTemplateCanceled starts applying the fixture Knight template to the fixture character, then cancels
// at the second picker, and checks that the character is left exactly as it was.
func TestSmokeApplyTemplateCanceled(t *testing.T) {
	s := startSmoke(t, aldricVane)
	sheet := s.sheet()
	var before, after []string
	names := func() []string {
		var list []string
		s.screen.Do(func() {
			gurps.Traverse(func(one *gurps.Trait) bool {
				list = append(list, one.String())
				return false
			}, false, false, sheet.Entity().Traits...)
		})
		return list
	}
	before = names()
	s.screen.DoubleClick(s.nodeCenter(s.find("row", "Knight")))
	s.menu("Edit", "Apply Template to Character Sheet")
	answerKnightAdvantages(s)
	s.c.Equal("-15 points from here or the other group", s.dialogPrompt(), "the second picker should be up")
	s.click("button", "Cancel")
	after = names()
	var modified bool
	var st fxp.Int
	s.screen.Do(func() {
		modified = sheet.Modified()
		st = sheet.Entity().Attributes.Current("st")
	})
	s.c.True(s.screen.FocusedWindow() == s.wnd, "canceling should leave no dialog up")
	s.c.True(slices.Equal(before, after), "canceling should leave the traits as they were")
	s.c.False(modified, "canceling should leave the sheet unmodified")
	s.c.Equal(fxp.FromInteger(12), st, "canceling should leave ST as it was")
}
