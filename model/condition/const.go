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
	"hash"

	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Const{}

// Const is a constant Met (true) or Unmet (false). Wire type "const".
type Const struct {
	Value bool `json:"value"`
}

// constFields is the shadow Const is written and read through; see codec.go.
type constFields Const

// Clone implements Node.
func (c *Const) Clone() Node {
	return &Const{Value: c.Value}
}

// Hash implements Node.
func (c *Const) Hash(h hash.Hash) {
	xhash.Num8(h, hashConst)
	xhash.Bool(h, c.Value)
}

// FillWithNameableKeys implements Node.
func (c *Const) FillWithNameableKeys(_, _ map[string]string) {}

func (c *Const) validate(_ *validator) {}

func (c *Const) walk(_ *walker) {}

// MarshalJSONTo implements json.MarshalerTo.
func (c *Const) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encodeTyped(enc, typeConst, (*constFields)(c))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (c *Const) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, c)
}

func (c *Const) decode(raw jsontext.Value, depth int) error {
	return decodeFields(raw, depth, (*constFields)(c))
}

// eval implements Node.
func (c *Const) eval(_ *evaluator, _ frame) Result {
	return Result{State: metState(c.Value, false)}
}
