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
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/filternode"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

// conditionFromFilterGroup converts the root group of a legacy list filter that tests fields into a condition tree
// that a condition.Matcher with a filterHost for fields checks exactly as the filter does. Each condition keeps only
// the criteria its field's kind reads. A group with no children stays an empty group, which is Skipped, as legacy has
// it. Unknown nodes and conditions on unknown fields, which legacy can't check, become Unknown nodes holding their
// legacy JSON verbatim (an unknown node's raw data byte for byte), which are Failed.
func conditionFromFilterGroup[T Node[T]](root *FilterGroup, fields []*FilterField[T]) condition.Node {
	if root == nil {
		return nil
	}
	return convertFilterNode(root, filterFieldKinds(fields))
}

func unknownFieldReason(field string) string {
	return i18n.Text("a filter field this version of GCS doesn't know: ") + field
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
			return &condition.Unknown{Type: filternode.Condition.Key(), Data: legacyJSON(t), Reason: unknownFieldReason(t.Field)}
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
		return &condition.Unknown{Type: t.Kind, Data: t.Data, Reason: i18n.Text("a filter node this version of GCS doesn't know")}
	default:
		return nil
	}
}
