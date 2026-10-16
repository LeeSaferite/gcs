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
	"bufio"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/condition"
)

// TestPrereqCorpus converts a sample of the Master Library's prerequisite trees, chosen to cover every legacy type,
// sub type and owner kind. Each converted tree must be valid, survive the condition codec with its hash, hash the same
// when converted again or from a clone, and evaluate as legacy does on the constructed entity, for an owner of its
// kind and for the entity's own items.
func TestPrereqCorpus(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "condition_prereqs.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	e, owners := newConditionTestEntity("3")
	h := newConditionHost(e)
	var d diffTally
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 1<<20)
	trees := 0
	for scanner.Scan() {
		var line struct {
			Kind    string      `json:"kind"`
			Prereqs *PrereqList `json:"prereqs"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatal(err)
		}
		trees++
		owner := newOwner(line.Kind)
		name := line.Kind + " " + legacyJSONString(t, line.Prereqs)
		tree := conditionFromPrereqs(line.Prereqs, owner)
		if problems := tree.Validate(); len(problems) != 0 {
			t.Errorf("%s: invalid tree: %v", name, problems)
		}
		if treeHash(tree) != treeHash(conditionFromPrereqs(line.Prereqs, owner)) ||
			treeHash(tree) != treeHash(conditionFromPrereqs(line.Prereqs.CloneAsPrereqList(nil), owner)) {
			t.Errorf("%s: converting again or converting a clone hashes differently", name)
		}
		var decoded condition.Ref
		if err = json.Unmarshal([]byte(treeJSON(t, tree)), &decoded); err != nil {
			t.Fatal(err)
		}
		if treeHash(decoded.Node) != treeHash(tree) {
			t.Errorf("%s: the condition codec changed the hash", name)
		}
		d.compare(t, h, e, line.Prereqs, owner, name)
		for _, one := range owners {
			d.compare(t, h, e, line.Prereqs, one, name)
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if trees < 300 {
		t.Errorf("expected at least 300 trees, found %d", trees)
	}
	t.Logf("%d trees: %s", trees, d.String())
}
