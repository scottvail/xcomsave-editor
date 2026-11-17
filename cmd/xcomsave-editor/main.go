// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

// Command xcomsave-editor is a cross-platform GUI for editing XCOM: Enemy
// Unknown / Enemy Within saves, either native save files or xcom2json JSON.
//
// The landing page shows the loaded save's details; each editing feature
// (e.g. "Barracks") opens on its own page.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/scottvail/xcomsave-editor/internal/save"
)

const appTitle = "XCOM: EW Save Editor"

var rankTitles = []string{
	"Rookie", "Squaddie", "Corporal", "Sergeant",
	"Lieutenant", "Captain", "Major", "Colonel",
}

// rankNames are the choices in the rank selector, e.g. "7 - Colonel".
var rankNames = func() []string {
	n := make([]string, len(rankTitles))
	for i, t := range rankTitles {
		n[i] = fmt.Sprintf("%d - %s", i, t)
	}
	return n
}()

// psiRankNames are the choices in the psi rank selector.
var psiRankNames = []string{"0", "1", "2", "3"}

// difficultyNames are the choices in the difficulty selector, e.g. "2 - Classic".
func difficultyNames() []string {
	n := make([]string, len(save.Difficulties))
	for i, d := range save.Difficulties {
		n[i] = fmt.Sprintf("%d - %s", i, d)
	}
	return n
}

func rankTitle(r int) string {
	if r >= 0 && r < len(rankTitles) {
		return rankTitles[r]
	}
	return strconv.Itoa(r)
}

// listColumn is a column of the soldier list. Extra width goes to the
// columns with grow set.
type listColumn struct {
	title string
	width float32
	grow  bool
}

// listColumns are the soldier list's columns in every save.
var listColumns = []listColumn{
	{"First Name", 180, true}, {"Last Name", 180, true}, {"Rank", 140, false}, {"Class", 120, false}, {"Wounded", 90, false}, {"Psi Gift", 80, false},
}

// geneModColumn is the soldier list's Enemy Within column: the number of
// gene mods a soldier has (blank for none).
var geneModColumn = listColumn{"Gene Mods", 100, false}

// page identifies which screen is shown below the toolbar.
type page int

const (
	homePage     page = iota
	soldiersPage      // list of soldiers
	soldierPage       // editor for e.file.Soldiers[e.soldier]
	storagePage       // stored item counts
	hangarPage        // list of interceptors
	shipPage          // editor for e.file.Ships[e.ship]
	labPage           // research labs
	workshopPage      // engineering
)

type editor struct {
	app     fyne.App
	win     fyne.Window
	file    *save.File
	page    page
	soldier int    // index of the soldier shown on soldierPage
	ship    int    // index of the interceptor shown on shipPage
	onEdit  func() // called after any edit on the current page
	body    *fyne.Container
	status  *widget.Label
	saveBtn *widget.Button
	saveAs  *widget.Button
	invalid map[*widget.Entry]bool
	// debug shows each field's save property under it, and the save entry
	// names of soldiers and interceptors (-debug, or Ctrl+Shift+D).
	debug bool
}

func main() {
	debug := flag.Bool("debug", false, "show the save property behind each field")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [-debug] [save file]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	a := app.NewWithID("dev.xcomew.saveeditor")
	w := a.NewWindow(appTitle)
	e := newEditor(a, w)
	e.debug = *debug
	w.Resize(windowSize)
	if flag.NArg() > 0 {
		e.load(flag.Arg(0))
	}
	w.ShowAndRun()
}

// item is a form row. In debug mode its hint names the save property (or
// properties) the field shows; hint is shown either way.
func (e *editor) item(label, prop string, w fyne.CanvasObject, hint ...string) *widget.FormItem {
	it := widget.NewFormItem(label, w)
	var h []string
	if e.debug && prop != "" {
		h = append(h, prop)
	}
	it.HintText = strings.Join(append(h, hint...), " — ")
	return it
}

// windowSize is the window's initial size: tall enough for two more form
// rows (39 pixels each, a field and its spacing) than the original 650.
var windowSize = fyne.NewSize(1200, 650+2*39)

// checkGroup is a row of check boxes used as one form field. The form
// shows a field's label in the disabled color when the field reports itself
// disabled, so the group does when all of its boxes are.
type checkGroup struct {
	widget.BaseWidget
	checks []*widget.Check
}

func newCheckGroup(checks ...*widget.Check) *checkGroup {
	g := &checkGroup{checks: checks}
	g.ExtendBaseWidget(g)
	return g
}

func (g *checkGroup) CreateRenderer() fyne.WidgetRenderer {
	row := container.NewHBox()
	for _, c := range g.checks {
		row.Add(c)
	}
	return widget.NewSimpleRenderer(row)
}

// Disabled reports whether every box is disabled.
func (g *checkGroup) Disabled() bool {
	for _, c := range g.checks {
		if !c.Disabled() {
			return false
		}
	}
	return true
}

func (g *checkGroup) Enable() {
	for _, c := range g.checks {
		c.Enable()
	}
}

func (g *checkGroup) Disable() {
	for _, c := range g.checks {
		c.Disable()
	}
}

// subtitle is a card's subtitle: the save entry name, in debug mode only.
func (e *editor) subtitle(instanceName string) string {
	if e.debug {
		return instanceName
	}
	return ""
}

// toggleDebug switches debug mode and redraws the current page.
func (e *editor) toggleDebug() {
	e.debug = !e.debug
	e.show(e.page)
	if e.debug {
		e.status.SetText("Debug mode: fields show their save properties (Ctrl+Shift+D to turn off).")
	} else if e.file != nil {
		e.setStatus()
	} else {
		e.status.SetText("Debug mode off.")
	}
}

func newEditor(a fyne.App, w fyne.Window) *editor {
	e := &editor{app: a, win: w, invalid: map[*widget.Entry]bool{}}

	openBtn := widget.NewButtonWithIcon("Open…", theme.FolderOpenIcon(), e.openDialog)
	e.saveBtn = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), e.save)
	e.saveAs = widget.NewButtonWithIcon("Save As…", theme.DocumentSaveIcon(), e.saveAsDialog)
	e.saveBtn.Disable()
	e.saveAs.Disable()
	e.status = widget.NewLabel("Open an XCOM save file (or xcom2json JSON) to begin.")
	e.status.Truncation = fyne.TextTruncateEllipsis

	toolbar := container.NewBorder(nil, nil, container.NewHBox(openBtn, e.saveBtn, e.saveAs), nil, e.status)
	e.body = container.NewStack()
	w.SetContent(container.NewBorder(container.NewVBox(toolbar, widget.NewSeparator()), nil, nil, nil, e.body))
	e.show(homePage)

	w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("File",
		fyne.NewMenuItem("Open…", e.openDialog),
		fyne.NewMenuItem("Save", e.save),
		fyne.NewMenuItem("Save As…", e.saveAsDialog),
	)))
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) { e.openDialog() })
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) { e.save() })
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}, func(fyne.Shortcut) { e.toggleDebug() })

	w.SetCloseIntercept(func() {
		if e.file == nil || !e.file.Modified() {
			w.Close()
			return
		}
		dialog.ShowConfirm("Unsaved changes", "Discard unsaved changes and quit?", func(ok bool) {
			if ok {
				w.Close()
			}
		}, w)
	})

	return e
}

func (e *editor) openDialog() {
	open := func() {
		d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, e.win)
				return
			}
			if rc == nil {
				return
			}
			rc.Close()
			e.load(rc.URI().Path())
		}, e.win)
		// No extension filter: native saves (e.g. "save62") have no extension.
		if loc := e.lastDir(); loc != nil {
			d.SetLocation(loc)
		}
		d.Resize(fyne.NewSize(900, 600))
		d.Show()
	}
	if e.file != nil && e.file.Modified() {
		dialog.ShowConfirm("Unsaved changes", "Discard unsaved changes and open another file?", func(ok bool) {
			if ok {
				open()
			}
		}, e.win)
		return
	}
	open()
}

func (e *editor) lastDir() fyne.ListableURI {
	dir := e.app.Preferences().String("lastDir")
	if e.file != nil {
		dir = filepath.Dir(e.file.Path)
	}
	if dir == "" {
		return nil
	}
	l, err := storage.ListerForURI(storage.NewFileURI(dir))
	if err != nil {
		return nil
	}
	return l
}

func (e *editor) load(path string) {
	f, err := save.Load(path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("could not load %s:\n%w", filepath.Base(path), err), e.win)
		return
	}
	if len(f.Soldiers) == 0 {
		dialog.ShowInformation("No soldiers", "No XGStrategySoldier entries were found in this file.", e.win)
	}
	e.app.Preferences().SetString("lastDir", filepath.Dir(path))
	e.file = f
	e.soldier = 0
	e.ship = 0
	e.saveBtn.Enable()
	e.saveAs.Enable()
	e.setStatus()
	e.show(homePage)
}

func (e *editor) save() {
	if e.file == nil {
		return
	}
	e.writeTo(e.file.Path)
}

func (e *editor) saveAsDialog() {
	if e.file == nil || !e.canSave() {
		return
	}
	// The dialog creates/truncates the chosen file before calling back, so
	// validate first to avoid leaving an empty file behind on failure.
	d := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, e.win)
			return
		}
		if wc == nil {
			return
		}
		wc.Close()
		e.writeTo(wc.URI().Path())
	}, e.win)
	d.SetFileName(filepath.Base(e.file.Path))
	if loc := e.lastDir(); loc != nil {
		d.SetLocation(loc)
	}
	d.Resize(fyne.NewSize(900, 600))
	d.Show()
}

// canSave reports (and shows an error for) values that cannot be saved.
func (e *editor) canSave() bool {
	if len(e.invalid) > 0 {
		dialog.ShowError(fmt.Errorf("fix the %d highlighted invalid value(s) before saving", len(e.invalid)), e.win)
		return false
	}
	if err := e.file.Validate(); err != nil {
		dialog.ShowError(err, e.win)
		return false
	}
	return true
}

func (e *editor) writeTo(path string) {
	if !e.canSave() {
		return
	}
	samePath := path == e.file.Path
	if err := e.file.SaveAs(path, samePath); err != nil {
		dialog.ShowError(fmt.Errorf("save failed: %w", err), e.win)
		return
	}
	// The soldiers were reloaded from the written data, so rebuild the page
	// to bind its widgets to them.
	e.show(e.page)
	msg := fmt.Sprintf("Saved %s (%s)", filepath.Base(path), e.file.Format)
	if samePath {
		if _, err := os.Stat(path + ".bak"); err == nil {
			msg += "  (original backed up to " + filepath.Base(path) + ".bak)"
		}
	}
	e.status.SetText(msg)
}

func (e *editor) updateTitle() {
	t := appTitle
	if e.file != nil {
		t = filepath.Base(e.file.Path) + " - " + appTitle
		if e.file.Modified() {
			t = "* " + t
		}
	}
	e.win.SetTitle(t)
}

func (e *editor) changed() {
	e.updateTitle()
	if e.onEdit != nil {
		e.onEdit()
	}
}

func (e *editor) setStatus() {
	e.status.SetText(fmt.Sprintf("%s — %s, %s  (Save As a .json name to export JSON)",
		e.file.Path, e.file.Format, e.file.Version()))
}

// show builds page p from e.file and displays it.
func (e *editor) show(p page) {
	if e.file == nil {
		p = homePage
	}
	if p == soldierPage && (e.soldier < 0 || e.soldier >= len(e.file.Soldiers)) {
		p = soldiersPage
	}
	if p == shipPage && (e.ship < 0 || e.ship >= len(e.file.Ships)) {
		p = hangarPage
	}
	e.page = p
	e.invalid = map[*widget.Entry]bool{}
	e.onEdit = nil
	var content fyne.CanvasObject
	switch p {
	case soldiersPage:
		content = e.soldiersPage()
	case soldierPage:
		content = e.soldierPage()
	case storagePage:
		content = e.storagePage()
	case hangarPage:
		content = e.hangarPage()
	case shipPage:
		content = e.shipPage()
	case labPage:
		content = e.labPage()
	case workshopPage:
		content = e.workshopPage()
	default:
		content = e.homePage()
	}
	e.body.Objects = []fyne.CanvasObject{content}
	e.body.Refresh()
	e.updateTitle()
}

// homePage is the landing screen: the save's details and the editing
// features available for it.
func (e *editor) homePage() fyne.CanvasObject {
	if e.file == nil {
		open := widget.NewButtonWithIcon("Open Save…", theme.FolderOpenIcon(), e.openDialog)
		open.Importance = widget.HighImportance
		return container.NewCenter(container.NewVBox(
			widget.NewLabelWithStyle(appTitle, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Open an XCOM save file (or xcom2json JSON) to begin.", fyne.TextAlignCenter, fyne.TextStyle{}),
			container.NewCenter(open),
		))
	}

	h := e.file.Header()
	yesNo := func(b bool) string {
		if b {
			return "Yes"
		}
		return "No"
	}
	value := func(s string) *widget.Label {
		l := widget.NewLabel(s)
		l.Wrapping = fyne.TextWrapWord
		return l
	}
	ironman := widget.NewCheck("", func(b bool) {
		e.file.Ironman = b
		e.changed()
	})
	ironman.SetChecked(e.file.Ironman)
	// difficultySelect edits *v (an index into save.Difficulties). A value
	// outside the list is shown as a placeholder and kept unless changed.
	difficultySelect := func(v *int, present bool) fyne.CanvasObject {
		sel := widget.NewSelect(difficultyNames(), nil)
		if *v >= 0 && *v < len(save.Difficulties) {
			sel.SetSelectedIndex(*v)
		} else {
			sel.PlaceHolder = strconv.Itoa(*v)
		}
		sel.OnChanged = func(string) {
			if i := sel.SelectedIndex(); i >= 0 {
				*v = i
				e.changed()
			}
		}
		if !present {
			sel.Disable()
		}
		return sel
	}
	difficulty := difficultySelect(&e.file.Difficulty, e.file.HasDifficulty())
	lowest := difficultySelect(&e.file.LowestDifficulty, e.file.HasLowestDifficulty())
	continent := "—"
	if e.file.HasContinent() {
		continent = strconv.Itoa(e.file.Continent)
		if name := e.file.ContinentName(); name != "" {
			continent = fmt.Sprintf("%s (%d)", name, e.file.Continent)
		}
	}
	info := widget.NewForm(
		e.item("File", "", value(e.file.Path)),
		e.item("Format", "header: version", value(fmt.Sprintf("%s, %s", e.file.Format, e.file.Version()))),
		e.item("Description", "header: save_description", value(h.SaveDescription.Str)),
		e.item("Saved", "header: time", value(h.Time.Str)),
		e.item("Cash", "XGHeadQuarters.m_iCash", e.intEntry(&e.file.Cash, 0, 1<<31-1, e.file.HasCash())),
		e.item("HQ Continent", "XGHeadQuarters.m_iContinent", value(continent)),
		e.item("Difficulty", "XGStrategy.m_iDifficulty", difficulty),
		e.item("Lowest Difficulty", "XGStrategy.m_iLowestDifficulty", lowest),
		e.item("Ironman", "header: ironman, XGStrategy.m_bIronMan", ironman),
		e.item("Autosave", "header: autosave", value(yesNo(h.Autosave))),
	)

	soldiers := widget.NewButtonWithIcon("Barracks", theme.AccountIcon(), func() { e.leave(soldiersPage) })
	soldiers.Importance = widget.HighImportance
	if len(e.file.Soldiers) == 0 {
		soldiers.Disable()
	}

	storage := widget.NewButtonWithIcon("Storage", theme.StorageIcon(), func() { e.leave(storagePage) })
	storage.Importance = widget.HighImportance
	hangar := widget.NewButtonWithIcon("Hangar", theme.HomeIcon(), func() { e.leave(hangarPage) })
	hangar.Importance = widget.HighImportance
	if len(e.file.Ships) == 0 && e.file.PendingOrders() == 0 {
		hangar.Disable()
	}
	lab := widget.NewButtonWithIcon("Research", theme.SearchIcon(), func() { e.leave(labPage) })
	lab.Importance = widget.HighImportance
	if !e.file.HasScientists() {
		lab.Disable()
	}
	workshop := widget.NewButtonWithIcon("Engineering", theme.SettingsIcon(), func() { e.leave(workshopPage) })
	workshop.Importance = widget.HighImportance
	if !e.file.HasEngineers() {
		workshop.Disable()
	}
	buttons := container.NewHBox(soldiers, hangar, storage, lab, workshop)
	if !e.file.HasStorage() {
		storage.Disable()
		if e.file.EnemyWithin() {
			note := widget.NewLabel("Storage editing is not yet available for Enemy Within saves.")
			note.Importance = widget.LowImportance
			buttons.Add(note)
		}
	}

	card := widget.NewCard("Saved Game", "", container.NewVBox(
		info,
		widget.NewSeparator(),
		buttons,
	))
	return container.NewVScroll(container.NewCenter(container.New(&minWidthLayout{560}, card)))
}

// leave shows page p, first asking whether to discard any invalid values
// on the current page. Invalid text is never stored in the soldier, so it
// is lost when the page changes; valid edits are kept.
func (e *editor) leave(p page) {
	if len(e.invalid) == 0 {
		e.show(p)
		return
	}
	dialog.ShowConfirm("Invalid values",
		fmt.Sprintf("Discard the %d highlighted invalid value(s)? Other edits are kept.", len(e.invalid)),
		func(ok bool) {
			if ok {
				e.show(p)
			}
		}, e.win)
}

// pageHeader is the bar at the top of a sub-page: a back button and a title.
// Any actions are placed at the right of the bar.
func pageHeader(backText string, back func(), title string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	b := widget.NewButtonWithIcon(backText, theme.NavigateBackIcon(), back)
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	bar := container.NewBorder(nil, nil, container.NewHBox(b, t), container.NewHBox(actions...))
	return container.NewVBox(bar, widget.NewSeparator())
}

// soldiersPage lists the soldiers; selecting one opens it on soldierPage.
func (e *editor) soldiersPage() fyne.CanvasObject {
	soldiers := e.file.Soldiers
	var list *widget.List
	var actions []*bulkAction
	// bulk adds a button that applies fix to every soldier for which
	// needs reports true. It is disabled when no soldier needs it.
	bulk := func(label string, icon fyne.Resource, done string,
		needs func(*save.Soldier) bool, fix func(*save.Soldier)) *widget.Button {
		a := &bulkAction{needs: needs}
		a.btn = widget.NewButtonWithIcon(label, icon, func() {
			n := 0
			for _, s := range soldiers {
				if needs(s) {
					fix(s)
					n++
				}
			}
			list.Refresh()
			for _, a := range actions {
				a.update(soldiers)
			}
			e.changed()
			e.status.SetText(fmt.Sprintf(done+"; save to keep the change.", n))
		})
		actions = append(actions, a)
		return a.btn
	}
	// Heal All clears m_iTurnsOut, and finishes any augmentation in the
	// Cybernetics Lab or gene mod in the Genetics Lab (whose hours left count
	// down alongside it).
	heal := bulk("Heal All", theme.ConfirmIcon(), "Healed %d wounded, augmenting or gene modding soldier(s)",
		func(s *save.Soldier) bool {
			return (s.TurnsOut != 0 && s.Present(save.TurnsOut)) || s.AugmentHoursLeft() > 0 || s.GeneModHoursLeft() > 0
		},
		func(s *save.Soldier) {
			s.TurnsOut = 0
			s.FinishSurgery()
		})
	gift := bulk("Gift All", theme.VisibilityIcon(), "Gave %d soldier(s) the Psi gift",
		// Living soldiers without the gift, or not yet tested. Force Psi Gift
		// only guarantees the gift at testing, so a gifted, tested soldier
		// (e.g. after God Squad, which leaves it off) needs nothing more.
		func(s *save.Soldier) bool {
			return !s.IsDead() && ((!s.HasPsiGift && s.Present(save.HasPsiGift)) ||
				(!s.PsiTested && s.Present(save.PsiTested)))
		},
		func(s *save.Soldier) {
			// Missing properties are ignored when saving.
			s.HasPsiGift, s.PsiTested, s.ForcePsiGift = true, true, true
		})
	// God Squad: Colonels with the psi gift, classes for every rookie, the best load
	// out for each class, and maxed stats (see save.File.GodSquad).
	god := widget.NewButtonWithIcon("God Squad", theme.MediaFastForwardIcon(), func() {
		upgraded, assigned, skipped, err := e.file.GodSquad()
		list.Refresh()
		for _, a := range actions {
			a.update(soldiers)
		}
		e.changed()
		if err != nil {
			dialog.ShowError(err, e.win)
			return
		}
		e.status.SetText(fmt.Sprintf("God Squad: %d soldier(s) upgraded, %d rookie(s) given a class, %d dead soldier(s) skipped (MECs get stats only); save to keep the change.",
			upgraded, assigned, skipped))
	})
	if len(soldiers) == 0 {
		god.Disable()
	}
	hire := widget.NewButtonWithIcon("Hire Soldiers", theme.ContentAddIcon(), e.hireSoldiers)
	if !e.file.CanHireSoldiers() {
		hire.Disable()
	}
	// Rush sets every soldier hiring order's iHours to 0.
	var rush *widget.Button
	rush = widget.NewButtonWithIcon("Rush", theme.MediaSkipNextIcon(), func() {
		n := e.file.PendingHires()
		e.file.RushHires()
		e.changed()
		rush.SetText("Rushed")
		rush.Disable()
		e.status.SetText(fmt.Sprintf("%d hired soldier(s) arrive as soon as time passes in game; save to keep the change.", n))
	})
	if n := e.file.PendingHires(); n > 0 {
		rush.SetText(fmt.Sprintf("Rush (%d)", n))
	} else {
		rush.Disable()
	}
	top := pageHeader("Back", func() { e.show(homePage) },
		fmt.Sprintf("Soldiers (%d) — select a soldier to edit", len(soldiers)), rush, hire, god, gift, heal)

	// Enemy Within adds a column with each soldier's number of gene mods.
	ew := e.file.EnemyWithin()
	columns := listColumns
	if ew {
		columns = append(slices.Clone(listColumns), geneModColumn)
	}
	widths := make([]float32, len(columns))
	grow := make([]bool, len(columns))
	header := make([]fyne.CanvasObject, len(columns))
	for i, c := range columns {
		widths[i], grow[i] = c.width, c.grow
		header[i] = widget.NewLabelWithStyle(c.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	newRow := func(objs ...fyne.CanvasObject) *fyne.Container {
		return container.New(&columnsLayout{widths: widths, grow: grow}, objs...)
	}

	list = widget.NewList(
		func() int { return len(soldiers) },
		func() fyne.CanvasObject {
			cells := []fyne.CanvasObject{widget.NewLabel(""), widget.NewLabel(""), widget.NewLabel(""), widget.NewLabel(""),
				widget.NewLabel(""), container.NewHBox(widget.NewIcon(nil))}
			if ew {
				cells = append(cells, widget.NewLabel(""))
			}
			return newRow(cells...)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			s := soldiers[id]
			cells := o.(*fyne.Container).Objects
			cells[0].(*widget.Label).SetText(s.FirstName)
			cells[1].(*widget.Label).SetText(s.LastName)
			rank := rankTitle(s.Rank)
			if s.IsSHIV() {
				rank = "—" // S.H.I.V.s have no rank (-1)
			}
			cells[2].(*widget.Label).SetText(rank)
			cells[3].(*widget.Label).SetText(s.ClassLabel())
			cells[4].(*widget.Label).SetText(strconv.Itoa(s.TurnsOut))
			icon := cells[5].(*fyne.Container).Objects[0].(*widget.Icon)
			if s.HasPsiGift {
				icon.SetResource(theme.ConfirmIcon())
			} else {
				icon.SetResource(nil)
			}
			if ew {
				mods := ""
				if n := s.GeneModCount(); n > 0 {
					mods = strconv.Itoa(n)
				}
				cells[6].(*widget.Label).SetText(mods)
			}
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		e.soldier = id
		e.show(soldierPage)
	}
	// Coming back from a soldier, keep them in view.
	list.ScrollTo(e.soldier)
	for _, a := range actions {
		a.update(soldiers)
	}

	head := newRow(header...)
	return container.NewBorder(container.NewVBox(top, head, widget.NewSeparator()), nil, nil, nil, list)
}

// bulkAction is a soldier list button that changes every soldier needing it.
type bulkAction struct {
	btn   *widget.Button
	needs func(*save.Soldier) bool
}

// update enables the button only if some soldier needs the action.
func (a *bulkAction) update(soldiers []*save.Soldier) {
	for _, s := range soldiers {
		if a.needs(s) {
			a.btn.Enable()
			return
		}
	}
	a.btn.Disable()
}

// intEntry is an entry editing *v as a whole number in [min, max]. Invalid
// text is highlighted and recorded in e.invalid instead of being stored.
func (e *editor) intEntry(v *int, min, max int, enabled bool) *widget.Entry {
	en := widget.NewEntry()
	en.SetText(strconv.Itoa(*v))
	en.Validator = func(t string) error {
		n, err := strconv.Atoi(t)
		if err != nil {
			return fmt.Errorf("not a whole number")
		}
		if n < min || n > max {
			return fmt.Errorf("must be %d–%d", min, max)
		}
		return nil
	}
	en.OnChanged = func(t string) {
		if en.Validator(t) != nil {
			e.invalid[en] = true
			return
		}
		delete(e.invalid, en)
		*v, _ = strconv.Atoi(t)
		e.changed()
	}
	if !enabled {
		en.Disable()
	}
	return en
}

// countDialog asks for a number from 1 to limit() (with an optional select shown
// above it) and calls ok with it.
func (e *editor) countDialog(title, label string, sel *widget.Select, limit func() int, ok func(n int) error) {
	count := widget.NewEntry()
	count.SetText("1")
	count.Validator = func(t string) error {
		n, err := strconv.Atoi(t)
		if m := limit(); err != nil || n < 1 || n > m {
			return fmt.Errorf("must be 1–%d", m)
		}
		return nil
	}
	var items []*widget.FormItem
	if sel != nil {
		items = append(items, widget.NewFormItem("Continent", sel))
	}
	items = append(items, widget.NewFormItem(label, count))
	d := dialog.NewForm(title, "Order", "Cancel", items, func(confirm bool) {
		if !confirm {
			return
		}
		n, _ := strconv.Atoi(count.Text)
		if err := ok(n); err != nil {
			dialog.ShowError(err, e.win)
		}
	}, e.win)
	if sel != nil {
		sel.OnChanged = func(string) { count.Validate() }
	}
	d.Resize(fyne.NewSize(400, d.MinSize().Height))
	d.Show()
}

// orderInterceptors orders interceptors for a continent, due at once (see
// save.File.OrderInterceptors). A continent's hangar holds
// save.MaxInterceptorsPerContinent interceptors.
func (e *editor) orderInterceptors() {
	names := e.file.ContinentLabels()
	label := func(c int) string { return fmt.Sprintf("%s (%d free)", names[c], e.file.FreeHangarBays(c)) }
	opts := make([]string, len(names))
	best := -1
	for c := range names {
		opts[c] = label(c)
		if e.file.FreeHangarBays(c) > 0 && (best < 0 || (e.file.HasContinent() && c == e.file.Continent)) {
			best = c
		}
	}
	if best < 0 {
		dialog.ShowInformation("Hangars full", fmt.Sprintf("Every continent already has %d interceptors (including orders).", save.MaxInterceptorsPerContinent), e.win)
		return
	}
	sel := widget.NewSelect(opts, nil)
	sel.SetSelectedIndex(best)
	e.countDialog("Order Interceptors", "Interceptors", sel,
		func() int { return e.file.FreeHangarBays(max(0, sel.SelectedIndex())) },
		func(n int) error {
			c := sel.SelectedIndex()
			if err := e.file.OrderInterceptors(c, n); err != nil {
				return err
			}
			e.changed()
			e.status.SetText(fmt.Sprintf("Ordered %d interceptor(s) for %s; they arrive as soon as time passes in game. Save to keep the order.", n, names[c]))
			return nil
		})
}

// hireSoldiers hires soldiers, due at once (see save.File.HireSoldiers).
// The barracks holds save.MaxSoldiersEU or save.MaxSoldiersEW soldiers.
func (e *editor) hireSoldiers() {
	if e.file.FreeBarracks() == 0 {
		dialog.ShowInformation("Barracks full", fmt.Sprintf("The barracks already has %d soldiers (including any on their way).", e.file.BarracksCount()), e.win)
		return
	}
	e.countDialog(fmt.Sprintf("Hire Soldiers (%d free)", e.file.FreeBarracks()), "Soldiers", nil, e.file.FreeBarracks,
		func(n int) error {
			if err := e.file.HireSoldiers(n); err != nil {
				return err
			}
			e.changed()
			e.status.SetText(fmt.Sprintf("Hired %d soldier(s); they arrive as rookies as soon as time passes in game. Save to keep the order.", n))
			return nil
		})
}

// labPage edits XCOM's research labs (the XGFacility_Labs entry).
func (e *editor) labPage() fyne.CanvasObject {
	// Finish Research sets the current project's iActualHoursLeft to 0.
	var finish *widget.Button
	finish = widget.NewButtonWithIcon("Finish Research", theme.ConfirmIcon(), func() {
		e.file.FinishResearch()
		e.changed()
		finish.SetText("Research Finished")
		finish.Disable()
	})
	if h := e.file.ResearchHoursLeft(); h > 0 {
		finish.SetText(fmt.Sprintf("Finish Research (%d hours left)", h))
	} else {
		finish.Disable()
	}
	top := pageHeader("Back", func() { e.leave(homePage) }, "Research", finish)
	form := widget.NewForm(
		e.item("# Scientists", "XGFacility_Labs.m_iNumScientists", e.intEntry(&e.file.Scientists, 0, 1<<31-1, e.file.HasScientists())),
	)
	card := widget.NewCard("Labs", "", form)
	return container.NewBorder(top, nil, nil, nil,
		container.NewVScroll(container.NewCenter(container.New(&minWidthLayout{400}, card))))
}

// workshopPage edits XCOM's engineering (the XGFacility_Engineering entry).
func (e *editor) workshopPage() fyne.CanvasObject {
	// Finish Construction sets every construction and facility project's
	// iHoursLeft to 0.
	var finish *widget.Button
	finish = widget.NewButtonWithIcon("Finish Construction", theme.ConfirmIcon(), func() {
		e.file.FinishConstruction()
		e.changed()
		finish.SetText("Construction Finished")
		finish.Disable()
	})
	if n := e.file.PendingConstruction(); n > 0 {
		finish.SetText(fmt.Sprintf("Finish Construction (%d)", n))
	} else {
		finish.Disable()
	}
	// Complete Projects sets every item and Foundry project's iHoursLeft
	// to 0.
	var complete *widget.Button
	complete = widget.NewButtonWithIcon("Complete Projects", theme.ConfirmIcon(), func() {
		e.file.CompleteProjects()
		e.changed()
		complete.SetText("Projects Completed")
		complete.Disable()
	})
	if n := e.file.PendingItemProjects(); n > 0 {
		complete.SetText(fmt.Sprintf("Complete Projects (%d)", n))
	} else {
		complete.Disable()
	}
	// Deploy Now sets every launched satellite's iTravelTime to 0.
	var deploy *widget.Button
	deploy = widget.NewButtonWithIcon("Deploy Now", theme.UploadIcon(), func() {
		e.file.DeploySatellites()
		e.changed()
		deploy.SetText("Satellites Deployed")
		deploy.Disable()
	})
	if n := e.file.PendingSatellites(); n > 0 {
		deploy.SetText(fmt.Sprintf("Deploy Now (%d)", n))
	} else {
		deploy.Disable()
	}
	top := pageHeader("Back", func() { e.leave(homePage) }, "Engineering", deploy, complete, finish)
	form := widget.NewForm(
		e.item("# Engineers", "XGFacility_Engineering.m_iNumEngineers", e.intEntry(&e.file.Engineers, 0, 1<<31-1, e.file.HasEngineers())),
	)
	card := widget.NewCard("Workshops", "", form)
	return container.NewBorder(top, nil, nil, nil,
		container.NewVScroll(container.NewCenter(container.New(&minWidthLayout{400}, card))))
}

// shipStatLabels are the Hangar page labels for the m_kTShip stats
// (save.ShipStats).
var shipStatLabels = map[string]string{
	"iSpeed":           "Speed",
	"iEngagementSpeed": "Engagement Speed",
	"iHP":              "Hit Points",
	"iAP":              "Armor Penetration",
	"iArmor":           "Armor",
}

// shipColumns are the columns of the interceptor list.
var shipColumns = []struct {
	title string
	width float32
	grow  bool
}{
	{"Callsign", 180, true}, {"Type", 140, false}, {"Weapon", 200, true}, {"Down Time", 120, false}, {"Continent", 160, false},
}

// hangarPage lists XCOM's interceptors (the XGShip_Interceptor_<n> entries);
// selecting one opens it on shipPage.
func (e *editor) hangarPage() fyne.CanvasObject {
	ships := e.file.Ships
	continents := e.file.ContinentLabels()
	var list *widget.List

	// Fulfill Orders sets every pending interceptor order's iHours to 0.
	var fulfill *widget.Button
	fulfill = widget.NewButtonWithIcon("Fulfill Orders", theme.ConfirmIcon(), func() {
		e.file.FulfillOrders()
		e.changed()
		fulfill.SetText("Orders Fulfilled")
		fulfill.Disable()
	})
	if n := e.file.PendingOrders(); n > 0 {
		fulfill.SetText(fmt.Sprintf("Fulfill Orders (%d)", n))
	} else {
		fulfill.Disable()
	}
	// Repair All sets every interceptor's m_iHoursDown to 0.
	var repair *widget.Button
	repair = widget.NewButtonWithIcon("Repair All", theme.SettingsIcon(), func() {
		e.file.RepairAll()
		list.Refresh()
		e.changed()
		repair.SetText("All Repaired")
		repair.Disable()
	})
	if n := e.file.ShipsDown(); n > 0 {
		repair.SetText(fmt.Sprintf("Repair All (%d)", n))
	} else {
		repair.Disable()
	}
	order := widget.NewButtonWithIcon("Order Interceptors", theme.ContentAddIcon(), e.orderInterceptors)
	if !e.file.CanOrderInterceptors() {
		order.Disable()
	}
	top := pageHeader("Back", func() { e.show(homePage) },
		fmt.Sprintf("Hangar (%d) — select an interceptor to edit", len(ships)), fulfill, order, repair)

	widths := make([]float32, len(shipColumns))
	grow := make([]bool, len(shipColumns))
	header := make([]fyne.CanvasObject, len(shipColumns))
	for i, c := range shipColumns {
		widths[i], grow[i] = c.width, c.grow
		header[i] = widget.NewLabelWithStyle(c.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	newRow := func(objs ...fyne.CanvasObject) *fyne.Container {
		return container.New(&columnsLayout{widths: widths, grow: grow}, objs...)
	}

	list = widget.NewList(
		func() int { return len(ships) },
		func() fyne.CanvasObject {
			return newRow(widget.NewLabel(""), widget.NewLabel(""), widget.NewLabel(""), widget.NewLabel(""), widget.NewLabel(""))
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			s := ships[id]
			cells := o.(*fyne.Container).Objects
			cells[0].(*widget.Label).SetText(s.Callsign)
			cells[1].(*widget.Label).SetText(s.TypeName)
			cells[2].(*widget.Label).SetText(save.WeaponName(s.Weapon))
			down := "—"
			if s.HasHoursDown() {
				down = fmt.Sprintf("%d hours", s.HoursDown)
			}
			cells[3].(*widget.Label).SetText(down)
			continent := "—"
			if s.HasHomeContinent() {
				continent = strconv.Itoa(s.HomeContinent)
				if s.HomeContinent >= 0 && s.HomeContinent < len(continents) {
					continent = continents[s.HomeContinent]
				}
			}
			cells[4].(*widget.Label).SetText(continent)
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		e.ship = id
		e.show(shipPage)
	}
	// Coming back from an interceptor, keep it in view.
	list.ScrollTo(e.ship)

	head := newRow(header...)
	return container.NewBorder(container.NewVBox(top, head, widget.NewSeparator()), nil, nil, nil, list)
}

// shipPage edits e.file.Ships[e.ship]: callsign, weapon, type (which also
// sets the type's stats), down time, home continent, and the m_kTShip stats.
func (e *editor) shipPage() fyne.CanvasObject {
	s := e.file.Ships[e.ship]
	top := pageHeader("Hangar", func() { e.leave(hangarPage) },
		fmt.Sprintf("Interceptor %d of %d", e.ship+1, len(e.file.Ships)))

	// choice edits *v as one of values (labelled by labels). A value not in
	// values is shown as a placeholder and kept unless another is chosen.
	choice := func(v *string, values, labels []string, enabled bool) *widget.Select {
		sel := widget.NewSelect(labels, nil)
		if i := slices.Index(values, *v); i >= 0 {
			sel.SetSelectedIndex(i)
		} else {
			sel.PlaceHolder = *v
		}
		sel.OnChanged = func(string) {
			if i := sel.SelectedIndex(); i >= 0 {
				*v = values[i]
				e.changed()
			}
		}
		if !enabled {
			sel.Disable()
		}
		return sel
	}
	weaponValues := make([]string, len(save.ShipWeapons))
	weaponLabels := make([]string, len(save.ShipWeapons))
	for i, w := range save.ShipWeapons {
		weaponValues[i] = w.Value
		weaponLabels[i] = w.Name
	}

	// continent edits m_iHomeContinent as an EContinent index. A value
	// outside the continents is shown as a placeholder and kept unless one
	// is chosen.
	continents := e.file.ContinentLabels()
	labels := make([]string, len(continents))
	for i, c := range continents {
		labels[i] = fmt.Sprintf("%s (%d)", c, i)
	}
	continent := widget.NewSelect(labels, nil)
	if s.HomeContinent >= 0 && s.HomeContinent < len(labels) {
		continent.SetSelectedIndex(s.HomeContinent)
	} else {
		continent.PlaceHolder = strconv.Itoa(s.HomeContinent)
	}
	continent.OnChanged = func(string) {
		if i := continent.SelectedIndex(); i >= 0 {
			s.HomeContinent = i
			e.changed()
		}
	}
	if !s.HasHomeContinent() {
		continent.Disable()
	}

	const maxInt = 1<<31 - 1
	callsign := widget.NewEntry()
	callsign.SetText(s.Callsign)
	callsign.OnChanged = func(t string) { s.Callsign = t; e.changed() }
	if !s.HasCallsign() {
		callsign.Disable()
	}
	// The stats, and current hit points shown after Hit Points.
	stats := map[string]*widget.Entry{}
	for _, n := range save.ShipStats {
		stats[n] = e.intEntry(s.Stat(n), 0, maxInt, s.HasStat(n))
	}
	currentHP := e.intEntry(&s.CurrentHP, 0, maxInt, s.HasCurrentHP())

	// The ship type sets m_kTShip.strName, eType and the entity graphic, and
	// (when it changes) the type's stats and full hit points, so refresh
	// those entries.
	name := widget.NewSelect(save.ShipNames, nil)
	if i := slices.Index(save.ShipNames, s.TypeName); i >= 0 {
		name.SetSelectedIndex(i)
	} else {
		name.PlaceHolder = s.TypeName
	}
	name.OnChanged = func(n string) {
		s.SetName(n)
		for st, en := range stats {
			en.SetText(strconv.Itoa(*s.Stat(st)))
		}
		currentHP.SetText(strconv.Itoa(s.CurrentHP))
		e.changed()
	}
	if !s.HasTypeName() || !s.HasType() {
		name.Disable()
	}

	form := widget.NewForm(
		e.item("Callsign", "m_strCallsign", callsign),
		e.item("Weapon", "m_eWeapon, m_kTShip.arrWeapons", choice(&s.Weapon, weaponValues, weaponLabels, s.HasWeapon())),
		e.item("Type", "m_kTShip.strName + eType, XGShipEntity.m_eEntityGraphic", name),
		e.item("Down Time", "m_iHoursDown", e.intEntry(&s.HoursDown, 0, maxInt, s.HasHoursDown())),
		e.item("Continent", "m_iHomeContinent", continent),
	)
	for _, n := range save.ShipStats {
		form.AppendItem(e.item(shipStatLabels[n], "m_kTShip."+n, stats[n]))
		if n == "iHP" {
			form.AppendItem(e.item("Current Hit Points", "m_iHP", currentHP))
		}
	}
	card := widget.NewCard(s.Callsign, e.subtitle(s.InstanceName), form)
	return container.NewBorder(top, nil, nil, nil,
		container.NewVScroll(container.NewCenter(container.New(&minWidthLayout{560}, card))))
}

// storageColumns is the number of item columns in each storage group.
const storageColumns = 3

// storagePage edits the number of each item in storage (XGStorage_0's
// m_arrItems), grouped by kind.
func (e *editor) storagePage() fyne.CanvasObject {
	top := pageHeader("Back", func() { e.leave(homePage) }, "Storage — number of each item held")

	const maxInt = 1<<31 - 1
	groups := container.NewVBox()
	for _, g := range save.StorageGroups {
		var items []save.StorageItem
		for _, it := range e.file.StorageItems() {
			if it.Group == g {
				items = append(items, it)
			}
		}
		if len(items) == 0 {
			continue
		}
		// Fill columns top to bottom so items read in order down each column.
		perCol := (len(items) + storageColumns - 1) / storageColumns
		cols := make([]fyne.CanvasObject, storageColumns)
		for c := range cols {
			form := widget.NewForm()
			for _, it := range items[min(c*perCol, len(items)):min((c+1)*perCol, len(items))] {
				var en *widget.Entry
				if e.file.HasItem(it) {
					en = e.intEntry(&e.file.Items[it.Index()], 0, maxInt, true)
				} else {
					var none int
					en = e.intEntry(&none, 0, maxInt, false)
				}
				form.AppendItem(e.item(it.Name, fmt.Sprintf("m_arrItems[%d]", it.Index()), en))
			}
			cols[c] = form
		}
		groups.Add(widget.NewCard(g, "", container.NewGridWithColumns(storageColumns, cols...)))
	}
	return container.NewBorder(top, nil, nil, nil,
		container.NewVScroll(container.NewCenter(container.New(&minWidthLayout{1100}, groups))))
}

// soldierTitle is a soldier's display name, e.g. Katia "Snake Eyes" Delgado.
func soldierTitle(s *save.Soldier) string {
	parts := []string{}
	if s.FirstName != "" {
		parts = append(parts, s.FirstName)
	}
	if s.NickName != "" {
		parts = append(parts, `"`+s.NickName+`"`)
	}
	if s.LastName != "" {
		parts = append(parts, s.LastName)
	}
	if len(parts) == 0 {
		return s.InstanceName
	}
	return strings.Join(parts, " ")
}

// soldierPage edits the properties of e.file.Soldiers[e.soldier].
func (e *editor) soldierPage() fyne.CanvasObject {
	s := e.file.Soldiers[e.soldier]
	top := pageHeader("Barracks", func() { e.leave(soldiersPage) },
		fmt.Sprintf("Soldier %d of %d", e.soldier+1, len(e.file.Soldiers)))

	strEntry := func(f save.Field, v *string) fyne.CanvasObject {
		en := widget.NewEntry()
		en.SetText(*v)
		en.OnChanged = func(t string) { *v = t; e.changed() }
		if !s.Present(f) {
			en.Disable()
		}
		return en
	}
	intEntry := func(f save.Field, v *int, min, max int) fyne.CanvasObject {
		return e.intEntry(v, min, max, s.Present(f))
	}

	// indexSelect edits *v as an index into options. A value outside the
	// options is shown as a placeholder and kept unless another is chosen.
	indexSelect := func(f save.Field, options []string, v *int) *widget.Select {
		sel := widget.NewSelect(options, nil)
		if *v >= 0 && *v < len(options) {
			sel.SetSelectedIndex(*v)
		} else {
			sel.PlaceHolder = strconv.Itoa(*v)
		}
		sel.OnChanged = func(string) {
			if i := sel.SelectedIndex(); i >= 0 {
				*v = i
				e.changed()
			}
		}
		if !s.Present(f) {
			sel.Disable()
		}
		return sel
	}
	rank := indexSelect(save.Rank, rankNames, &s.Rank)
	psiRank := indexSelect(save.PsiRank, psiRankNames, &s.PsiRank)

	// Classes, plus MEC Trooper in Enemy Within saves (option index
	// save.MECIndex, which follows the classes).
	classNames := make([]string, len(save.Classes))
	for i, c := range save.Classes {
		classNames[i] = c.Label()
	}
	if e.file.EnemyWithin() {
		classNames = append(classNames, save.MECLabel)
	}
	class := widget.NewSelect(classNames, nil)
	if s.Class >= 0 && s.Class < len(classNames) {
		class.SetSelectedIndex(s.Class)
	} else {
		// A class this editor does not know (e.g. from a mod); it is kept
		// unless another class is chosen.
		class.PlaceHolder = s.ClassLabel()
	}
	if s.IsSHIV() {
		// S.H.I.V.s have no class or rank: show the S.H.I.V. type instead.
		class.ClearSelected()
		class.PlaceHolder = s.ClassLabel()
		rank.PlaceHolder = "—"
	}
	var onClassChanged func()
	class.OnChanged = func(string) {
		i := class.SelectedIndex()
		if i < 0 || i == s.Class {
			return
		}
		mec := s.IsMEC() || i == save.MECIndex
		s.SetClass(i)
		e.changed()
		if mec {
			// Becoming or leaving a MEC changes the load out and psi:
			// rebuild the page to show them.
			e.show(soldierPage)
			return
		}
		if onClassChanged != nil {
			onClassChanged()
		}
	}
	if !s.Present(save.Class) {
		class.Disable()
	}

	check := func(f save.Field, label string, v *bool) *widget.Check {
		c := widget.NewCheck(label, func(b bool) { *v = b; e.changed() })
		c.SetChecked(*v)
		if !s.Present(f) {
			c.Disable()
		}
		return c
	}
	psi := newCheckGroup(
		check(save.HasPsiGift, "Gifted", &s.HasPsiGift),
		check(save.PsiTested, "Tested", &s.PsiTested),
		check(save.ForcePsiGift, "Forced", &s.ForcePsiGift),
	)

	// In Enemy Within the Genetics Lab refuses any soldier with a psi rank.
	var psiHint []string
	if e.file.EnemyWithin() && !s.IsMEC() {
		psiHint = append(psiHint, "Above 0, the Genetics Lab will not accept this soldier")
	}
	psiRankItem := e.item("Psi Rank", "m_kSoldier.iPsiRank", psiRank, psiHint...)

	const maxInt = 1<<31 - 1
	// Base stats (m_kChar.aStats), one per row in a card of their own.
	stat := func(f save.Field, label string, i int, v *int, min int) *widget.FormItem {
		return e.item(label, fmt.Sprintf("m_kChar.aStats[%d]", i), intEntry(f, v, min, maxInt))
	}
	stats := widget.NewCard("Stats", "", widget.NewForm(
		stat(save.Health, "Health", 0, &s.Health, 1),
		stat(save.Aim, "Aim", 1, &s.Aim, 0),
		stat(save.Defense, "Defense", 2, &s.Defense, 0),
		stat(save.Mobility, "Travel", 3, &s.Mobility, 0),
		stat(save.Will, "Will", 7, &s.Will, 0),
	))
	form := widget.NewForm(
		e.item("First Name", "m_kSoldier.strFirstName", strEntry(save.FirstName, &s.FirstName)),
		e.item("Last Name", "m_kSoldier.strLastName", strEntry(save.LastName, &s.LastName)),
		e.item("Nickname", "m_kSoldier.strNickName", strEntry(save.NickName, &s.NickName)),
		e.item("Class", "m_kSoldier.kClass (strName, eType, eWeaponType), m_kChar.eClass", class),
		e.item("Rank", "m_kSoldier.iRank", rank),
		e.item("Experience", "m_kSoldier.iXP", intEntry(save.XP, &s.XP, 0, maxInt)),
		psiRankItem,
		e.item("Psi Experience", "m_kSoldier.iPsiXP", intEntry(save.PsiXP, &s.PsiXP, 0, maxInt)),
		e.item("Psi", "m_kChar.bHasPsiGift, m_bPsiTested, bForcePsiGift", psi),
	)
	// Load out: item type numbers from m_kChar.kInventory, chosen by name.
	// With none, 0 is offered as "None". A value not in items (e.g. from a
	// mod) is shown as the placeholder and kept unless another is chosen.
	// setItems replaces the options and reselects *v.
	item := func(f save.Field, v *int, items []save.LoadOutItem, none bool) (sel *widget.Select, setItems func([]save.LoadOutItem)) {
		var values []int
		sel = widget.NewSelect(nil, nil)
		setItems = func(items []save.LoadOutItem) {
			values = nil
			var names []string
			if none {
				values, names = append(values, 0), append(names, "None")
			}
			for _, it := range items {
				values, names = append(values, it.Value), append(names, it.Name)
			}
			sel.Options = names
			sel.PlaceHolder = fmt.Sprintf("Unlisted (%d)", *v)
			sel.ClearSelected()
			for i, val := range values {
				if val == *v {
					sel.SetSelectedIndex(i)
				}
			}
			sel.Refresh()
		}
		setItems(items)
		sel.OnChanged = func(string) {
			if i := sel.SelectedIndex(); i >= 0 && values[i] != *v {
				*v = values[i]
				e.changed()
			}
		}
		if !s.Present(f) {
			sel.Disable()
		}
		return sel, setItems
	}
	ew := e.file.EnemyWithin()
	armorItems := save.ArmorFor(ew)
	if s.IsMEC() {
		armorItems = save.MECArmorItems
	}
	weaponItems := save.WeaponsFor(s.Class, ew)
	if s.IsSHIV() {
		// Not editable; listed so the deck and weapon are shown by name.
		armorItems, weaponItems = save.SHIVDecksFor(ew), save.SHIVWeaponsFor(ew)
	}
	armor, _ := item(save.Armor, &s.Armor, armorItems, false)
	pistol, _ := item(save.Pistol, &s.Pistol, save.PistolItems, true)
	weapon, setWeapons := item(save.Weapon, &s.Weapon, weaponItems, false)
	rocket, _ := item(save.Rocket, &s.Rocket, save.RocketsFor(ew), true)
	item1, _ := item(save.Item1, &s.Item1, save.SmallItemsFor(ew), true)
	item2, _ := item(save.Item2, &s.Item2, save.SmallItemsFor(ew), true)
	// Heavies carry a rocket instead of a pistol; on save a Heavy's pistol,
	// or another known class's rocket, is cleared. Unknown classes can use
	// both. Each class has its own weapons.
	enable := func(f save.Field, sel *widget.Select, on bool) {
		if !s.Present(f) {
			return
		}
		if on {
			sel.Enable()
		} else {
			sel.Disable()
		}
	}
	updateClass := func() {
		if s.IsSHIV() {
			return // load out fixed; keep the S.H.I.V. weapon list
		}
		unknown := s.Class == save.UnknownClass
		enable(save.Rocket, rocket, s.IsHeavy() || unknown)
		enable(save.Pistol, pistol, !s.IsHeavy())
		s.Weapon = save.ClassWeapon(s.Class, s.Weapon)
		setWeapons(save.WeaponsFor(s.Class, ew))
	}
	updateClass()
	onClassChanged = updateClass
	if s.IsMEC() {
		// A MEC wears Base Augments or a MEC suit and carries nothing but its
		// weapon (save.MECArmorItems, save.MECWeaponItems).
		for _, sel := range []*widget.Select{pistol, rocket, item1, item2} {
			sel.Disable()
		}
	}
	armorItem := e.item("Armor", "m_kChar.kInventory.iArmor", armor)
	loadOutForm := widget.NewForm(
		armorItem,
		e.item("Pistol", "m_kChar.kInventory.iPistol", pistol),
		e.item("Weapon", "m_kChar.kInventory.arrLargeItems[0]", weapon),
		e.item("Rocket", "m_kChar.kInventory.arrLargeItems[1]", rocket),
		e.item("Item 1", "m_kChar.kInventory.arrSmallItems[0]", item1),
		e.item("Item 2", "m_kChar.kInventory.arrSmallItems[1]", item2),
	)
	loadOut := widget.NewCard("Load Out", "", loadOutForm)
	loadOutWidth := float32(260)
	if s.IsMEC() {
		loadOutWidth = 380 // room for suit names
		// Say when the chosen suit isn't in storage, so saving will add one.
		// A form item's hint is fixed once shown, so a new suit redraws the
		// page.
		if e.file.SuitAdded(s, s.Armor) {
			armorItem.HintText = strings.TrimPrefix(armorItem.HintText+" — None in storage: saving adds one", " — ")
		}
		changed := armor.OnChanged
		armor.OnChanged = func(v string) {
			old := s.Armor
			changed(v)
			if s.Armor != old {
				e.show(soldierPage)
			}
		}
	}

	wounded := widget.NewCard("Wounded", "", widget.NewForm(
		e.item("Turns Out", "m_iTurnsOut", intEntry(save.TurnsOut, &s.TurnsOut, 0, maxInt)),
	))
	// Right of the details: the load out and wounds, then the stats above
	// (Enemy Within) the gene mods.
	third := container.NewVBox(stats)
	if ew {
		third.Add(e.geneModsCard(s))
	}
	side := container.NewHBox(
		container.New(&minWidthLayout{loadOutWidth}, container.NewVBox(loadOut, wounded)),
		container.New(&minWidthLayout{260}, third),
	)
	body := container.NewBorder(nil, nil, nil, side, form)
	card := widget.NewCard(soldierTitle(s), e.subtitle(s.InstanceName), body)
	e.onEdit = func() { card.SetTitle(soldierTitle(s)) }
	return container.NewBorder(top, nil, nil, nil,
		container.NewVScroll(container.NewCenter(container.New(&minWidthLayout{820}, card))))
}

// geneModsCard edits an Enemy Within soldier's gene mods: for each body
// part, None or one of its two mods. MECs and S.H.I.V.s cannot be modded.
func (e *editor) geneModsCard(s *save.Soldier) fyne.CanvasObject {
	form := widget.NewForm()
	for i, part := range save.BodyParts {
		options := []string{"None", part.Mods[0].Name, part.Mods[1].Name}
		sel := widget.NewSelect(options, nil)
		sel.SetSelectedIndex(s.GeneMods[i])
		sel.OnChanged = func(string) {
			if j := sel.SelectedIndex(); j >= 0 && j != s.GeneMods[i] {
				s.GeneMods[i] = j
				e.changed()
			}
		}
		if !s.HasGeneMods() {
			sel.Disable()
		}
		form.AppendItem(e.item(part.Name, fmt.Sprintf("m_kChar.aUpgrades[%d], [%d]", part.Mods[0].Ability, part.Mods[1].Ability), sel))
	}
	return widget.NewCard("Gene Mods", "", form)
}

// minWidthLayout stacks its objects, giving them at least a fixed width.
type minWidthLayout struct{ width float32 }

func (m *minWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

func (m *minWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	s.Width = max(s.Width, m.width)
	return s
}

// columnsLayout lays objects out in rows with fixed minimum column widths.
// Any extra horizontal space is shared among the columns with grow set.
type columnsLayout struct {
	widths []float32
	grow   []bool
}

func (c *columnsLayout) colWidths(total float32) []float32 {
	w := append([]float32(nil), c.widths...)
	var sum float32
	for _, x := range w {
		sum += x
	}
	sum += theme.Padding() * float32(len(w)-1)
	var n float32
	for _, g := range c.grow {
		if g {
			n++
		}
	}
	if extra := total - sum; extra > 0 && n > 0 {
		for i, g := range c.grow {
			if g {
				w[i] += extra / n
			}
		}
	}
	return w
}

func (c *columnsLayout) rowHeight(objs []fyne.CanvasObject, r int) float32 {
	var h float32
	n := len(c.widths)
	for i := r * n; i < (r+1)*n && i < len(objs); i++ {
		h = max(h, objs[i].MinSize().Height)
	}
	return h
}

func (c *columnsLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	n := len(c.widths)
	w := c.colWidths(size.Width)
	pad := theme.Padding()
	var y float32
	for r := 0; r*n < len(objs); r++ {
		h := c.rowHeight(objs, r)
		var x float32
		for i := 0; i < n && r*n+i < len(objs); i++ {
			o := objs[r*n+i]
			o.Move(fyne.NewPos(x, y))
			o.Resize(fyne.NewSize(w[i], h))
			x += w[i] + pad
		}
		y += h + pad
	}
}

func (c *columnsLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	n := len(c.widths)
	pad := theme.Padding()
	var width float32
	for _, x := range c.widths {
		width += x
	}
	width += pad * float32(n-1)
	var h float32
	rows := (len(objs) + n - 1) / n
	for r := 0; r < rows; r++ {
		h += c.rowHeight(objs, r)
	}
	if rows > 1 {
		h += pad * float32(rows-1)
	}
	return fyne.NewSize(width, h)
}
