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
	"sync"

	"github.com/richardwilkes/gcs/v5/model/condition"
)

// NewListFilterChecker returns a function that checks a node against the filter. The filter is converted to a condition
// tree once, here, and the fields it refers to are looked up through the filter host. A nil filter, or one without a
// root, is skipped for every node.
//
// A group is checked as a list of prerequisites is: children that are skipped are left out, and a group with nothing
// left is skipped. Of the rest, one unmet child makes an "all of" group unmet, and one met child makes an "any of"
// group met, even when others failed. Otherwise the group has failed when any of the rest has, and is met or unmet as
// they all are. A condition is met when the node's field satisfies its criteria. A node whose text field is empty or
// only space, whose list field holds nothing, or that the field's Has says lacks it doesn't have the field, so it
// satisfies no criteria. Negating a group or condition swaps met and unmet. A condition on an unknown field, and a
// node kind this version of GCS doesn't understand, fail, since their intent can't be known.
func NewListFilterChecker[T Node[T]](f *ListFilter, fields []*FilterField[T]) func(T) CheckResult {
	if f == nil || f.Root == nil {
		return func(T) CheckResult { return CheckSkipped }
	}
	m := condition.NewMatcher(conditionFromFilterGroup(f.Root, fields), newFilterHost(fields), condition.Options{})
	var lock sync.Mutex // a Matcher reuses its scratch space between nodes
	return func(node T) CheckResult {
		lock.Lock()
		defer lock.Unlock()
		return checkResultOf(m.State(node))
	}
}

// NewListFilterMatcher returns a function that reports whether a node passes the filter: whether NewListFilterChecker
// finds the filter met or skipped for it. A filter with nothing in it is skipped, so it passes everything, while one
// that is unmet or can't be checked hides the node.
func NewListFilterMatcher[T Node[T]](f *ListFilter, fields []*FilterField[T]) func(T) bool {
	check := NewListFilterChecker(f, fields)
	return func(node T) bool {
		result := check(node)
		return result == CheckMet || result == CheckSkipped
	}
}
