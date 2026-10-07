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
	"bytes"
	"encoding/json/v2"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

// FuzzDecode checks, for any input that is valid JSON, that decoding never panics, that the streaming decoder agrees
// with the raw one, that encoding the result and
// decoding it again is a fixed point, that the hash survives the round trip, and that a clone hashes the same. The
// decoded tree is then evaluated on a random host, seeded from the input, which must end within its budgets with both
// modes agreeing on the state. Seeds are the wire files and their goldens.
func FuzzDecode(f *testing.F) {
	for _, data := range wireFiles(f) {
		f.Add(data)
	}
	goldens, err := filepath.Glob(filepath.Join("testdata", "wire", "*.golden"))
	if err != nil {
		f.Fatal(err)
	}
	for _, p := range goldens {
		var data []byte
		if data, err = os.ReadFile(p); err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var r Ref
		if json.Unmarshal(data, &r) != nil {
			return
		}
		// The streaming decoder must give exactly what the raw one gives.
		if streamed, plain := decodeNode(data, 1), decodeNodeRaw(data, 1); nodeHash(streamed) != nodeHash(plain) ||
			!bytes.Equal(encodeCompact(t, Ref{Node: streamed}), encodeCompact(t, Ref{Node: plain})) {
			t.Fatalf("streaming decode differs:\n%s\n%s", encodeCompact(t, Ref{Node: streamed}),
				encodeCompact(t, Ref{Node: plain}))
		}
		first, encErr := json.Marshal(r)
		if encErr != nil {
			t.Fatalf("encode after decode: %v", encErr)
		}
		var again Ref
		if encErr = json.Unmarshal(first, &again); encErr != nil {
			t.Fatalf("decode of encoded data: %v\n%s", encErr, first)
		}
		second, encErr := json.Marshal(again)
		if encErr != nil {
			t.Fatalf("second encode: %v", encErr)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("not idempotent:\n%s\n%s", first, second)
		}
		if refHash(r) != refHash(again) {
			t.Fatalf("hash changed across a round trip:\n%s", first)
		}
		if refHash(r) != refHash(r.clone()) {
			t.Fatalf("clone hashes differently:\n%s", first)
		}
		if r.Node != nil {
			r.Node.Validate()
			Walk(r.Node, func(Path, Node) bool { return true })
			r.Node.FillWithNameableKeys(make(map[string]string), nil)
			h, self := randomHost(rand.New(rand.NewPCG(uint64(len(data)), refHash(r)))) //nolint:gosec // Not for security.
			opt := Options{Budget: 2000, DetailBudget: 2000}
			yes := Evaluate(r.Node, h, self, opt)
			opt.Mode = Collect
			if full := Evaluate(r.Node, h, self, opt); full.State != yes.State {
				t.Fatalf("modes disagree: %v and %v\n%s", yes.State, full.State, first)
			}
		}
	})
}
