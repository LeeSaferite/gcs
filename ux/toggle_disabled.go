// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ux

import (
	"github.com/richardwilkes/gcs/v5/model/gurps"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison"
)

// disableableExtractor accepts a row that can be disabled where it is: a trait anywhere, except as an enabled template
// choice container, whose editor offers no way to enable it again (one disabled by an older version may still be
// enabled), and anything else only on a sheet, the only place where disabling it means anything.
func disableableExtractor[T gurps.Node[T]](node T) (gurps.Disableable, bool) {
	d, ok := any(node).(gurps.Disableable)
	if !ok {
		return nil, false
	}
	if t, isTrait := any(node).(*gurps.Trait); isTrait {
		return d, t.Disabled || !gurps.IsTemplateChoiceContainer(t)
	}
	return d, gurps.EntityFromNode(node) != nil
}

func canToggleDisabled[T gurps.Node[T]](table *unison.Table[*Node[T]]) bool {
	return canAdjustSelection(table, disableableExtractor[T])
}

// toggleDisabled flips the enabled state of each selected row. The owner is rebuilt rather than merely marked as
// modified, since a row that stops contributing takes its weapons, reactions and conditional modifiers out of play with
// it, and which lists the owner shows -- along with which columns they hold -- is decided only when it creates them.
func toggleDisabled[T gurps.Node[T]](owner Rebuildable, table *unison.Table[*Node[T]]) {
	adjustSelection(i18n.Text("Toggle Enablement"), owner, table, disableableExtractor[T],
		gurps.Disableable.IsDisabled,
		gurps.Disableable.SetDisabled,
		func(d gurps.Disableable) { d.SetDisabled(!d.IsDisabled()) },
		true, true)
}
