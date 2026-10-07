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
	"sync/atomic"

	"github.com/richardwilkes/gcs/v5/model/criteria"

	"github.com/richardwilkes/gcs/v5/model/fxp"
)

// fakeItem is an in-memory host object: its fields, and the relative collections it owns.
type fakeItem struct {
	fields   map[string]Value
	relative map[string][]*fakeItem
	name     string
}

func newItem(name string, fields map[string]Value) *fakeItem {
	return &fakeItem{name: name, fields: fields}
}

// with adds a relative collection to the item and returns it.
func (it *fakeItem) with(key string, members ...*fakeItem) *fakeItem {
	if it.relative == nil {
		it.relative = make(map[string][]*fakeItem)
	}
	it.relative[key] = members
	return it
}

// fakeScript is the outcome of a fake script, which the host looks up by the script's text.
type fakeScript struct {
	reason string
	state  State
}

// fakeHost is an in-memory Host. Entity collections exclude self; relative collections belong to the subject. A
// field named "name" falls back to the item's name. Scripts are looked up by their text and counted.
type fakeHost struct {
	entity       map[string][]*fakeItem
	attributes   map[string]fxp.Int
	entities     map[string]fxp.Int
	replacements map[string]string
	scripts      map[string]fakeScript
	runs         atomic.Int64
	replaced     atomic.Int64
}

func newHost() *fakeHost {
	return &fakeHost{
		entity:     make(map[string][]*fakeItem),
		attributes: make(map[string]fxp.Int),
		entities:   make(map[string]fxp.Int),
		scripts: map[string]fakeScript{
			"met":     {state: Met},
			"unmet":   {state: Unmet},
			"skipped": {state: Skipped},
			"failed":  {state: Failed, reason: "script failed"},
		},
	}
}

func (h *fakeHost) Collection(key string, self, subject Item) ([]Item, bool) {
	if list, ok := h.entity[key]; ok {
		items := make([]Item, 0, len(list))
		for _, one := range list {
			if Item(one) != self {
				items = append(items, one)
			}
		}
		return items, true
	}
	if it, ok := subject.(*fakeItem); ok {
		if list, has := it.relative[key]; has {
			items := make([]Item, len(list))
			for i, one := range list {
				items[i] = one
			}
			return items, true
		}
	}
	return nil, false
}

func (h *fakeHost) Field(item Item, key string) (Value, bool) {
	it, ok := item.(*fakeItem)
	if !ok {
		return Value{}, false
	}
	if v, has := it.fields[key]; has {
		return v, true
	}
	if key == "name" {
		return Value{Kind: ValueText, Text: it.name}, true
	}
	return Value{}, false
}

func (h *fakeHost) Attribute(key string) (fxp.Int, bool) {
	v, ok := h.attributes[key]
	return v, ok
}

func (h *fakeHost) Entity(key string) (fxp.Int, bool) {
	v, ok := h.entities[key]
	return v, ok
}

func (h *fakeHost) Replacements(_, _ Item) map[string]string {
	h.replaced.Add(1)
	return h.replacements
}

func (h *fakeHost) RunScript(s *Script, _, _ Item) (state State, reason string) {
	h.runs.Add(1)
	out, ok := h.scripts[s.Script]
	if !ok {
		return Failed, "no such script"
	}
	return out.state, out.reason
}

// resolvingHost adds a FieldResolver to fakeHost. It resolves every field except "unresolved", and counts how often
// it is asked.
type resolvingHost struct {
	*fakeHost
	asked atomic.Int64
}

func (h *resolvingHost) FieldFunc(_, key string) (func(Item) (Value, bool), bool) {
	h.asked.Add(1)
	if key == "unresolved" {
		return nil, false
	}
	return func(item Item) (Value, bool) { return h.Field(item, key) }, true
}

func textValue(s string) Value {
	return Value{Kind: ValueText, Text: s}
}

func listValue(list ...string) Value {
	return Value{Kind: ValueList, List: list}
}

func numberValue(n int64) Value {
	return Value{Kind: ValueNumber, Number: fxp.FromInteger(n)}
}

func weightValue(n int64) Value {
	return Value{Kind: ValueWeight, Number: fxp.FromInteger(n)}
}

func boolValue(b bool) Value {
	return Value{Kind: ValueBool, Bool: b}
}

func absentValue() Value {
	return Value{Kind: ValueAbsent}
}

// leafFor returns a leaf that evaluates to the given state: a const for Met and Unmet, a script for Skipped, and an
// Unknown node (a global Failed, which FailedIsUnmet turns into Unmet) for Failed.
func leafFor(s State) Node {
	switch s {
	case Met:
		return &Const{Value: true}
	case Unmet:
		return &Const{}
	case Skipped:
		return &Script{Script: "skipped"}
	default:
		return &Unknown{Type: "x", Reason: "unknown"}
	}
}

// localFailure returns a check that is a member-local Failed for any subject lacking the field "missing".
func localFailure() Node {
	return &Check{Field: "missing", Text: textCrit(criteria.IsText, "x")}
}

func yesNo() Options {
	return Options{}
}

func collect() Options {
	return Options{Mode: Collect}
}
