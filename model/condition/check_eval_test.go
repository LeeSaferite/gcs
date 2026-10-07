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
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/toolbox/v2/check"
)

func checkText(field string, cmp criteria.StringComparison, qualifier string) *Check {
	return &Check{Field: field, Text: textCrit(cmp, qualifier)}
}

// TestCheckValueKinds checks every value kind against every criterion kind, including the member-local Failed of a
// criterion that does not suit the value, and Absent values with each criterion kind.
func TestCheckValueKinds(t *testing.T) {
	h := newHost()
	subject := newItem("Fireball", map[string]Value{
		"tags":    listValue("Fire", "Missile"),
		"none":    listValue(),
		"level":   numberValue(3),
		"weight":  weightValue(5),
		"on":      boolValue(true),
		"off":     boolValue(false),
		"tl":      absentValue(),
		"college": textValue("Fire"),
	})
	for _, tc := range []struct {
		name  string
		node  *Check
		state State
		spec  int
		hit   bool
	}{
		{"text is", checkText("name", criteria.IsText, "fireball"), Met, 1, true},
		{"text is not", checkText("name", criteria.IsNotText, "fireball"), Unmet, 0, false},
		{"text no comma split", checkText("name", criteria.IsText, "Fireball, Water"), Unmet, 0, false},
		{"list is", checkText("tags", criteria.IsText, "missile"), Met, 1, true},
		{"list comma", checkText("tags", criteria.IsText, "Water, Fire"), Met, 1, true},
		{"list is not", checkText("tags", criteria.IsNotText, "Water, Fire"), Unmet, 0, false},
		{"empty list is not", checkText("none", criteria.IsNotText, "x"), Met, 1, true},
		{"empty list is", checkText("none", criteria.IsText, "x"), Unmet, 0, false},
		{"number", &Check{Field: "level", Number: numberCrit(criteria.AtLeastNumber, 3)}, Met, 1, true},
		{"weight", &Check{Field: "weight", Weight: weightCrit(criteria.AtMostNumber, 4)}, Unmet, 0, false},
		{"bool true", &Check{Field: "on"}, Met, 1, true},
		{"bool false", &Check{Field: "off"}, Unmet, 0, false},
		{"not bool false", &Check{Field: "off", Not: true}, Met, 1, false},
		{"any", &Check{Field: "name"}, Met, 0, false},
		{"any list", &Check{Field: "none"}, Met, 0, false},
		{"text on number", checkText("level", criteria.IsText, "3"), Failed, 0, false},
		{"number on text", &Check{Field: "name", Number: numberCrit(criteria.AtLeastNumber, 3)}, Failed, 0, false},
		{"weight on number", &Check{Field: "level", Weight: weightCrit(criteria.AtLeastNumber, 3)}, Failed, 0, false},
		{"text on bool", checkText("on", criteria.IsText, "true"), Failed, 0, false},
		{"missing field", checkText("missing", criteria.IsText, "x"), Failed, 0, false},
		{"absent passes", &Check{Field: "tl", Number: numberCrit(criteria.EqualsNumber, 3), AbsentPasses: true}, Met, 1, true},
		{"absent passes any", &Check{Field: "tl", AbsentPasses: true}, Met, 0, false},
		{"absent not passes", &Check{Field: "tl", Not: true, AbsentPasses: true}, Unmet, 0, false},
		{"absent text zero", checkText("tl", criteria.IsText, ""), Met, 1, true},
		{"absent text", checkText("tl", criteria.IsText, "3"), Unmet, 0, false},
		{"absent number zero", &Check{Field: "tl", Number: numberCrit(criteria.EqualsNumber, 0)}, Met, 1, true},
		{"absent number", &Check{Field: "tl", Number: numberCrit(criteria.AtLeastNumber, 1)}, Unmet, 0, false},
		{"absent weight zero", &Check{Field: "tl", Weight: weightCrit(criteria.AtMostNumber, 0)}, Met, 1, true},
		{"absent any", &Check{Field: "tl"}, Met, 0, false},
		{"two criteria", &Check{Field: "level", Number: numberCrit(criteria.AtLeastNumber, 3), Text: textCrit(criteria.IsText, "x")}, Failed, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			r := EvaluateFor(tc.node, h, nil, subject, collect())
			c.Equal(tc.state, r.State)
			c.Equal(tc.spec, r.Specificity)
			hits := 0
			for hit := range r.Hits() {
				c.Equal(Item(subject), hit.Item)
				c.Equal(Node(tc.node), hit.Node)
				hits++
			}
			c.Equal(tc.hit, hits == 1)
			c.Equal(r.State, EvaluateFor(tc.node, h, nil, subject, yesNo()).State)
		})
	}
}

// TestCheckHaveNeed checks the values a Number or Weight check reports, and the local kind of its failures.
func TestCheckHaveNeed(t *testing.T) {
	c := check.New(t)
	h := newHost()
	subject := newItem("x", map[string]Value{"level": numberValue(3), "weight": weightValue(5)})
	r := EvaluateFor(&Check{Field: "level", Number: numberCrit(criteria.AtLeastNumber, 4)}, h, nil, subject, yesNo())
	c.Equal(fxp.FromInteger(3), r.Have)
	c.Equal(ValueNumber, r.HaveKind)
	c.Equal(fxp.FromInteger(4), r.Need)
	r = EvaluateFor(&Check{Field: "weight", Weight: weightCrit(criteria.AtLeastNumber, 2)}, h, nil, subject, yesNo())
	c.Equal(fxp.FromInteger(5), r.Have)
	c.Equal(ValueWeight, r.HaveKind)
	c.Equal(fxp.FromInteger(2), r.Need)
	r = EvaluateFor(checkText("level", criteria.IsText, "3"), h, nil, subject, yesNo())
	c.True(r.MemberLocal())
	c.Contains(r.Reason, "number")
	r = EvaluateFor(&Check{}, h, nil, subject, yesNo())
	c.Equal(Failed, r.State)
	c.False(r.MemberLocal())
}

// TestFailedIsUnmet checks that FailedIsUnmet turns a Failed check or Unknown leaf into Unmet, ignoring the leaf's own
// Not, while an enclosing negated group still flips it.
func TestFailedIsUnmet(t *testing.T) {
	c := check.New(t)
	h := newHost()
	subject := newItem("x", nil)
	opt := Options{FailedIsUnmet: true}
	for _, n := range []Node{
		&Check{Field: "missing"},
		&Check{Field: "missing", Not: true},
		&Unknown{},
		&Check{Field: "x", Text: textCrit(criteria.IsText, "a"), Number: numberCrit(criteria.AtLeastNumber, 1)},
	} {
		c.Equal(Unmet, EvaluateFor(n, h, nil, subject, opt).State)
		c.Equal(Met, EvaluateFor(&Group{Not: true, Children: Nodes{n}}, h, nil, subject, opt).State)
	}
	// Other nodes keep their Failed.
	c.Equal(Failed, EvaluateFor(&Script{Script: "failed"}, h, nil, subject, opt).State)
	c.Equal(Failed, EvaluateFor(&Group{N: 3, Children: Nodes{&Const{}}}, h, nil, subject, opt).State)
}

// TestReplacements checks that qualifiers use the host's replacements, fetched once per (self, subject).
func TestReplacements(t *testing.T) {
	c := check.New(t)
	h := newHost()
	h.replacements = map[string]string{"Spell": "Fireball"}
	subject := newItem("Fireball", nil)
	g := &Group{Children: Nodes{
		checkText("name", criteria.IsText, "@Spell@"),
		checkText("name", criteria.StartsWithText, "@Spell@"),
		checkText("name", criteria.IsText, "Fireball"),
	}}
	c.Equal(Met, EvaluateFor(g, h, nil, subject, collect()).State)
	c.Equal(int64(1), h.replaced.Load())
}
