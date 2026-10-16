// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

// Member names that several node types share.
const (
	memberNot    = "not"
	memberWeight = "weight"
	memberWhere  = "where"
)

// The streaming decoders of each node type. Each mirrors its node's wire form and decode method exactly; returning nil
// sends the node to decodeNodeRaw.

func (s *nodeStream) group(depth int) Node {
	g := &Group{}
	var mode *string
	extra, ok := s.members(func(name string) (bool, bool) {
		switch name {
		case "mode":
			return true, s.value(&mode)
		case "n":
			return true, s.value(&g.N)
		case memberNot:
			return true, s.value(&g.Not)
		case "exclusive":
			return true, s.value(&g.Exclusive)
		case "only_when":
			var ok bool
			g.OnlyWhen, ok = s.ref(depth + 1)
			return true, ok
		case "children":
			var ok bool
			g.Children, ok = s.nodes(depth + 1)
			return true, ok
		default:
			return false, true
		}
	})
	if !ok || mode == nil {
		return nil
	}
	var known bool
	if g.Mode, known = ExtractKnownGroupMode(*mode); !known {
		return nil
	}
	g.Extra = extra
	*g = g.canonical()
	return g
}

func (s *nodeStream) check() Node {
	var w checkWire
	extra, ok := s.members(func(name string) (bool, bool) {
		switch name {
		case "field":
			return true, s.value(&w.Field)
		case memberNot:
			return true, s.value(&w.Not)
		case "text":
			return true, s.value(&w.Text)
		case "number":
			return true, s.value(&w.Number)
		case memberWeight:
			return true, s.value(&w.Weight)
		case "absent_passes":
			return true, s.value(&w.AbsentPasses)
		default:
			return false, true
		}
	})
	if !ok {
		return nil
	}
	c := &Check{Field: w.Field, Not: w.Not, AbsentPasses: w.AbsentPasses, Extra: extra}
	var err error
	if c.Text, err = w.Text.criteria(); err != nil {
		return nil
	}
	if c.Number, err = w.Number.criteria(); err != nil {
		return nil
	}
	if c.Weight, err = w.Weight.criteria(); err != nil {
		return nil
	}
	return c
}

func (s *nodeStream) constant() Node {
	c := &Const{}
	extra, ok := s.members(func(name string) (bool, bool) {
		if name == "value" {
			return true, s.value(&c.Value)
		}
		return false, true
	})
	if !ok {
		return nil
	}
	c.Extra = extra
	return c
}

func (s *nodeStream) script() Node {
	sc := &Script{}
	extra, ok := s.members(func(name string) (bool, bool) {
		switch name {
		case "name":
			return true, s.value(&sc.Name)
		case "script":
			return true, s.value(&sc.Script)
		case "lang":
			return true, s.value(&sc.Lang)
		default:
			return false, true
		}
	})
	if !ok {
		return nil
	}
	sc.Extra = extra
	return sc
}

func (s *nodeStream) count(depth int) Node {
	c := &Count{}
	var quantity, perGroup *numberWire
	extra, ok := s.members(func(name string) (bool, bool) {
		switch name {
		case "of":
			return true, s.value(&c.Of)
		case memberWhere:
			var ok bool
			c.Where, ok = s.ref(depth + 1)
			return true, ok
		case "quantity":
			return true, s.value(&quantity)
		case "group_by":
			return true, s.value(&c.GroupBy)
		case "per_group":
			return true, s.value(&perGroup)
		case "exclusive":
			return true, s.value(&c.Exclusive)
		case memberNot:
			return true, s.value(&c.Not)
		default:
			return false, true
		}
	})
	if !ok {
		return nil
	}
	var err error
	if c.Quantity, err = quantity.quantity(); err != nil {
		return nil
	}
	if c.PerGroup, err = perGroup.quantity(); err != nil {
		return nil
	}
	c.Extra = extra
	return c
}
