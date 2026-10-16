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

	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Const{}

// Const is a constant Met (true) or Unmet (false). Wire type "const".
type Const struct {
	Value bool           `json:"value"`
	Extra jsontext.Value `json:",embed"`
}

// constFields is the method-less shadow Const is written through.
type constFields Const

// constWire is the raw form Const is read through.
type constWire struct {
	Value bool           `json:"value"`
	Type  string         `json:"type"`
	Extra jsontext.Value `json:",embed"`
}

// Kind implements Node.
func (c *Const) Kind() NodeKind {
	return KindConst
}

// Clone implements Node.
func (c *Const) Clone() Node {
	return &Const{Value: c.Value, Extra: slices.Clone(c.Extra)}
}

// Hash implements Node.
func (c *Const) Hash(h hash.Hash) {
	xhash.Num8(h, KindConst)
	xhash.Bool(h, c.Value)
	hashExtra(h, c.Extra)
}

// FillWithNameableKeys implements Node.
func (c *Const) FillWithNameableKeys(_, _ map[string]string) {}

// Validate implements Node.
func (c *Const) Validate() []Problem {
	return validateNode(c)
}

func (c *Const) validate(_ *validator, _ int) {}

func (c *Const) walk(_ *walker) {}

// MarshalJSONTo implements json.MarshalerTo.
func (c *Const) MarshalJSONTo(enc *jsontext.Encoder) error {
	return encodeTyped(enc, typeConst, &constFields{Value: c.Value, Extra: canonicalExtra(c.Extra)})
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (c *Const) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, c)
}

func (c *Const) decode(raw jsontext.Value, _ int) error {
	var w constWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	*c = Const{Value: w.Value, Extra: canonicalExtra(w.Extra)}
	return nil
}

// eval implements Node.
func (c *Const) eval(e *evaluator, f frame) Result {
	if r, bad := e.invalid(c, f); bad {
		return r
	}
	return Result{State: metState(c.Value, false)}
}
