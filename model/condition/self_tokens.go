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
	"strconv"
	"strings"

	"github.com/richardwilkes/gcs/v5/model/criteria"
	"github.com/richardwilkes/gcs/v5/model/nameable"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

// maxAlternatives caps how many alternatives self tokens may expand a qualifier into.
const maxAlternatives = 64

const selfTokenPrefix = "{self."

// qualifierPart is a literal run of a text qualifier, or a {self.<field>} token naming a field of self.
type qualifierPart struct {
	text  string
	token bool
}

// selfQualifier is a text qualifier parsed for self tokens. Parsing works on the literal qualifier, so replacement
// text can never inject a token.
type selfQualifier struct {
	parts []qualifierPart
}

// parseSelfTokens splits a qualifier into literal runs and {self.<field>} tokens. "{{" is a literal "{"; any other "{"
// that does not start a complete token is literal too.
func parseSelfTokens(q string) *selfQualifier {
	var sq selfQualifier
	var lit strings.Builder
	flush := func() {
		if lit.Len() != 0 {
			sq.parts = append(sq.parts, qualifierPart{text: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(q); {
		if strings.HasPrefix(q[i:], "{{") {
			lit.WriteByte('{')
			i += 2
			continue
		}
		if strings.HasPrefix(q[i:], selfTokenPrefix) {
			if end := strings.IndexByte(q[i:], '}'); end > len(selfTokenPrefix) {
				flush()
				sq.parts = append(sq.parts, qualifierPart{text: q[i+len(selfTokenPrefix) : i+end], token: true})
				i += end + 1
				continue
			}
		}
		lit.WriteByte(q[i])
		i++
	}
	flush()
	return &sq
}

// selfQualifier returns the parsed qualifier of a check's text criterion, or nil when it holds no "{" and so can be
// matched by the criterion directly. A Matcher caches the parse.
func (e *evaluator) selfQualifier(c *Check) *selfQualifier {
	if !strings.ContainsRune(c.Text.Qualifier, '{') {
		return nil
	}
	if e.qualifiers == nil {
		return parseSelfTokens(c.Text.Qualifier)
	}
	sq, ok := e.qualifiers[c]
	if !ok {
		sq = parseSelfTokens(c.Text.Qualifier)
		e.qualifiers[c] = sq
	}
	return sq
}

// matchTokens matches a value against a qualifier holding self tokens: replacements apply to the literal runs, a List
// value splits the qualifier on commas, token values are substituted verbatim (a List token value giving one
// alternative per element), and the alternatives are then matched with no further replacements.
func (e *evaluator) matchTokens(c *Check, sq *selfQualifier, v Value, f frame) (bool, *Result) {
	repl := e.replacementsFor(f)
	var pieces [][]string
	total := 0
	for _, piece := range splitPieces(sq.parts, repl, v.Kind == ValueList) {
		alts := []string{""}
		for _, part := range piece {
			values := []string{part.text}
			if part.token {
				var absentPasses bool
				var fail *Result
				if values, absentPasses, fail = e.tokenValues(c, part.text); fail != nil {
					return false, fail
				} else if absentPasses {
					return true, nil
				}
			}
			next := make([]string, 0, len(alts)*len(values))
			for _, alt := range alts {
				for _, one := range values {
					next = append(next, alt+one)
				}
			}
			if alts = next; total+len(alts) > maxAlternatives {
				r := failGlobal(fmt.Sprintf(i18n.Text("self tokens expand to more than %d alternatives"),
					maxAlternatives))
				return false, &r
			}
		}
		total += len(alts)
		pieces = append(pieces, alts)
	}
	elements := []string{v.Text}
	if v.Kind == ValueList {
		elements = v.List
		if len(elements) == 0 {
			elements = []string{""}
		}
	}
	positive, negative := positiveForm(c.Text.Compare)
	for _, alts := range pieces {
		for _, alt := range alts {
			for _, one := range elements {
				if positive.Matches(alt, one) {
					return !negative, nil
				}
			}
		}
	}
	return negative, nil
}

// splitPieces applies the replacements to the literal runs and, for a List value, splits the parts into pieces on the
// commas of those runs, trimming each piece and dropping empty ones. Token values are never split.
func splitPieces(parts []qualifierPart, repl map[string]string, split bool) [][]qualifierPart {
	var pieces [][]qualifierPart
	var current []qualifierPart
	end := func() {
		if n := len(current); n != 0 {
			if !current[0].token {
				current[0].text = strings.TrimLeft(current[0].text, " \t\n\r")
			}
			if !current[n-1].token {
				current[n-1].text = strings.TrimRight(current[n-1].text, " \t\n\r")
			}
			if n > 1 || current[0].token || current[0].text != "" {
				pieces = append(pieces, current)
			}
		}
		current = nil
	}
	for _, part := range parts {
		if part.token {
			current = append(current, part)
			continue
		}
		text := nameable.Apply(part.text, repl)
		if !split {
			current = append(current, qualifierPart{text: text})
			continue
		}
		segments := strings.Split(text, ",")
		for i, segment := range segments {
			if i != 0 {
				end()
			}
			current = append(current, qualifierPart{text: segment})
		}
	}
	end()
	if len(pieces) == 0 {
		pieces = append(pieces, []qualifierPart{{}})
	}
	return pieces
}

// tokenValues returns the values a self token stands for. A field self lacks is a global Failed. An Absent field
// passes the whole check when absent_passes is set, and is otherwise "".
func (e *evaluator) tokenValues(c *Check, name string) (values []string, absentPasses bool, fail *Result) {
	v, ok := e.field(e.self, "", name)
	if !ok {
		r := failGlobal(fmt.Sprintf(i18n.Text("self has no field %q"), name))
		return nil, false, &r
	}
	switch v.Kind {
	case ValueAbsent:
		if c.AbsentPasses {
			return nil, true, nil
		}
		return []string{""}, false, nil
	case ValueList:
		if len(v.List) == 0 {
			return []string{""}, false, nil
		}
		return v.List, false, nil
	case ValueNumber, ValueWeight:
		return []string{v.Number.String()}, false, nil
	case ValueBool:
		return []string{strconv.FormatBool(v.Bool)}, false, nil
	default:
		return []string{v.Text}, false, nil
	}
}

// positiveForm returns the positive comparison of a negative one, and whether it was negative.
func positiveForm(cmp criteria.StringComparison) (criteria.StringComparison, bool) {
	switch cmp {
	case criteria.IsNotText:
		return criteria.IsText, true
	case criteria.DoesNotContainText:
		return criteria.ContainsText, true
	case criteria.DoesNotStartWithText:
		return criteria.StartsWithText, true
	case criteria.DoesNotEndWithText:
		return criteria.EndsWithText, true
	default:
		return cmp, false
	}
}
