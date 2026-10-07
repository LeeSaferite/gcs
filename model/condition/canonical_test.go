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
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/check"
)

// TestCanonicalEquivalentsHashEqual checks pairs of trees built in code that differ only in ways the canonical form
// erases: each pair must hash and encode the same.
func TestCanonicalEquivalentsHashEqual(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b Ref
	}{
		{"nil and empty group", Ref{}, Ref{Node: &Group{Mode: GroupAny, Not: true, N: 3}}},
		{
			"empty only_when",
			Ref{Node: &Group{Children: Nodes{&Const{}}}},
			Ref{Node: &Group{Children: Nodes{&Const{}}, OnlyWhen: Ref{Node: &Group{OnlyWhen: Ref{Node: &Group{}}}}}},
		},
		{"N outside at_least", Ref{Node: &Group{Children: Nodes{&Const{}}}}, Ref{Node: &Group{N: 2, Children: Nodes{&Const{}}}}},
		{"empty children", Ref{Node: &Group{OnlyWhen: Ref{Node: &Const{}}}}, Ref{Node: &Group{OnlyWhen: Ref{Node: &Const{}}, Children: Nodes{}}}},
		{"Extra whitespace", Ref{Node: &Const{Extra: extra(`{"a":[1,2]}`)}}, Ref{Node: &Const{Extra: extra(" { \"a\" : [ 1, 2 ] } ")}}},
		{"empty Extra", Ref{Node: &Const{}}, Ref{Node: &Const{Extra: extra(" { } ")}}},
		{"null Extra", Ref{Node: &Script{Script: "x"}}, Ref{Node: &Script{Script: "x", Extra: extra("null")}}},
		{
			"Unknown whitespace",
			Ref{Node: &Unknown{Type: "x", Data: extra(`{"type":"x","a":[1]}`), Reason: "one"}},
			Ref{Node: &Unknown{Type: "x", Data: extra("{ \"type\" : \"x\", \"a\" : [ 1 ] }"), Reason: "two"}},
		},
		{"scale 1", Ref{Node: &Compare{Left: Operand{Value: intPtr(1)}}}, Ref{Node: &Compare{Left: Operand{Value: intPtr(1), Scale: fxp.One}}}},
		{
			"members without aggregate",
			Ref{Node: &Compare{Right: Operand{Attribute: "st"}}},
			Ref{Node: &Compare{Right: Operand{
				Attribute: "st",
				Members:   Members{Of: "x", Where: Ref{Node: &Const{}}},
				Field:     "f",
				Default:   intPtr(2),
			}}},
		},
		{
			"quantity at_least 1",
			Ref{Node: &Count{Of: "spells"}},
			Ref{Node: &Count{
				Of:       "spells",
				Quantity: numberCritPtr(criteria.AtLeastNumber, 1),
				PerGroup: numberCritPtr(criteria.AtLeastNumber, 1),
			}},
		},
		{
			"any quantity with a stale qualifier",
			Ref{Node: &Count{Of: "spells", Quantity: &criteria.Number{}}},
			Ref{Node: &Count{Of: "spells", Quantity: numberCritPtr(criteria.AnyNumber, 4)}},
		},
		{
			"any criteria with stale qualifiers",
			Ref{Node: &Check{Field: "f"}},
			Ref{Node: &Check{
				Field:  "f",
				Text:   textCrit(criteria.AnyText, "x"),
				Number: numberCrit(criteria.AnyNumber, 2),
				Weight: weightCrit(criteria.AnyNumber, 3),
			}},
		},
		{"invalid mode", Ref{Node: &Group{Children: Nodes{&Const{}}}}, Ref{Node: &Group{Mode: 99, Children: Nodes{&Const{}}}}},
		{"invalid op", Ref{Node: &Compare{}}, Ref{Node: &Compare{Op: 99}}},
		{"invalid aggregate", Ref{Node: &Compare{}}, Ref{Node: &Compare{Left: Operand{Aggregate: 99, Of: "x"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			c.Equal(refHash(tc.a), refHash(tc.b))
			c.Equal(string(encodeCompact(t, tc.a)), string(encodeCompact(t, tc.b)))
		})
	}
}

// TestRefIsZero checks which Refs count as empty.
func TestRefIsZero(t *testing.T) {
	c := check.New(t)
	c.True(Ref{}.IsZero())
	c.True(Ref{Node: &Group{Mode: GroupAtLeast, N: 1, Not: true, Exclusive: true}}.IsZero())
	c.False(Ref{Node: &Group{Extra: extra(`{"a":1}`)}}.IsZero())
	c.False(Ref{Node: &Group{Children: Nodes{nil}}}.IsZero())
	c.False(Ref{Node: &Group{OnlyWhen: Ref{Node: &Const{}}}}.IsZero())
	c.False(Ref{Node: &Const{}}.IsZero())
}

// TestCompactJSON covers the copy made of JSON that cannot be compacted.
func TestCompactJSON(t *testing.T) {
	c := check.New(t)
	c.Nil(compactJSON(nil))
	bad := extra(`{"a": `)
	out := compactJSON(bad)
	c.Equal(string(bad), string(out))
	out[0] = 'x'
	c.Equal(byte('{'), bad[0])
	c.Equal(nodeHash(&Const{Extra: bad}), nodeHash(&Const{Extra: extra(`{"a": `)}))
}
