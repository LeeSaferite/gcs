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

// filterMatches runs a converted list filter the way filters run: a Matcher over a host with no entity.
func filterMatches(tree Node, row *fakeItem) bool {
	return NewMatcher(tree, newHost(), Options{}).Matches(row)
}

// TestLegacyFilterShapes reproduces the cases of model/gurps/list_filter_test.go on converted trees.
func TestLegacyFilterShapes(t *testing.T) {
	trait := newItem("Alertness", map[string]Value{
		"tags": listValue("Mental", "Physical"), "points": numberValue(5), "container": boolValue(false),
	})
	untagged := newItem("Untagged", map[string]Value{"tags": listValue(), "container": boolValue(false)})
	group := newItem("Group", map[string]Value{"tags": listValue(), "container": boolValue(true)})
	sack := newItem("Sack", map[string]Value{"weight": weightValue(5)})
	text := func(field string, cmp criteria.StringComparison, q string) Node { return checkText(field, cmp, q) }
	number := func(cmp criteria.NumericComparison, n int64) Node {
		return &Check{Field: "points", Number: numberCrit(cmp, n)}
	}
	weight := func(cmp criteria.NumericComparison, n int64) Node {
		return &Check{Field: "weight", Weight: weightCrit(cmp, n)}
	}
	unknown := &Unknown{Type: "newer"}
	for _, tc := range []struct {
		name string
		tree Node
		row  *fakeItem
		want bool
	}{
		{"text is", text("name", criteria.IsText, "alertness"), trait, true},
		{"text is not", text("name", criteria.IsNotText, "alertness"), trait, false},
		{"text contains", text("name", criteria.ContainsText, "LERT"), trait, true},
		{"text does not contain", text("name", criteria.DoesNotContainText, "vision"), trait, true},
		{"text starts with", text("name", criteria.StartsWithText, "ness"), trait, false},
		{"text ends with", text("name", criteria.EndsWithText, "NESS"), trait, true},
		{"text anything", &Check{Field: "name"}, trait, true},
		{"list is", text("tags", criteria.IsText, "physical"), trait, true},
		{"list comma", text("tags", criteria.IsText, "Mental, Physical"), trait, true},
		{"list comma no match", text("tags", criteria.IsText, "Social, Exotic"), trait, false},
		{"list does not contain", text("tags", criteria.DoesNotContainText, "ment"), trait, false},
		{"empty list is", text("tags", criteria.IsText, "Mental"), untagged, false},
		{"empty list is not", text("tags", criteria.IsNotText, "Mental"), untagged, true},
		{"empty list anything", &Check{Field: "tags"}, untagged, true},
		{"number is", number(criteria.EqualsNumber, 5), trait, true},
		{"number at least", number(criteria.AtLeastNumber, 6), trait, false},
		{"number at most", number(criteria.AtMostNumber, 5), trait, true},
		{"bool true", &Check{Field: "container"}, group, true},
		{"bool false", &Check{Field: "container"}, trait, false},
		{"weight is", weight(criteria.EqualsNumber, 5), sack, true},
		{"weight at least", weight(criteria.AtLeastNumber, 6), sack, false},
		{"weight anything", &Check{Field: "weight"}, sack, true},
		// Empty groups are folded to constants by the converter: true, and false when negated.
		{"empty group", &Const{Value: true}, trait, true},
		{"negated empty group", &Const{}, trait, false},
		// Negation on a condition and on a group.
		{"negated condition", &Check{Field: "name", Not: true, Text: textCrit(criteria.IsText, "Alertness")}, trait, false},
		{"negated group", &Group{Not: true, Children: Nodes{text("name", criteria.IsText, "Acute Vision")}}, trait, true},
		{"negated is not", &Check{Field: "name", Not: true, Text: textCrit(criteria.IsNotText, "Alertness")}, trait, true},
		// A condition on a field this build doesn't know never matches, whatever its negation says.
		{"unknown field", text("newer", criteria.IsText, "x"), trait, false},
		{"negated unknown field", &Check{Field: "newer", Not: true, Text: textCrit(criteria.IsText, "x")}, trait, false},
		{"unknown field anything", &Check{Field: "newer"}, trait, false},
		// An unknown node never passes; a negation on its group inverts the group's result.
		{"unknown in all", &Group{Children: Nodes{text("name", criteria.IsText, "Alertness"), unknown}}, trait, false},
		{"unknown in any", &Group{Mode: GroupAny, Children: Nodes{unknown}}, trait, false},
		{"unknown in any with match", &Group{Mode: GroupAny, Children: Nodes{unknown, &Check{Field: "name"}}}, trait, true},
		{"negated group of unknown", &Group{Not: true, Children: Nodes{unknown}}, trait, false},
	} {
		check.New(t).Equal(tc.want, filterMatches(tc.tree, tc.row), tc.name)
	}
}
