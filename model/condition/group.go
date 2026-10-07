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
	"encoding/json/jsontext"
	"encoding/json/v2"
	"hash"
	"slices"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Group{}

// Group combines its children: all of them, any of them, or at least N of them must be met. Wire type "group".
type Group struct {
	Mode      GroupMode      `json:"mode"`
	N         int            `json:"n,omitzero"` // at_least only
	Not       bool           `json:"not,omitzero"`
	Exclusive bool           `json:"exclusive,omitzero"`
	OnlyWhen  Ref            `json:"only_when,omitzero"`
	Children  Nodes          `json:"children,omitempty"`
	Extra     jsontext.Value `json:",embed"`
}

// groupFields is the method-less shadow Group is written through.
type groupFields Group

// groupWire is the raw form Group is read through, so that a missing or unknown mode can be detected and the depth of
// each child tracked.
type groupWire struct {
	Mode      *string          `json:"mode"`
	N         int              `json:"n"`
	Not       bool             `json:"not"`
	Exclusive bool             `json:"exclusive"`
	OnlyWhen  jsontext.Value   `json:"only_when"`
	Children  []jsontext.Value `json:"children"`
	Type      string           `json:"type"`
	Extra     jsontext.Value   `json:",embed"`
}

// Kind implements Node.
func (g *Group) Kind() NodeKind {
	return KindGroup
}

// Clone implements Node.
func (g *Group) Clone() Node {
	c := *g
	c.OnlyWhen = g.OnlyWhen.clone()
	c.Children = g.Children.clone()
	c.Extra = slices.Clone(g.Extra)
	return &c
}

// Hash implements Node.
func (g *Group) Hash(h hash.Hash) {
	c := g.canonical()
	xhash.Num8(h, KindGroup)
	xhash.StringWithLen(h, c.Mode.Key())
	xhash.Num64(h, c.N)
	xhash.Bool(h, c.Not)
	xhash.Bool(h, c.Exclusive)
	c.OnlyWhen.hash(h)
	c.Children.hash(h)
	hashExtra(h, c.Extra)
}

// FillWithNameableKeys implements Node.
func (g *Group) FillWithNameableKeys(m, existing map[string]string) {
	g.OnlyWhen.fillWithNameableKeys(m, existing)
	g.Children.fillWithNameableKeys(m, existing)
}

// Validate implements Node.
func (g *Group) Validate() []Problem {
	return validateNode(g)
}

func (g *Group) validate(v *validator, depth int) {
	switch {
	case g.Mode != g.Mode.EnsureValid():
		v.report(i18n.Text("unknown group mode"))
	case g.Mode == GroupAtLeast:
		if g.N < 1 || g.N > len(g.Children) {
			v.report(i18n.Text("the number of children needed is outside 1 to the number of children"))
		}
	case g.N != 0:
		v.report(i18n.Text("a number of children needed is set, but the mode is not \"at least\""))
	}
	v.ref(SlotOnlyWhen, g.OnlyWhen, depth+1)
	for i, child := range g.Children {
		v.child(Slot{Kind: SlotChild, Index: i}, child, depth+1)
	}
}

func (g *Group) walk(w *walker) {
	w.ref(SlotOnlyWhen, g.OnlyWhen)
	for i, child := range g.Children {
		w.child(Slot{Kind: SlotChild, Index: i}, child)
	}
}

// canonical returns a shallow copy in canonical form.
func (g *Group) canonical() Group {
	c := *g
	c.Mode = c.Mode.EnsureValid()
	if c.Mode != GroupAtLeast {
		c.N = 0
	}
	c.Extra = canonicalExtra(c.Extra)
	return c
}

// MarshalJSONTo implements json.MarshalerTo.
func (g *Group) MarshalJSONTo(enc *jsontext.Encoder) error {
	c := g.canonical()
	return encodeTyped(enc, typeGroup, (*groupFields)(&c))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (g *Group) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, g)
}

func (g *Group) decode(raw jsontext.Value, depth int) error {
	var w groupWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	mode, err := requiredKey(w.Mode, "mode", ExtractKnownGroupMode)
	if err != nil {
		return err
	}
	*g = Group{
		Mode:      mode,
		N:         w.N,
		Not:       w.Not,
		Exclusive: w.Exclusive,
		OnlyWhen:  Ref{Node: decodeRef(w.OnlyWhen, depth+1)},
		Children:  decodeNodes(w.Children, depth+1),
		Extra:     canonicalExtra(w.Extra),
	}
	*g = g.canonical()
	return nil
}
