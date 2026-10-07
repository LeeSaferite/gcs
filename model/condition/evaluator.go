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
	"slices"
	"sync"

	"github.com/richardwilkes/toolbox/v2/i18n"
)

// Default budgets, used when Options leaves them at 0.
const (
	DefaultBudget       = 200_000
	DefaultDetailBudget = 200_000
)

// Options control an evaluation.
type Options struct {
	Mode          Mode
	FailedIsUnmet bool // filters: a Failed check or Unknown leaf counts as Unmet, ignoring the leaf's own Not
	Budget        int  // state budget in member visits; 0 means DefaultBudget
	DetailBudget  int  // Collect detail budget; 0 means DefaultDetailBudget
}

// evaluator holds everything one evaluation needs. Nodes are read-only during evaluation; all caches, memos and
// budgets live here. A Matcher keeps one and reuses it, so the field and self-token caches, which depend only on the
// tree and host, outlive a single evaluation.
type evaluator struct {
	host         Host
	self         Item
	resolver     FieldResolver
	replacements map[itemPair]map[string]string
	scripts      map[scriptKey]scriptOutcome
	fields       map[fieldKey]func(Item) (Value, bool)
	checkFields  map[*Check]func(Item) (Value, bool)
	qualifiers   map[*Check]*selfQualifier
	shallow      validator
	opt          Options
	budget       int
	detail       int
	collect      bool // the detail pass is still collecting
	trusted      bool // the tree is known to be valid throughout, so nodes need no validity check
	detailPass   bool
	exhausted    bool
}

func newEvaluator(h Host, opt Options) *evaluator {
	e := &evaluator{}
	e.init(h, opt)
	return e
}

// init readies e for a new call, keeping only its memo maps, which run clears.
func (e *evaluator) init(h Host, opt Options) {
	if opt.Budget <= 0 {
		opt.Budget = DefaultBudget
	}
	if opt.DetailBudget <= 0 {
		opt.DetailBudget = DefaultDetailBudget
	}
	*e = evaluator{
		host:         h,
		opt:          opt,
		shallow:      validator{shallow: true},
		replacements: e.replacements,
		scripts:      e.scripts,
	}
}

// Evaluate evaluates n for self, with self also as the root subject. Evaluate(nil, ...) is Skipped.
func Evaluate(n Node, h Host, self Item, opt Options) Result {
	return EvaluateFor(n, h, self, self, opt)
}

// EvaluateFor evaluates n for self with the given root subject. The state is always decided by a YesNo pass under
// opt.Budget; in Collect mode a second pass under opt.DetailBudget then fills in the children, hits and specificity.
func EvaluateFor(n Node, h Host, self, subject Item, opt Options) Result {
	e, ok := evaluators.Get().(*evaluator)
	if !ok {
		e = &evaluator{}
	}
	e.init(h, opt)
	r := e.run(n, self, subject, opt.Mode)
	e.host = nil
	e.self = nil
	clear(e.replacements)
	clear(e.scripts)
	evaluators.Put(e)
	return r
}

// evaluators keeps one-shot evaluators, and their memo maps, for reuse. An evaluator holds nothing of a call once it
// returns: its host is dropped and its memos are cleared.
var evaluators sync.Pool

// run performs one evaluation. Both passes share the script and replacement memos.
func (e *evaluator) run(n Node, self, subject Item, mode Mode) Result {
	if n == nil {
		return Result{State: Skipped}
	}
	clear(e.replacements)
	clear(e.scripts)
	e.self = self
	e.trusted = e.trusted || e.treeValid(n, 1)
	f := frame{subject: subject, depth: 1}
	e.startPass(false)
	yesNo := e.eval(n, f)
	if mode != Collect {
		return yesNo
	}
	e.startPass(true)
	r := e.eval(n, f)
	if r.State != yesNo.State {
		// Possible only when the YesNo pass ran out of budget: keep its state and report nothing Met beneath it.
		r.State = yesNo.State
		r.Reason = yesNo.Reason
		r.local = yesNo.local
		r.Truncated = true
		r.Specificity = 0
		r.hits = nil
		r.Children = slices.DeleteFunc(r.Children, func(c Result) bool { return c.State == Met })
	}
	return r
}

// eval evaluates one node. A nil node is Skipped.
func (e *evaluator) eval(n Node, f frame) Result {
	if n == nil {
		return Result{State: Skipped}
	}
	collecting := e.collect
	if collecting {
		e.spendDetail()
	}
	r := n.eval(e, f)
	r.Node = n
	if r.State == Failed && e.opt.FailedIsUnmet {
		switch n.(type) {
		case *Check, *Unknown:
			r.State = Unmet
		default:
		}
	}
	if e.detailPass && (!collecting || !e.collect) {
		r.Truncated = true
	}
	return r
}

// evalRef evaluates the node of a slot; an empty one is Skipped.
func (e *evaluator) evalRef(r Ref, f frame) Result {
	if r.IsZero() {
		return Result{State: Skipped}
	}
	return e.eval(r.Node, f)
}

// evalYesNo evaluates a node without collecting details, as guards are.
func (e *evaluator) evalYesNo(r Ref, f frame) Result {
	collecting := e.collect
	e.collect = false
	result := e.evalRef(r, f)
	e.collect = collecting && e.detail > 0
	return result
}

// problem returns the first problem of n itself at depth, or "" when it is valid.
func (e *evaluator) problem(n Node, depth int) string {
	if e.trusted {
		return ""
	}
	return e.shallow.ownProblem(n, depth)
}

// invalid returns a global Failed result when the node itself is invalid: anything Validate reports for it other than
// a nil child, including a depth over MaxDepth.
func (e *evaluator) invalid(n Node, f frame) (Result, bool) {
	if msg := e.problem(n, int(f.depth)); msg != "" {
		return failGlobal(msg), true
	}
	return Result{}, false
}

func reasonTooComplex() string {
	return i18n.Text("too complex to decide")
}
