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
	"hash"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

// hashIntPtr writes a presence byte, then the value when present.
func hashIntPtr[T ~int64](h hash.Hash, p *T) {
	xhash.Bool(h, p != nil)
	if p != nil {
		xhash.Num64(h, *p)
	}
}

// canonicalText returns t with the qualifier cleared when the comparison is "any", since it then means nothing.
func canonicalText(t criteria.Text) criteria.Text {
	if t.IsZero() {
		return criteria.Text{}
	}
	return t
}

// canonicalNumber returns n with the qualifier cleared when the comparison is "any".
func canonicalNumber(n criteria.Number) criteria.Number {
	if n.IsZero() {
		return criteria.Number{}
	}
	return n
}

// canonicalWeight returns w with the qualifier cleared when the comparison is "any".
func canonicalWeight(w criteria.Weight) criteria.Weight {
	if w.IsZero() {
		return criteria.Weight{}
	}
	return w
}

// canonicalQuantity returns the canonical form of a quantity of a Count or count Group: nil for nil and for "at least
// 1", which is what nil means, since a count with no quantity asks whether anything passes, unlike a criterion, whose
// zero value means "any"; "at least 0" for "any", which every number passes; otherwise a copy.
func canonicalQuantity(p *criteria.Number) *criteria.Number {
	switch {
	case p == nil || (p.Compare == criteria.AtLeastNumber && p.Qualifier == fxp.One):
		return nil
	case p.Compare == criteria.AnyNumber:
		return &criteria.Number{Compare: criteria.AtLeastNumber}
	default:
		n := *p
		return &n
	}
}

// quantityKnown returns true for a quantity that is nil or has a known comparison.
func quantityKnown(p *criteria.Number) bool {
	return p == nil || p.Compare == p.Compare.EnsureValid()
}

// vacuous returns true for a quantity that every number of members passes.
func vacuous(p *criteria.Number) bool {
	q := canonicalQuantity(p)
	return q != nil && q.Compare == criteria.AtLeastNumber && q.Qualifier <= 0
}

// hashQuantity writes the canonical form of a quantity into the hasher.
func hashQuantity(h hash.Hash, p *criteria.Number) {
	q := canonicalQuantity(p)
	xhash.Bool(h, q != nil)
	if q != nil {
		q.Hash(h)
	}
}
