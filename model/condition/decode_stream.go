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
	"encoding/json/jsontext"
	"encoding/json/v2"
	"sync"
)

// nodeStream decodes a tree in one pass over its bytes, which the caller has already read, and so validated, as one
// JSON value. Each node is decoded straight from the token stream, children included. Anything unusual (a node whose
// first member isn't "type", a member name or type with escapes, a node too deep, a value of the wrong kind, an
// unknown type, enum key or compare key) makes it skip to the end of that node and hand the node's bytes to
// decodeNodeRaw, so every such case decodes exactly as before, Unknown reasons included.
type nodeStream struct {
	dec *jsontext.Decoder
	raw jsontext.Value
}

// nodeStreams keeps streams for reuse. A pooled stream's decoder still refers to the bytes it last read until it is
// reused or the pool lets it go at the next collection.
var nodeStreams sync.Pool

// decodeNode decodes one node at the given depth from raw, which must be one valid JSON value. It never fails:
// anything it can't turn into a typed node becomes an Unknown node holding the compacted raw bytes and the reason.
func decodeNode(raw jsontext.Value, depth int) Node {
	s, ok := nodeStreams.Get().(*nodeStream)
	if ok {
		s.dec.Reset(bytes.NewBuffer(raw))
	} else {
		s = &nodeStream{dec: jsontext.NewDecoder(bytes.NewBuffer(raw))}
	}
	s.raw = raw
	n := s.node(depth)
	s.raw = nil
	nodeStreams.Put(s)
	return n
}

// node decodes the next value as a node.
func (s *nodeStream) node(depth int) Node {
	start := s.dec.InputOffset()
	if s.dec.PeekKind() != '{' || depth > MaxDepth {
		return decodeNodeRaw(s.next(), depth)
	}
	base := s.dec.StackDepth()
	s.skip()
	var n Node
	if name, ok := s.name(); ok && name == "type" {
		if typ, isStr := s.name(); isStr {
			n = s.typed(typ, depth)
		}
	}
	if n == nil {
		return s.fallback(start, base, depth)
	}
	return n
}

// typed decodes the members after "type" of a node of the given type, returning nil when it can't.
func (s *nodeStream) typed(typ string, depth int) Node {
	switch typ {
	case typeGroup:
		return s.group(depth)
	case typeCheck:
		return s.check()
	case typeConst:
		return s.constant()
	case typeCount:
		return s.count(depth)
	case typeCompare:
		return s.compare(depth)
	case typeScript:
		return s.script()
	default:
		return nil
	}
}

// fallback skips to the end of the node that started at start, base being the stack depth before its opening brace,
// and decodes its bytes with decodeNodeRaw.
func (s *nodeStream) fallback(start int64, base, depth int) Node {
	for s.dec.StackDepth() > base && s.dec.PeekKind() != 0 {
		s.skip()
	}
	raw := s.raw[start:s.dec.InputOffset()]
	return decodeNodeRaw(raw[bytes.IndexByte(raw, '{'):], depth)
}

// name reads a string, reporting false for anything else or for a string that still holds an escape, which the
// decoder keeps only for characters such as quotes that no known name has.
func (s *nodeStream) name() (string, bool) {
	if s.dec.PeekKind() != '"' {
		return "", false
	}
	raw := s.next()
	if bytes.IndexByte(raw, '\\') >= 0 {
		return "", false
	}
	return string(raw[1 : len(raw)-1]), true
}

// members calls fn with the name of each remaining member of the current object, then reads its closing brace.
// Members fn doesn't take are kept as unknown members. It reports false when fn does, or for a name with escapes.
func (s *nodeStream) members(fn func(name string) (handled, ok bool)) (extra jsontext.Value, ok bool) {
	for more(s.dec.PeekKind(), '}') {
		name, isName := s.name()
		if !isName {
			return nil, false
		}
		handled, good := fn(name)
		if !good {
			return nil, false
		}
		if !handled {
			value := s.next()
			if len(extra) == 0 {
				extra = append(extra, '{')
			} else {
				extra = append(extra, ',')
			}
			extra = append(append(append(append(extra, '"'), name...), '"', ':'), value...)
		}
	}
	s.skip()
	if len(extra) != 0 {
		extra = append(extra, '}')
	}
	return canonicalExtra(extra), true
}

// value decodes the next value into v, exactly as the wire forms do. The value is read first and then decoded on its
// own, since a failed decode straight from the stream would leave the stream unusable for skipping past the node.
func (s *nodeStream) value(v any) bool {
	raw := s.next()
	switch p := v.(type) {
	case *string:
		if raw.Kind() == '"' && bytes.IndexByte(raw, '\\') < 0 {
			*p = string(raw[1 : len(raw)-1])
			return true
		}
	case *bool:
		switch raw.Kind() {
		case 't', 'f':
			*p = raw.Kind() == 't'
			return true
		default:
		}
	default:
	}
	return json.Unmarshal(raw, v) == nil
}

// ref decodes a single node slot: null is nil, and so is the canonical empty group.
func (s *nodeStream) ref(depth int) (Ref, bool) {
	if s.dec.PeekKind() == 'n' {
		s.skip()
		return Ref{}, true
	}
	n := s.node(depth)
	if (Ref{Node: n}).IsZero() {
		n = nil
	}
	return Ref{Node: n}, true
}

// nodes decodes a list of nodes: null is nil, and an empty array an empty list.
func (s *nodeStream) nodes(depth int) (Nodes, bool) {
	switch s.dec.PeekKind() {
	case 'n':
		s.skip()
		return nil, true
	case '[':
	default:
		return nil, false
	}
	s.skip()
	list := Nodes{}
	for more(s.dec.PeekKind(), ']') {
		list = append(list, s.node(depth))
	}
	s.skip()
	return list, true
}

// skip reads one token and next one value. The bytes were validated as one JSON value before decoding began, and the
// stream only reads where the structure has something to read, so neither can fail; were the stream ever to break,
// PeekKind would report 0, which ends every loop here.
func (s *nodeStream) skip() {
	s.dec.ReadToken() //nolint:errcheck // See above.
}

func (s *nodeStream) next() jsontext.Value {
	raw, _ := s.dec.ReadValue() //nolint:errcheck // See skip.
	return raw
}

// more reports whether a list or object goes on: the next kind is not its end, and the stream is not broken.
func more(kind, end jsontext.Kind) bool {
	return kind != end && kind != 0
}
