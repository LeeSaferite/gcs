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
	"sync"

	"github.com/richardwilkes/gcs/v5/model/condition"
)

// maxPrereqTrees bounds the conversion cache; it is cleared when it would grow past this.
const maxPrereqTrees = 4096

// prereqTree is a converted prerequisite list. Converted trees are never changed, so one is shared by every list with
// the same content and owner type, and may be evaluated concurrently.
type prereqTree struct {
	node         condition.Node
	scripts      bool // the tree holds a script node
	countsSpells bool // the tree holds a count over spells, which the #737 guard looks for
}

type prereqTreeKey struct {
	hash  uint64
	owner prereqOwner
}

var prereqTrees = struct {
	m    map[prereqTreeKey]*prereqTree
	lock sync.Mutex
}{m: make(map[prereqTreeKey]*prereqTree)}

// prereqTreeFor returns the converted tree of a prerequisite list, which may be nil, for an owner of the given type.
// The cache is keyed by the list's content hash rather than its identity, so an edited list is converted afresh and a
// discarded one holds nothing alive.
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
	tree = &prereqTree{node: convertPrereqList(list, owner)}
	condition.Walk(tree.node, func(_ condition.Path, n condition.Node) bool {
		switch x := n.(type) {
		case *condition.Script:
			tree.scripts = true
		case *condition.Count:
			tree.countsSpells = tree.countsSpells || x.Of == conditionSpells
		default:
		}
		return true
	})
	prereqTrees.lock.Lock()
	if len(prereqTrees.m) >= maxPrereqTrees {
		clear(prereqTrees.m)
	}
	prereqTrees.m[key] = tree
	prereqTrees.lock.Unlock()
	return tree
}

// evaluate evaluates the tree for the owner with the host.
func (t *prereqTree) evaluate(h *conditionHost, owner any, mode condition.Mode) condition.Result {
	return condition.Evaluate(t.node, h, owner, condition.Options{Mode: mode})
}
