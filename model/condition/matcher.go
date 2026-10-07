// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

// Matcher matches items against one tree, as filters do, reusing its scratch space and caches between items. It is
// not safe for concurrent use; give each goroutine its own.
type Matcher struct {
	root Node
	e    *evaluator
}

// NewMatcher returns a Matcher for n. When h implements FieldResolver, each field a check reads is looked up once with
// FieldFunc and the func cached. The self tokens of every text qualifier are parsed up front, and when Validate finds
// nothing wrong but nil children, no node is checked for validity again.
func NewMatcher(n Node, h Host, opt Options) *Matcher {
	m := &Matcher{root: n, e: newEvaluator(h, opt)}

	m.e.qualifiers = make(map[*Check]*selfQualifier)
	if resolver, ok := h.(FieldResolver); ok {
		m.e.resolver = resolver
		m.e.fields = make(map[fieldKey]func(Item) (Value, bool))
		m.e.checkFields = make(map[*Check]func(Item) (Value, bool))
	}
	Walk(n, func(_ Path, node Node) bool {
		if c, ok := node.(*Check); ok {
			m.e.selfQualifier(c)
		}
		return true
	})
	m.e.trusted = m.e.treeValid(n, 1)
	return m
}

// Matches returns EvaluateFor(n, h, item, item, opt).State.Passes(). Only the YesNo pass runs, since it alone decides
// the state, and even the root only reports its outcome.
func (m *Matcher) Matches(item Item) bool {
	if m.root == nil {
		return true
	}
	e := m.e
	clear(e.replacements)
	clear(e.scripts)
	e.startPass(false)
	e.self = item
	return e.test(m.root, frame{subject: item, depth: 1}).state.Passes()
}

// Selection is one member Select chose.
type Selection struct {
	Item   Item
	Result Result // in Collect mode
}

// Select returns the members of m's collection that pass m.Where, in collection order, each with the Collect result
// of its where (Skipped when there is no where). It is meant for overrides and the editor; bonus selection on the
// recalc path uses EvaluateFor in YesNo mode per candidate instead. It returns nil when the host doesn't have the
// collection or when any member's where ends in a global Failed, since the selection then cannot be known.
func Select(m Members, h Host, self Item) []Selection {
	items, ok := h.Collection(m.Of, self, self)
	if !ok {
		return nil
	}
	var where Node
	if !m.Where.IsZero() {
		where = m.Where.Node
	}
	var list []Selection
	for _, item := range items {
		r := EvaluateFor(where, h, self, item, Options{Mode: Collect})
		if r.State == Failed && !r.local {
			return nil
		}
		if r.State.Passes() {
			list = append(list, Selection{Item: item, Result: r})
		}
	}
	return list
}
