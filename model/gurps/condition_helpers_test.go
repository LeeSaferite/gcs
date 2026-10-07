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
	"encoding/json/v2"
	"fmt"
	"os"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/zeebo/xxh3"
)

// masterLibrary returns the Master Library directory the full differential tests run against, skipping the test when
// GCS_MASTER_LIBRARY is unset.
func masterLibrary(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("GCS_MASTER_LIBRARY")
	if dir == "" {
		t.Skip("GCS_MASTER_LIBRARY is not set")
	}
	return dir
}

// treeJSON returns the canonical JSON of a condition tree.
func treeJSON(t *testing.T, n condition.Node) string {
	t.Helper()
	data, err := json.Marshal(condition.Ref{Node: n})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// legacyJSONString returns the JSON of a legacy value.
func legacyJSONString(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// prereqVerdict is what one evaluation of a prerequisite list says about its owner.
type prereqVerdict struct {
	state     condition.State
	satisfied bool
	penalty   bool
}

func (v prereqVerdict) String() string {
	return fmt.Sprintf("%v satisfied=%v penalty=%v", v.state, v.satisfied, v.penalty)
}

// legacyVerdict evaluates a legacy list as recalc does.
func legacyVerdict(e *Entity, list *PrereqList, owner any) prereqVerdict {
	if list == nil {
		return prereqVerdict{state: condition.Skipped, satisfied: true}
	}
	var v prereqVerdict
	v.state = [...]condition.State{
		PrereqMet:     condition.Met,
		PrereqUnmet:   condition.Unmet,
		PrereqSkipped: condition.Skipped,
		PrereqFailed:  condition.Failed,
	}[list.Evaluate(e, owner, nil)]
	v.satisfied = list.Satisfied(e, owner, nil, "", &v.penalty)
	return v
}

// engineVerdict evaluates the converted list with the engine, deriving the penalty from its Collect result.
func engineVerdict(h *conditionHost, list *PrereqList, owner any) prereqVerdict {
	r := condition.Evaluate(conditionFromPrereqs(list, owner), h, owner, condition.Options{Mode: condition.Collect})
	return prereqVerdict{state: r.State, satisfied: r.State.Passes(), penalty: prereqEquipmentPenalty(&r)}
}

// newOwner returns an empty owner of the kind the survey names.
func newOwner(kind string) any {
	switch kind {
	case "skill", "technique":
		return NewSkill(nil, nil, false)
	case "spell":
		return NewSpell(nil, nil, false)
	case "equipment":
		return NewEquipment(nil, nil, false)
	case "equipment_container":
		return NewEquipment(nil, nil, true)
	case "trait_container":
		return NewTrait(nil, nil, true)
	default:
		return NewTrait(nil, nil, false)
	}
}

// treeHash returns the hash of a condition tree.
func treeHash(n condition.Node) uint64 {
	h := xxh3.New()
	condition.HashNode(h, n)
	return h.Sum64()
}
