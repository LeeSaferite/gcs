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
	"fmt"
	"strings"

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

// criterionKind identifies which criterion of a check is set.
type criterionKind byte

const (
	criterionNone criterionKind = iota
	criterionText
	criterionNumber
	criterionWeight
)

func (c *Check) criterion() criterionKind {
	switch {
	case !c.Text.IsZero():
		return criterionText
	case !c.Number.IsZero():
		return criterionNumber
	case !c.Weight.IsZero():
		return criterionWeight
	default:
		return criterionNone
	}
}

// decision is what a check decided before Not. When failed is set, fail is the outcome to report instead.
type decision struct {
	have           fxp.Int // the number or weight compared
	haveKind       ValueKind
	fail           outcome
	met            bool
	testsSomething bool // an "any" check scores 0 and gives no hit
	absentPassed   bool // absent_passes decided it
	failed         bool
}

// decide evaluates the check up to its state before Not, filling in d, which the caller owns so that it is never
// copied.
func (c *Check) decide(e *evaluator, f frame, d *decision) {
	if msg := e.problem(c, int(f.depth)); msg != "" {
		d.fail, d.failed = outcome{reason: msg, state: Failed}, true
		return
	}
	v, ok := e.checkField(c, f)
	if !ok {
		d.fail = outcome{reason: fmt.Sprintf(i18n.Text("no field %q"), c.Field), state: Failed, local: true}
		d.failed = true
		return
	}
	kind := c.criterion()
	if v.Kind == ValueAbsent {
		// The subject lacks the field. absent_passes makes that Met; otherwise it meets no criterion, "any" included,
		// so a negated check finds the subjects that lack the field, as legacy filters do since PR #1182.
		if c.AbsentPasses {
			d.met, d.testsSomething, d.absentPassed = true, kind != criterionNone, true
		} else {
			d.testsSomething = true
		}
		return
	}
	d.have, d.haveKind, d.testsSomething = v.Number, v.Kind, true
	switch kind {
	case criterionText:
		if v.Kind != ValueText && v.Kind != ValueList {
			d.fail, d.failed = c.mismatch(v), true
			return
		}
		var fail *Result
		if d.met, fail = e.matchText(c, v, f); fail != nil {
			d.fail, d.failed = fail.outcome(), true
		}
	case criterionNumber:
		if v.Kind != ValueNumber {
			d.fail, d.failed = c.mismatch(v), true
			return
		}
		d.met = c.Number.Matches(v.Number)
	case criterionWeight:
		if v.Kind != ValueWeight {
			d.fail, d.failed = c.mismatch(v), true
			return
		}
		d.met = c.Weight.Matches(fxp.Weight(v.Number))
	default:
		d.met = v.Kind != ValueBool || v.Bool
		d.testsSomething = v.Kind == ValueBool
	}
}

// test is the YesNo evaluation of a check.
func (c *Check) test(e *evaluator, f frame) outcome {
	var d decision
	c.decide(e, f, &d)
	if d.failed {
		return d.fail
	}
	return outcome{state: metState(d.met, c.Not)}
}

// eval implements Node.
func (c *Check) eval(e *evaluator, f frame) Result {
	var d decision
	c.decide(e, f, &d)
	if d.failed {
		return d.fail.result()
	}
	var r Result
	if !d.absentPassed {
		switch c.criterion() {
		case criterionNumber:
			r.Have, r.HaveKind, r.Need = d.have, d.haveKind, c.Number.Qualifier
		case criterionWeight:
			r.Have, r.HaveKind, r.Need = d.have, d.haveKind, fxp.Int(c.Weight.Qualifier)
		default:
		}
	}
	return c.finish(e, f, &r, d.met, d.testsSomething)
}

// finish sets the state, applying Not, and in Collect mode the specificity and hit of a Met check. A check that tests
// nothing ("any") scores 0 and gives no hit.
func (c *Check) finish(e *evaluator, f frame, r *Result, met, testsSomething bool) Result {
	r.State = metState(met, c.Not)
	if e.collect && r.State == Met && testsSomething {
		r.Specificity = 1
		if !c.Not {
			r.hits = []Hit{{Node: c, Item: f.subject}}
		}
	}
	return *r
}

// mismatch returns the member-local Failed outcome of a criterion that does not suit the kind of value the subject
// reported.
func (c *Check) mismatch(v Value) outcome {
	return outcome{
		reason: fmt.Sprintf(i18n.Text("the criterion does not suit the value of field %q (%s)"), c.Field,
			v.Kind.String()),
		state: Failed,
		local: true,
	}
}

// matchText matches a Text or List value against the check's text criterion, expanding any self tokens. A non-nil
// result is the Failed result to report instead.
func (e *evaluator) matchText(c *Check, v Value, f frame) (bool, *Result) {
	special := strings.IndexByte(c.Text.Qualifier, '{') >= 0 ||
		strings.IndexByte(c.Text.Qualifier, byte(nameable.MarkerDelimiter)) >= 0
	var sq *selfQualifier
	if special {
		sq = e.selfQualifier(c)
	}
	if sq == nil {
		var repl map[string]string
		if special && strings.IndexByte(c.Text.Qualifier, byte(nameable.MarkerDelimiter)) >= 0 {
			repl = e.replacementsFor(f)
		}
		if v.Kind == ValueList {
			return c.Text.MatchesList(repl, v.List...), nil
		}
		return c.Text.Matches(repl, v.Text), nil
	}
	return e.matchTokens(c, sq, v, f)
}
