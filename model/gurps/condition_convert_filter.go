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
	"github.com/richardwilkes/gcs/v5/model/condition"
)

// conditionFromFilterGroup converts the root group of a legacy list filter that tests fields into a condition tree
// that a condition.Matcher with a filterHost for fields and FailedIsUnmet set matches exactly as the filter does. Each
// condition keeps only the criteria its field's kind reads. Nodes whose outcome is fixed become legacy consts: empty
// groups (passing unless negated), unknown nodes and conditions on unknown fields (never passing).
func conditionFromFilterGroup[T Node[T]](root *FilterGroup, fields []*FilterField[T]) condition.Node {
	if root == nil {
		return nil
	}
	return convertFilterNode(root, filterFieldKinds(fields))
}

func filterFieldKinds[T Node[T]](fields []*FilterField[T]) map[string]FilterFieldKind {
	kinds := make(map[string]FilterFieldKind, len(fields))
	for _, f := range fields {
		kinds[f.Key] = f.Kind
	}
	return kinds
}

func convertFilterNode(n FilterNode, kinds map[string]FilterFieldKind) condition.Node {
	switch t := n.(type) {
	case *FilterGroup:
		if len(t.Children) == 0 {
			return legacyConst(!t.Not, legacyJSON(t))
		}
		g := &condition.Group{Mode: condition.GroupAny, Not: t.Not, Children: make(condition.Nodes, len(t.Children))}
		if t.All {
			g.Mode = condition.GroupAll
		}
		for i, child := range t.Children {
			g.Children[i] = convertFilterNode(child, kinds)
		}
		return g
	case *FilterCondition:
		kind, known := kinds[t.Field]
		if !known {
			return legacyConst(false, legacyJSON(t))
		}
		c := &condition.Check{Field: t.Field, Not: t.Not}
		switch kind {
		case FilterFieldText, FilterFieldList:
			c.Text = escapeText(t.Text)
		case FilterFieldNumber:
			c.Number = t.Number
		case FilterFieldWeight:
			c.Weight = t.Weight
		default:
		}
		return c
	case *UnknownFilterNode:
		return legacyConst(false, t.Data)
	default:
		return nil
	}
}
