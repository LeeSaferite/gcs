// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package condition

func (s *nodeStream) compare(depth int) Node {
	c := &Compare{}
	var op *string
	var hasLeft, hasRight bool
	extra, ok := s.members(func(name string) (bool, bool) {
		switch name {
		case "left":
			hasLeft = true
			return true, s.operand(&c.Left, depth+1)
		case "op":
			return true, s.value(&op)
		case "right":
			hasRight = true
			return true, s.operand(&c.Right, depth+1)
		case memberNot:
			return true, s.value(&c.Not)
		default:
			return false, true
		}
	})
	if !ok || op == nil || ((!hasLeft || !hasRight) && depth >= MaxDepth) {
		return nil
	}
	var known bool
	if c.Op, known = ExtractKnownCompareOp(*op); !known {
		return nil
	}
	c.Extra = extra
	return c
}

// operand decodes an operand of a compare into o. It reports false for anything the wire form would reject, which
// makes the whole compare go to decodeNodeRaw.
func (s *nodeStream) operand(o *Operand, depth int) bool {
	if depth > MaxDepth || s.dec.PeekKind() != '{' {
		return false
	}
	s.skip()
	var aggregate string
	extra, ok := s.members(func(name string) (bool, bool) {
		switch name {
		case "value":
			return true, s.value(&o.Value)
		case memberWeight:
			return true, s.value(&o.Weight)
		case "attribute":
			return true, s.value(&o.Attribute)
		case "entity":
			return true, s.value(&o.Entity)
		case "self":
			return true, s.value(&o.Self)
		case "aggregate":
			return true, s.value(&aggregate)
		case "of":
			return true, s.value(&o.Of)
		case memberWhere:
			var ok bool
			o.Where, ok = s.ref(depth + 1)
			return true, ok
		case "field":
			return true, s.value(&o.Field)
		case "default":
			return true, s.value(&o.Default)
		case "sum":
			return true, s.sum(o, depth+1)
		case "scale":
			return true, s.value(&o.Scale)
		case "offset":
			return true, s.value(&o.Offset)
		default:
			return false, true
		}
	})
	if !ok {
		return false
	}
	var known bool
	if o.Aggregate, known = ExtractKnownAggregate(aggregate); !known {
		return false
	}
	o.Extra = extra
	*o = o.canonical()
	return true
}

// sum decodes the sum of an operand: null is nil, and an empty array an empty sum.
func (s *nodeStream) sum(o *Operand, depth int) bool {
	switch s.dec.PeekKind() {
	case 'n':
		o.Sum = nil
		return s.value(&o.Sum)
	case '[':
	default:
		return false
	}
	s.skip()
	o.Sum = []Operand{}
	for more(s.dec.PeekKind(), ']') {
		o.Sum = append(o.Sum, Operand{})
		if !s.operand(&o.Sum[len(o.Sum)-1], depth) {
			return false
		}
	}
	s.skip()
	return true
}
