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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/toolbox/v2/check"
)

// TestNodesMarshaledOnTheirOwn checks that every node type writes its type and canonical form when marshaled
// directly rather than through a Ref, and decodes back into its own type.
func TestNodesMarshaledOnTheirOwn(t *testing.T) {
	for _, tc := range []struct {
		node Node
		want string
		into Node
	}{
		{&Group{N: 4, Children: Nodes{&Const{}}}, `{"type":"group","mode":"all","children":[{"type":"const","value":false}]}`, &Group{}},
		{&Check{Field: "f", Text: textCrit(criteria.AnyText, "stale")}, `{"type":"check","field":"f"}`, &Check{}},
		{&Const{Value: true, Extra: extra(` { "a" : 1 } `)}, `{"type":"const","value":true,"a":1}`, &Const{}},
		{&Count{Of: "x", Quantity: numberCritPtr(criteria.AtLeastNumber, 1)}, `{"type":"count","of":"x"}`, &Count{}},
		{&Compare{Left: Operand{Value: intPtr(1), Scale: 10000}, Right: Operand{Value: intPtr(0)}}, `{"type":"compare","left":{"value":1},"op":"is","right":{"value":0}}`, &Compare{}},
		{&Script{Script: "x", Extra: extra(`{}`)}, `{"type":"script","script":"x"}`, &Script{}},
		{&Unknown{Type: "z", Data: extra(`{"type":"z","q":1}`)}, `{"type":"z","q":1}`, &Unknown{}},
	} {
		t.Run(kindNames[tc.node.Kind()], func(t *testing.T) {
			c := check.New(t)
			data, err := json.Marshal(tc.node)
			c.NoError(err)
			c.Equal(tc.want, string(data))
			c.NoError(json.Unmarshal(data, tc.into))
			again, err := json.Marshal(tc.into)
			c.NoError(err)
			c.Equal(tc.want, string(again))
			c.Equal(nodeHash(tc.node), nodeHash(tc.into))
		})
	}
}

// TestNodeUnmarshalErrors checks the errors a typed node reports when the data can't become that type.
func TestNodeUnmarshalErrors(t *testing.T) {
	c := check.New(t)
	var g Group
	c.HasError(json.Unmarshal([]byte(`{"type":"const","value":true}`), &g))
	err := json.Unmarshal([]byte(`{"type":"group","mode":"bogus"}`), &g)
	c.HasError(err)
	c.Contains(err.Error(), `unknown mode "bogus"`)
	c.HasError(json.Unmarshal([]byte(`{"type":"group"`), &g))
	for _, n := range []any{&Check{}, &Const{}, &Count{}, &Compare{}, &Script{}, &Unknown{}, &Operand{}, &Ref{}, &Nodes{}, &Selector{}} {
		c.HasError(json.Unmarshal([]byte(`{"type":`), n))
	}
	var o Operand
	c.HasError(json.Unmarshal([]byte(`[]`), &o))
	c.HasError(json.Unmarshal([]byte(`{"value":"x"}`), &o))
	c.HasError(json.Unmarshal([]byte(`{"aggregate":"most"}`), &o))
	c.HasError(json.Unmarshal([]byte(`{"sum":[{"aggregate":"most"}]}`), &o))
	c.NoError(json.Unmarshal([]byte(`{"sum":[{"value":1},{"attribute":"st"}],"scale":1,"aggregate":"count","of":"x"}`), &o))
	c.Equal(SourceMany, o.Source())
	data, err := json.Marshal(&o)
	c.NoError(err)
	c.Equal(`{"aggregate":"count","of":"x","sum":[{"value":1},{"attribute":"st"}]}`, string(data))
}

// TestUnknownDecodeReasons checks that each kind of bad node becomes Unknown, with a reason, rather than an error.
func TestUnknownDecodeReasons(t *testing.T) {
	for _, tc := range []struct {
		data   string
		reason string
	}{
		{`{"type":5}`, "type"},
		{`{"type":"check","text":{"compare":"bad"}}`, `unknown text comparison "bad"`},
		{`{"type":"check","number":{"compare":"bad"}}`, `unknown numeric comparison "bad"`},
		{`{"type":"check","weight":{"compare":"bad"}}`, `unknown weight comparison "bad"`},
		{`{"type":"check","text":5}`, "text"},
		{`{"type":"const","value":"yes"}`, "value"},
		{`{"type":"count","per_group":{"compare":"bad"}}`, `unknown numeric comparison "bad"`},
		{`{"type":"count","of":7}`, "of"},
		{`{"type":"compare","op":"is","left":{},"right":[]}`, "not an object"},
		{`{"type":"compare","op":"is","left":{},"right":{},"not":3}`, "not"},
		{`{"type":"compare","op":"is","left":null}`, "not an object"},
		{`{"type":"script","script":false}`, "script"},
		{`{"type":"group","mode":"all","n":"two"}`, "n"},
	} {
		t.Run(tc.data, func(t *testing.T) {
			c := check.New(t)
			r := decodeRefBytes(t, []byte(tc.data))
			u, ok := r.Node.(*Unknown)
			c.True(ok)
			if ok {
				c.Contains(u.Reason, tc.reason)
				c.Equal(tc.data, string(u.Data))
			}
		})
	}
}

// TestMissingOperand checks that an absent side of a compare reads as an operand with no source.
func TestMissingOperand(t *testing.T) {
	c := check.New(t)
	r := decodeRefBytes(t, []byte(`{"type":"compare","op":"is","right":{"value":1}}`))
	c.Equal(`{"type":"compare","left":{},"op":"is","right":{"value":1}}`, string(encodeCompact(t, r)))
	c.Equal([]string{"left: an operand has no value"}, problemStrings(r.Node.Validate()))
}

// TestDepthLimits checks that a depth-33 file and a too-deep sum round-trip byte for byte, compact as well as
// indented, with the too-deep subtree held as Unknown.
func TestDepthLimits(t *testing.T) {
	for _, name := range []string{"depth_33", "too_deep_sum"} {
		t.Run(name, func(t *testing.T) {
			c := check.New(t)
			data, err := os.ReadFile(filepath.Join("testdata", "wire", name+".json"))
			c.NoError(err)
			data = []byte(strings.TrimSpace(string(data)))
			r := decodeRefBytes(t, data)
			c.Equal(string(data), string(encodeCompact(t, r)))
			deepest := 0
			var unknown *Unknown
			Walk(r.Node, func(path Path, n Node) bool {
				if len(path) > deepest {
					deepest = len(path)
				}
				if u, ok := n.(*Unknown); ok {
					unknown = u
				}
				return true
			})
			c.NotNil(unknown)
			c.Equal("nested more than 32 deep", unknown.Reason)
			problems := r.Node.Validate()
			c.Equal(1, len(problems))
		})
	}
}

// TestNullsAndEmpties covers how nil and empty slots are written and read.
func TestNullsAndEmpties(t *testing.T) {
	c := check.New(t)
	data, err := json.Marshal(Ref{Node: &Group{Mode: GroupAny}})
	c.NoError(err)
	c.Equal("null", string(data))
	data, err = json.Marshal(Nodes{nil, &Const{}})
	c.NoError(err)
	c.Equal(`[null,{"type":"const","value":false}]`, string(data))
	var nodes Nodes
	c.NoError(json.Unmarshal(data, &nodes))
	c.Equal(2, len(nodes))
	u, ok := nodes[0].(*Unknown)
	c.True(ok)
	if ok {
		c.Equal("", u.Type)
		c.Equal("null", string(u.Data))
	}
	c.NoError(json.Unmarshal([]byte(`null`), &nodes))
	c.Nil(nodes)
	var r Ref
	c.NoError(json.Unmarshal([]byte(`{"type":"group","mode":"all","only_when":{"type":"group","mode":"any"}}`), &r))
	c.Nil(r.Node)

	data, err = json.Marshal(&struct {
		Root Ref `json:"root,omitzero"`
	}{Root: Ref{Node: &Group{}}})
	c.NoError(err)
	c.Equal(`{}`, string(data))
}

// TestUnknownEncodings covers an Unknown built in code, and one read directly.
func TestUnknownEncodings(t *testing.T) {
	c := check.New(t)
	data, err := json.Marshal(&Unknown{Type: "later"})
	c.NoError(err)
	c.Equal(`{"type":"later"}`, string(data))
	data, err = json.Marshal(&Unknown{})
	c.NoError(err)
	c.Equal(`null`, string(data))
	var u Unknown
	c.NoError(json.Unmarshal([]byte(` [ 1 ] `), &u))
	c.Equal("", u.Type)
	c.Equal("[1]", string(u.Data))
	c.NoError(json.Unmarshal([]byte(`{"type":"group","mode":"all"}`), &u))
	c.Equal("group", u.Type)
	c.NoError(json.Unmarshal([]byte(`{"type":3}`), &u))
	c.Equal("", u.Type)
	c.Equal(KindUnknown, u.Kind())
	c.Equal(nodeHash(&u), nodeHash(u.Clone()))
}

// TestEncodeErrors checks that a bad value built in code fails to encode rather than writing invalid JSON.
func TestEncodeErrors(t *testing.T) {
	c := check.New(t)
	bad := extra(`[1]`)
	for _, v := range []any{
		Nodes{&Const{Extra: bad}},
		Ref{Node: &Group{Children: Nodes{&Const{Extra: bad}}}},
		&Unknown{Data: extra(`{"a":`)},
	} {
		_, err := json.Marshal(v)
		c.HasError(err)
	}
	var buf strings.Builder
	enc := jsontext.NewEncoder(&buf)
	c.NoError(enc.WriteToken(jsontext.BeginObject))
	c.HasError(Nodes{}.MarshalJSONTo(enc))
}

// TestSelector covers the standalone persisted form of Members.
func TestSelector(t *testing.T) {
	c := check.New(t)
	var s Selector
	c.NoError(json.Unmarshal([]byte(`{"of":"skills","where":{"type":"check","field":"name","text":{"compare":"is","qualifier":"@Skill@"}},"later":[ 1 ]}`), &s))
	c.Equal("skills", s.Of)
	data, err := json.Marshal(s)
	c.NoError(err)
	c.Equal(`{"of":"skills","where":{"type":"check","field":"name","text":{"compare":"is","qualifier":"@Skill@"}},"later":[1]}`, string(data))
	clone := s.Clone()
	c.Equal(hashWith(s.Hash), hashWith(clone.Hash))
	clone.Extra[1] = 'X'
	c.NotEqual(hashWith(s.Hash), hashWith(clone.Hash))
	clone = s.Clone()
	clone.Of = "spells"
	c.NotEqual(hashWith(s.Hash), hashWith(clone.Hash))
	m := make(map[string]string)
	s.FillWithNameableKeys(m, nil)
	c.Equal(1, len(m))
	c.Nil(s.Validate())
	var paths []string
	s.Walk(func(path Path, _ Node) bool {
		paths = append(paths, path.String())
		return true
	})
	c.Equal([]string{"where"}, paths)

	empty := Selector{Where: Ref{Node: &Check{}}}
	c.Equal([]string{": no collection", "where: no field"}, problemStrings(empty.Validate()))
	data, err = json.Marshal(Selector{Where: Ref{Node: &Group{}}, Extra: extra(`{}`)})
	c.NoError(err)
	c.Equal(`{}`, string(data))
}
