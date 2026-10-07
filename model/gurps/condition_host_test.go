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
	"testing"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/fxp"
)

// TestUnknownPrereqBytes checks that an unknown prerequisite's data is kept byte for byte, whitespace included, so a
// later step can write it back, and that it evaluates unmet, as legacy does.
func TestUnknownPrereqBytes(t *testing.T) {
	data := jsontext.Value("{ \"type\" : \"future_prereq\",\n\t\"weird\": [1, 2 ] }")
	tree := conditionFromPrereqs(listOf(NewUnknownPrereq("future_prereq", data)), nil)
	kept, ok := tree.(*condition.Group).Children[0].(*condition.Const)
	if !ok || kept.Value || string(kept.Extra) != legacyConstPrefix+string(data)+"}" {
		t.Errorf("unknown prerequisite converted to %s", treeJSON(t, tree))
	}
}

// TestConditionHostCollectionsAndFields covers the host collections and fields the converted legacy trees don't use.
func TestConditionHostCollectionsAndFields(t *testing.T) {
	e, _ := newConditionTestEntity("3")
	other := NewEquipment(e, nil, false)
	other.Name = "Stash"
	e.OtherEquipment = []*Equipment{other}
	eqpMod := NewEquipmentModifier(e, nil, false)
	eqpMod.Name, eqpMod.Tags = "Fine", []string{"Quality"}
	pack := e.CarriedEquipment[1]
	pack.Modifiers = []*EquipmentModifier{eqpMod}
	h := newConditionHost(e)
	names := func(key string, self, subject condition.Item) []string {
		items, ok := h.Collection(key, self, subject)
		if !ok {
			t.Fatalf("no %s collection", key)
		}
		list := make([]string, 0, len(items))
		for _, one := range items {
			v, _ := h.Field(one, filterFieldKeyName)
			list = append(list, v.Text)
		}
		return list
	}
	magery, group := e.Traits[0], e.Traits[2]
	for _, tc := range []struct {
		key     string
		self    condition.Item
		subject condition.Item
		want    string
	}{
		{conditionTraits, magery, magery, `["Advantages","Fit"]`},
		{conditionCarried, pack, pack, `["Sword","Rope","Pouch","Torch","Lockpicks","Ghost"]`},
		{conditionOtherEquipment, nil, nil, `["Stash"]`},
		{conditionEquipped, nil, nil, `["Sword","Backpack","Rope","Pouch","Torch"]`},
		{conditionChildren, nil, pack, `["Rope","Pouch"]`},
		{conditionContents, nil, pack, `["Rope","Torch"]`},
		{conditionModifiers, nil, pack, `["Fine"]`},
		{conditionModifiers, nil, magery, `["Air"]`},
		{conditionChildren, nil, group, `["Fit"]`},
		{conditionChildren, nil, e.Skills[0], `[]`},
		{conditionChildren, nil, e.Spells[0], `[]`},
		{conditionChildren, nil, eqpMod, `[]`},
	} {
		if got := legacyJSONString(t, names(tc.key, tc.self, tc.subject)); got != tc.want {
			t.Errorf("%s of %v: want %s, got %s", tc.key, tc.subject, tc.want, got)
		}
	}
	if _, ok := h.Collection("weapons", nil, nil); ok {
		t.Error("the host should not have a weapons collection yet")
	}
	for _, tc := range []struct {
		item condition.Item
		key  string
		want condition.Value
	}{
		{magery, conditionLevel, numberValue(fxp.Two)},
		{group, conditionLevel, numberValue(0)},
		{magery, filterFieldKeyNotes, textValue("one college (Air)\nAir (Air only)")},
		{magery, filterFieldKeyPoints, numberValue(magery.AdjustedPoints(nil))},
		{group, filterFieldKeyContainer, boolValue(true)},
		{e.Skills[1], filterFieldKeyTechLevel, textValue("")},
		{e.Skills[2], filterFieldKeyTechLevel, condition.Value{}},
		{e.Skills[0], conditionOptionalSpecialty, textValue("")},
		{e.Skills[0], filterFieldKeyNotes, textValue("")},
		{e.Spells[2], conditionCollege, condition.Value{Kind: condition.ValueList, List: []string{"Fire", "Air"}}},
		{e.Spells[0], filterFieldKeyNotes, textValue("")},
		{pack, conditionContainedQuantity, numberValue(fxp.Five)},
		{pack, conditionContainedWeight, condition.Value{Kind: condition.ValueWeight, Number: fxp.Five}},
		{pack, filterFieldKeyTechLevel, textValue("")},
		{pack, filterFieldKeyNotes, textValue("")},
		{eqpMod, filterFieldKeyTags, condition.Value{Kind: condition.ValueList, List: []string{"Quality"}}},
		{magery.Modifiers[0], filterFieldKeyContainer, boolValue(false)},
	} {
		if got, ok := h.Field(tc.item, tc.key); !ok || legacyJSONString(t, got) != legacyJSONString(t, tc.want) {
			t.Errorf("%s of %T: want %v, got %v (%v)", tc.key, tc.item, tc.want, got, ok)
		}
	}
	for _, item := range []condition.Item{magery, e.Skills[0], e.Spells[0], pack, eqpMod, "x"} {
		if _, ok := h.Field(item, "no_such_field"); ok {
			t.Errorf("%T should not have the field", item)
		}
	}
	if _, ok := h.Entity("sm"); ok {
		t.Error("the host should only know the tech level")
	}
	if h.Replacements("x", nil) != nil {
		t.Error("a non-nameable owner has no replacements")
	}
}

// TestFilterHost covers the filter host's refusals.
func TestFilterHost(t *testing.T) {
	h := newFilterHost(NoteFilterFields())
	if _, ok := h.Collection(conditionTraits, nil, nil); ok {
		t.Error("a filter has no collections")
	}
	if _, ok := h.Attribute("st"); ok {
		t.Error("a filter has no attributes")
	}
	if _, ok := h.Entity(filterFieldKeyTechLevel); ok {
		t.Error("a filter has no entity")
	}
	if state, _ := h.RunScript(&condition.Script{}, nil, nil); state != condition.Failed {
		t.Error("a filter can't run scripts")
	}
	if _, ok := h.Field(NewTrait(nil, nil, false), "text"); ok {
		t.Error("a row of another type has no fields")
	}
	if h.Replacements(nil, nil) != nil {
		t.Error("a filter applies no replacements")
	}
}
