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

// presentCheck returns c, or for a negative criterion the all group of a check that the subject has the field and c.
// The engine finds a negative criterion met by a subject that lacks the field, where legacy finds it unmet.
func presentCheck(c *condition.Check) condition.Node {
	if !c.Text.Compare.IsNotType() && c.Number.Compare != criteria.NotEqualsNumber &&
		c.Weight.Compare != criteria.NotEqualsNumber {
		return c
	}
	return &condition.Group{Children: condition.Nodes{&condition.Check{Field: c.Field}, c}}
}

// selfCheck returns the check that field is the same as self's, which the spell same power source rule uses.
func selfCheck(field string) *condition.Check {
	return &condition.Check{Field: field, Text: criteria.Text{Compare: criteria.IsText, Qualifier: "{self." + field + "}"}}
}

// techLevelRule returns the rule a skill or spell with a tech level applies to the skills or spells its
// prerequisites count: one without a tech level counts, and one with a tech level counts when it is the owner's.
func techLevelRule() condition.Node {
	return &condition.Group{Mode: condition.GroupAny, Children: condition.Nodes{
		&condition.Check{Field: filterFieldKeyTechLevel, Not: true},
		selfCheck(filterFieldKeyTechLevel),
	}}
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
// "any", or with the opposite operator when not is set.
func numberCompare(left *condition.Operand, cmp criteria.NumericComparison, right *condition.Operand, not bool) *condition.Compare {
	op := condition.ExtractCompareOp(cmp.Key())
	if not {
		op = oppositeOps[op]
	}
	return &condition.Compare{Left: *left, Op: op, Right: *right}
}

// oppositeOps maps each compare operator to the one that holds exactly when it doesn't.
var oppositeOps = map[condition.CompareOp]condition.CompareOp{
	condition.OpIs:       condition.OpIsNot,
	condition.OpIsNot:    condition.OpIs,
	condition.OpAtLeast:  condition.OpLessThan,
	condition.OpLessThan: condition.OpAtLeast,
	condition.OpAtMost:   condition.OpMoreThan,
	condition.OpMoreThan: condition.OpAtMost,
}
