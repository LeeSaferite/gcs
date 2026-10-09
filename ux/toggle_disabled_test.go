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
	"testing"

	"github.com/richardwilkes/gcs/v5/model/gurps"
	"github.com/richardwilkes/toolbox/v2/check"
)

func TestDisableableExtractorLimitsAllButTraitsToSheets(t *testing.T) {
	c := check.New(t)
	entity := gurps.NewEntity()
	_, ok := disableableExtractor(gurps.NewSkill(entity, nil, false))
	c.True(ok, "a skill on a sheet can be disabled")
	_, ok = disableableExtractor(gurps.NewSkill(nil, nil, false))
	c.False(ok, "a skill off a sheet can't be disabled")
	_, ok = disableableExtractor(gurps.NewTrait(nil, nil, false))
	c.True(ok, "a trait can be disabled anywhere")
	_, ok = disableableExtractor(gurps.NewNote(entity, nil, false))
	c.False(ok, "a note can't be disabled")
}
