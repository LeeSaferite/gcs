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
	"testing"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/toolbox/v2/check"
)

func problemStrings(problems []Problem) []string {
	if len(problems) == 0 {
		return nil
	}
	list := make([]string, len(problems))
	for i, p := range problems {
		list[i] = p.Path.String() + ": " + p.Msg
	}
	return list
}

// nested returns levels groups nested one inside the next, with leaf as the child of the innermost.
func nested(levels int, leaf Node) Node {
	n := leaf
	for range levels {
		n = &Group{Children: Nodes{n}}
	}
	return n
}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		node Node
		want []string
	}{
		{"valid check", &Check{Field: "name", Text: textCrit(criteria.IsText, "x")}, nil},
		{"valid const", &Const{}, nil},
		{"valid script", &Script{Script: "true"}, nil},
		{"valid group", &Group{Mode: GroupAtLeast, N: 1, Children: Nodes{&Const{}}}, nil},
		{"valid count", &Count{Of: "spells", GroupBy: "college", PerGroup: &criteria.Number{}, Exclusive: true}, nil},
		{"valid compare", &Compare{Left: Operand{Aggregate: AggregateSum, Members: Members{Of: "traits"}, Field: "level"}, Right: Operand{Value: intPtr(1)}}, nil},
		{"empty group", &Group{}, nil},
		{"at_least zero", &Group{Mode: GroupAtLeast, Children: Nodes{&Const{}}}, []string{
			": the number of children needed is outside 1 to the number of children",
		}},
		{"at_least too many", &Group{Mode: GroupAtLeast, N: 2, Children: Nodes{&Const{}}}, []string{
			": the number of children needed is outside 1 to the number of children",
		}},
		{"N on all", &Group{N: 1, Children: Nodes{&Const{}}}, []string{
			`: a number of children needed is set, but the mode is not "at least"`,
		}},
		{"unknown mode", &Group{Mode: 9}, []string{": unknown group mode"}},
		{"nil child", &Group{Children: Nodes{&Const{}, nil}}, []string{"children[1]: missing child"}},
		{"check without field", &Check{}, []string{": no field"}},
		{"two criteria", &Check{Field: "f", Text: textCrit(criteria.IsText, "a"), Number: numberCrit(criteria.AtLeastNumber, 1)}, []string{
			": more than one criterion",
		}},
		{"unknown text comparison", &Check{Field: "f", Text: textCrit(99, "a")}, []string{": a criterion has an unknown comparison"}},
		{"unknown number comparison", &Check{Field: "f", Number: numberCrit(99, 1)}, []string{": a criterion has an unknown comparison"}},
		{"unknown weight comparison", &Check{Field: "f", Weight: weightCrit(99, 1)}, []string{": a criterion has an unknown comparison"}},
		{"count without of", &Count{}, []string{": no collection"}},
		{"per_group without group_by", &Count{Of: "x", PerGroup: &criteria.Number{}}, []string{
			": a per-group quantity without a group-by field",
		}},
		{"exclusive without group_by", &Count{Of: "x", Exclusive: true}, []string{
			": exclusive without a group-by field",
		}},
		{"count where", &Count{Of: "x", Where: Ref{Node: &Check{}}}, []string{"where: no field"}},
		{"operands without source", &Compare{}, []string{"left: an operand has no value", "right: an operand has no value"}},
		{"operand with two sources", &Compare{Left: Operand{Attribute: "st", Entity: "sm"}, Right: Operand{Self: "level"}}, []string{
			"left: an operand has more than one value",
		}},
		{"unknown op", &Compare{Op: 9, Left: Operand{Value: intPtr(1)}, Right: Operand{Value: intPtr(1)}}, []string{
			": unknown operator",
		}},
		{"mixed constants", &Compare{Left: Operand{Sum: []Operand{{Value: intPtr(1)}, {Attribute: "st"}}}, Right: Operand{Weight: weightPtr(1)}}, []string{
			": number and weight constants are mixed",
		}},
		{"mixed constants on one side", &Compare{Left: Operand{Sum: []Operand{{Value: intPtr(1)}, {Weight: weightPtr(1)}}}, Right: Operand{Entity: "sm"}}, []string{
			": number and weight constants are mixed",
		}},
		{"aggregate problems", &Compare{
			Left:  Operand{Aggregate: AggregateMax, Where: Ref{Node: &Script{Lang: "lua"}}},
			Right: Operand{Aggregate: AggregateCount, Members: Members{Of: "x"}, Default: intPtr(1)},
		}, []string{
			"left: an aggregate has no field",
			"left: an aggregate has no collection",
			"left.where: unknown script language",
			"right: a count has a default",
		}},
		{"unknown aggregate", &Compare{Left: Operand{Aggregate: 77}, Right: Operand{Value: intPtr(1)}}, []string{"left: unknown aggregate"}},
		{"sum problems", &Compare{Left: Operand{Sum: []Operand{{}, {Value: intPtr(1)}}}, Right: Operand{Value: intPtr(1)}}, []string{
			"left.sum[0]: an operand has no value",
		}},
		{"script language", &Script{Script: "x", Lang: "js"}, []string{": unknown script language"}},
		{"unknown", &Unknown{Reason: "because"}, []string{": because"}},
		{"unknown built in code", &Unknown{}, []string{": unknown node"}},
		{"pre-order", &Group{
			N:        1,
			OnlyWhen: Ref{Node: &Check{}},
			Children: Nodes{&Count{}, &Group{Children: Nodes{nil}}},
		}, []string{
			`: a number of children needed is set, but the mode is not "at least"`,
			"only_when: no field",
			"children[0]: no collection",
			"children[1].children[0]: missing child",
		}},
		{"depth 32", nested(31, &Const{}), nil},
		{"depth 33", nested(32, &Check{}), []string{
			"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]: " +
				"nested more than 32 deep",
		}},
		{"too deep operand", nested(31, &Compare{Left: Operand{Value: intPtr(1)}, Right: Operand{Value: intPtr(1)}}), []string{
			"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].left: " +
				"nested more than 32 deep",
			"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].children[0]." +
				"children[0].children[0].children[0].children[0].children[0].children[0].children[0].right: " +
				"nested more than 32 deep",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check.New(t).Equal(tc.want, problemStrings(tc.node.Validate()))
		})
	}
}

// TestValidateDecodedFile checks that decoding flags what Validate then reports: an Unknown node keeps its place and
// its reason.
func TestValidateDecodedFile(t *testing.T) {
	c := check.New(t)
	r := decodeRefBytes(t, []byte(`{"type":"group","mode":"any","children":[null,{"type":"const","value":true},{"type":"nope"}]}`))
	c.Equal([]string{
		"children[0]: missing node",
		`children[2]: unknown node type "nope"`,
	}, problemStrings(r.Node.Validate()))
}
