// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package main

import (
	"image/png"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/scottvail/xcomsave-editor/internal/save"
)

// The UI tests drive the editor headlessly on saves as the game wrote them:
// example/eu-source (Enemy Unknown) and example/ew-source (Enemy Within),
// relative to this package (../../example).
// Values are taken from the loaded save rather than hard-coded, so the
// sources can be replaced. Set OUT=prefix to also write screenshots
// (prefix-eu-home.png etc.).

// openSave loads path into a headless editor window on the landing page.
func openSave(t *testing.T, path string) (*editor, fyne.Window) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s not present", path)
	}
	a := test.NewApp()
	w := a.NewWindow(appTitle)
	e := newEditor(a, w)
	w.Resize(windowSize)
	e.load(path)
	if e.file == nil {
		t.Fatalf("%s did not load", path)
	}
	if e.page != homePage {
		t.Fatalf("after load, page = %v, want home", e.page)
	}
	return e, w
}

// TestUIGodSquadGift: God Squad gives the psi gift, so Gift All stays
// disabled after it.
func TestUIGodSquadGift(t *testing.T) {
	for _, path := range []string{"../../example/eu-source", "../../example/ew-source"} {
		e, w := openSave(t, path)
		tapButton(t, w, "Barracks")
		tapButton(t, w, "Gift All")
		if !findButton(w.Canvas().Content(), "Gift All").Disabled() {
			t.Fatalf("%s: Gift All enabled after Gift All", path)
		}
		tapButton(t, w, "God Squad")
		if !findButton(w.Canvas().Content(), "Gift All").Disabled() {
			t.Fatalf("%s: Gift All re-enabled by God Squad", path)
		}
		// The Psi row reports disabled (so its label is dimmed) only when
		// none of its boxes can be edited, as for a S.H.I.V.
		for i, s := range e.file.Soldiers {
			if i > 0 && !s.IsSHIV() {
				continue
			}
			openSoldier(t, e, w, i)
			g := find[*checkGroup](w.Canvas().Content(), func(*checkGroup) bool { return true })
			boxes := 0
			if g != nil {
				boxes = len(test.WidgetRenderer(g).Objects()[0].(*fyne.Container).Objects)
			}
			if g == nil || boxes != 3 || g.Disabled() != !s.Present(save.HasPsiGift) {
				t.Fatalf("%s: %s psi row: %d boxes, disabled %v, editable %v", path, s.LastName, boxes, g != nil && g.Disabled(), s.Present(save.HasPsiGift))
			}
			tapButton(t, w, "Barracks")
		}
	}
}

// TestUIEnemyUnknown: the landing page, the soldier list's bulk buttons, a
// Heavy's load out and class change, a S.H.I.V., storage, the hangar and the
// research and engineering pages, on an Enemy Unknown save.
func TestUIEnemyUnknown(t *testing.T) {
	e, w := openSave(t, "../../example/eu-source")
	f := e.file
	if f.EnemyWithin() || len(f.Soldiers) == 0 || len(f.Ships) == 0 {
		t.Fatalf("expected an Enemy Unknown save with soldiers and interceptors")
	}
	capture(t, w, "eu-home")
	checkCash(t, e, w)

	// Barracks: Heal All and Gift All.
	tapButton(t, w, "Barracks")
	if e.page != soldiersPage {
		t.Fatalf("after Barracks, page = %v, want soldiers", e.page)
	}
	capture(t, w, "eu-soldiers")
	if headerLabel(w, "Gene Mods") != nil {
		t.Fatal("Enemy Unknown soldier list has a Gene Mods column")
	}
	soldiers := slices.Collect(func(yield func(int) bool) {
		for i, s := range f.Soldiers {
			if !s.IsSHIV() && s.Present(save.TurnsOut) && !yield(i) {
				return
			}
		}
	})
	f.Soldiers[soldiers[0]].TurnsOut, f.Soldiers[soldiers[1]].TurnsOut = 4, 9
	e.show(soldiersPage)
	tapButton(t, w, "Heal All")
	for _, s := range f.Soldiers {
		if s.TurnsOut != 0 && s.Present(save.TurnsOut) {
			t.Fatalf("%s still has TurnsOut %d after Heal All", s.InstanceName, s.TurnsOut)
		}
	}
	if !findButton(w.Canvas().Content(), "Heal All").Disabled() {
		t.Fatal("Heal All should be disabled once everyone is healed")
	}
	if b := findButton(w.Canvas().Content(), "Gift All"); b != nil && !b.Disabled() {
		test.Tap(b)
	}
	for _, s := range f.Soldiers {
		gifted := s.HasPsiGift && s.PsiTested // Forced is not needed once tested
		if s.Present(save.HasPsiGift) && !s.IsDead() && !gifted {
			t.Fatalf("%s not fully gifted after Gift All", s.InstanceName)
		}
		if s.IsSHIV() && (s.Present(save.HasPsiGift) || gifted) {
			t.Fatalf("S.H.I.V. %s given the psi gift", s.LastName)
		}
	}
	if !findButton(w.Canvas().Content(), "Gift All").Disabled() {
		t.Fatal("Gift All should be disabled once everyone is gifted")
	}

	// A Heavy: the rocket is editable, the pistol not; weapons are the
	// class's own; changing class swaps the weapon for the same tier.
	heavy := slices.IndexFunc(f.Soldiers, func(s *save.Soldier) bool {
		return s.IsHeavy() && s.Present(save.Rocket) && slices.ContainsFunc(save.WeaponsFor(4, false),
			func(it save.LoadOutItem) bool { return it.Value == s.Weapon })
	})
	if heavy < 0 {
		t.Fatal("no Heavy with a Heavy weapon in the save")
	}
	s := f.Soldiers[heavy]
	openSoldier(t, e, w, heavy)
	first := find[*widget.Entry](w.Canvas().Content(), func(en *widget.Entry) bool { return en.Text == s.FirstName })
	if first == nil {
		t.Fatal("first name entry not found on soldier page")
	}
	test.Type(first, "X")
	if !f.Modified() {
		t.Fatal("edit was not recorded")
	}
	capture(t, w, "eu-heavy")
	if geneModSelect(w) != nil {
		t.Fatal("Enemy Unknown soldiers have no gene mods")
	}
	p := soldierPageSelects(t, w, false)
	if p.class.Selected != "Heavy" || p.rocket.Disabled() || !p.pistol.Disabled() || p.pistol.Selected != "None" {
		t.Fatalf("Heavy: class %q rocket disabled %v pistol %q disabled %v", p.class.Selected, p.rocket.Disabled(), p.pistol.Selected, p.pistol.Disabled())
	}
	if got := strings.Join(p.weapon.Options, ","); got != "LMG,Heavy Laser,Heavy Plasma" {
		t.Fatalf("Heavy weapons = %s", got)
	}
	p.weapon.SetSelected("Heavy Laser")
	if s.Weapon != 11 {
		t.Fatalf("weapon = %d after choosing Heavy Laser, want 11", s.Weapon)
	}
	// Enemy Unknown's armor and item numbers.
	if got := strings.Join(p.item1.Options, ","); !strings.HasPrefix(got, "None,Medikit,Combat Stims,") {
		t.Fatalf("Enemy Unknown items = %s", got)
	}
	if slices.Contains(p.class.Options, save.MECLabel) {
		t.Fatal("MEC Trooper offered in an Enemy Unknown save")
	}
	p.class.SetSelectedIndex(1) // Sniper
	if !p.rocket.Disabled() || p.pistol.Disabled() || p.weapon.Selected != "Laser Sniper Rifle" || s.Weapon != 12 {
		t.Fatalf("Sniper: rocket disabled %v pistol disabled %v weapon %q (%d)", p.rocket.Disabled(), p.pistol.Disabled(), p.weapon.Selected, s.Weapon)
	}
	capture(t, w, "eu-sniper")
	p.class.SetSelectedIndex(4) // Heavy
	if p.rocket.Disabled() || !p.pistol.Disabled() || p.weapon.Selected != "Heavy Laser" {
		t.Fatalf("back to Heavy: rocket disabled %v pistol disabled %v weapon %q", p.rocket.Disabled(), p.pistol.Disabled(), p.weapon.Selected)
	}
	tapButton(t, w, "Barracks")
	if e.page != soldiersPage || !f.Modified() {
		t.Fatalf("after Barracks: page %v, modified %v", e.page, f.Modified())
	}

	// A S.H.I.V.: rank, class and load out locked; stats editable.
	if shiv := slices.IndexFunc(f.Soldiers, (*save.Soldier).IsSHIV); shiv >= 0 {
		openSoldier(t, e, w, shiv)
		capture(t, w, "eu-shiv")
		p := soldierPageSelects(t, w, false)
		if !p.class.Disabled() || !p.rank.Disabled() || !p.weapon.Disabled() || !p.armor.Disabled() {
			t.Fatal("a S.H.I.V.'s rank, class and load out should not be editable")
		}
		// Shown by type, deck and weapon name (eu-source: GOLIATH-1 is an
		// Alloy S.H.I.V. with a Plasma Cannon).
		if p.class.PlaceHolder != f.Soldiers[shiv].SHIVType() || p.class.Selected != "" || p.rank.PlaceHolder != "—" ||
			p.armor.Selected == "" || p.weapon.Selected == "" {
			t.Fatalf("S.H.I.V. shown as class %q/%q rank %q armor %q weapon %q",
				p.class.PlaceHolder, p.class.Selected, p.rank.PlaceHolder, p.armor.Selected, p.weapon.Selected)
		}
		health := find[*widget.Entry](w.Canvas().Content(), func(en *widget.Entry) bool {
			return en.Text == strconv.Itoa(f.Soldiers[shiv].Health)
		})
		if health == nil || health.Disabled() {
			t.Fatal("a S.H.I.V.'s health should be editable")
		}
		tapButton(t, w, "Barracks")
	}
	tapButton(t, w, "Back")
	if e.page != homePage {
		t.Fatalf("after Back, page = %v, want home", e.page)
	}

	checkStorage(t, e, w, save.StorageItems)
	checkHangar(t, e, w)
	checkLabs(t, e, w)
}

// TestUIEnemyWithin: Enemy Within's items, the MEC Trooper (its page, and
// changing it back to a soldier class), and the pages, on an Enemy Within
// save.
func TestUIEnemyWithin(t *testing.T) {
	e, w := openSave(t, "../../example/ew-source")
	f := e.file
	if !f.EnemyWithin() {
		t.Fatal("expected an Enemy Within save")
	}
	capture(t, w, "ew-home")
	checkCash(t, e, w)

	// A soldier: Enemy Within's items, and MEC Trooper among the classes.
	tapButton(t, w, "Barracks")
	if headerLabel(w, "Gene Mods") == nil {
		t.Fatal("no Gene Mods column in the Enemy Within soldier list")
	}
	// The first soldier (a rookie with none) given two gene mods shows "2".
	first := f.Soldiers[0]
	saved := first.GeneMods
	first.GeneMods[0], first.GeneMods[4] = 1, 2
	e.show(soldiersPage)
	if find[*widget.Label](w.Canvas().Content(), func(l *widget.Label) bool { return l.Text == "2" }) == nil {
		t.Fatal("soldier list does not show 2 gene mods")
	}
	capture(t, w, "ew-soldiers")
	first.GeneMods = saved
	e.show(soldiersPage)
	soldier := slices.IndexFunc(f.Soldiers, func(s *save.Soldier) bool { return !s.IsMEC() && s.Present(save.Class) })
	openSoldier(t, e, w, soldier)
	p := soldierPageSelects(t, w, true)
	if got := strings.Join(p.item1.Options, ","); !strings.HasPrefix(got, "None,Medkit,Mind Shield,") {
		t.Fatalf("Enemy Within items = %s", got)
	}
	if p.class.Options[save.MECIndex] != save.MECLabel {
		t.Fatalf("classes = %v, want MEC Trooper at %d", p.class.Options, save.MECIndex)
	}
	// Gene mods: one select per body part.
	brain := geneModSelect(w)
	if brain == nil || brain.Disabled() {
		t.Fatal("no editable Brain gene mod select")
	}
	brain.SetSelectedIndex(2)
	if got := f.Soldiers[soldier].GeneMods[0]; got != 2 {
		t.Fatalf("after choosing Neural Feedback, Brain gene mod = %d", got)
	}
	capture(t, w, "ew-soldier")

	// Debug mode: the entry name and each field's save property.
	s := f.Soldiers[soldier]
	card := func() *widget.Card {
		return find[*widget.Card](w.Canvas().Content(), func(c *widget.Card) bool { return c.Title == soldierTitle(s) })
	}
	hint := func(label string) string {
		form := find[*widget.Form](w.Canvas().Content(), func(fm *widget.Form) bool {
			return slices.ContainsFunc(fm.Items, func(it *widget.FormItem) bool { return it.Text == label })
		})
		for _, it := range form.Items {
			if it.Text == label {
				return it.HintText
			}
		}
		return ""
	}
	if c := card(); c == nil || c.Subtitle != "" || hint("Rank") != "" {
		t.Fatal("entry name or property shown outside debug mode")
	}
	e.toggleDebug()
	if c := card(); c == nil || c.Subtitle != s.InstanceName || hint("Rank") != "m_kSoldier.iRank" ||
		hint("Brain") != "m_kChar.aUpgrades[28], [29]" ||
		!strings.HasPrefix(hint("Psi Rank"), "m_kSoldier.iPsiRank — Above 0") {
		t.Fatalf("debug mode: subtitle %q, Rank hint %q, Brain hint %q, Psi Rank hint %q",
			card().Subtitle, hint("Rank"), hint("Brain"), hint("Psi Rank"))
	}
	capture(t, w, "ew-soldier-debug")
	e.toggleDebug()
	tapButton(t, w, "Barracks")

	// The MEC: its load out and psi locked; made a Sniper, it gets a
	// soldier's load out and the page shows it.
	mec := slices.IndexFunc(f.Soldiers, (*save.Soldier).IsMEC)
	if mec < 0 {
		t.Fatal("no MEC Trooper in the save")
	}
	m := f.Soldiers[mec]
	openSoldier(t, e, w, mec)
	capture(t, w, "ew-mec")
	p = soldierPageSelects(t, w, true)
	if p.class.Selected != save.MECLabel || !p.item1.Disabled() {
		t.Fatalf("MEC: class %q, items disabled %v", p.class.Selected, p.item1.Disabled())
	}
	// The MEC's armor: Base Augments or a MEC suit.
	suits := find[*widget.Select](w.Canvas().Content(), func(sel *widget.Select) bool {
		return len(sel.Options) > 0 && sel.Options[0] == save.MECArmorItems[0].Name
	})
	if suits == nil || suits.Disabled() || len(suits.Options) != len(save.MECArmorItems) {
		t.Fatal("no editable MEC armor select")
	}
	suits.SetSelected("MEC-2 Sentinel: Kinetic, Mist")
	if m.Armor != 199 {
		t.Fatalf("MEC armor after choosing the Mk2 = %d", m.Armor)
	}
	// The page is redrawn, noting the Mk2 isn't in storage.
	if h := hint("Armor"); !strings.HasSuffix(h, "None in storage: saving adds one") {
		t.Fatalf("MEC armor hint %q", h)
	}
	p = soldierPageSelects(t, w, true)
	capture(t, w, "ew-mec-suit")
	// Only the weapon is editable: a MEC's own weapons.
	if p.weapon.Disabled() || strings.Join(p.weapon.Options, ",") != "Minigun,Railgun,Particle Cannon" {
		t.Fatalf("MEC weapon: disabled %v options %v", p.weapon.Disabled(), p.weapon.Options)
	}
	p.weapon.SetSelected("Particle Cannon")
	if m.Weapon != 30 {
		t.Fatalf("MEC weapon after choosing Particle Cannon = %d", m.Weapon)
	}
	gift := find[*widget.Check](w.Canvas().Content(), func(c *widget.Check) bool { return c.Text == "Gifted" })
	if gift == nil || !gift.Disabled() {
		t.Fatal("a MEC's psi gift should not be editable")
	}
	if g := geneModSelect(w); g == nil || !g.Disabled() {
		t.Fatal("a MEC's gene mods should not be editable")
	}
	suit, mecWeapon := m.Armor, m.Weapon
	p.class.SetSelectedIndex(1) // Sniper: the page is rebuilt
	if m.IsMEC() || m.Armor == suit || m.Weapon == mecWeapon {
		t.Fatalf("after leaving MEC: MEC %v armor %d weapon %d", m.IsMEC(), m.Armor, m.Weapon)
	}
	p = soldierPageSelects(t, w, true)
	if p.class.Selected != "Sniper" || p.armor.Disabled() || p.weapon.Disabled() {
		t.Fatalf("page after leaving MEC: class %q, armor disabled %v", p.class.Selected, p.armor.Disabled())
	}
	capture(t, w, "ew-mec-sniper")
	if err := f.Validate(); err != nil {
		t.Fatalf("former MEC not savable: %v", err)
	}
	tapButton(t, w, "Barracks")
	tapButton(t, w, "Back")

	checkStorage(t, e, w, save.EWStorageItems)
	checkHangar(t, e, w)
	checkLabs(t, e, w)
}

// checkCash finds the cash entry on the landing page and edits it.
func checkCash(t *testing.T, e *editor, w fyne.Window) {
	t.Helper()
	e.file.Cash = 4242424 // a value no other field has
	e.show(homePage)
	cash := find[*widget.Entry](w.Canvas().Content(), func(en *widget.Entry) bool { return en.Text == "4242424" })
	if cash == nil {
		t.Fatal("cash entry not found on the landing page")
	}
	cash.SetText("50000")
	if e.file.Cash != 50000 {
		t.Fatalf("cash = %d, want 50000", e.file.Cash)
	}
}

// headerLabel finds a bold (column header) label with the given text, or nil.
func headerLabel(w fyne.Window, text string) *widget.Label {
	return find[*widget.Label](w.Canvas().Content(), func(l *widget.Label) bool {
		return l.Text == text && l.TextStyle.Bold
	})
}

// geneModSelect finds the soldier page's Brain gene mod select, or nil.
func geneModSelect(w fyne.Window) *widget.Select {
	return find[*widget.Select](w.Canvas().Content(), func(sel *widget.Select) bool {
		return slices.Equal(sel.Options, []string{"None", save.BodyParts[0].Mods[0].Name, save.BodyParts[0].Mods[1].Name})
	})
}

// openSoldier opens soldier i from the soldier list.
func openSoldier(t *testing.T, e *editor, w fyne.Window, i int) {
	t.Helper()
	list := find[*widget.List](w.Canvas().Content(), func(*widget.List) bool { return true })
	if list == nil {
		t.Fatal("no soldier list")
	}
	list.Select(i)
	if e.page != soldierPage || e.soldier != i {
		t.Fatalf("after selecting soldier %d, page = %v soldier = %d", i, e.page, e.soldier)
	}
}

// soldierSelects are the selects on a soldier's page.
type soldierSelects struct {
	class, rank, armor, pistol, weapon, rocket, item1 *widget.Select
}

// soldierPageSelects finds the soldier page's selects by their options.
func soldierPageSelects(t *testing.T, w fyne.Window, ew bool) soldierSelects {
	t.Helper()
	content := w.Canvas().Content()
	byOptions := func(first string) *widget.Select {
		return find[*widget.Select](content, func(sel *widget.Select) bool {
			return len(sel.Options) > 0 && sel.Options[0] == first
		})
	}
	optionsOf := func(items []save.LoadOutItem) *widget.Select {
		return find[*widget.Select](content, func(sel *widget.Select) bool {
			return len(sel.Options) == len(items)+1 && sel.Options[0] == "None" && sel.Options[1] == items[0].Name
		})
	}
	p := soldierSelects{
		class: find[*widget.Select](content, func(sel *widget.Select) bool {
			return slices.Contains(sel.Options, "Assault") && slices.Contains(sel.Options, "Heavy")
		}),
		rank: byOptions(rankNames[0]),
		armor: find[*widget.Select](content, func(sel *widget.Select) bool { // a soldier's, a MEC's or a S.H.I.V.'s
			return len(sel.Options) > 0 && (sel.Options[0] == save.ArmorFor(ew)[0].Name || sel.Options[0] == save.MECArmorItems[0].Name ||
				sel.Options[0] == save.SHIVDecksFor(ew)[0].Name)
		}),
		pistol: optionsOf(save.PistolItems),
		rocket: optionsOf(save.RocketsFor(ew)),
		item1:  optionsOf(save.SmallItemsFor(ew)),
		weapon: find[*widget.Select](content, func(sel *widget.Select) bool {
			return slices.Contains(sel.Options, "Laser Rifle") || slices.Contains(sel.Options, "LMG") ||
				slices.Contains(sel.Options, "Sniper Rifle") || slices.Contains(sel.Options, "Minigun")
		}),
	}
	if p.class == nil || p.rank == nil || p.armor == nil || p.pistol == nil || p.rocket == nil || p.item1 == nil || p.weapon == nil {
		t.Fatalf("soldier page selects not found: %+v", p)
	}
	return p
}

// checkStorage edits the Elerium count on the storage page.
func checkStorage(t *testing.T, e *editor, w fyne.Window, items []save.StorageItem) {
	t.Helper()
	tapButton(t, w, "Storage")
	if e.page != storagePage {
		t.Fatalf("after Storage, page = %v, want storage", e.page)
	}
	elerium := items[slices.IndexFunc(items, func(it save.StorageItem) bool { return it.Name == "Elerium" })].Index()
	e.file.Items[elerium] = 987654 // a value no other item has
	e.show(storagePage)
	capture(t, w, "storage")
	en := find[*widget.Entry](w.Canvas().Content(), func(en *widget.Entry) bool { return en.Text == "987654" })
	if en == nil {
		t.Fatal("Elerium entry not found")
	}
	en.SetText("5000")
	if e.file.Items[elerium] != 5000 {
		t.Fatalf("Elerium = %d, want 5000", e.file.Items[elerium])
	}
	tapButton(t, w, "Back")
	if e.page != homePage {
		t.Fatalf("after storage Back, page = %v, want home", e.page)
	}
}

// checkHangar opens the first interceptor and changes its type.
func checkHangar(t *testing.T, e *editor, w fyne.Window) {
	t.Helper()
	tapButton(t, w, "Hangar")
	if e.page != hangarPage {
		t.Fatalf("after Hangar, page = %v, want hangar", e.page)
	}
	capture(t, w, "hangar")
	list := find[*widget.List](w.Canvas().Content(), func(*widget.List) bool { return true })
	if list == nil {
		t.Fatal("no interceptor list")
	}
	list.Select(0)
	if e.page != shipPage || e.ship != 0 {
		t.Fatalf("after selecting interceptor 0, page = %v ship = %d", e.page, e.ship)
	}
	s := e.file.Ships[0]
	typ := find[*widget.Select](w.Canvas().Content(), func(sel *widget.Select) bool {
		return slices.Equal(sel.Options, save.ShipNames)
	})
	if typ == nil {
		t.Fatal("interceptor type select not found")
	}
	other := "Firestorm"
	if s.TypeName == other {
		other = "Interceptor"
	}
	typ.SetSelected(other)
	hp := strconv.Itoa(s.HP)
	if s.TypeName != other || s.CurrentHP != s.HP ||
		find[*widget.Entry](w.Canvas().Content(), func(en *widget.Entry) bool { return en.Text == hp }) == nil {
		t.Fatalf("after choosing %s: type %q HP %d/%d, or the page was not refreshed", other, s.TypeName, s.CurrentHP, s.HP)
	}
	capture(t, w, "interceptor")
	tapButton(t, w, "Hangar")
	tapButton(t, w, "Back")
}

// checkLabs visits the research and engineering pages.
func checkLabs(t *testing.T, e *editor, w fyne.Window) {
	t.Helper()
	for _, c := range []struct {
		button string
		page   page
		value  *int
	}{
		{"Research", labPage, &e.file.Scientists},
		{"Engineering", workshopPage, &e.file.Engineers},
	} {
		tapButton(t, w, c.button)
		if e.page != c.page {
			t.Fatalf("after %s, page = %v", c.button, e.page)
		}
		en := find[*widget.Entry](w.Canvas().Content(), func(en *widget.Entry) bool { return en.Text == strconv.Itoa(*c.value) })
		if en == nil {
			t.Fatalf("%s: count entry not found", c.button)
		}
		en.SetText("77")
		if *c.value != 77 {
			t.Fatalf("%s: count = %d, want 77", c.button, *c.value)
		}
		tapButton(t, w, "Back")
	}
}

func tapButton(t *testing.T, w fyne.Window, text string) {
	t.Helper()
	b := findButton(w.Canvas().Content(), text)
	if b == nil {
		t.Fatalf("no %q button", text)
	}
	test.Tap(b)
}

func capture(t *testing.T, w fyne.Window, name string) {
	prefix := os.Getenv("OUT")
	if prefix == "" {
		return
	}
	prefix = strings.TrimSuffix(prefix, ".png")
	f, err := os.Create(prefix + "-" + name + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, w.Canvas().Capture())
}

func findButton(o fyne.CanvasObject, text string) *widget.Button {
	return find[*widget.Button](o, func(b *widget.Button) bool { return b.Text == text })
}

// find returns the first object of type T under o that satisfies match.
func find[T fyne.CanvasObject](o fyne.CanvasObject, match func(T) bool) T {
	var zero T
	if v, ok := o.(T); ok && match(v) {
		return v
	}
	var children []fyne.CanvasObject
	switch o := o.(type) {
	case *fyne.Container:
		children = o.Objects
	case fyne.Widget:
		children = test.WidgetRenderer(o).Objects()
	}
	for _, c := range children {
		if v := find(c, match); any(v) != any(zero) {
			return v
		}
	}
	return zero
}
