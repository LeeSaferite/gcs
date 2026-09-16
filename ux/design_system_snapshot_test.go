// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build dssnapshot

// Built with the dssnapshot tag, this renders the widgets of the design system to PNGs. The tag keeps it out of the
// normal test run: it writes files and exists to feed documentation rather than to check behavior.
//
// Run it with:
//
//	GCS_DS_SNAPSHOT_DIR=/some/dir go test -tags dssnapshot -run TestDesignSystemSnapshots ./ux/
//
// Each specimen is built from the same constructors the application uses, rendered in a headless window at 3x, and
// captured in both the light and the dark theme, so the images show the widgets the real code draws rather than a
// reconstruction of them.

package ux

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/gcs/v5/model/colors"
	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps"
	"github.com/richardwilkes/gcs/v5/svg"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// snapshotScale is the backing scale the specimens are captured at. The interface is drawn at 10 points and the sheet
// at 7, so a 1x capture is too small to read in documentation.
const snapshotScale = 3

// snapshotFocusTarget is the widget, if any, the specimen being built wants focused before it is captured, so that a
// focused field shows its focused border rather than a faked one. A builder sets it; the driver consumes and clears it.
var snapshotFocusTarget unison.Paneler

// specimen is one image to produce: the base name of the file and the panel to render.
type specimen struct {
	name  string
	build func(t *testing.T) unison.Paneler
}

// TestDesignSystemSnapshots renders each specimen and writes <name>-light.png and <name>-dark.png into the directory
// named by GCS_DS_SNAPSHOT_DIR.
func TestDesignSystemSnapshots(t *testing.T) {
	dir := os.Getenv("GCS_DS_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set GCS_DS_SNAPSHOT_DIR to the directory the PNGs should be written to")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("unable to create the output directory: %v", err)
	}
	RegisterKnownFileTypes()

	screen, err := unison.StartHeadless(unison.HeadlessConfig{Width: 1600, Height: 1200, Scale: snapshotScale})
	if err != nil {
		t.Fatalf("unable to start the headless session: %v", err)
	}
	defer screen.Stop()

	var wnd *unison.Window
	screen.Do(func() {
		var wndErr error
		if wnd, wndErr = unison.NewWindow("Specimen"); wndErr != nil {
			t.Errorf("unable to create the window: %v", wndErr)
		}
	})
	if wnd == nil {
		return
	}
	// The session ends with its last window, so the one window is reused for every specimen and only disposed once
	// they have all been captured.
	defer func() { screen.Do(wnd.Dispose) }()

	for _, spec := range specimens() {
		t.Run(spec.name, func(t *testing.T) {
			snapshotFocusTarget = nil
			screen.Do(func() {
				content := unison.NewPanel()
				content.SetLayout(&unison.FlexLayout{Columns: 1})
				content.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(unison.StdHSpacing)))
				content.DrawCallback = func(gc *unison.Canvas, rect geom.Rect) {
					gc.DrawRect(rect, unison.ThemeSurface.Paint(gc, rect, paintstyle.Fill))
				}
				panel := spec.build(t).AsPanel()
				panel.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Fill, VAlign: align.Fill, HGrab: true})
				content.AddChild(panel)
				wnd.SetContent(content)
				wnd.Pack()
				wnd.ToFront()
				if snapshotFocusTarget != nil {
					snapshotFocusTarget.AsPanel().RequestFocus()
				}
			})
			for _, theme := range []struct {
				suffix string
				dark   bool
			}{{suffix: "light"}, {suffix: "dark", dark: true}} {
				screen.SetDarkMode(theme.dark)
				screen.Do(func() { wnd.Content().MarkForRedraw() })
				screen.Sync()
				img := screen.CaptureWindow(wnd)
				if img == nil {
					t.Fatalf("%s was never drawn", spec.name)
				}
				writeSnapshotPNG(t, filepath.Join(dir, spec.name+"-"+theme.suffix+".png"), img)
			}
		})
	}

	for _, one := range screen.Errors() {
		t.Errorf("the headless session recorded an error: %v", one)
	}
}

// writeSnapshotPNG writes img to path.
func writeSnapshotPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("unable to create %s: %v", path, err)
	}
	defer f.Close()
	if err = png.Encode(f, img); err != nil {
		t.Fatalf("unable to write %s: %v", path, err)
	}
}

// specimens returns every image the design system wants, in the order its components are documented.
func specimens() []specimen {
	return []specimen{
		{name: "button", build: buildButtons},
		{name: "field", build: buildFields},
		{name: "toolbar", build: buildToolbar},
		{name: "tooltip", build: buildTooltip},
		{name: "page-block", build: buildPageBlocks},
		{name: "page-header", build: buildPageHeaders},
		{name: "page-field", build: buildPageFields},
		{name: "tint", build: buildTint},
		{name: "table", build: buildTable},
	}
}

// snapshotRow returns a panel that lays its children out in a single row at the standard spacing.
func snapshotRow(columns int) *unison.Panel {
	p := unison.NewPanel()
	p.SetLayout(&unison.FlexLayout{Columns: columns, HSpacing: unison.StdHSpacing, VSpacing: unison.StdVSpacing})
	return p
}

// snapshotEntity returns a character with enough filled in that the sheet blocks have something to show.
func snapshotEntity() *gurps.Entity {
	entity := gurps.NewEntity()
	entity.Profile.Name = "Dai Blackthorn"
	entity.Profile.Title = "Thief"
	entity.Profile.Organization = "The Nightwatch"
	entity.Profile.PlayerName = "Lee"
	entity.Profile.Age = "27"
	entity.Profile.Gender = "Male"
	entity.Profile.Eyes = "Brown"
	entity.Profile.Hair = "Black, Short, Straight"
	entity.Profile.Skin = "Tan"
	entity.Profile.Handedness = "Right"
	entity.Profile.Religion = "None"
	entity.Profile.TechLevel = "3"
	return entity
}

// buildButtons shows the button in its text, icon-only and icon-with-text forms, including a pressed one.
func buildButtons(_ *testing.T) unison.Paneler {
	row := snapshotRow(7)
	for _, title := range []string{"Apply", "Cancel"} {
		b := unison.NewButton()
		b.SetTitle(title)
		row.AddChild(b)
	}
	sticky := unison.NewButton()
	sticky.SetTitle("Sticky On")
	sticky.Sticky = true
	sticky.Pressed = true
	row.AddChild(sticky)
	for _, icon := range []*unison.SVG{svg.Help, svg.Settings, svg.Randomize} {
		row.AddChild(unison.NewSVGButton(icon))
	}
	withIcon := unison.NewButton()
	withIcon.SetTitle("New Sheet")
	baseline := withIcon.Font.Baseline()
	withIcon.Drawable = &unison.DrawableSVG{SVG: svg.GCSSheet, Size: geom.NewSize(baseline, baseline).Ceil()}
	row.AddChild(withIcon)
	return row
}

// buildFields shows a plain field, a focused one and one failing validation.
func buildFields(_ *testing.T) unison.Paneler {
	row := snapshotRow(3)
	plain := unison.NewField()
	plain.SetText("Dai Blackthorn")
	row.AddChild(plain)

	focused := unison.NewField()
	focused.SetText("Thief")
	snapshotFocusTarget = focused
	row.AddChild(focused)

	invalid := unison.NewField()
	invalid.ValidateCallback = func() bool { return false }
	invalid.SetText("5' 11\"")
	invalid.Validate()
	row.AddChild(invalid)
	return row
}

// buildToolbar shows the standard dockable toolbar shell with the controls a list dockable puts in it.
func buildToolbar(_ *testing.T) unison.Paneler {
	toolbar := newToolbar()
	addHelpButton(toolbar, "")
	for _, icon := range []*unison.SVG{svg.SideBar, svg.NewFolder, svg.NotesToggle, svg.Hierarchy} {
		b := unison.NewSVGButton(icon)
		toolbar.AddChild(b)
	}
	finishToolbarLayout(toolbar)
	return toolbar
}

// buildTooltip shows both tooltip forms, which draw their own fill and edge.
func buildTooltip(_ *testing.T) unison.Paneler {
	col := unison.NewPanel()
	col.SetLayout(&unison.FlexLayout{Columns: 1, VSpacing: unison.StdVSpacing})
	col.AddChild(newWrappedTooltip("Randomize the name using the current ancestry"))
	col.AddChild(newWrappedTooltipWithSecondaryText("Encumbrance, Move & Dodge",
		"Move and dodge fall as carried weight rises"))
	return col
}

// buildPageBlocks shows two banded sheet blocks side by side, one page gutter apart.
func buildPageBlocks(_ *testing.T) unison.Paneler {
	entity := snapshotEntity()
	targetMgr := NewTargetMgr(newPageUndoRoot())
	row := unison.NewPanel()
	row.SetLayout(&unison.FlexLayout{Columns: 2, HSpacing: 1, VSpacing: 1})
	for _, panel := range []unison.Paneler{
		NewIdentityPanel(entity, targetMgr),
		NewPointsPanel(entity, targetMgr),
	} {
		panel.AsPanel().SetLayoutData(&unison.FlexLayoutData{HAlign: align.Fill, VAlign: align.Fill, HGrab: true})
		row.AddChild(panel)
	}
	return row
}

// buildPageHeaders shows a block whose first row is page headers rather than labels.
func buildPageHeaders(_ *testing.T) unison.Paneler {
	return NewEncumbrancePanel(snapshotEntity())
}

// buildPageFields shows the label-and-underlined-field rows of a sheet, including the randomizer labels.
func buildPageFields(_ *testing.T) unison.Paneler {
	return NewDescriptionPanel(snapshotEntity(), NewTargetMgr(newPageUndoRoot()))
}

// buildTint shows a tinted block beside an untinted one. The tints ship transparent, so one is set here for the sake
// of the image and put back afterwards.
func buildTint(t *testing.T) unison.Paneler {
	saved := *colors.TintIdentity
	t.Cleanup(func() { *colors.TintIdentity = saved })
	*colors.TintIdentity = unison.ThemeColor{Light: unison.RGB(0, 128, 64), Dark: unison.RGB(0, 160, 80)}

	entity := snapshotEntity()
	targetMgr := NewTargetMgr(newPageUndoRoot())
	row := unison.NewPanel()
	row.SetLayout(&unison.FlexLayout{Columns: 2, HSpacing: 1, VSpacing: 1})
	for _, panel := range []unison.Paneler{
		NewMiscPanel(entity, targetMgr),
		NewIdentityPanel(entity, targetMgr),
	} {
		panel.AsPanel().SetLayoutData(&unison.FlexLayoutData{HAlign: align.Fill, VAlign: align.Fill, HGrab: true})
		row.AddChild(panel)
	}
	return row
}

// buildTable shows a trait table with a container, a second line on a row, and a selected row.
func buildTable(_ *testing.T) unison.Paneler {
	entity := snapshotEntity()
	entity.Traits = snapshotTraits(entity)

	col := unison.NewPanel()
	col.SetLayout(&unison.FlexLayout{Columns: 1})
	header, table := NewNodeTable(NewTraitsProvider(entity, false), nil)
	table.SyncToModel()
	table.SizeColumnsToFit(true)
	table.SelectByIndex(2)
	col.AddChild(header)
	col.AddChild(table)
	return col
}

// snapshotTraits returns a small trait list: a container holding two traits, then two more beside it.
func snapshotTraits(entity *gurps.Entity) []*gurps.Trait {
	container := gurps.NewTrait(entity, nil, true)
	container.Name = "Thief Package"
	var children []*gurps.Trait
	for _, one := range []struct {
		name   string
		notes  string
		points fxp.Int
	}{
		{name: "Combat Reflexes", notes: "+1 to all active defenses", points: fxp.FromInteger(15)},
		{name: "Danger Sense", points: fxp.FromInteger(15)},
	} {
		trait := gurps.NewTrait(entity, container, false)
		trait.Name = one.name
		trait.LocalNotes = one.notes
		trait.BasePoints = one.points
		children = append(children, trait)
	}
	container.SetChildren(children)

	night := gurps.NewTrait(entity, nil, false)
	night.Name = "Night Vision 5"
	night.BasePoints = fxp.FromInteger(5)

	flexible := gurps.NewTrait(entity, nil, false)
	flexible.Name = "Flexibility"
	flexible.LocalNotes = "+3 to climbing and escape rolls"
	flexible.BasePoints = fxp.FromInteger(5)

	return []*gurps.Trait{container, night, flexible}
}
