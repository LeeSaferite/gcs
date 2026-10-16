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
	"encoding/json/jsontext"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
)

func intPtr(v int64) *fxp.Int {
	x := fxp.FromInteger(v)
	return &x
}

func weightPtr(v int64) *fxp.Weight {
	x := fxp.Weight(fxp.FromInteger(v))
	return &x
}

func textCrit(cmp criteria.StringComparison, qualifier string) criteria.Text {
	var t criteria.Text
	t.Compare = cmp
	t.Qualifier = qualifier
	return t
}

func numberCrit(cmp criteria.NumericComparison, qualifier int64) criteria.Number {
	var n criteria.Number
	n.Compare = cmp
	n.Qualifier = fxp.FromInteger(qualifier)
	return n
}

func numberCritPtr(cmp criteria.NumericComparison, qualifier int64) *criteria.Number {
	n := numberCrit(cmp, qualifier)
	return &n
}

func weightCrit(cmp criteria.NumericComparison, qualifier int64) criteria.Weight {
	var w criteria.Weight
	w.Compare = cmp
	w.Qualifier = fxp.Weight(fxp.FromInteger(qualifier))
	return w
}

func extra(s string) jsontext.Value {
	return jsontext.Value(s)
}

// fullTree returns a tree holding every node type, with every field reachable from it set to a value that is in
// canonical form and that matters to the canonical view. It is not a valid tree; Validate reports several problems.
func fullTree() *Group {
	return &Group{
		Mode:      GroupAtLeast,
		N:         2,
		Not:       true,
		Exclusive: true,
		OnlyWhen: Ref{Node: &Check{
			Field:        "container",
			Not:          true,
			Text:         textCrit(criteria.ContainsText, "box"),
			Number:       numberCrit(criteria.AtLeastNumber, 2),
			Weight:       weightCrit(criteria.AtMostNumber, 3),
			AbsentPasses: true,
			Extra:        extra(`{"c":1}`),
		}},
		Children: Nodes{
			&Const{Value: true, Extra: extra(`{"k":1}`)},
			&Count{
				Of:        "spells",
				Where:     Ref{Node: &Check{Field: "college", Text: textCrit(criteria.IsText, "Fire")}},
				Quantity:  numberCritPtr(criteria.EqualsNumber, 3),
				GroupBy:   "college",
				PerGroup:  numberCritPtr(criteria.AtLeastNumber, 2),
				Exclusive: true,
				Not:       true,
				Extra:     extra(`{"n":1}`),
			},
			&Compare{
				Left: Operand{
					Value:     intPtr(1),
					Weight:    weightPtr(2),
					Attribute: "iq",
					Entity:    "tech_level",
					Self:      "level",
					Aggregate: AggregateMax,
					Members: Members{
						Of:    "skills",
						Where: Ref{Node: &Check{Field: "name", Text: textCrit(criteria.StartsWithText, "Sword")}},
					},
					Field:   "level",
					Default: intPtr(4),
					Sum: []Operand{
						{Value: intPtr(5), Extra: extra(`{"s":1}`)},
						{
							Aggregate: AggregateCount,
							Of:        "traits",
							Where:     Ref{Node: &Check{Field: "tags", Text: textCrit(criteria.IsText, "Psi")}},
						},
					},
					Scale:  fxp.FromInteger(2),
					Offset: fxp.FromInteger(3),
					Extra:  extra(`{"o":1}`),
				},
				Op:    OpAtMost,
				Right: Operand{Attribute: "dx"},
				Not:   true,
				Extra: extra(`{"p":1}`),
			},
			&Script{Name: "Check @Thing@", Script: "has('@Thing@')", Lang: "cel", Extra: extra(`{"q":1}`)},
			&Unknown{Type: "future", Data: extra(`{"type":"future","x":[1,2]}`), Reason: "unknown node type"},
			&Group{Mode: GroupAny, Children: Nodes{&Const{}}},
		},
		Extra: extra(`{"g":1}`),
	}
}
