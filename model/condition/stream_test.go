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

// TestStreamingDecodeAgrees checks that the streaming decoder gives exactly what the raw decoder gives for the forms
// it handles itself and for each kind of input it hands back to the raw decoder.
func TestStreamingDecodeAgrees(t *testing.T) {
	for _, data := range []string{
		`{"type":"group","mode":"all","only_when":null,"children":[{"type":"const","value":true}]}`,
		`{"type":"group","mode":"all","children":null,"x":1}`,
		`{"type":"group","mode":"all","children":5}`,
		`{"type":"group","mode":"all","children":[]}`,
		`{"type":"check","fi\u0065ld":"x"}`,
		`{"t\u0079pe":"check","field":"x"}`,
		`{"type":"ch\u0065ck","field":"x"}`,
		`{"type":"check","a\"b":"x"}`,
		`{"type":"x\"y","field":"x"}`,
		`{"type":"check","field":"a\"b","text":{"compare":"is","qualifier":"q"}}`,
		`{"type":"check","field":"x","not":null,"absent_passes":"yes"}`,
		`{"type":"count","of":"spells","later":{"a":1},"quantity":{"compare":"is","qualifier":2}}`,
		`{"type":"compare","left":{"value":1,"later":[1]},"op":"is","right":{"sum":null},"z":true}`,
		`{"type":"compare","left":{"sum":[]},"op":"is","right":{"sum":5}}`,
		`{"type":"compare","left":{"sum":[{"value":1},{"aggregate":"count","of":"kids","where":{"type":"const"}}]},"op":"is","right":{"value":1}}`,
		`{"op":"is","right":{"value":1},"type":"compare"}`,
		`{"type":"compare","op":"is","right":{"value":1}}`,
		`{"type":"script","script":"x","name":null,"lang":"","k":0}`,
		`{"type":"const","value":null,"k":0}`,
		`[{"type":"const","value":true}]`,
		`"text"`,
	} {
		t.Run(data, func(t *testing.T) {
			c := check.New(t)
			streamed := decodeNode([]byte(data), 1)
			plain := decodeNodeRaw([]byte(data), 1)
			c.Equal(nodeHash(plain), nodeHash(streamed))
			c.Equal(string(encodeCompact(t, Ref{Node: plain})), string(encodeCompact(t, Ref{Node: streamed})))
		})
	}
}

// TestEvaluationEdges covers invalid nodes met in Collect mode, a Matcher reading fields inside a where through the
// host's resolver, and a tree whose only invalid node sits under an operand's sum.
func TestEvaluationEdges(t *testing.T) {
	c := check.New(t)
	h := &resolvingHost{fakeHost: newHost()}
	c.Equal(Failed, Evaluate(nested(32, &Const{Value: true}), h, nil, collect()).State)
	c.Equal(Failed, Evaluate(nested(33, &Const{Value: true}), h, nil, collect()).State)
	kid := newItem("kid", map[string]Value{"level": numberValue(3)})
	row := newItem("row", nil).with("kids", kid)
	count := &Count{Of: "kids", Where: Ref{Node: &Check{
		Field: "level", Number: numberCrit(criteria.AtLeastNumber, 2),
	}}}
	m := NewMatcher(count, h, Options{})
	c.True(m.Matches(row))
	c.True(m.Matches(row))
	sum := &Compare{
		Left: Operand{Sum: []Operand{
			{Value: intPtr(1)},
			{Aggregate: AggregateCount, Of: "kids", Where: Ref{Node: &Unknown{}}},
		}},
		Right: Operand{Value: intPtr(1)},
	}
	c.Equal(Failed, Evaluate(sum, h, row, yesNo()).State)
}
