// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package gurps

import (
	"context"
	"sync"

	"github.com/richardwilkes/gcs/v5/model/condition"
)

// maxPrereqTrees bounds the conversion cache; it is cleared when it would grow past this.
const maxPrereqTrees = 4096

// prereqTree is a converted prerequisite list, or the error that kept it from being converted. Converted trees are
// never changed, so one is shared by every list with the same content and owner, and may be evaluated concurrently.
type prereqTree struct {
	node   condition.Node
	err    error
	guards []spellGuardTree // what the #737 guard checks a spell against
}

type prereqTreeKey struct {
	hash  uint64
	owner prereqOwner
}

var prereqTrees = struct {
	m    map[prereqTreeKey]*prereqTree
	lock sync.Mutex
}{m: make(map[prereqTreeKey]*prereqTree)}

// prereqTreeFor returns the converted tree of a prerequisite list, which may be nil, for the given owner. The cache is
// keyed by the list's content hash rather than its identity, so an edited list is converted afresh and a discarded one
// holds nothing alive.
func prereqTreeFor(list *PrereqList, owner prereqOwner) *prereqTree {
	if list == nil {
		return &prereqTree{}
	}
	key := prereqTreeKey{hash: Hash64(list), owner: owner}
	prereqTrees.lock.Lock()
	tree, ok := prereqTrees.m[key]
	prereqTrees.lock.Unlock()
	if ok {
		return tree
	}
	tree = &prereqTree{}
	var g *condition.Group
	if g, tree.err = convertPrereqList(list, owner); tree.err == nil {
		tree.node = g
		tree.guards = spellGuards(g, false, nil)
	}
	prereqTrees.lock.Lock()
	if len(prereqTrees.m) >= maxPrereqTrees {
		clear(prereqTrees.m)
	}
	prereqTrees.m[key] = tree
	prereqTrees.lock.Unlock()
	return tree
}

// evaluate evaluates the tree for the owner with the host. A tree that couldn't be converted is Failed, with the
// conversion error as the reason.
func (t *prereqTree) evaluate(h *conditionHost, owner any, mode condition.Mode) condition.Result {
	if t.err != nil {
		return condition.Result{State: condition.Failed, Reason: t.err.Error()}
	}
	r, err := condition.Evaluate(context.Background(), t.node, h, owner, condition.Options{Mode: mode})
	if err != nil {
		return condition.Result{State: condition.Failed, Reason: err.Error()}
	}
	return r
}
