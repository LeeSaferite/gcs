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
	"github.com/richardwilkes/toolbox/v2/check"
)

// TestMatcher checks that a Matcher agrees with EvaluateFor, asks a FieldResolver once per field and falls back to
// Field for a field the resolver can't give, and works with self tokens and hosts that resolve nothing.
func TestMatcher(t *testing.T) {
	c := check.New(t)
	base := newHost()
	h := &resolvingHost{fakeHost: base}
	tree := &Group{Children: Nodes{
		checkText("name", criteria.StartsWithText, "F"),
		&Group{Mode: GroupAny, Children: Nodes{
			checkText("unresolved", criteria.IsText, "{self.name}"),
			&Check{Field: "level", Number: numberCrit(criteria.AtLeastNumber, 2)},
		}},
	}}
	rows := []*fakeItem{
		newItem("Fireball", map[string]Value{"level": numberValue(1), "unresolved": textValue("Fireball")}),
		newItem("Flame Jet", map[string]Value{"level": numberValue(3), "unresolved": textValue("x")}),
		newItem("Water", map[string]Value{"level": numberValue(3)}),
		newItem("Fog", map[string]Value{"unresolved": textValue("x"), "level": numberValue(0)}),
	}
	for _, host := range []Host{h, base} {
		m := NewMatcher(tree, host, Options{FailedIsUnmet: true, Mode: Collect})
		for range 3 {
			for _, row := range rows {
				c.Equal(EvaluateFor(tree, host, row, row, Options{FailedIsUnmet: true}).State.Passes(), m.Matches(row),
					row.name)
			}
		}
	}
	c.Equal(int64(3), h.asked.Load())
	m := NewMatcher(tree, h, Options{})
	c.Equal([]bool{true, true, false, false}, []bool{
		m.Matches(rows[0]), m.Matches(rows[1]), m.Matches(rows[2]), m.Matches(rows[3]),
	})
	c.True(NewMatcher(nil, h, Options{}).Matches(rows[0]), "an empty tree passes everything")
}

// TestSelect checks that Select returns the passing members in order with Collect results, and nil when the
// selection cannot be known.
func TestSelect(t *testing.T) {
	c := check.New(t)
	h, self, spells := spellsHost()
	fire := checkText("college", criteria.IsText, "Fire")
	list := Select(Members{Of: "spells", Where: Ref{Node: fire}}, h, self)
	c.Equal(3, len(list))
	c.Equal(Item(spells[0]), list[0].Item)
	c.Equal(1, list[0].Result.Specificity)
	c.Equal(Item(spells[7]), list[2].Item)
	all := Select(Members{Of: "spells", Where: Ref{Node: &Group{}}}, h, self)
	c.Equal(10, len(all))
	c.Equal(Skipped, all[0].Result.State)
	c.Nil(Select(Members{Of: "zz"}, h, self))
	c.Nil(Select(Members{Of: "spells", Where: Ref{Node: &Unknown{}}}, h, self))
	c.Equal(0, len(Select(Members{Of: "spells", Where: Ref{Node: localFailure()}}, h, self)))
}

// TestPositivelyRequires checks the spell circularity guard: only a positive, demanding count over the collection
// whose where's all-path names the field, with every check on it holding, counts.
func TestPositivelyRequires(t *testing.T) {
	c := check.New(t)
	h := newHost()
	owner := newItem("Owner", nil)
	subject := newItem("Fireball", map[string]Value{"college": textValue("Fire")})
	name := checkText("name", criteria.IsText, "Fireball")
	college := checkText("college", criteria.IsText, "Fire")
	count := func(where Node) *Count {
		return &Count{Of: "spells", Where: Ref{Node: where}}
	}
	for _, tc := range []struct {
		name string
		tree Node
		want bool
	}{
		{"name count", count(name), true},
		{"name in all with college", count(&Group{Children: Nodes{college, nil, name}}), true},
		{"in a group", &Group{Mode: GroupAny, Children: Nodes{&Const{}, count(name)}}, true},
		{"college count", count(college), false},
		{"other collection", &Count{Of: "skills", Where: Ref{Node: name}}, false},
		{"negated count", &Count{Of: "spells", Where: Ref{Node: name}, Not: true}, false},
		{"under not", &Group{Not: true, Children: Nodes{count(name)}}, false},
		{"in a guard", &Group{OnlyWhen: Ref{Node: count(name)}}, false},
		{"is 0", &Count{Of: "spells", Where: Ref{Node: name}, Quantity: numberCritPtr(criteria.EqualsNumber, 0)}, false},
		{"at least 2", &Count{Of: "spells", Where: Ref{Node: name}, Quantity: numberCritPtr(criteria.AtLeastNumber, 2)}, true},
		{"at most 2", &Count{Of: "spells", Where: Ref{Node: name}, Quantity: numberCritPtr(criteria.AtMostNumber, 2)}, false},
		{"negated check", count(&Check{Field: "name", Not: true, Text: textCrit(criteria.IsText, "Water")}), false},
		{"any check", count(&Check{Field: "name"}), false},
		{"name does not hold", count(checkText("name", criteria.IsText, "Water")), false},
		{"sibling does not hold", count(&Group{Children: Nodes{name, checkText("college", criteria.IsText, "Air")}}), false},
		{"sibling not a check", count(&Group{Children: Nodes{name, &Const{Value: true}}}), false},
		{"any group", count(&Group{Mode: GroupAny, Children: Nodes{name}}), false},
		{"negated group", count(&Group{Not: true, Children: Nodes{name}}), false},
		{"guarded group", count(&Group{OnlyWhen: Ref{Node: &Const{Value: true}}, Children: Nodes{name}}), false},
		{"no where", count(nil), false},
		{"compare", &Compare{}, false},
	} {
		c.Equal(tc.want, PositivelyRequires(tc.tree, "spells", "name", h, owner, subject), tc.name)
	}
}
