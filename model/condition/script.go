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

	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhash"
)

var _ Node = &Script{}

// Script is evaluated by the host. It has no Not. Wire type "script".
type Script struct {
	Name   string         `json:"name,omitzero"`
	Script string         `json:"script"`
	Lang   string         `json:"lang,omitzero"` // "" is js; anything else evaluates Failed and is never run
	Extra  jsontext.Value `json:",embed"`
}

// scriptFields is the method-less shadow Script is written through.
type scriptFields Script

// scriptWire is the raw form Script is read through.
type scriptWire struct {
	Name   string         `json:"name"`
	Script string         `json:"script"`
	Lang   string         `json:"lang"`
	Type   string         `json:"type"`
	Extra  jsontext.Value `json:",embed"`
}

// Kind implements Node.
func (s *Script) Kind() NodeKind {
	return KindScript
}

// Clone implements Node.
func (s *Script) Clone() Node {
	other := *s
	other.Extra = slices.Clone(s.Extra)
	return &other
}

// Hash implements Node.
func (s *Script) Hash(h hash.Hash) {
	xhash.Num8(h, KindScript)
	xhash.StringWithLen(h, s.Name)
	xhash.StringWithLen(h, s.Script)
	xhash.StringWithLen(h, s.Lang)
	hashExtra(h, s.Extra)
}

// FillWithNameableKeys implements Node.
func (s *Script) FillWithNameableKeys(m, existing map[string]string) {
	nameable.Extract(m, existing, s.Name, s.Script)
}

// Validate implements Node.
func (s *Script) Validate() []Problem {
	return validateNode(s)
}

func (s *Script) validate(v *validator, _ int) {
	if s.Lang != "" {
		v.report(i18n.Text("unknown script language"))
	}
}

func (s *Script) walk(_ *walker) {}

// MarshalJSONTo implements json.MarshalerTo.
func (s *Script) MarshalJSONTo(enc *jsontext.Encoder) error {
	other := *s
	other.Extra = canonicalExtra(s.Extra)
	return encodeTyped(enc, typeScript, (*scriptFields)(&other))
}

// UnmarshalJSONFrom implements json.UnmarshalerFrom.
func (s *Script) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return unmarshalNode(dec, s)
}

func (s *Script) decode(raw jsontext.Value, _ int) error {
	var w scriptWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	*s = Script{Name: w.Name, Script: w.Script, Lang: w.Lang, Extra: canonicalExtra(w.Extra)}
	return nil
}

// eval implements Node. The host runs the script, at most once per (node, self, subject) in one evaluation. A script
// that fails is a global Failed. A script in any language but the default is invalid and never runs.
func (s *Script) eval(e *evaluator, f frame) Result {
	if r, bad := e.invalid(s, f); bad {
		return r
	}
	out := e.runScript(s, f)
	return Result{State: out.state, Reason: out.reason}
}
