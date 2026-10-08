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
	"cmp"
	"io/fs"
	"os"
	"path"
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/zeebo/xxh3"
)

// conditionLibrary is what the full differential tests load from the Master Library.
type conditionLibrary struct {
	sheets map[string]*Entity
	items  []prereqOwnerItem // every item of every trait, skill, spell, equipment and template file
	byTID  map[tid.TID]prereqOwnerItem
	files  int
}

func loadConditionLibrary(t *testing.T, root string) *conditionLibrary {
	t.Helper()
	fsys := os.DirFS(root)
	lib := &conditionLibrary{sheets: make(map[string]*Entity), byTID: make(map[tid.TID]prereqOwnerItem)}
	add := func(file string, owner any, name string, id tid.TID, list *PrereqList) {
		if list != nil {
			item := prereqOwnerItem{owner: owner, name: file + ": " + name, list: list}
			lib.items = append(lib.items, item)
			lib.byTID[id] = item
		}
	}
	addTraits := func(file string, list []*Trait) {
		Traverse(func(x *Trait) bool { add(file, x, x.Name, x.TID, x.Prereq); return false }, false, false, list...)
	}
	addSkills := func(file string, list []*Skill) {
		Traverse(func(x *Skill) bool { add(file, x, x.Name, x.TID, x.Prereq); return false }, false, false, list...)
	}
	addSpells := func(file string, list []*Spell) {
		Traverse(func(x *Spell) bool { add(file, x, x.Name, x.TID, x.Prereq); return false }, false, false, list...)
	}
	addEquipment := func(file string, list []*Equipment) {
		Traverse(func(x *Equipment) bool { add(file, x, x.Name, x.TID, x.Prereq); return false }, false, false,
			list...)
	}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var loadErr error
		switch path.Ext(p) {
		case SheetExt:
			lib.sheets[p], loadErr = NewEntityFromFile(fsys, p)
		case TraitsExt:
			var list []*Trait
			list, loadErr = NewTraitsFromFile(fsys, p)
			addTraits(p, list)
		case SkillsExt:
			var list []*Skill
			list, loadErr = NewSkillsFromFile(fsys, p)
			addSkills(p, list)
		case SpellsExt:
			var list []*Spell
			list, loadErr = NewSpellsFromFile(fsys, p)
			addSpells(p, list)
		case EquipmentExt:
			var list []*Equipment
			list, loadErr = NewEquipmentFromFile(fsys, p)
			addEquipment(p, list)
		case TemplatesExt:
			var tmpl *Template
			if tmpl, loadErr = NewTemplateFromFile(fsys, p); loadErr == nil {
				addTraits(p, tmpl.Traits)
				addSkills(p, tmpl.Skills)
				addSpells(p, tmpl.Spells)
				addEquipment(p, tmpl.Equipment)
			}
		default:
			return nil
		}
		if loadErr != nil {
			t.Errorf("%s: %v", p, loadErr)
		}
		lib.files++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

// representativeSheets returns the two sheets with the most spells, the one with the most traits and skills, and the
// one with the most equipment.
func (lib *conditionLibrary) representativeSheets() []string {
	names := make([]string, 0, len(lib.sheets))
	for name := range lib.sheets {
		names = append(names, name)
	}
	slices.Sort(names)
	count := func(name string, spells bool) int {
		e := lib.sheets[name]
		n := 0
		if spells {
			Traverse(func(*Spell) bool { n++; return false }, false, false, e.Spells...)
			return n
		}
		Traverse(func(*Trait) bool { n++; return false }, false, false, e.Traits...)
		Traverse(func(*Skill) bool { n++; return false }, false, false, e.Skills...)
		return n
	}
	most := func(less func(a, b string) int) string {
		return slices.MaxFunc(names, less)
	}
	picked := []string{most(func(a, b string) int { return cmp.Compare(count(a, true), count(b, true)) })}
	rest := slices.DeleteFunc(slices.Clone(names), func(s string) bool { return s == picked[0] })
	picked = append(picked, slices.MaxFunc(rest, func(a, b string) int {
		return cmp.Compare(count(a, true), count(b, true))
	}), most(func(a, b string) int { return cmp.Compare(count(a, false), count(b, false)) }))
	equipment := func(name string) int {
		n := 0
		Traverse(func(*Equipment) bool { n++; return false }, false, false, lib.sheets[name].CarriedEquipment...)
		return n
	}
	picked = append(picked, most(func(a, b string) int { return cmp.Compare(equipment(a), equipment(b)) }))
	return picked
}

// TestConditionDifferentialLibrary compares legacy and engine results over the Master Library: every item of every
// sheet on its own sheet, then every library item on each representative sheet. It also checks that a sheet item
// converts to the same hash as the library item it came from when their legacy prerequisites hash the same.
func TestConditionDifferentialLibrary(t *testing.T) {
	lib := loadConditionLibrary(t, masterLibrary(t))
	var onSheets, onRepresentatives diffTally
	sheetItems, hashPairs, hashDiffer := 0, 0, 0
	for name, e := range lib.sheets {
		h := newConditionHost(e)
		for _, item := range entityPrereqOwners(e) {
			sheetItems++
			onSheets.compare(t, h, e, item.list, item.owner, name+": "+item.name)
			from, ok := lib.byTID[sourceTID(item.owner)]
			if !ok || prereqOwnerOf(from.owner) != prereqOwnerOf(item.owner) ||
				legacyHash(from.list) != legacyHash(item.list) {
				continue
			}
			hashPairs++
			if treeHash(conditionFromPrereqs(from.list, from.owner)) !=
				treeHash(conditionFromPrereqs(item.list, item.owner)) {
				hashDiffer++
				t.Errorf("%s: converted hash differs from %s", name+": "+item.name, from.name)
			}
		}
	}
	representatives := lib.representativeSheets()
	for _, name := range representatives {
		e := lib.sheets[name]
		h := newConditionHost(e)
		for _, item := range lib.items {
			onRepresentatives.compare(t, h, e, item.list, item.owner, name+" with "+item.name)
		}
	}
	t.Logf("files %d, sheets %d, sheet items %d: %s", lib.files, len(lib.sheets), sheetItems,
		onSheets.String())
	t.Logf("library items %d on %v: %s", len(lib.items), representatives,
		onRepresentatives.String())
	t.Logf("sheet items matched to their library item %d, converted hashes differing %d", hashPairs, hashDiffer)
}

func sourceTID(owner any) tid.TID {
	switch x := owner.(type) {
	case *Trait:
		return x.Source.TID
	case *Skill:
		return x.Source.TID
	case *Spell:
		return x.Source.TID
	case *Equipment:
		return x.Source.TID
	default:
		return ""
	}
}

func legacyHash(list *PrereqList) uint64 {
	h := xxh3.New()
	list.Hash(h)
	return h.Sum64()
}

// libraryRows returns every row, containers and their contents included, of the library files with the extension.
func libraryRows[T Node[T]](t testing.TB, root, ext string, load func(fs.FS, string) ([]T, error)) []T {
	t.Helper()
	fsys := os.DirFS(root)
	var rows []T
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ext {
			return err
		}
		list, loadErr := load(fsys, p)
		if loadErr != nil {
			t.Errorf("%s: %v", p, loadErr)
		}
		Traverse(func(row T) bool { rows = append(rows, row); return false }, false, false, list...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// libraryFilterDiff runs diffFilters over the rows of one library list type, with shapes drawn from its rows.
func libraryFilterDiff[T Node[T]](t *testing.T, root, ext string, load func(fs.FS, string) ([]T, error),
	fields []*FilterField[T],
) {
	t.Helper()
	rows := libraryRows(t, root, ext, load)
	shapes := filterShapes(fields, rows)
	t.Logf("%s: %d rows, %d filters, %d matches agree", ext, len(rows), len(shapes),
		diffFilters(t, ext, fields, rows, shapes))
}

// TestFilterDifferentialLibrary compares the legacy filter matcher with the engine's over every row of every library
// list type.
func TestFilterDifferentialLibrary(t *testing.T) {
	root := masterLibrary(t)
	libraryFilterDiff(t, root, TraitsExt, NewTraitsFromFile, TraitFilterFields())
	libraryFilterDiff(t, root, TraitModifiersExt, NewTraitModifiersFromFile, TraitModifierFilterFields())
	libraryFilterDiff(t, root, SkillsExt, NewSkillsFromFile, SkillFilterFields())
	libraryFilterDiff(t, root, SpellsExt, NewSpellsFromFile, SpellFilterFields())
	libraryFilterDiff(t, root, EquipmentExt, NewEquipmentFromFile, EquipmentFilterFields())
	libraryFilterDiff(t, root, EquipmentModifiersExt, NewEquipmentModifiersFromFile, EquipmentModifierFilterFields())
	libraryFilterDiff(t, root, NotesExt, NewNotesFromFile, NoteFilterFields())
}

// BenchmarkConditionFilter matches every library trait against the filter shapes with the legacy matcher and with the
// engine's.
func BenchmarkConditionFilter(b *testing.B) {
	rows := libraryRows(b, masterLibrary(b), TraitsExt, NewTraitsFromFile)
	fields := TraitFilterFields()
	shapes := filterShapes(fields, rows)
	for _, one := range []struct {
		name    string
		matcher func(*ListFilter, []*FilterField[*Trait]) func(*Trait) bool
	}{{"legacy", legacyListFilterMatcher[*Trait]}, {"engine", NewListFilterMatcher[*Trait]}} {
		b.Run(one.name, func(b *testing.B) {
			for b.Loop() {
				for _, root := range shapes {
					m := one.matcher(&ListFilter{Name: "x", Root: root}, fields)
					for _, row := range rows {
						m(row)
					}
				}
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(shapes)*len(rows)), "ns/match")
		})
	}
}
