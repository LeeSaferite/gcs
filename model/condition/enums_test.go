// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

import (
	"hash"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestEnumKeys checks the persisted keys of the generated enums, which the wire format depends on.
func TestEnumKeys(t *testing.T) {
	c := check.New(t)
	c.Equal([]string{"all", "any", "at_least"}, keysOf(GroupModes, GroupMode.Key))
	c.Equal([]string{"", "count", "sum", "max", "min"}, keysOf(Aggregates, Aggregate.Key))
	c.Equal([]string{"is", "is_not", "at_least", "at_most", "more_than", "less_than"}, keysOf(CompareOps, CompareOp.Key))
	c.Equal("", AggregateNone.String())
	for _, key := range []string{"bogus", "All"} {
		v, known := ExtractKnownGroupMode(key)
		c.Equal(key == "All", known)
		c.Equal(GroupAll, v)
	}
}

func keysOf[E any](values []E, key func(E) string) []string {
	keys := make([]string, len(values))
	for i, v := range values {
		keys[i] = key(v)
	}
	return keys
}

func TestKindStrings(t *testing.T) {
	c := check.New(t)
	c.Equal([]string{"absent", "text", "list", "number", "weight", "bool", "unknown"}, keysOf(
		[]ValueKind{ValueAbsent, ValueText, ValueList, ValueNumber, ValueWeight, ValueBool, 99}, ValueKind.String,
	))
	c.Equal([]string{"failed", "met", "unmet", "skipped", "?"}, keysOf(
		[]State{Failed, Met, Unmet, Skipped, 9}, State.String,
	))
}

func TestHashRef(t *testing.T) {
	c := check.New(t)
	c.Equal(nodeHash(nil), hashWith(func(h hash.Hash) { HashRef(h, Ref{Node: &Group{}}) }))
	c.Equal(nodeHash(&Const{}), hashWith(func(h hash.Hash) { HashRef(h, Ref{Node: &Const{}}) }))
}
