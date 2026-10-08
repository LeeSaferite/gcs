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
	"strings"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/criteria"
)

// The collection, field and entity keys the converted trees use, beyond the filter field keys.
const (
	conditionTraits            = "traits"
	conditionSkills            = "skills"
	conditionSpells            = "spells"
	conditionEquipped          = "equipped"
	conditionCarried           = "carried"
	conditionOtherEquipment    = "other_equipment"
	conditionChildren          = "children"
	conditionContents          = "contents"
	conditionModifiers         = "modifiers"
	conditionLevel             = "level"
	conditionSpecialization    = "specialization"
	conditionOptionalSpecialty = "optional_specialization"
	conditionCollege           = "college"
	conditionPowerSource       = "power_source"
	conditionContainedWeight   = "contained_weight"
	conditionContainedQuantity = "contained_quantity"
)

// legacyConstPrefix starts the Extra of a const node that stands for a legacy prerequisite or filter node whose outcome
// is a constant: an unknown node, an unknown filter field, an empty filter group, a contained prerequisite on an owner
// that isn't equipment, or a numeric criteria of "any" where the engine has none. The member holds the legacy node's
// JSON verbatim (an unknown node's raw data byte for byte), so a later step can write the legacy node back.
const legacyConstPrefix = `{"legacy":`

// legacyConst returns a const node with the given value standing for the legacy node whose JSON is data.
func legacyConst(value bool, data []byte) *condition.Const {
	extra := make([]byte, 0, len(legacyConstPrefix)+len(data)+1)
	extra = append(extra, legacyConstPrefix...)
	extra = append(append(extra, data...), '}')
	return &condition.Const{Value: value, Extra: extra}
}

// legacyJSON returns the JSON of a legacy node, or null if it can't be encoded.
func legacyJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return data
}

// escapeText returns t with every "{" doubled, so the engine never reads a legacy qualifier as holding self tokens.
func escapeText(t criteria.Text) criteria.Text {
	t.Qualifier = strings.ReplaceAll(t.Qualifier, "{", "{{")
	return t
}

// textCheck returns a check of field against t, or nil when t accepts anything.
func textCheck(field string, t criteria.Text) condition.Node {
	if t.Compare == criteria.AnyText {
		return nil
	}
	return &condition.Check{Field: field, Text: escapeText(t)}
}

// numberCheck returns a check of field against n, or nil when n accepts anything.
func numberCheck(field string, n criteria.Number) condition.Node {
	if n.Compare == criteria.AnyNumber {
		return nil
	}
	return &condition.Check{Field: field, Number: n}
}

// selfCheck returns the check that field is the same as self's, which the skill and spell tech level rule (with
// absentPasses) and the spell same power source rule use.
func selfCheck(field string, absentPasses bool) *condition.Check {
	return &condition.Check{
		Field:        field,
		Text:         criteria.Text{Compare: criteria.IsText, Qualifier: "{self." + field + "}"},
		AbsentPasses: absentPasses,
	}
}

// whereOf returns the where of the non-nil checks: nothing, the check itself, or an all group of them.
func whereOf(checks ...condition.Node) condition.Ref {
	var list condition.Nodes
	for _, one := range checks {
		if one != nil {
			list = append(list, one)
		}
	}
	switch len(list) {
	case 0:
		return condition.Ref{}
	case 1:
		return condition.Ref{Node: list[0]}
	default:
		return condition.Ref{Node: &condition.Group{Children: list}}
	}
}

// numberCompare returns a compare of left against the constant right, with the operator of cmp, which must not be
// "any".
func numberCompare(left *condition.Operand, cmp criteria.NumericComparison, right *condition.Operand, not bool) *condition.Compare {
	return &condition.Compare{Left: *left, Op: condition.ExtractCompareOp(cmp.Key()), Right: *right, Not: not}
}
