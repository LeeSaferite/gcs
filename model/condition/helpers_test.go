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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zeebo/xxh3"
)

// hashWith returns the hash fn writes.
func hashWith(fn func(h hash.Hash)) uint64 {
	h := xxh3.New()
	fn(h)
	return h.Sum64()
}

// refHash returns the hash of a Ref's canonical view.
func refHash(r Ref) uint64 {
	return hashWith(r.hash)
}

// nodeHash returns the hash of n.
func nodeHash(n Node) uint64 {
	return hashWith(func(h hash.Hash) { HashNode(h, n) })
}

// decodeRefBytes decodes data as a Ref, failing the test on error.
func decodeRefBytes(t *testing.T, data []byte) Ref {
	t.Helper()
	var r Ref
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return r
}

// encodeIndented writes v the way GCS saves files, with a trailing newline.
func encodeIndented(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v, jsontext.WithIndent("\t"), json.Deterministic(true))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return string(data) + "\n"
}

// encodeCompact writes v compactly.
func encodeCompact(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return data
}

// wireFiles returns the hand-written inputs under testdata/wire, keyed by base name.
func wireFiles(t testing.TB) map[string][]byte {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "wire", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no wire files: %v", err)
	}
	files := make(map[string][]byte, len(paths))
	for _, p := range paths {
		var data []byte
		if data, err = os.ReadFile(p); err != nil {
			t.Fatal(err)
		}
		files[strings.TrimSuffix(filepath.Base(p), ".json")] = data
	}
	return files
}
