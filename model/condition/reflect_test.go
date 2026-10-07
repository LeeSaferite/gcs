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
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/check"
)

var (
	jsonValueType = reflect.TypeFor[jsontext.Value]()
	textType      = reflect.TypeFor[criteria.Text]()
	scriptType    = reflect.TypeFor[Script]()
	unknownType   = reflect.TypeFor[Unknown]()
)

// visitFields calls fn for every exported field reachable from v, depth first, descending through structs, non-nil
// pointers, non-nil interfaces and slice elements. jsontext.Value fields are leaves. Unknown.Reason is skipped, since
// it is exempt from hashing.
func visitFields(v reflect.Value, path string, fn func(path string, owner reflect.Type, f reflect.Value)) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			visitFields(v.Elem(), path, fn)
		}
	case reflect.Struct:
		t := v.Type()
		for i := range t.NumField() {
			sf := t.Field(i)
			if !sf.IsExported() || (t == unknownType && sf.Name == "Reason") {
				continue
			}
			f := v.Field(i)
			p := path + "." + sf.Name
			fn(p, t, f)
			if f.Type() != jsonValueType {
				visitFields(f, p, fn)
			}
		}
	case reflect.Slice:
		if v.Type() != jsonValueType {
			for i := range v.Len() {
				p := path + "[" + strconv.Itoa(i) + "]"
				fn(p, nil, v.Index(i))
				visitFields(v.Index(i), p, fn)
			}
		}
	default:
	}
}

// mutate changes f to a different value. It reports false for a value it has no mutation for (plain structs, whose
// fields are mutated individually).
func mutate(f reflect.Value) bool {
	switch p := f.Addr().Interface().(type) {
	case *GroupMode:
		*p = (*p + 1) % (LastGroupMode + 1)
	case *CompareOp:
		*p = (*p + 1) % (LastCompareOp + 1)
	case *Aggregate:
		*p = (*p + 1) % (LastAggregate + 1)
	case *criteria.StringComparison:
		*p = (*p + 1) % (criteria.LastStringComparison + 1)
	case *criteria.NumericComparison:
		*p = (*p + 1) % (criteria.LastNumericComparison + 1)
	case *jsontext.Value:
		*p = jsontext.Value(`{"mutated":true}`)
	case *Nodes:
		*p = append(slices.Clone(*p), &Const{})
	case *[]Operand:
		*p = append(slices.Clone(*p), Operand{Value: intPtr(9)})
	default:
		switch f.Kind() {
		case reflect.Bool:
			f.SetBool(!f.Bool())
		case reflect.String:
			f.SetString(f.String() + "x")
		case reflect.Int:
			f.SetInt(f.Int() + 1)
		case reflect.Int64:
			f.SetInt(f.Int() + 2*int64(fxp.One))
		case reflect.Pointer:
			if f.IsNil() {
				f.Set(reflect.New(f.Type().Elem()))
			} else {
				f.SetZero()
			}
		case reflect.Interface:
			if f.IsNil() {
				f.Set(reflect.ValueOf(&Const{Value: true}))
			} else {
				f.SetZero()
			}
		default:
			return false
		}
	}
	return true
}

// TestHashFollowsEncoding mutates every exported field reachable from a full tree, one at a time, and checks that the
// hash changes exactly when the encoding does: a mutation to a canonical equivalent leaves both alone. The exception
// is Unknown.Type, which is hashed but not written, since an Unknown writes its data back as it was read.
func TestHashFollowsEncoding(t *testing.T) {
	c := check.New(t)
	tree := fullTree()
	baseHash := nodeHash(tree)
	baseEnc := string(encodeCompact(t, tree))
	var equivalents []string
	mutations := 0
	visitFields(reflect.ValueOf(tree), "", func(path string, owner reflect.Type, f reflect.Value) {
		saved := reflect.New(f.Type()).Elem()
		saved.Set(f)
		if !mutate(f) {
			return
		}
		mutations++
		sameEnc := string(encodeCompact(t, tree)) == baseEnc
		sameHash := nodeHash(tree) == baseHash
		switch {
		case owner == unknownType && path[len(path)-5:] == ".Type":
			c.False(sameHash, path)
		case sameEnc:
			c.True(sameHash, path+": a canonical equivalent changed the hash")
			equivalents = append(equivalents, path)
		default:
			c.False(sameHash, path+": a change in meaning left the hash alone")
		}
		f.Set(saved)
		c.Equal(baseHash, nodeHash(tree), path+" was not restored")
	})
	c.True(mutations > 80, "too few mutations: "+strconv.Itoa(mutations))
	// Each of these is cleared by the canonical form: the qualifier of an "any" criterion, the members, field and
	// default of an operand with no aggregate, and N on a group whose mode is not at_least.
	c.Equal([]string{
		".Children[1].Members.Where.Node.Number.NumberData.Qualifier",
		".Children[1].Members.Where.Node.Weight.WeightData.Qualifier",
		".Children[2].Left.Members.Where.Node.Number.NumberData.Qualifier",
		".Children[2].Left.Members.Where.Node.Weight.WeightData.Qualifier",
		".Children[2].Left.Sum[0].Members.Of",
		".Children[2].Left.Sum[0].Members.Where.Node",
		".Children[2].Left.Sum[0].Field",
		".Children[2].Left.Sum[0].Default",
		".Children[2].Left.Sum[1].Members.Where.Node.Number.NumberData.Qualifier",
		".Children[2].Left.Sum[1].Members.Where.Node.Weight.WeightData.Qualifier",
		".Children[2].Right.Members.Of",
		".Children[2].Right.Members.Where.Node",
		".Children[2].Right.Field",
		".Children[2].Right.Default",
		".Children[5].N",
	}, equivalents)
}

// TestCloneSharesNothing walks a full tree and its clone side by side and checks that no pointer, slice or interface
// target is shared, and that the clone encodes and hashes the same.
func TestCloneSharesNothing(t *testing.T) {
	c := check.New(t)
	tree := fullTree()
	clone := CloneNode(tree)
	c.Equal(nodeHash(tree), nodeHash(clone))
	c.Equal(string(encodeCompact(t, tree)), string(encodeCompact(t, clone)))
	var originals []reflect.Value
	visitFields(reflect.ValueOf(tree), "", func(_ string, _ reflect.Type, f reflect.Value) {
		originals = append(originals, f)
	})
	i := 0
	visitFields(reflect.ValueOf(clone), "", func(path string, _ reflect.Type, f reflect.Value) {
		o := originals[i]
		i++
		switch f.Kind() {
		case reflect.Pointer, reflect.Slice:
			if !f.IsNil() && (f.Kind() == reflect.Pointer || f.Len() != 0) {
				c.NotEqual(o.Pointer(), f.Pointer(), path+" is shared")
			}
		case reflect.Interface:
			if !f.IsNil() {
				c.NotEqual(o.Elem().Pointer(), f.Elem().Pointer(), path+" is shared")
			}
		default:
		}
	})
	c.Equal(len(originals), i)
	c.Nil(CloneNode(nil))
}

// TestFillReachesEveryQualifier gives every text qualifier and script string reachable from a full tree a distinct
// nameable marker and checks that FillWithNameableKeys finds each one.
func TestFillReachesEveryQualifier(t *testing.T) {
	c := check.New(t)
	tree := fullTree()
	var sources []string
	visitFields(reflect.ValueOf(tree), "", func(path string, owner reflect.Type, f reflect.Value) {
		switch {
		case f.Type() == textType:
			text := f.Addr().Interface().(*criteria.Text) //nolint:errcheck // The type was just checked.
			if text.Compare == criteria.AnyText {
				text.Compare = criteria.IsText
			}
			text.Qualifier = "@Text " + strconv.Itoa(len(sources)) + "@"
			sources = append(sources, text.Qualifier)
		case owner == scriptType && (strings.HasSuffix(path, ".Name") || strings.HasSuffix(path, ".Script")):
			f.SetString("@Script " + strconv.Itoa(len(sources)) + "@")
			sources = append(sources, f.String())
		default:
		}
	})
	c.Equal(6, len(sources))
	m := make(map[string]string)
	tree.FillWithNameableKeys(m, map[string]string{"Text 0": "zero"})
	want := nameable.Extract(nil, map[string]string{"Text 0": "zero"}, sources...)
	c.Equal(len(sources), len(want))
	c.Equal(want, m)

	// Scripts at the top level of a tree are filled too.
	script := tree.Children[3]
	m = make(map[string]string)
	script.FillWithNameableKeys(m, nil)
	c.Equal(2, len(m))

	// Nodes with nothing to fill leave the map alone.
	m = make(map[string]string)
	for _, n := range []Node{&Const{}, &Unknown{}, &Check{Field: "x"}, &Group{Children: Nodes{nil}}} {
		n.FillWithNameableKeys(m, nil)
	}
	c.Equal(0, len(m))
}

// TestEncodingIsDeterministic checks that a full tree encodes identically with and without json.Deterministic.
func TestEncodingIsDeterministic(t *testing.T) {
	c := check.New(t)
	tree := fullTree()
	plain, err := json.Marshal(tree)
	c.NoError(err)
	deterministic, err := json.Marshal(tree, json.Deterministic(true))
	c.NoError(err)
	c.Equal(string(plain), string(deterministic))
}
