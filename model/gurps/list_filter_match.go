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

// NewListFilterMatcher returns a function that reports whether a node passes the filter. The filter is converted to a
// condition tree once, here, and the fields it refers to are looked up through the filter host. A nil filter, or one
// without a root, passes everything.
//
// A group passes when all of its children, or any one of them, pass, inverted if the group is negated; a group with
// no children asks for nothing, so it passes everything, negated or not. A condition passes when the node's field
// satisfies its criteria, inverted if negated. A node whose text field is blank, or whose list field holds nothing,
// doesn't have the field, so it satisfies no criteria. A condition on an unknown field, and a node kind this version of
// GCS doesn't understand, never pass, whatever their negation says, since their intent can't be known.
func NewListFilterMatcher[T Node[T]](f *ListFilter, fields []*FilterField[T]) func(T) bool {
	if f == nil || f.Root == nil {
		return func(T) bool { return true }
	}
	m := condition.NewMatcher(conditionFromFilterGroup(f.Root, fields), newFilterHost(fields),
		condition.Options{FailedIsUnmet: true})
	var lock sync.Mutex // a Matcher reuses its scratch space between nodes
	return func(node T) bool {
		lock.Lock()
		defer lock.Unlock()
		return m.Matches(node)
	}
}
