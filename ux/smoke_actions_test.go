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
	"fmt"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
)

// The helpers here drive the application the way a user would: through its menus, by clicking controls and by typing.
// Controls are found by the role and name they are described by in the accessibility tree, which are what the golden
// GUI descriptions show, rather than by position.

// menu chooses an item from one of the menus in the menu bar, the way a user would.
func (s *smokeSession) menu(menuTitle, itemTitle string) {
	s.t.Helper()
	chooseMenuBarItem(s.t, s.screen, s.wnd, menuTitle, itemTitle)
}

// activeDockable returns the dockable that currently has the focus in the workspace.
func (s *smokeSession) activeDockable() unison.Dockable {
	var d unison.Dockable
	s.screen.Do(func() { d = ActiveDockable() })
	return d
}

// sheet returns the character sheet that currently has the focus in the workspace, failing the test if something else
// has it.
func (s *smokeSession) sheet() *Sheet {
	s.t.Helper()
	sheet, ok := s.activeDockable().(*Sheet)
	if !ok {
		s.t.Fatal("no character sheet has the focus")
	}
	return sheet
}

// find returns the one node in the focused window's accessibility tree with the given role and name, failing the test
// if there is not exactly one. The role is given by its key, as it appears in the golden GUI descriptions.
func (s *smokeSession) find(roleKey, name string) *accessibility.Node {
	s.t.Helper()
	tree := s.screen.AccessibilityTree(s.screen.FocusedWindow())
	var found []*accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if !n.Ignored && n.Role.Key() == roleKey && n.Name == name {
			found = append(found, n)
		}
		return true
	})
	if len(found) != 1 {
		s.t.Fatalf("found %d %s nodes named %q in the focused window, rather than one", len(found), roleKey, name)
	}
	return found[0]
}

// click clicks the middle of the control with the given role and name in the focused window, the way a user would.
func (s *smokeSession) click(roleKey, name string) {
	s.t.Helper()
	s.screen.Click(s.nodeCenter(s.find(roleKey, name)))
}

// nodeCenter returns the middle of a node from the focused window's accessibility tree, in screen coordinates, failing
// the test if the node is scrolled out of view.
func (s *smokeSession) nodeCenter(n *accessibility.Node) geom.Point {
	s.t.Helper()
	if n.Offscreen {
		s.t.Fatalf("the %s named %q is not in view", n.Role.Key(), n.Name)
	}
	var pt geom.Point
	s.screen.Do(func() { pt = screenPoint(s.screen.FocusedWindow(), n.Bounds.Center()) })
	return pt
}

// replaceText clicks the text field with the given name in the focused window, replaces everything in it with text and
// then presses Tab, which is what commits an edit.
func (s *smokeSession) replaceText(name, text string) {
	s.t.Helper()
	s.click("text-field", name)
	s.key(unison.KeyA, mod.OSMenuCommand())
	s.screen.Type(text)
	s.key(unison.KeyTab, 0)
}

// key presses and releases a key with the given modifiers held.
func (s *smokeSession) key(code unison.KeyCode, mods mod.Modifiers) {
	s.screen.KeyPress(code, mods)
}

// command presses the key that, with the platform's menu command modifier held, triggers a menu item: Z for Undo, S
// for Save and so on.
func (s *smokeSession) command(code unison.KeyCode) {
	s.key(code, mod.OSMenuCommand())
}

// dialogPrompt returns the text of the first label in the focused window, which for the application's dialogs is the
// question being asked, failing the test if the focused window is the workspace rather than a dialog.
func (s *smokeSession) dialogPrompt() string {
	s.t.Helper()
	w := s.screen.FocusedWindow()
	if w == s.wnd {
		s.t.Fatal("no dialog has the focus")
	}
	var prompt string
	defer func() { s.checkInvariants(fmt.Sprintf("the %q dialog", prompt), w) }()
	s.screen.AccessibilityTree(w).Walk(func(n *accessibility.Node) bool {
		if !n.Ignored && n.Role.Key() == "label" {
			prompt = n.Name
			return false
		}
		return true
	})
	return prompt
}

// answerPicker answers the template picker dialog that has the focus: it checks that the dialog is asking prompt, ticks
// the named choices, checks that they satisfy the picker, which is what enables its OK button, and presses OK. Choices
// are named as their check boxes are, point costs included, e.g. "Fit [5 points]".
func (s *smokeSession) answerPicker(prompt string, choices ...string) {
	s.t.Helper()
	if got := s.dialogPrompt(); got != prompt {
		s.t.Fatalf("expected the picker for %q, but the focused dialog asks %q", prompt, got)
	}
	for _, choice := range choices {
		s.click("check-box", choice)
	}
	if s.find("button", "OK").Disabled {
		s.t.Fatalf("the choices for %q do not satisfy the picker: %q", prompt, choices)
	}
	s.click("button", "OK")
}
