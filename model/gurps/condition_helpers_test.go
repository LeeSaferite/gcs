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
	"strings"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/toolbox/v2/xbytes"
	"github.com/zeebo/xxh3"
)

// masterLibrary returns the Master Library directory the full differential tests run against, skipping the test when
// GCS_MASTER_LIBRARY is unset.
func masterLibrary(t testing.TB) string {
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

// prereqVerdict is what one evaluation of a prerequisite list says about its owner: its result and every visit
// Evaluate reports, whether Satisfied passes it, the equipment penalty and unsatisfied text Satisfied gives as recalc
// asks for them, and the text each of its prerequisites gives on its own, as the prerequisite editor shows it.
type prereqVerdict struct {
	visits    string
	text      string
	nodes     string
	state     CheckResult
	satisfied bool
	penalty   bool
}

func (v prereqVerdict) String() string {
	return fmt.Sprintf("%v satisfied=%v penalty=%v text=%q visits=%q nodes=%q", v.state, v.satisfied, v.penalty, v.text,
		v.visits, v.nodes)
}

// recordVisits returns a visit func writing each visit to b.
func recordVisits(b *strings.Builder) func(one Prereq, result CheckResult, reason string) {
	return func(one Prereq, result CheckResult, reason string) {
		fmt.Fprintf(b, "%p %d %q;", one, result, reason)
	}
}

// eachPrereq calls fn for the list and every prerequisite in it, in pre-order.
func eachPrereq(list *PrereqList, fn func(one Prereq)) {
	fn(list)
	for _, one := range list.Prereqs {
		if sub, ok := one.(*PrereqList); ok {
			eachPrereq(sub, fn)
		} else if one != nil {
			fn(one)
		}
	}
}

// legacyVerdict evaluates a list with the legacy evaluation, as recalc and the prerequisite editor did.
func legacyVerdict(e *Entity, list *PrereqList, owner any) prereqVerdict {
	if list == nil {
		return prereqVerdict{state: CheckSkipped, satisfied: true}
	}
	var v prereqVerdict
	var visits, nodes strings.Builder
	v.state = list.legacyEvaluate(e, owner, recordVisits(&visits))
	var buffer xbytes.InsertBuffer
	v.satisfied = list.legacySatisfied(e, owner, &buffer, unsatisfiedReasonPrefix, &v.penalty)
	eachPrereq(list, func(one Prereq) {
		var b xbytes.InsertBuffer
		if sub, ok := one.(*PrereqList); ok {
			sub.legacySatisfied(e, owner, &b, "\n- ", nil)
		} else {
			legacyLeaf(one).Satisfied(e, owner, &b, "\n- ", nil)
		}
		nodes.WriteString(b.String() + "|")
	})
	v.text, v.visits, v.nodes = buffer.String(), visits.String(), nodes.String()
	return v
}

// engineVerdict evaluates a list with the production code, which runs the engine: Evaluate, Satisfied with the host
// recalc shares between lists, and UnsatisfiedPrereqText.
func engineVerdict(h *conditionHost, list *PrereqList, owner any) prereqVerdict {
	if list == nil {
		return prereqVerdict{state: CheckSkipped, satisfied: true}
	}
	var v prereqVerdict
	var visits, nodes strings.Builder
	v.state = list.Evaluate(h.entity, owner, recordVisits(&visits))
	var buffer xbytes.InsertBuffer
	v.satisfied = prereqsSatisfied(list, h, owner, &buffer, unsatisfiedReasonPrefix, &v.penalty)
	eachPrereq(list, func(one Prereq) {
		nodes.WriteString(UnsatisfiedPrereqText(one, h.entity, owner, "\n- ") + "|")
	})
	v.text, v.visits, v.nodes = buffer.String(), visits.String(), nodes.String()
	return v
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
