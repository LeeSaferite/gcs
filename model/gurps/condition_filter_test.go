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
	"encoding/json/jsontext"
	"strings"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/filternode"
)

func filterGroupOf(all, not bool, children ...FilterNode) *FilterGroup {
	g := &FilterGroup{Type: filternode.Group, All: all, Not: not, Children: children}
	for _, child := range children {
		child.SetParentGroup(g)
	}
	return g
}

func filterCondition(field string, not bool, text criteria.Text, number criteria.Number, weight criteria.Weight) *FilterCondition {
	return &FilterCondition{Type: filternode.Condition, Field: field, Not: not, Text: text, Number: number, Weight: weight}
}

func weightCriteria(compare criteria.NumericComparison, w fxp.Weight) criteria.Weight {
	return criteria.Weight{Compare: compare, Qualifier: w}
}

func TestConditionFromFilterGroup(t *testing.T) {
	fields := EquipmentFilterFields()
	isSword := textCriteria(criteria.IsText, "{Sword}")
	stale := numberCriteria(criteria.AtLeastNumber, fxp.One)
	unknown := NewUnknownFilterNode("future_node", jsontext.Value(`{"type": "future_node", "weird": [1, 2]}`))
	for _, tc := range []struct {
		name string
		root *FilterGroup
		want string
	}{
		{"empty", filterGroupOf(true, false), `null`},
		{"empty negated", filterGroupOf(false, true), `null`},
		{
			"text, stale number", filterGroupOf(true, false, filterCondition("name", true, isSword, stale, criteria.Weight{})),
			`{"type":"group","mode":"all","children":[{"type":"check","field":"name","not":true,"text":{"compare":"is","qualifier":"{{Sword}"}}]}`,
		},
		{
			"list", filterGroupOf(false, false, filterCondition("tags", false, textCriteria(criteria.ContainsText, "Melee, Ranged"), criteria.Number{}, criteria.Weight{})),
			`{"type":"group","mode":"any","children":[{"type":"check","field":"tags","text":{"compare":"contains","qualifier":"Melee, Ranged"}}]}`,
		},
		{
			"number, stale text", filterGroupOf(true, false, filterCondition("cost", false, isSword, numberCriteria(criteria.AtMostNumber, fxp.Ten), criteria.Weight{})),
			`{"type":"group","mode":"all","children":[{"type":"check","field":"cost","number":{"compare":"at_most","qualifier":10}}]}`,
		},
		{
			"weight", filterGroupOf(true, false, filterCondition("weight", false, criteria.Text{}, stale, weightCriteria(criteria.AtLeastNumber, fxp.WeightFromInteger(2, fxp.Pound)))),
			`{"type":"group","mode":"all","children":[{"type":"check","field":"weight","weight":{"compare":"at_least","qualifier":"2 lb"}}]}`,
		},
		{
			"bool, stale text", filterGroupOf(true, false, filterCondition("container", true, isSword, criteria.Number{}, criteria.Weight{})),
			`{"type":"group","mode":"all","children":[{"type":"check","field":"container","not":true}]}`,
		},
		{
			"negative", filterGroupOf(true, false, filterCondition("tags", true, textCriteria(criteria.IsNotText, "Melee"), stale, criteria.Weight{})),
			`{"type":"group","mode":"all","children":[{"type":"group","mode":"all","not":true,"children":[{"type":"check","field":"tags"},{"type":"check","field":"tags","text":{"compare":"is_not","qualifier":"Melee"}}]}]}`,
		},
	} {
		tree, err := conditionFromFilterGroup(tc.root, fields)
		if err != nil {
			t.Fatal(err)
		}
		if got := treeJSON(t, tree); got != tc.want {
			t.Errorf("%s:\nwant %s\n got %s", tc.name, tc.want, got)
		}
	}
	for _, root := range []*FilterGroup{
		filterGroupOf(true, false, filterCondition("newer", true, isSword, stale, criteria.Weight{})),
		filterGroupOf(true, true, unknown, filterGroupOf(false, true)),
	} {
		if _, err := conditionFromFilterGroup(root, fields); err == nil {
			t.Error("an unknown field or node should fail the conversion")
		}
	}
	if tree, err := conditionFromFilterGroup[*Equipment](nil, fields); tree != nil || err != nil {
		t.Error("a nil root should convert to a nil tree")
	}
}

// filterShapes returns root groups over fields: a condition for each field, comparison its kind suits and qualifier
// drawn from the rows' values, plain and negated, with and without the disabled modifiers where the field has them,
// alone and in all, any and negated groups, plus empty groups, an unknown field, an unknown node and stale criteria.
func filterShapes[T Node[T]](fields []*FilterField[T], rows []T) []*FilterGroup {
	var conditions []FilterNode
	for _, f := range fields {
		for _, not := range []bool{false, true} {
			switch f.Kind {
			case FilterFieldText, FilterFieldList, FilterFieldTextList:
				for _, q := range textQualifiers(f, rows) {
					for _, cmp := range criteria.StringComparisons {
						for _, disabled := range []bool{false, true} {
							c := filterCondition(f.Key, not, textCriteria(cmp, q),
								numberCriteria(criteria.AtLeastNumber, fxp.One), criteria.Weight{})
							c.IncludeDisabled = disabled
							conditions = append(conditions, c)
						}
					}
				}
			case FilterFieldNumber, FilterFieldWeight:
				for _, q := range numberQualifiers(f, rows) {
					for _, cmp := range criteria.NumericComparisons {
						c := filterCondition(f.Key, not, textCriteria(criteria.IsText, "x"), criteria.Number{},
							criteria.Weight{})
						if f.Kind == FilterFieldNumber {
							c.Number = numberCriteria(cmp, q)
						} else {
							c.Weight = weightCriteria(cmp, fxp.Weight(q))
						}
						conditions = append(conditions, c)
					}
				}
			default:
				conditions = append(conditions, filterCondition(f.Key, not, textCriteria(criteria.IsText, "x"),
					criteria.Number{}, criteria.Weight{}))
			}
		}
		conditions = append(conditions, filterCondition("newer_"+f.Key, false, criteria.Text{}, criteria.Number{},
			criteria.Weight{}))
	}
	unknown := func() FilterNode { return NewUnknownFilterNode("future", jsontext.Value(`{"type":"future"}`)) }
	roots := []*FilterGroup{filterGroupOf(true, false), filterGroupOf(false, true)}
	for i, c := range conditions {
		roots = append(roots, filterGroupOf(true, false, c.Clone(nil)))
		if i%5 == 0 && i+3 < len(conditions) {
			pick := func(j int) FilterNode { return conditions[j].Clone(nil) }
			roots = append(roots,
				filterGroupOf(false, false, pick(i), pick(i+1), pick(i+3)),
				filterGroupOf(true, i%2 == 0, pick(i), pick(i+2), filterGroupOf(false, true, pick(i+1), unknown())),
				filterGroupOf(false, true, filterGroupOf(true, false), pick(i+3), unknown()),
			)
		}
	}
	return roots
}

func textQualifiers[T Node[T]](f *FilterField[T], rows []T) []string {
	qualifiers := []string{"", "zzz"}
	for _, row := range rows {
		values := []string{""}
		if f.Kind == FilterFieldText {
			values[0] = f.Text(row)
		} else if list := f.List(row); len(list) != 0 {
			values = list
		} else if f.ListWithDisabled != nil {
			if list = f.ListWithDisabled(row); len(list) != 0 {
				values = list
			}
		}
		if v := values[len(values)-1]; v != "" {
			part := strings.ToUpper(v[:min(3, len(v))])
			return append(qualifiers, v, part, " {"+part, v+", zzz", "zzz,"+part+" ")
		}
	}
	return qualifiers
}

func numberQualifiers[T Node[T]](f *FilterField[T], rows []T) []fxp.Int {
	qualifiers := []fxp.Int{0, fxp.One}
	for _, row := range rows {
		var v fxp.Int
		if f.Kind == FilterFieldNumber {
			v = f.Number(row)
		} else {
			v = fxp.Int(f.Weight(row))
		}
		if v != 0 {
			return append(qualifiers, v, v+fxp.One)
		}
	}
	return qualifiers
}

// diffFilters compares the legacy checker with the engine's, and the matchers built on them, for every shape over every
// row, and returns how many checks agreed. A shape holding a node or field this version of GCS doesn't know fails as a
// whole, which is the one accepted divergence.
func diffFilters[T Node[T]](t *testing.T, label string, fields []*FilterField[T], rows []T, shapes []*FilterGroup) int {
	t.Helper()
	agree := 0
	for _, root := range shapes {
		f := &ListFilter{Name: "x", Root: root}
		legacy, engine, matcher := legacyListFilterChecker(f, fields), NewListFilterChecker(f, fields),
			NewListFilterMatcher(f, fields)
		_, unknown := conditionFromFilterGroup(root, fields)
		for _, row := range rows {
			want, got := legacy(row), engine(row)
			if unknown != nil {
				want = CheckFailed
			}
			if want != got || matcher(row) != (want == CheckMet || want == CheckSkipped) {
				t.Errorf("%s: legacy %v, engine %v for %s", label, want, got, legacyJSONString(t, root))
				break
			}
			agree++
		}
	}
	return agree
}

// TestFilterDifferentialConstructed compares the matchers over the rows the legacy filter tests use.
func TestFilterDifferentialConstructed(t *testing.T) {
	alertness := NewTrait(nil, nil, false)
	alertness.Name, alertness.Tags, alertness.BasePoints = "Alertness", []string{"Mental", "Physical"}, fxp.Five
	untagged := NewTrait(nil, nil, false)
	untagged.Name = "Untagged"
	group := NewTrait(nil, nil, true)
	group.Name = "Group"
	braces := NewTrait(nil, nil, false)
	braces.Name, braces.Tags = " {ALE", []string{"zzz", " {MEN"}
	blank := NewTrait(nil, nil, false)
	blank.Name, blank.PageRef, blank.Tags = "  ", "\t", []string{""}
	alertness.Modifiers = []*TraitModifier{
		{Name: "Hard to fool, easy", LocalNotes: "sharp, keen", Tags: []string{"Q"}},
		{Name: "Cheap", LocalNotes: "dull", Disabled: true},
	}
	traits := []*Trait{alertness, untagged, group, braces, blank}
	sack := NewEquipment(nil, nil, false)
	sack.Name, sack.BaseWeight, sack.TechLevel = "Sack", "5 lb", "3"
	nameless := NewEquipment(nil, nil, false)
	chest := NewEquipment(nil, nil, true)
	chest.Name = "Chest"
	sack.Modifiers = []*EquipmentModifier{
		{Name: "Fine", LocalNotes: "well made, light"},
		{Name: "Torn", LocalNotes: "leaks", Tags: []string{"Flaw"}, Disabled: true},
	}
	equipment := []*Equipment{sack, chest, nameless}
	agree := diffFilters(t, "traits", TraitFilterFields(), traits, filterShapes(TraitFilterFields(), traits))
	agree += diffFilters(t, "equipment", EquipmentFilterFields(), equipment,
		filterShapes(EquipmentFilterFields(), equipment))
	t.Logf("%d matches agree", agree)
}
