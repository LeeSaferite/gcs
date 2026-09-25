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
	"os"
	"testing"
	"time"

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps"
	"github.com/richardwilkes/unison"
)

// aldricVane is the fixture character most of these tests open: 150 points, 42 of them unspent, with ST 12, DX 13 and
// HT 11, three traits, three skills and two pieces of equipment, all taken from the fixture master library.
const aldricVane = "Aldric Vane.gcs"

// TestSmokeNewCharacterSheet creates a new character sheet from the File menu and checks that it starts out as a new
// user's first sheet would: unnamed, with the default points, created and modified now, and with its natural attacks.
func TestSmokeNewCharacterSheet(t *testing.T) {
	s := startSmoke(t)
	s.menu("File", "New Character Sheet")
	sheet := s.sheet()
	var e *gurps.Entity
	s.screen.Do(func() { e = sheet.Entity() })
	general := gurps.NewGeneralSettings()
	s.c.Equal("", e.Profile.Name, "name")
	s.c.Equal(general.InitialPoints, e.TotalPoints, "total points")
	s.c.Equal(general.InitialPoints, e.UnspentPoints(), "unspent points")
	s.c.True(time.Time(e.CreatedOn).Equal(smokeNow), "created")
	s.c.Equal(1, len(e.Traits), "the natural attacks should be the only trait")
	s.expectGUI("new_sheet", sheet)
	s.expectScreenshot("new_sheet", nil)
}

// TestSmokeOpenCharacterSheet opens the fixture character as though it had been given on the command line and checks
// that it loads with everything it was saved with.
func TestSmokeOpenCharacterSheet(t *testing.T) {
	s := startSmoke(t, aldricVane)
	sheet := s.sheet()
	var e *gurps.Entity
	s.screen.Do(func() { e = sheet.Entity() })
	s.c.Equal("Aldric Vane", e.Profile.Name, "name")
	s.c.Equal(fxp.FromInteger(150), e.TotalPoints, "total points")
	s.c.Equal(fxp.FromInteger(42), e.UnspentPoints(), "unspent points")
	s.c.Equal(fxp.FromInteger(12), e.Attributes.Current("st"), "ST")
	s.c.Equal(3, len(e.Traits), "traits")
	s.c.Equal(3, len(e.Skills), "skills")
	s.c.Equal(2, len(e.CarriedEquipment), "carried equipment")
	s.expectGUI("sheet", sheet)
	s.expectScreenshot("sheet", nil)
}

// TestSmokeEditAttribute raises ST on the fixture character by typing into the field, then checks that the points are
// recalculated, that undo and redo move between the two values, and that saving writes the new value to the file.
func TestSmokeEditAttribute(t *testing.T) {
	s := startSmoke(t, aldricVane)
	sheet := s.sheet()
	var e *gurps.Entity
	s.screen.Do(func() { e = sheet.Entity() })
	strength := func() fxp.Int {
		var v fxp.Int
		s.screen.Do(func() { v = e.Attributes.Current("st") })
		return v
	}
	unspent := func() fxp.Int {
		var v fxp.Int
		s.screen.Do(func() { v = e.UnspentPoints() })
		return v
	}
	modified := func() bool {
		var m bool
		s.screen.Do(func() { m = sheet.Modified() })
		return m
	}

	s.replaceText("Strength (ST)", "13")
	s.c.Equal(fxp.FromInteger(13), strength(), "ST after the edit")
	s.c.Equal(fxp.FromInteger(32), unspent(), "unspent points after the edit")
	s.c.True(modified(), "the sheet should be modified after the edit")
	s.expectGUI("edited", sheet)
	s.expectScreenshot("edited", nil)

	s.command(unison.KeyZ)
	s.c.Equal(fxp.FromInteger(12), strength(), "ST after undo")
	s.c.Equal(fxp.FromInteger(42), unspent(), "unspent points after undo")

	s.command(unison.KeyY)
	s.c.Equal(fxp.FromInteger(13), strength(), "ST after redo")

	s.menu("File", "Save")
	s.c.False(modified(), "the sheet should not be modified once saved")
	saved, err := gurps.NewEntityFromFile(os.DirFS(s.dir), "files/"+aldricVane)
	s.c.NoError(err)
	s.c.Equal(fxp.FromInteger(13), saved.Attributes.Current("st"), "ST in the saved file")
	s.c.True(time.Time(saved.ModifiedOn).Equal(smokeNow), "modified date in the saved file")
}

// TestSmokeCloseWithUnsavedChanges edits the fixture character and closes it, checking that the user is asked about
// the unsaved changes, that canceling keeps the sheet open with its changes, and that declining to save closes it
// without touching the file.
func TestSmokeCloseWithUnsavedChanges(t *testing.T) {
	s := startSmoke(t, aldricVane)
	sheet := s.sheet()
	s.replaceText("Strength (ST)", "13")

	s.menu("File", "Close")
	dialogWnd, _ := modalDialog(t, s.screen, s.wnd)
	s.expectGUI("save_prompt", dialogWnd.Content())
	s.expectScreenshot("save_prompt", nil)
	s.click("button", "Cancel")
	var open, modified bool
	s.screen.Do(func() {
		open = LocateFileBackedDockable(s.file(aldricVane)) != nil
		modified = sheet.Modified()
	})
	s.c.True(open, "canceling should leave the sheet open")
	s.c.True(modified, "canceling should keep the changes")

	s.menu("File", "Close")
	modalDialog(t, s.screen, s.wnd)
	s.click("button", "No")
	s.screen.Do(func() { open = LocateFileBackedDockable(s.file(aldricVane)) != nil })
	s.c.False(open, "declining to save should close the sheet")
	saved, err := gurps.NewEntityFromFile(os.DirFS(s.dir), "files/"+aldricVane)
	s.c.NoError(err)
	s.c.Equal(fxp.FromInteger(12), saved.Attributes.Current("st"), "declining to save should leave the file as it was")
}
