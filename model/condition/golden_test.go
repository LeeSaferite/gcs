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
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

var updateGoldens = flag.Bool("update", false, "rewrite the .golden files under testdata/wire")

// wireExpectation describes what the root of a wire file decodes to. A nil root has noRoot set; an Unknown root names
// the reason it must carry.
type wireExpectation struct {
	kind   NodeKind
	noRoot bool
	reason string
}

var wireExpectations = map[string]wireExpectation{
	"canonicalize":         {kind: KindGroup},
	"case_keys":            {kind: KindGroup},
	"check_any":            {kind: KindCheck},
	"check_number":         {kind: KindCheck},
	"check_text":           {kind: KindCheck},
	"check_weight":         {kind: KindCheck},
	"compare":              {kind: KindCompare},
	"compare_max":          {kind: KindCompare},
	"compare_weight":       {kind: KindCompare},
	"const":                {kind: KindConst},
	"count":                {kind: KindCount},
	"count_any":            {kind: KindCount},
	"count_plain":          {kind: KindCount},
	"deep_compare_ok":      {kind: KindGroup},
	"depth_32":             {kind: KindGroup},
	"depth_33":             {kind: KindGroup},
	"empty_group_extra":    {kind: KindGroup},
	"empty_only_when":      {kind: KindGroup},
	"empty_root":           {noRoot: true},
	"exclusive_pool":       {kind: KindGroup},
	"extra_whitespace":     {kind: KindConst},
	"group":                {kind: KindGroup},
	"group_at_least":       {kind: KindGroup},
	"malformed":            {kind: KindUnknown, reason: "field"},
	"malformed_operand":    {kind: KindUnknown, reason: "not an object"},
	"missing_mode":         {kind: KindUnknown, reason: "missing mode"},
	"missing_op":           {kind: KindUnknown, reason: "missing op"},
	"nil_ref":              {noRoot: true},
	"not_object":           {kind: KindUnknown, reason: "array"},
	"null_child":           {kind: KindGroup},
	"script":               {kind: KindScript},
	"script_cel":           {kind: KindScript},
	"script_extra":         {kind: KindScript},
	"tl_rule":              {kind: KindCheck},
	"too_deep_sum":         {kind: KindGroup},
	"unknown_aggregate":    {kind: KindUnknown, reason: `unknown aggregate "average"`},
	"unknown_compare_key":  {kind: KindUnknown, reason: `unknown text comparison "matches"`},
	"unknown_member":       {kind: KindCheck},
	"unknown_mode":         {kind: KindUnknown, reason: `unknown mode "most"`},
	"unknown_op":           {kind: KindUnknown, reason: `unknown op "approximately"`},
	"unknown_quantity_key": {kind: KindUnknown, reason: `unknown numeric comparison "around"`},
	"unknown_type":         {kind: KindUnknown, reason: `unknown node type "future_thing"`},
	"unknown_whitespace":   {kind: KindUnknown, reason: `unknown node type "future"`},
}

// TestWireGoldens decodes every hand-written wire file and checks that it encodes to its .golden file, that the
// .golden file round-trips byte for byte both indented and compact, and that the input, the golden and a clone all
// hash equal.
func TestWireGoldens(t *testing.T) {
	files := wireFiles(t)
	for name, data := range files {
		t.Run(name, func(t *testing.T) {
			c := check.New(t)
			expect, ok := wireExpectations[name]
			c.True(ok, "no expectation for "+name)
			r := decodeRefBytes(t, data)
			if expect.noRoot {
				c.Nil(r.Node)
			} else {
				c.NotNil(r.Node)
				c.Equal(expect.kind, r.Node.Kind())
				if u, isUnknown := r.Node.(*Unknown); isUnknown {
					c.Contains(u.Reason, expect.reason)
				}
			}
			out := encodeIndented(t, r)
			goldenPath := filepath.Join("testdata", "wire", name+".golden")
			if *updateGoldens {
				c.NoError(os.WriteFile(goldenPath, []byte(out), 0o640))
			}
			golden, err := os.ReadFile(goldenPath)
			c.NoError(err)
			c.Equal(string(golden), out)

			again := decodeRefBytes(t, golden)
			c.Equal(string(golden), encodeIndented(t, again))
			compact := encodeCompact(t, again)
			c.Equal(string(compact), string(encodeCompact(t, decodeRefBytes(t, compact))))
			c.Equal(string(golden), encodeIndented(t, decodeRefBytes(t, compact)))

			c.Equal(refHash(r), refHash(again))
			c.Equal(refHash(r), refHash(r.clone()))
		})
	}
	for name := range wireExpectations {
		_, ok := files[name]
		check.New(t).True(ok, "expectation without a wire file: "+name)
	}
}
