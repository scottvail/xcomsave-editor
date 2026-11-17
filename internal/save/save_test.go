// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

// The tests use the example saves described in sample_test.go: an Enemy
// Unknown and an Enemy Within sample (each native and JSON), built from real
// saves with known values set. Tests needing a save as the game wrote it use
// the sources.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/scottvail/go-xcomsave"
)

func copyTo(t *testing.T, src, name string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("%s not present", src)
	}
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func load(t *testing.T, path string) *File {
	t.Helper()
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// resave writes f in its own format and loads the result.
func resave(t *testing.T, f *File) *File {
	t.Helper()
	data, err := f.Bytes(f.Format)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// soldierNamed returns the soldier of f with entry name name.
func soldierNamed(f *File, name string) *Soldier {
	return f.Soldiers[slices.IndexFunc(f.Soldiers, func(s *Soldier) bool { return s.Name == name })]
}

// reloadAsMEC makes f.Soldiers[i] a MEC in the save data itself, as the
// game stores one (kClass.eType eSC_Mec), and reloads the save.
func reloadAsMEC(t *testing.T, f *File, i int) (*File, *Soldier) {
	t.Helper()
	name := f.Soldiers[i].Name
	f.Soldiers[i].class.typ.Value = MECClass
	g := resave(t, f)
	return g, soldierNamed(g, name)
}

// ---- File formats ----

func TestDetectFormat(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		want := Native
		if strings.HasSuffix(sm.path, ".json") {
			want = JSON
		}
		if f.Format != want || f.EnemyWithin() != sm.enemyWithin {
			t.Errorf("%s: format %v Enemy Within %v", sm.path, f.Format, f.EnemyWithin())
		}
	}
	if FormatForPath("a/B.JSON") != JSON || FormatForPath("save62") != Native {
		t.Error("FormatForPath")
	}
}

func TestSamplesMatchSources(t *testing.T) {
	// Each sample's native and JSON files hold the same save.
	for _, p := range [][2]string{{euSample, euSampleJSON}, {ewSample, ewSampleJSON}} {
		a, b := loadSample(t, p[0]), loadSample(t, p[1])
		ja, _ := a.Bytes(JSON)
		jb, _ := b.Bytes(JSON)
		if !bytes.Equal(ja, jb) {
			t.Errorf("%s and %s differ", p[0], p[1])
		}
	}
	// The sources are untouched game saves of each game.
	for path, ew := range map[string]bool{euSource: false, ewSource: true} {
		if f := loadSample(t, path); f.EnemyWithin() != ew || f.Format != Native {
			t.Errorf("%s: Enemy Within %v format %v", path, f.EnemyWithin(), f.Format)
		}
	}
}

func TestUnmodifiedRoundTrip(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		orig, _ := os.ReadFile(sm.path)
		got, err := f.Bytes(f.Format)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, orig) {
			t.Errorf("%s: unmodified output differs from input", sm.path)
		}
		if f.Modified() {
			t.Errorf("%s: unedited file reports modifications", sm.path)
		}
	}
	// A save as the game wrote it keeps its contents (the compressed bytes
	// may differ).
	for _, path := range []string{euSource, ewSource} {
		f := loadSample(t, path)
		before, _ := f.Bytes(JSON)
		after, _ := resave(t, f).Bytes(JSON)
		if !bytes.Equal(before, after) {
			t.Errorf("%s: contents changed by an unedited save", path)
		}
	}
}

// ---- Soldiers ----

func TestParseSoldiers(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		want := map[bool]int{false: 31, true: 15}[sm.enemyWithin]
		if len(f.Soldiers) != want {
			t.Fatalf("%s: %d soldiers, want %d", sm.path, len(f.Soldiers), want)
		}
		r := sampleRoles(f)
		for i, s := range r.varied {
			w := sampleSoldier(i, sm.enemyWithin)
			got := s.soldierValues
			got.FirstName, got.LastName, got.NickName, got.ForcePsiGift = "", "", "", false
			if got != w {
				t.Fatalf("%s: %s\n got %+v\nwant %+v", sm.path, s.InstanceName, got, w)
			}
			if s.ClassLabel() != Classes[w.Class].Label() {
				t.Errorf("%s: %s class label %q", sm.path, s.InstanceName, s.ClassLabel())
			}
			for fld := Field(0); fld < numFields; fld++ {
				if !s.Present(fld) {
					t.Errorf("%s: %s missing %s", sm.path, s.InstanceName, fld.PropName())
				}
			}
		}
		if !r.dead.IsDead() {
			t.Errorf("%s: %s should be dead", sm.path, r.dead.InstanceName)
		}
		if sm.enemyWithin != (len(r.mecs) == 1) {
			t.Errorf("%s: %d MECs", sm.path, len(r.mecs))
		}
		// The Enemy Unknown save has two S.H.I.V.s (GOLIATH-1, LIGHTNING-1):
		// soldier entries with rank -1 whose rank, class, psi and load out
		// are not editable.
		if want := map[bool]int{false: 2, true: 0}[sm.enemyWithin]; len(r.shivs) != want {
			t.Errorf("%s: %d S.H.I.V.s, want %d", sm.path, len(r.shivs), want)
		}
		for _, s := range r.shivs {
			if s.Rank != -1 || s.FirstName != "" || s.Present(Rank) || s.Present(Class) || s.Present(Weapon) ||
				s.Present(HasPsiGift) || !s.Present(Health) || !s.Present(LastName) {
				t.Errorf("%s: S.H.I.V. %s rank %d", sm.path, s.LastName, s.Rank)
			}
		}
		for _, m := range r.mecs {
			if m.ClassLabel() != MECLabel || m.Armor != 196 || m.Weapon != mecWeapon {
				t.Errorf("%s: MEC %q armor %d weapon %d", sm.path, m.ClassLabel(), m.Armor, m.Weapon)
			}
		}
	}
}

func edit(f *File) *Soldier {
	r := sampleRoles(f)
	s := r.varied[3] // a Heavy with laser gear
	s.FirstName = `Zoë "Q"`
	s.NickName = "日本"
	s.Rank = 5
	s.XP = 99999
	s.HasPsiGift = !s.HasPsiGift
	s.PsiTested = !s.PsiTested
	s.ForcePsiGift = !s.ForcePsiGift
	s.Class = 1 // Sniper
	s.Health, s.Aim, s.Will = 12, 99, 120
	s.Item2 = SmallItemsFor(f.EnemyWithin())[1].Value
	s.TurnsOut = 5
	r.varied[0].PsiXP = 1
	return s
}

func checkEdited(t *testing.T, g *File, s *Soldier, sample string) {
	t.Helper()
	orig := sampleSoldier(3, g.EnemyWithin())
	gs := soldierNamed(g, s.Name)
	if gs.FirstName != `Zoë "Q"` || gs.NickName != "日本" || gs.Rank != 5 || gs.XP != 99999 ||
		gs.HasPsiGift == orig.HasPsiGift || gs.PsiTested == orig.PsiTested || gs.ForcePsiGift == orig.ForcePsiGift ||
		gs.Class != 1 || gs.Item2 != s.Item2 || gs.TurnsOut != 5 || gs.ints[NumSmallItems].Value != 2 ||
		gs.Rocket != 0 || gs.ints[NumLargeItems].Value != 1 || // no longer Heavy
		gs.Health != 12 || gs.Aim != 99 || gs.Will != 120 || gs.Defense != orig.Defense ||
		gs.class.name.Value.Str != "Sniper" || gs.class.typ.Value != "eSC_Sniper" || gs.class.weapon.Value != "eWP_Sniper" {
		t.Fatalf("edits not persisted: %+v", gs.soldierValues)
	}
	if gs.strs[FirstName].Value.IsWide || !gs.strs[NickName].Value.IsWide {
		t.Fatal("Latin-1 text should stay narrow; non-Latin-1 text should become wide")
	}
	r, before := sampleRoles(g), sampleRoles(loadSample(t, sample))
	if r.varied[0].PsiXP != 1 || r.varied[1].soldierValues != before.varied[1].soldierValues {
		t.Fatal("other soldiers changed unexpectedly")
	}
	if g.Modified() {
		t.Fatal("reloaded file reports modifications")
	}
}

func TestEditNativeSave(t *testing.T) {
	for _, path := range []string{euSample, ewSample} {
		p := copyTo(t, path, "save")
		f := load(t, p)
		s := edit(f)
		if !f.Modified() {
			t.Fatal("expected Modified")
		}
		if err := f.SaveAs(p, true); err != nil {
			t.Fatal(err)
		}
		if f.Format != Native || f.Modified() {
			t.Fatal("file state not refreshed after save")
		}
		want, _ := os.ReadFile(path)
		if bak, _ := os.ReadFile(p + ".bak"); !bytes.Equal(bak, want) {
			t.Fatal("backup does not match original")
		}
		checkEdited(t, load(t, p), s, path)
	}
}

func TestEditJSON(t *testing.T) {
	path := copyTo(t, euSampleJSON, "save.json")
	f := load(t, path)
	s := edit(f)
	if err := f.SaveAs(path, true); err != nil {
		t.Fatal(err)
	}
	checkEdited(t, load(t, path), s, euSampleJSON)

	// Only the edited lines differ, with the file's line endings kept.
	orig, _ := os.ReadFile(euSampleJSON)
	data, _ := os.ReadFile(path)
	crlf := bytes.Contains(orig, []byte("\r\n"))
	a, b := strings.Split(string(orig), "\n"), strings.Split(string(data), "\n")
	if len(a) != len(b) {
		t.Fatalf("line count changed %d -> %d", len(a), len(b))
	}
	var changed []string
	for i := range a {
		if a[i] != b[i] {
			changed = append(changed, strings.TrimSpace(b[i]))
			if crlf != strings.HasSuffix(b[i], "\r") {
				t.Fatal("line ending changed")
			}
		}
	}
	// first and nick names, rank, xp, 3 psi flags, 3 class props and
	// m_kChar.eClass, aStats, arrLargeItems + iNumLargeItems (rocket removed
	// with the Heavy class), arrSmallItems, m_iTurnsOut, the other soldier's
	// psi xp
	if len(changed) != 17 {
		t.Fatalf("expected 17 changed lines, got %d:\n%s", len(changed), strings.Join(changed, "\n"))
	}
}

// TestConvert checks Save As converts between formats by file extension.
func TestConvert(t *testing.T) {
	for _, path := range []string{euSample, ewSample} {
		dir := t.TempDir()
		f := load(t, copyTo(t, path, "save"))
		js := filepath.Join(dir, "out.json")
		if err := f.SaveAs(js, false); err != nil {
			t.Fatal(err)
		}
		if f.Format != JSON {
			t.Fatal("expected JSON after saving as .json")
		}
		bin := filepath.Join(dir, "out")
		if err := f.SaveAs(bin, false); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(bin)
		want, _ := os.ReadFile(path)
		if !bytes.Equal(got, want) {
			t.Errorf("%s: native -> JSON -> native did not reproduce the save", path)
		}
	}
}

func TestValidate(t *testing.T) {
	f := loadSample(t, euSample)
	f.Soldiers[0].Rank = 8
	if f.Validate() == nil {
		t.Fatal("expected rank validation error")
	}
	if err := f.SaveAs(filepath.Join(t.TempDir(), "x"), false); err == nil {
		t.Fatal("expected SaveAs to refuse invalid data")
	}
}

// TestBackupAfterTruncate simulates a "save as" dialog that truncates the
// chosen file before the save: the backup must still hold the original.
func TestBackupAfterTruncate(t *testing.T) {
	path := copyTo(t, euSample, "save")
	f := load(t, path)
	f.Soldiers[0].XP = 1
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path, true); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.ReadFile(euSample)
	if bak, _ := os.ReadFile(path + ".bak"); !bytes.Equal(bak, orig) {
		t.Fatal("backup should contain the file as originally loaded")
	}
	if load(t, path).Soldiers[0].XP != 1 {
		t.Fatal("edit not saved")
	}
}

func TestLoadOut(t *testing.T) {
	for _, path := range []string{euSample, ewSample} {
		f := loadSample(t, path)
		r := sampleRoles(f)
		// Varied soldiers 1-4 are Assault, Support, Heavy and Sniper.
		support, heavy, sniper := r.varied[2], r.varied[3], r.varied[4]
		if !heavy.IsHeavy() || sniper.IsHeavy() || heavy.Rocket == 0 || heavy.ints[NumLargeItems].Value != 2 {
			t.Fatalf("%s: Heavy %v rocket %d", path, heavy.IsHeavy(), heavy.Rocket)
		}
		// A rocket on a non-Heavy is not written.
		sniper.Rocket = 19
		sniper.Weapon = 17
		// Making a Heavy writes the rocket and two large items.
		support.Class = 4
		support.Rocket = 19
		g := resave(t, f)
		gs, gh := soldierNamed(g, sniper.Name), soldierNamed(g, support.Name)
		if gs.Weapon != 17 || gs.Rocket != 0 || gs.ints[NumLargeItems].Value != 1 {
			t.Fatalf("%s: non-Heavy: weapon %d rocket %d count %d", path, gs.Weapon, gs.Rocket, gs.ints[NumLargeItems].Value)
		}
		if !gh.IsHeavy() || gh.Rocket != 19 || gh.ints[NumLargeItems].Value != 2 || gh.Pistol != 0 {
			t.Fatalf("%s: Heavy: rocket %d count %d pistol %d", path, gh.Rocket, gh.ints[NumLargeItems].Value, gh.Pistol)
		}
		// Untouched soldiers keep their load out, including two small items.
		u := soldierNamed(g, r.varied[0].Name)
		if u.Item2 == 0 || u.ints[NumSmallItems].Value != 2 || u.soldierValues != r.varied[0].soldierValues {
			t.Fatalf("%s: untouched soldier's load out changed: %+v", path, u.soldierValues)
		}
	}
}

func TestPromotedRookieNeedsClass(t *testing.T) {
	f := loadSample(t, euSampleJSON)
	s := sampleRoles(f).varied[0] // rank 0, class None
	if s.Rank != 0 || s.Class != 0 {
		t.Fatalf("expected a classless rookie, got rank %d class %d", s.Rank, s.Class)
	}
	s.Rank = 3
	if err := f.Validate(); err == nil {
		t.Fatal("a rookie promoted without a class should not be savable")
	}
	if _, err := f.Bytes(f.Format); err == nil {
		t.Fatal("Bytes should refuse a promoted rookie without a class")
	}
	s.Class = classIndex("eSC_Sniper")
	if err := f.Validate(); err != nil {
		t.Fatalf("with a class: %v", err)
	}
	// Changing other things about a classless rookie is still fine.
	s.Rank, s.Class, s.Aim = 0, 0, s.Aim+5
	if err := f.Validate(); err != nil {
		t.Fatalf("unpromoted rookie: %v", err)
	}
}

func TestClassWeapons(t *testing.T) {
	const none, sniper, assault, support, heavy = 0, 1, 2, 3, 4
	names := func(class int) (out []string) {
		for _, it := range WeaponsFor(class, false) {
			out = append(out, it.Name)
		}
		return out
	}
	for class, want := range map[int]string{
		none:         "Assault Rifle,Laser Rifle,Light Plasma Rifle,Plasma Rifle",
		support:      "Assault Rifle,Laser Rifle,Light Plasma Rifle,Plasma Rifle",
		assault:      "Assault Rifle,Shotgun,Laser Rifle,Scatter Laser,Light Plasma Rifle,Plasma Rifle,Alloy Cannon",
		sniper:       "Sniper Rifle,Laser Sniper Rifle,Plasma Sniper Rifle",
		heavy:        "LMG,Heavy Laser,Heavy Plasma",
		UnknownClass: "Assault Rifle,Shotgun,LMG,Sniper Rifle,Laser Rifle,Scatter Laser,Heavy Laser,Laser Sniper Rifle,Light Plasma Rifle,Plasma Rifle,Alloy Cannon,Heavy Plasma,Plasma Sniper Rifle",
	} {
		if got := strings.Join(names(class), ","); got != want {
			t.Errorf("WeaponsFor(%d) = %s, want %s", class, got, want)
		}
	}
	for _, c := range []struct{ class, weapon, want int }{
		{heavy, 18, 17},        // plasma sniper -> heavy plasma
		{sniper, 11, 12},       // heavy laser -> laser sniper
		{support, 4, 3},        // shotgun -> assault rifle
		{assault, 16, 16},      // alloy cannon is an assault weapon
		{support, 16, 15},      // alloy cannon -> plasma rifle
		{heavy, 88, 88},        // unknown weapon kept
		{UnknownClass, 18, 18}, // unknown class keeps anything
		{heavy, 215, 11},       // EXALT laser rifle -> heavy laser
		{support, 216, 9},      // EXALT laser sniper -> laser rifle
		{assault, 215, 215},    // EXALT laser rifle is an assault weapon
		{heavy, 212, 5},        // EXALT assault rifle -> LMG
		{sniper, 214, 6},       // EXALT LMG -> sniper rifle
		{support, 212, 212},    // EXALT assault rifle is a support weapon
	} {
		if got := ClassWeapon(c.class, c.weapon); got != c.want {
			t.Errorf("ClassWeapon(%d, %d) = %d, want %d", c.class, c.weapon, got, c.want)
		}
	}
}

func TestEnemyWithinItems(t *testing.T) {
	names := func(items []LoadOutItem) string {
		var out []string
		for _, it := range items {
			out = append(out, it.Name)
		}
		return strings.Join(out, ",")
	}
	if got := names(WeaponsFor(4, true)); got != "LMG,Heavy Laser,Heavy Plasma,EXALT LMG,EXALT Heavy Laser" {
		t.Errorf("EW Heavy weapons = %s", got)
	}
	if got := names(WeaponsFor(1, true)); got != "Sniper Rifle,Laser Sniper Rifle,Plasma Sniper Rifle,EXALT Sniper Rifle,EXALT Laser Sniper Rifle" {
		t.Errorf("EW Sniper weapons = %s", got)
	}
	if got := names(WeaponsFor(3, true)); !strings.Contains(got, ",EXALT Assault Rifle,EXALT Laser Assault Rifle") {
		t.Errorf("EW Support weapons = %s", got)
	}
	for class := range Classes {
		if got := names(WeaponsFor(class, false)); strings.Contains(got, "EXALT") {
			t.Errorf("EU %s weapons = %s", Classes[class].Name, got)
		}
	}
	if got := names(RocketsFor(true)); got != "Rocket Launcher,Blaster Launcher,EXALT Rocket Launcher" {
		t.Errorf("EW rockets = %s", got)
	}
	if got := names(RocketsFor(false)); got != "Rocket Launcher,Blaster Launcher" {
		t.Errorf("EU rockets = %s", got)
	}
	want := "Medkit,Mind Shield,Chitin Plating,Arc Thrower,S.C.O.P.E.,Nano-fiber Vest,Respirator Implant," +
		"Reaper Rounds,Frag Grenade,Flashbang Grenade,Alien Grenade,Ghost Grenade,Gas Grenade,Needle Grenade,Mimic Beacon"
	if got := names(SmallItemsFor(true)); got != want {
		t.Errorf("EW small items = %s", got)
	}
	want = "Medikit,Combat Stims,Mind Shield,Chitin Plating,Arc Thrower,S.C.O.P.E.,Nano-Fiber Vest,Frag Grenade,Alien Grenade,Battle Scanner"
	if got := names(SmallItemsFor(false)); got != want {
		t.Errorf("EU small items = %s", got)
	}
	if ArmorFor(false)[0].Value != 57 || ArmorFor(true)[0].Value != 59 {
		t.Error("Body Armor should be 57 in Enemy Unknown and 59 in Enemy Within")
	}
	// The storage items agree with the load out numbers.
	for _, c := range []struct {
		storage []StorageItem
		loadOut []LoadOutItem
	}{
		{StorageItems, slices.Concat(EUArmorItems, EUSmallItems)},
		{EWStorageItems, slices.Concat(EWArmorItems, EWSmallItems, EWWeaponItems, EWRocketItems, PistolItems, WeaponItems, RocketItems)},
	} {
		for _, it := range c.storage {
			for _, lo := range c.loadOut {
				if lo.Value == it.Index() && !strings.EqualFold(it.Name[:3], lo.Name[:3]) {
					t.Errorf("item %d is %q in storage but %q in the load out", lo.Value, it.Name, lo.Name)
				}
			}
		}
	}
}

// ---- Campaign values ----

func TestCampaignValues(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		if !f.HasCash() || f.Cash != sampleCash || !f.HasDifficulty() || f.Difficulty != sampleDifficulty ||
			!f.HasLowestDifficulty() || f.LowestDifficulty != sampleLowest ||
			!f.HasScientists() || f.Scientists != sampleScientists || !f.HasEngineers() || f.Engineers != sampleEngineers {
			t.Fatalf("%s: cash %d difficulty %d/%d scientists %d engineers %d", sm.path,
				f.Cash, f.Difficulty, f.LowestDifficulty, f.Scientists, f.Engineers)
		}
		f.Cash, f.Difficulty, f.LowestDifficulty, f.Scientists, f.Engineers = 1000000, 3, 0, 250, 300
		if !f.Modified() {
			t.Fatal("campaign edits not reported as modifications")
		}
		g := resave(t, f)
		if g.Cash != 1000000 || g.Difficulty != 3 || g.LowestDifficulty != 0 || g.Scientists != 250 || g.Engineers != 300 {
			t.Fatalf("%s: campaign edits not written: cash %d difficulty %d/%d scientists %d engineers %d",
				sm.path, g.Cash, g.Difficulty, g.LowestDifficulty, g.Scientists, g.Engineers)
		}
		for _, bad := range []func(*File){
			func(h *File) { h.Cash = -1 }, func(h *File) { h.Difficulty = 4 }, func(h *File) { h.LowestDifficulty = -1 },
			func(h *File) { h.Scientists = -1 }, func(h *File) { h.Engineers = -1 },
		} {
			h := loadSample(t, sm.path)
			bad(h)
			if h.Validate() == nil {
				t.Errorf("%s: invalid campaign value accepted", sm.path)
			}
		}
	}
}

func TestIronman(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		if f.ironmanProp == nil || f.ironmanProp.Value != f.Ironman {
			t.Fatalf("%s: m_bIronMan missing or not matching the header", sm.path)
		}
		f.Ironman = !f.Ironman
		if !f.Modified() {
			t.Fatal("Ironman change not reported as a modification")
		}
		g := resave(t, f)
		if g.Ironman != f.Ironman || g.save.Header.Ironman != f.Ironman || g.ironmanProp.Value != f.Ironman {
			t.Fatalf("%s: Ironman not written to both header and XGStrategy", sm.path)
		}
	}
}

func TestContinent(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		labels := f.ContinentLabels()
		if len(labels) != 5 || labels[4] != "Africa" || labels[0] != "North America" {
			t.Errorf("%s: continent labels %q", sm.path, labels)
		}
		if !f.HasContinent() || f.Continent < 0 || f.Continent > 4 || f.ContinentName() != labels[f.Continent] {
			t.Errorf("%s: HQ continent %v %d %q", sm.path, f.HasContinent(), f.Continent, f.ContinentName())
		}
	}
}

// ---- Storage ----

func TestStorage(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		items, n := StorageItems, 195
		if sm.enemyWithin {
			items, n = EWStorageItems, EWStorageLen
		}
		if !f.HasStorage() || len(f.Items) != n || &f.StorageItems()[0] != &items[0] {
			t.Fatalf("%s: storage %v, %d items", sm.path, f.HasStorage(), len(f.Items))
		}
		byName := map[string]StorageItem{}
		for _, it := range items {
			byName[it.Name] = it
			if !f.HasItem(it) {
				t.Errorf("%s: item %s (element %d) missing", sm.path, it.Name, it.Element)
			}
		}
		for name, want := range sampleStorage[sm.enemyWithin] {
			if got := f.Items[byName[name].Index()]; got != want {
				t.Errorf("%s: %s = %d, want %d", sm.path, name, got, want)
			}
		}
		f.Items[byName["Elerium"].Index()] = 9999
		if !f.Modified() {
			t.Fatal("storage edit not reported as a modification")
		}
		g := resave(t, f)
		if g.Items[byName["Elerium"].Index()] != 9999 || g.Items[byName["Alien Alloys"].Index()] != sampleStorage[sm.enemyWithin]["Alien Alloys"] {
			t.Fatalf("%s: storage edit not written correctly", sm.path)
		}
		f.Items[0] = -1
		if f.Validate() == nil {
			t.Fatal("negative storage count should be invalid")
		}
	}
}

func TestStorageTablesMatchGameSaves(t *testing.T) {
	// The item tables against saves as the game wrote them: the unlimited
	// starting gear (Body Armor, Frag Grenade) is held as 100, and in Enemy
	// Within building the MEC suit spent 40 Meld, leaving 25, and the suit
	// (position 196) is owned and in use.
	eu := loadSample(t, euSource)
	if eu.Items[ArmorFor(false)[0].Value] != 100 || eu.Items[85] != 100 {
		t.Errorf("Enemy Unknown: Body Armor %d Frag Grenade %d, want 100", eu.Items[57], eu.Items[85])
	}
	ew := loadSample(t, ewSource)
	meld := EWStorageItems[slices.IndexFunc(EWStorageItems, func(it StorageItem) bool { return it.Name == "Meld" })]
	if ew.Items[59] != 100 || ew.Items[81] != 100 || ew.Items[meld.Index()] != 25 {
		t.Errorf("Enemy Within: Body Armor %d Frag Grenade %d Meld %d", ew.Items[59], ew.Items[81], ew.Items[meld.Index()])
	}
	if ew.Items[196] != 1 || ew.claimsProp.Elements[196] != 1 {
		t.Errorf("Enemy Within: MEC suit owned %d in use %d", ew.Items[196], ew.claimsProp.Elements[196])
	}
}

func TestEWStorageItems(t *testing.T) {
	names := map[string]bool{}
	for _, it := range EWStorageItems {
		if names[it.Name] {
			t.Errorf("duplicate item %s", it.Name)
		}
		names[it.Name] = true
		if it.Index() < 0 || it.Index() >= EWStorageLen {
			t.Errorf("item %s (element %d) out of range", it.Name, it.Element)
		}
	}
}

func TestEnemyWithinStorageLength(t *testing.T) {
	// An Enemy Within save whose item list isn't EWStorageLen long (here the
	// Enemy Unknown sample marked as Enemy Within): storage is not editable.
	js, err := os.ReadFile(euSampleJSON)
	if err != nil {
		t.Skip("sample not present")
	}
	js = bytes.Replace(js, []byte(`"version": 15,`), []byte(`"version": 16,`), 1)
	g, err := Parse(js)
	if err != nil {
		t.Fatal(err)
	}
	if !g.EnemyWithin() || g.HasStorage() || g.Items != nil {
		t.Fatalf("Enemy Within: EnemyWithin %v HasStorage %v", g.EnemyWithin(), g.HasStorage())
	}
}

// ---- Hangar ----

func TestHangar(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		want := map[bool]int{false: 10, true: 5}[sm.enemyWithin]
		if len(f.Ships) != want {
			t.Fatalf("%s: %d interceptors, want %d", sm.path, len(f.Ships), want)
		}
		defaults := shipDefaults(sm.enemyWithin)
		graphics := map[*xcomsave.ByteProperty]bool{}
		for i, s := range f.Ships {
			v := sampleShip(i)
			d := defaults[slices.Index(ShipNames, v.typeName)]
			hp := d.HP
			if v.damaged {
				hp /= 2
			}
			w, _ := shipWeapon(v.weapon)
			if s.TypeName != v.typeName || s.Type != ShipTypes[slices.Index(ShipNames, v.typeName)] ||
				s.Graphic != ShipGraphics[slices.Index(ShipNames, v.typeName)] || s.Weapon != v.weapon ||
				s.HoursDown != v.hoursDown || s.HomeContinent != v.homeContinent || s.HP != d.HP || s.CurrentHP != hp ||
				!slices.Equal(s.arrWeapons.Elements, append([]int32{int32(w.ArrNum)}, make([]int32, w.Slots-1)...)) {
				t.Errorf("%s: %s %+v arrWeapons %v, want %+v", sm.path, s.InstanceName, s.shipValues, s.arrWeapons.Elements, v)
			}
			graphics[s.graphic] = true
			if i > 0 && shipNum(f.Ships[i-1].InstanceName) > shipNum(s.InstanceName) {
				t.Errorf("%s: interceptors not in instance order", sm.path)
			}
		}
		if len(graphics) != len(f.Ships) {
			t.Errorf("%s: interceptors share ship entities", sm.path)
		}
	}
}

func TestEditInterceptor(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		s := f.Ships[1] // a Firestorm
		s.Callsign, s.Weapon = "RAVEN-9", "eItem_IntWeap_VI"
		s.SetName("Interceptor")
		d := shipDefaults(sm.enemyWithin)[0]
		if s.TypeName != "Interceptor" || s.Type != "eShip_Interceptor" || s.Graphic != "eEntityGraphic_Interceptor" ||
			s.Speed != d.Speed || s.HP != d.HP || s.CurrentHP != d.HP {
			t.Fatalf("%s: SetName gave %+v", sm.path, s.shipValues)
		}
		s.Speed, s.EngagementSpeed, s.HP, s.AP, s.Armor, s.CurrentHP = 4000, 20, 9000, 30, 40, 8000
		s.HoursDown, s.HomeContinent = 12, 3
		if !f.Modified() {
			t.Fatal("interceptor edit not reported as a modification")
		}
		g := resave(t, f)
		if got := g.Ships[1]; got.shipValues != s.shipValues || !slices.Equal(got.arrWeapons.Elements, []int32{7}) {
			t.Fatalf("%s: wrote %+v %v, want %+v", sm.path, got.shipValues, got.arrWeapons.Elements, s.shipValues)
		}
		if g.Ships[0].shipValues != f.Ships[0].orig {
			t.Errorf("%s: untouched interceptor changed", sm.path)
		}
		for _, bad := range []func(*Ship){
			func(s *Ship) { s.TypeName = "Skyranger" }, func(s *Ship) { s.HomeContinent = 5 },
			func(s *Ship) { s.CurrentHP = -1 }, func(s *Ship) { s.Weapon = "eItem_IntWeap_IX" },
		} {
			h := loadSample(t, sm.path)
			bad(h.Ships[0])
			if h.Validate() == nil {
				t.Errorf("%s: invalid interceptor value accepted", sm.path)
			}
		}
	}
}

func TestShipTypeChange(t *testing.T) {
	// Each game's own stats: an Enemy Unknown Firestorm has engagement speed
	// 20 and AP 34, an Enemy Within one 15 and 16; the weapon slots follow
	// the weapon (I is [1], II [3, 0], V [6]).
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		s := f.Ships[0] // an Interceptor
		s.SetName("Firestorm")
		s.Weapon = "eItem_IntWeap_V"
		g := resave(t, f)
		got := g.Ships[0]
		eng, ap := 20, 34
		if sm.enemyWithin {
			eng, ap = 15, 16
		}
		if got.TypeName != "Firestorm" || got.EngagementSpeed != eng || got.AP != ap || got.HP != 6500 || got.CurrentHP != 6500 ||
			!slices.Equal(got.arrWeapons.Elements, []int32{6}) {
			t.Errorf("%s: Firestorm %+v %v", sm.path, got.shipValues, got.arrWeapons.Elements)
		}
		// Re-choosing the same type keeps edited stats.
		got.Speed = 4200
		got.SetName("Firestorm")
		if got.Speed != 4200 {
			t.Error("re-choosing the same type reset the stats")
		}
	}
	f := loadSample(t, ewSample)
	for _, c := range []struct {
		typ, weapon string
		want        []int32
	}{
		{"Interceptor", "eItem_IntWeap_I", []int32{1}},
		{"Interceptor", "eItem_IntWeap_II", []int32{3, 0}},
		{"Firestorm", "eItem_IntWeap_II", []int32{3, 0}},
	} {
		f.Ships[0].SetName(c.typ)
		f.Ships[0].Weapon = c.weapon
		f = resave(t, f)
		if a := f.Ships[0].arrWeapons.Elements; !slices.Equal(a, c.want) {
			t.Errorf("%s with %s: arrWeapons %v, want %v", c.typ, c.weapon, a, c.want)
		}
	}
}

func TestRepairAll(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		want := 0
		for i := range f.Ships {
			if sampleShip(i).hoursDown > 0 {
				want++
			}
		}
		if want == 0 || f.ShipsDown() != want {
			t.Fatalf("%s: %d interceptors down, want %d", sm.path, f.ShipsDown(), want)
		}
		f.RepairAll()
		if g := resave(t, f); g.ShipsDown() != 0 {
			t.Errorf("%s: %d interceptors down after Repair All", sm.path, g.ShipsDown())
		}
	}
}

func TestFulfillOrders(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		if len(f.Orders) != len(sampleOrders) || f.PendingOrders() != len(sampleOrders) || f.Modified() {
			t.Fatalf("%s: %d orders, %d pending", sm.path, len(f.Orders), f.PendingOrders())
		}
		for i, o := range f.Orders {
			w := sampleOrders[i]
			if o.NumInterceptors != w.NumInterceptors || o.DestinationContinent != w.DestinationContinent || o.Hours != w.Hours {
				t.Errorf("%s: order %d = %+v, want %+v", sm.path, i, *o, w)
			}
		}
		f.FulfillOrders()
		if !f.Modified() || f.PendingOrders() != 0 {
			t.Fatalf("%s: fulfil: modified %v pending %d", sm.path, f.Modified(), f.PendingOrders())
		}
		g := resave(t, f)
		if len(g.Orders) != len(sampleOrders) || g.PendingOrders() != 0 || g.Orders[0].DestinationContinent != sampleOrders[0].DestinationContinent {
			t.Fatalf("%s: after saving: %d orders, %d pending", sm.path, len(g.Orders), g.PendingOrders())
		}
	}
}

// orderSaves are saves with interceptor orders (the samples) and without
// them (the sources, whose empty order arrays have no element type).
var orderSaves = []string{euSample, euSampleJSON, ewSample, ewSampleJSON, "../../example/eu-source", "../../example/ew-source"}

func TestOrderInterceptors(t *testing.T) {
	for _, path := range orderSaves {
		f := loadSample(t, path)
		if !f.CanOrderInterceptors() {
			t.Fatalf("%s: can't order interceptors", path)
		}
		before := len(f.Orders)
		// The continent with the most free bays, and one more than fit.
		c := 0
		for i := range f.ContinentLabels() {
			if f.FreeHangarBays(i) > f.FreeHangarBays(c) {
				c = i
			}
		}
		free := f.FreeHangarBays(c)
		if free == 0 {
			t.Fatalf("%s: no free hangar bays", path)
		}
		if err := f.OrderInterceptors(c, free+1); err == nil {
			t.Errorf("%s: ordered %d interceptors with %d free bays", path, free+1, free)
		}
		if err := f.OrderInterceptors(c, free); err != nil {
			t.Fatal(err)
		}
		if f.FreeHangarBays(c) != 0 || f.OrderInterceptors(c, 1) == nil || !f.Modified() {
			t.Errorf("%s: %d bays free after filling", path, f.FreeHangarBays(c))
		}
		if _, err := f.Bytes(f.Format); err != nil { // applying twice must not duplicate
			t.Fatal(err)
		}
		g := resave(t, f)
		if len(g.Orders) != before+1 {
			t.Fatalf("%s: %d orders after saving, want %d", path, len(g.Orders), before+1)
		}
		o := g.Orders[before]
		if o.NumInterceptors != free || o.DestinationContinent != c || o.Hours != 0 || g.FreeHangarBays(c) != 0 {
			t.Errorf("%s: new order %+v, want %d to continent %d", path, *o, free, c)
		}
		if g.Modified() {
			t.Errorf("%s: modified after reload", path)
		}
	}
}

func TestHireSoldiers(t *testing.T) {
	for _, path := range orderSaves {
		f := loadSample(t, path)
		if want := map[bool]int{false: 99, true: 70}[f.EnemyWithin()]; f.MaxSoldiers() != want {
			t.Errorf("%s: barracks holds %d, want %d", path, f.MaxSoldiers(), want)
		}
		if !f.CanHireSoldiers() {
			t.Fatalf("%s: can't hire soldiers", path)
		}
		n := f.BarracksCount()
		if n == 0 || n > f.MaxSoldiers() {
			t.Fatalf("%s: %d soldiers in the barracks", path, n)
		}
		free := f.FreeBarracks()
		if free == 0 {
			t.Logf("%s: barracks full", path)
			continue
		}
		if err := f.HireSoldiers(free + 1); err == nil {
			t.Errorf("%s: hired %d soldiers with room for %d", path, free+1, free)
		}
		if err := f.HireSoldiers(free); err != nil {
			t.Fatal(err)
		}
		if f.FreeBarracks() != 0 || f.HireSoldiers(1) == nil || !f.Modified() {
			t.Errorf("%s: room for %d after filling", path, f.FreeBarracks())
		}
		if _, err := f.Bytes(f.Format); err != nil {
			t.Fatal(err)
		}
		g := resave(t, f)
		if g.BarracksCount() != g.MaxSoldiers() || g.FreeBarracks() != 0 || g.Modified() {
			t.Errorf("%s: %d soldiers after saving, want %d", path, g.BarracksCount(), g.MaxSoldiers())
		}
		p, ok := xcomsave.Find(checkpoint(g, HeadquartersClass).Properties, "m_arrHiringOrders").(*xcomsave.StructArrayProperty)
		if !ok || len(p.Structs) == 0 {
			t.Fatalf("%s: no hiring order", path)
		}
		st := p.Structs[len(p.Structs)-1]
		if intField(st, "iNumStaff") != free || intField(st, "iStaffType") != staffSoldier || intField(st, "iHours") != 0 {
			t.Errorf("%s: hiring order %v", path, st)
		}
	}
}

func TestRushHires(t *testing.T) {
	for _, path := range []string{"../../example/eu-source", "../../example/ew-source"} {
		f := loadSample(t, path)
		if f.PendingHires() != 0 {
			t.Fatalf("%s: %d soldiers on their way", path, f.PendingHires())
		}
		// A soldier hired in game, as in the user's save: due in 72 hours.
		setProp(&checkpoint(f, HeadquartersClass).Properties, &xcomsave.StructArrayProperty{Name: "m_arrHiringOrders", Structs: [][]xcomsave.Property{
			{intProp("iNumStaff", 2), intProp("iStaffType", staffSoldier), intProp("iHours", 72)},
		}})
		f = resave(t, f)
		if f.PendingHires() != 2 || f.Modified() {
			t.Fatalf("%s: %d soldiers pending, want 2", path, f.PendingHires())
		}
		f.RushHires()
		if f.PendingHires() != 0 || !f.Modified() {
			t.Fatalf("%s: rush: %d pending, modified %v", path, f.PendingHires(), f.Modified())
		}
		if _, err := f.Bytes(f.Format); err != nil { // applying twice is harmless
			t.Fatal(err)
		}
		g := resave(t, f)
		p := xcomsave.Find(checkpoint(g, HeadquartersClass).Properties, "m_arrHiringOrders").(*xcomsave.StructArrayProperty)
		if g.PendingHires() != 0 || g.Modified() || len(p.Structs) != 1 || intField(p.Structs[0], "iNumStaff") != 2 || intField(p.Structs[0], "iHours") != 0 {
			t.Errorf("%s: after saving: %d pending, orders %v", path, g.PendingHires(), p.Structs)
		}
	}
}

// ---- Engineering and research ----

func TestProjects(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		if f.PendingConstruction() != 2 || maxHours(f.construction) != sampleConstructionHours {
			t.Errorf("%s: construction %d pending, most hours %d", sm.path, f.PendingConstruction(), maxHours(f.construction))
		}
		if f.PendingItemProjects() != 1 || maxHours(f.itemProjects) != sampleItemHours {
			t.Errorf("%s: item projects %d pending", sm.path, f.PendingItemProjects())
		}
		if f.ResearchHoursLeft() != sampleResearchHours {
			t.Errorf("%s: research %d hours left", sm.path, f.ResearchHoursLeft())
		}
		for i, h := range sampleSatelliteHours {
			if f.satellites[i].hours != h {
				t.Errorf("%s: satellite %d travel time %d, want %d", sm.path, i, f.satellites[i].hours, h)
			}
		}
		if f.PendingSatellites() < len(sampleSatelliteHours) || f.Modified() {
			t.Errorf("%s: %d satellites pending", sm.path, f.PendingSatellites())
		}

		f.FinishConstruction()
		f.CompleteProjects()
		f.FinishResearch()
		f.DeploySatellites()
		if !f.Modified() {
			t.Fatal("finishing projects not reported as a modification")
		}
		g := resave(t, f)
		if g.PendingConstruction() != 0 || g.PendingItemProjects() != 0 || g.ResearchHoursLeft() != 0 || g.PendingSatellites() != 0 {
			t.Errorf("%s: after finishing: construction %d items %d research %d satellites %d", sm.path,
				g.PendingConstruction(), g.PendingItemProjects(), g.ResearchHoursLeft(), g.PendingSatellites())
		}
		if len(g.construction) != 2 || len(g.itemProjects) != 1 || len(g.satellites) != len(f.satellites) {
			t.Errorf("%s: projects lost: %d construction, %d items, %d satellites", sm.path, len(g.construction), len(g.itemProjects), len(g.satellites))
		}
	}
}

// ---- Enemy Within: surgery, MECs ----

func TestSurgery(t *testing.T) {
	for _, path := range ewSamples {
		f := loadSample(t, path)
		r := sampleRoles(f)
		for _, s := range f.Soldiers {
			aug, gene := 0, 0
			switch s {
			case r.augment:
				aug = sampleAugmentHours
			case r.geneMod:
				gene = sampleGeneModHours
			}
			if s.AugmentHoursLeft() != aug || s.GeneModHoursLeft() != gene {
				t.Errorf("%s: %s augment %d gene mod %d, want %d %d", path, s.InstanceName, s.AugmentHoursLeft(), s.GeneModHoursLeft(), aug, gene)
			}
		}
		if r.augment.Status != "eStatus_Augmenting" || r.augment.TurnsOut != sampleAugmentHours ||
			r.geneMod.Status != "eStatus_GeneMod" || r.geneMod.TurnsOut != sampleGeneModHours {
			t.Errorf("%s: augmenting %s/%d, gene modding %s/%d", path, r.augment.Status, r.augment.TurnsOut, r.geneMod.Status, r.geneMod.TurnsOut)
		}
		// Heal All: m_iTurnsOut and the lab hours.
		for _, s := range f.Soldiers {
			s.TurnsOut = 0
			s.FinishSurgery()
		}
		g := resave(t, f)
		if maxHours(g.surgeries) != 0 || len(g.surgeries) != 2 {
			t.Errorf("%s: after healing: %d lab timers, most hours %d", path, len(g.surgeries), maxHours(g.surgeries))
		}
		for _, s := range g.Soldiers {
			if s.TurnsOut != 0 {
				t.Errorf("%s: %s TurnsOut %d after healing", path, s.InstanceName, s.TurnsOut)
			}
		}
	}
}

func TestMECWeapon(t *testing.T) {
	if got := WeaponsFor(MECIndex, true); len(got) != 3 || got[0].Value != mecWeapon || got[2].Name != "Particle Cannon" {
		t.Fatalf("MEC weapons %v", got)
	}
	for _, path := range ewSamples {
		f := loadSample(t, path)
		m := sampleRoles(f).mecs[0]
		armor := m.Armor
		m.Weapon = 30 // Particle Cannon
		g := resave(t, f)
		gm := soldierNamed(g, m.Name)
		if gm.Weapon != 30 || gm.Armor != armor || gm.Rocket != 0 || gm.Item1 != 0 || gm.Item2 != 0 ||
			gm.mec.numLarge.Value != 1 || gm.mec.numSmall.Value != 0 || !gm.IsMEC() {
			t.Errorf("%s: MEC after weapon change %+v, counts %d/%d", path, gm.soldierValues, gm.mec.numLarge.Value, gm.mec.numSmall.Value)
		}
		// Leaving the MECs with no usable backed-up load out: the class's
		// conventional weapon, not the MEC's.
		for _, p := range []*xcomsave.IntProperty{gm.mec.backup.armor, element(gm.mec.backup.large, 0)} {
			if p != nil {
				p.Value = 0
			}
		}
		gm.SetClass(classIndex("eSC_Sniper"))
		if isMECWeapon(gm.Weapon) || !slices.ContainsFunc(WeaponsFor(gm.Class, true), func(it LoadOutItem) bool { return it.Value == gm.Weapon }) {
			t.Errorf("%s: former MEC weapon %d", path, gm.Weapon)
		}
	}
}

func TestMECSuits(t *testing.T) {
	const mk1, mk2 = 196, 199 // MEC-1 Warden: Kinetic; MEC-2 Sentinel: Kinetic, Mist
	for _, path := range ewSamples {
		f := loadSample(t, path)
		m := sampleRoles(f).mecs[0]
		if m.Armor != mk1 {
			t.Fatalf("%s: sample MEC wears %d, want the Mk1 suit %d", path, m.Armor, mk1)
		}
		count := func(f *File, i int) [3]int32 { // in storage, in use, ever had
			return [3]int32{int32(f.Items[i]), f.claimsProp.Elements[i], f.archivesProp.Elements[i]}
		}
		mk1Before, mk2Before := count(f, mk1), count(f, mk2)
		if mk2Before != [3]int32{} {
			t.Fatalf("%s: expected no Mk2 suit, got %v", path, mk2Before)
		}

		// A suit not in storage: saving adds one and marks it in use; the
		// Mk1 is taken off (still in storage, no longer in use).
		m.Armor = mk2
		if !f.SuitAdded(m, mk2) || f.SuitAdded(m, mk1) || f.SuitAdded(m, mecArmor) {
			t.Errorf("%s: SuitAdded mk2 %v mk1 %v base %v", path, f.SuitAdded(m, mk2), f.SuitAdded(m, mk1), f.SuitAdded(m, mecArmor))
		}
		if _, err := f.Bytes(Native); err != nil {
			t.Fatal(err)
		}
		f.Bytes(Native) // a second write changes nothing more
		g := resave(t, f)
		gm := soldierNamed(g, m.Name)
		want1 := mk1Before
		want1[1]--
		if gm.Armor != mk2 || count(g, mk2) != [3]int32{1, 1, 1} || count(g, mk1) != want1 {
			t.Errorf("%s: armor %d, Mk2 %v, Mk1 %v (want %v)", path, gm.Armor, count(g, mk2), count(g, mk1), want1)
		}

		// Back to the Mk1, now free in storage: nothing is added.
		gm.Armor = mk1
		if g.SuitAdded(gm, mk1) {
			t.Errorf("%s: the Mk1 is in storage", path)
		}
		h := resave(t, g)
		if count(h, mk1) != mk1Before || count(h, mk2) != [3]int32{1, 0, 1} {
			t.Errorf("%s: after swapping back Mk1 %v (want %v), Mk2 %v", path, count(h, mk1), mk1Before, count(h, mk2))
		}

		// Base Augments: no suit, nothing in use.
		soldierNamed(h, m.Name).Armor = mecArmor
		k := resave(t, h)
		if count(k, mk1)[1] != mk1Before[1]-1 || count(k, mk2)[1] != 0 || soldierNamed(k, m.Name).Armor != mecArmor {
			t.Errorf("%s: Base Augments: Mk1 %v Mk2 %v", path, count(k, mk1), count(k, mk2))
		}

		// MEC armor is for MECs only, and MECs wear only MEC armor.
		v := loadSample(t, path)
		vm := sampleRoles(v).mecs[0]
		vm.Armor = ArmorFor(true)[0].Value
		if v.Validate() == nil {
			t.Errorf("%s: a MEC accepted soldier armor", path)
		}
		vm.Armor = mk1
		o := sampleRoles(v).varied[0]
		o.Armor = mk2
		if v.Validate() == nil {
			t.Errorf("%s: a soldier accepted a MEC suit", path)
		}
	}
}

func TestMECPsi(t *testing.T) {
	// A MEC's psi fields cannot be edited (so Gift All leaves it alone), and
	// saving puts its psi in the game's MEC state.
	for _, path := range ewSamples {
		f := loadSample(t, path)
		m := sampleRoles(f).mecs[0]
		for _, fld := range []Field{PsiRank, PsiXP, HasPsiGift, PsiTested, ForcePsiGift} {
			if m.Present(fld) {
				t.Errorf("%s: MEC psi field %s is editable", path, fld.PropName())
			}
		}
		if !m.Present(Health) || !m.Present(Aim) || !m.Present(Will) {
			t.Error("MEC stats should stay editable")
		}
		m.HasPsiGift, m.PsiTested, m.ForcePsiGift, m.PsiRank, m.PsiXP = true, false, true, 2, 150
		o := sampleRoles(f).varied[1]
		o.HasPsiGift, o.PsiRank = true, 2
		g := resave(t, f)
		if gm := soldierNamed(g, m.Name); gm.HasPsiGift || !gm.PsiTested || gm.ForcePsiGift || gm.PsiRank != 0 || gm.PsiXP != 0 {
			t.Errorf("%s: MEC psi after saving %+v", path, gm.soldierValues)
		}
		if gx := soldierNamed(g, o.Name); !gx.HasPsiGift || gx.PsiRank != 2 {
			t.Errorf("%s: non-MEC psi not saved", path)
		}
	}
}

func TestBecomeMEC(t *testing.T) {
	for _, path := range ewSamples {
		f := loadSample(t, path)
		s := sampleRoles(f).varied[3] // a Heavy
		old := s.soldierValues
		s.SetClass(MECIndex)
		if s.Armor != mecArmor || s.Weapon != mecWeapon || s.Pistol != 0 || s.Rocket != 0 || s.Item1 != 0 || s.Item2 != 0 ||
			s.HasPsiGift || !s.PsiTested || s.PsiRank != 0 {
			t.Fatalf("%s: MEC values %+v", path, s.soldierValues)
		}
		g := resave(t, f)
		m := soldierNamed(g, s.Name)
		// As the game converts a soldier: class, m_kChar.eClass, item counts,
		// abilities, and the old load out backed up.
		if !m.IsMEC() || m.class.name.Value.Str != MECLabel || m.class.weapon.Value != "eWP_Heavy" || m.mec.charClass.Value != MECClass {
			t.Errorf("%s: class %q %q %q, m_kChar.eClass %q", path, m.class.name.Value.Str, m.class.typ.Value, m.class.weapon.Value, m.mec.charClass.Value)
		}
		if m.Armor != mecArmor || m.Weapon != mecWeapon || m.mec.numLarge.Value != 1 || m.mec.numSmall.Value != 0 {
			t.Errorf("%s: armor %d weapon %d counts %d/%d", path, m.Armor, m.Weapon, m.mec.numLarge.Value, m.mec.numSmall.Value)
		}
		for idx, want := range map[int]int32{121: 1, 145: 1, 165: 0, 38: 0} {
			if p := element(m.mec.abilities, idx); p == nil || p.Value != want {
				t.Errorf("%s: ability %d = %v, want %d", path, idx, p, want)
			}
		}
		if b := m.mec.backup; b.armor.Value != int32(old.Armor) || element(b.large, 0).Value != int32(old.Weapon) {
			t.Errorf("%s: backed up armor %d weapon %d, want %d %d", path, b.armor.Value, element(b.large, 0).Value, old.Armor, old.Weapon)
		}

		// And back to Heavy: the backed-up load out, abilities restored.
		m.SetClass(classIndex("eSC_HeavyWeapons"))
		if m.Armor != old.Armor || m.Weapon != old.Weapon || m.Pistol != 0 || m.Rocket == 0 {
			t.Errorf("%s: restored load out %+v, was %+v", path, m.soldierValues, old)
		}
		r := soldierNamed(resave(t, g), s.Name)
		if r.IsMEC() || !r.IsHeavy() || r.mec.charClass.Value != "eSC_HeavyWeapons" || r.class.name.Value.Str != "Heavy" {
			t.Errorf("%s: after leaving MEC: class %q eClass %q", path, r.class.typ.Value, r.mec.charClass.Value)
		}
		for idx, want := range map[int]int32{121: 0, 145: 0, 165: 1} {
			if p := element(r.mec.abilities, idx); p == nil || p.Value != want {
				t.Errorf("%s: ability %d = %v, want %d after leaving MEC", path, idx, p, want)
			}
		}
	}
}

func TestLeaveMECKeepsSuitInStorage(t *testing.T) {
	// The sample's MEC wears a suit (196) equipped in game: it is owned and in
	// use. Made a Sniper, the suit comes off and is no longer in use, but is
	// still owned.
	for _, path := range ewSamples {
		f := loadSample(t, path)
		m := sampleRoles(f).mecs[0]
		if m.Armor != 196 || f.Items[196] != 1 || f.claimsProp.Elements[196] != 1 {
			t.Fatalf("%s: MEC armor %d, suit owned %d in use %d", path, m.Armor, f.Items[196], f.claimsProp.Elements[196])
		}
		m.SetClass(classIndex("eSC_Sniper"))
		if m.Armor == 196 || m.Armor == mecArmor || m.Weapon == mecWeapon {
			t.Fatalf("%s: suit or MEC weapon kept: armor %d weapon %d", path, m.Armor, m.Weapon)
		}
		var data []byte
		for range 2 { // writing twice must not release the suit twice
			var err error
			if data, err = f.Bytes(f.Format); err != nil {
				t.Fatal(err)
			}
		}
		g, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		if g.claimsProp.Elements[196] != 0 || g.Items[196] != 1 {
			t.Errorf("%s: suit in use %d, owned %d; want 0 and 1", path, g.claimsProp.Elements[196], g.Items[196])
		}
		if r := soldierNamed(g, m.Name); r.IsMEC() || Classes[r.Class].Type != "eSC_Sniper" {
			t.Errorf("%s: class after leaving MEC: %q", path, r.ClassLabel())
		}
	}
}

func TestMECNeedsRank(t *testing.T) {
	f := loadSample(t, ewSampleJSON)
	s := sampleRoles(f).varied[0] // a rookie
	s.SetClass(MECIndex)
	if err := f.Validate(); err == nil || !strings.Contains(err.Error(), "MEC Trooper") {
		t.Fatalf("a rookie made a MEC should not be savable (err %v)", err)
	}
	s.Rank = 1
	if err := f.Validate(); err != nil {
		t.Fatalf("a MEC with a rank: %v", err)
	}
	// Only a class change is checked: an existing MEC is left as it is.
	f = loadSample(t, ewSampleJSON)
	m := sampleRoles(f).mecs[0]
	m.Rank = 0
	if err := f.Validate(); err != nil {
		t.Fatalf("existing MEC: %v", err)
	}
}

// ---- God Squad ----

func TestGodSquad(t *testing.T) {
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		r := sampleRoles(f)
		left := map[string]soldierValues{r.dead.Name: r.dead.soldierValues}
		for _, m := range slices.Concat(r.mecs, r.shivs) {
			left[m.Name] = m.soldierValues
		}
		rookies := 0
		for _, s := range f.Soldiers {
			if !s.IsDead() && !s.IsSHIV() && s.Class == 0 && s.Present(Class) {
				rookies++
			}
		}
		upgraded, assigned, skipped, err := f.GodSquad()
		if err != nil {
			t.Fatal(err)
		}
		// The dead soldier and S.H.I.V.s are skipped; MECs count as upgraded.
		if skip := 1 + len(r.shivs); assigned != rookies || skipped != skip || upgraded != len(f.Soldiers)-skip {
			t.Errorf("%s: upgraded %d assigned %d (want %d) skipped %d", sm.path, upgraded, assigned, rookies, skipped)
		}
		// Classes end up balanced: each rookie goes to the smallest class.
		counts := f.classCounts()
		if counts["eSC_None"] != 0 {
			t.Errorf("%s: %d soldiers still classless", sm.path, counts["eSC_None"])
		}

		armor, small := ArmorFor(sm.enemyWithin), SmallItemsFor(sm.enemyWithin)
		num := func(items []LoadOutItem, name string) int {
			n, err := itemNumber(items, name)
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		want := map[string][6]int{ // weapon, rocket, pistol, armor, item1, item2
			"eSC_Assault":      {16, 0, 13, num(armor, "Ghost Armor"), num(small, "Alien Grenade"), num(small, "Arc Thrower")},
			"eSC_Support":      {15, 0, 13, num(armor, "Ghost Armor"), num(small, "Arc Thrower"), num(small, "Medikit")},
			"eSC_HeavyWeapons": {17, 19, 0, num(armor, "Titan Armor"), num(small, "Alien Grenade"), 0},
			"eSC_Sniper":       {18, 0, 13, num(armor, "Archangel Armor"), num(small, "Alien Grenade"), 0},
		}
		g := resave(t, f)
		for _, s := range g.Soldiers {
			who := sm.path + ": " + s.InstanceName
			if v, ok := left[s.Name]; ok {
				// The dead soldier and S.H.I.V.s are untouched; a MEC gets the
				// stats only.
				if s.IsMEC() {
					v.Health, v.Aim, v.Defense, v.Mobility, v.Will = godHealth, godAim, godDefense, godMobility, godWill
				}
				if s.soldierValues != v {
					t.Errorf("%s: changed beyond what God Squad may: %+v", who, s.soldierValues)
				}
				continue
			}
			if s.Rank != godRank || s.PsiRank != godPsiRank || !s.HasPsiGift {
				t.Errorf("%s: rank %d psi rank %d gift %v", who, s.Rank, s.PsiRank, s.HasPsiGift)
			}
			if s.Health != godHealth || s.Aim != godAim || s.Defense != godDefense || s.Mobility != godMobility || s.Will != godWill {
				t.Errorf("%s: stats %d/%d/%d/%d/%d", who, s.Health, s.Aim, s.Defense, s.Mobility, s.Will)
			}
			// The game's second class field follows, as the game sets it.
			if c := s.mec.charClass; c != nil && c.Value != Classes[s.Class].Type {
				t.Errorf("%s: m_kChar.eClass %q, class %q", who, c.Value, Classes[s.Class].Type)
			}
			// Enemy Within: the class's gene mod and Bioelectric Skin (Brain,
			// Eyes, Chest, Skin, Legs); Enemy Unknown: none.
			classMod := map[string][2]int{ // body part, mod
				"eSC_Sniper": {1, 2}, "eSC_HeavyWeapons": {2, 2}, "eSC_Assault": {0, 2}, "eSC_Support": {4, 1},
			}[Classes[s.Class].Type]
			if sm.enemyWithin {
				if s.GeneMods[classMod[0]] != classMod[1] || s.GeneMods[3] != 1 {
					t.Errorf("%s (%s): gene mods %v", who, Classes[s.Class].Type, s.GeneMods)
				}
			} else if s.GeneMods != [NumBodyParts]int{} {
				t.Errorf("%s: gene mods %v in Enemy Unknown", who, s.GeneMods)
			}
			w := want[Classes[s.Class].Type]
			if got := [6]int{s.Weapon, s.Rocket, s.Pistol, s.Armor, s.Item1, s.Item2}; got != w {
				t.Errorf("%s (%s): load out %v, want %v", who, Classes[s.Class].Type, got, w)
			}
			// A second small item needs iNumSmallItems 2.
			if n := s.ints[NumSmallItems]; n != nil && int(n.Value) != 1+min(s.Item2, 1) {
				t.Errorf("%s: iNumSmallItems %d with items %d,%d", who, n.Value, s.Item1, s.Item2)
			}
		}
	}
}

// charClassMismatches returns the soldiers (not S.H.I.V.s) whose
// m_kChar.eClass differs from their kClass.eType, as read from the save.
func charClassMismatches(f *File) []string {
	var bad []string
	for _, s := range f.Soldiers {
		if c := s.mec.charClass; c != nil && s.class.typ != nil && !s.IsSHIV() && c.Value != s.class.typ.Value {
			bad = append(bad, s.Name+" "+c.Value+"/"+s.class.typ.Value)
		}
	}
	return bad
}

func TestCharClassFollowsClass(t *testing.T) {
	// The game keeps m_kChar.eClass equal to kClass.eType...
	if bad := charClassMismatches(loadSample(t, euSource)); len(bad) != 0 {
		t.Errorf("%s: game save has mismatched classes %v", euSource, bad)
	}
	// ...but ew-source has soldiers given classes by an older editor (God
	// Squad), which the Genetics Lab refused. Saving repairs them.
	f := loadSample(t, ewSource)
	if len(charClassMismatches(f)) == 0 {
		t.Fatalf("%s: expected soldiers with m_kChar.eClass eSC_None", ewSource)
	}
	if bad := charClassMismatches(resave(t, f)); len(bad) != 0 {
		t.Errorf("%s: not repaired on save: %v", ewSource, bad)
	}

	// Giving a rookie a class sets both fields.
	for _, sm := range samples {
		f := loadSample(t, sm.path)
		var rookie *Soldier
		for _, s := range f.Soldiers {
			if s.Class == 0 && !s.IsDead() && !s.IsSHIV() && s.Present(Class) && s.mec.charClass != nil {
				rookie = s
				break
			}
		}
		if rookie == nil {
			continue
		}
		rookie.SetClass(classIndex("eSC_Sniper"))
		rookie.Rank = 1
		g := resave(t, f)
		r := soldierNamed(g, rookie.Name)
		if r.class.typ.Value != "eSC_Sniper" || r.mec.charClass.Value != "eSC_Sniper" {
			t.Errorf("%s: kClass %q m_kChar.eClass %q", sm.path, r.class.typ.Value, r.mec.charClass.Value)
		}
		if bad := charClassMismatches(g); len(bad) != 0 {
			t.Errorf("%s: mismatched classes %v", sm.path, bad)
		}
	}
}

// ---- Gene mods ----

// abilities returns a copy of the soldier's m_kChar.aUpgrades.
func abilities(s *Soldier) []int32 {
	var a []int32
	for i := range s.mec.abilities.Properties {
		a = append(a, element(s.mec.abilities, i).Value)
	}
	return a
}

func TestGeneMods(t *testing.T) {
	// The game gave one soldier in ew-source Depth Perception (Eyes, 2nd).
	f := loadSample(t, ewSource)
	var modded []string
	for _, s := range f.Soldiers {
		if s.GeneMods != [NumBodyParts]int{} {
			modded = append(modded, fmt.Sprint(s.Name, s.GeneMods))
		}
	}
	if len(modded) != 1 || !strings.HasSuffix(modded[0], "[0 2 0 0 0]") {
		t.Errorf("%s: gene modded soldiers %v, want one with Depth Perception", ewSource, modded)
	}

	for _, path := range ewSamples {
		f := loadSample(t, path)
		s := sampleRoles(f).varied[0]
		if !s.HasGeneMods() {
			t.Fatalf("%s: %s cannot be gene modded", path, s.Name)
		}
		before := abilities(s)
		s.GeneMods = [NumBodyParts]int{1, 2, 1, 2, 2}
		g := resave(t, f)
		gs := soldierNamed(g, s.Name)
		if gs.GeneMods != s.GeneMods || gs.Modified() {
			t.Errorf("%s: gene mods read back as %v, want %v", path, gs.GeneMods, s.GeneMods)
		}
		want := slices.Clone(before)
		for i, p := range BodyParts {
			for j, m := range p.Mods {
				want[m.Ability] = int32(boolInt(s.GeneMods[i] == j+1))
			}
		}
		if got := abilities(gs); !slices.Equal(got, want) {
			t.Errorf("%s: aUpgrades\n got %v\nwant %v", path, got, want)
		}

		// Swap one part's mod and remove another's.
		gs.GeneMods[0], gs.GeneMods[1] = 2, 0
		hf := resave(t, g)
		h := soldierNamed(hf, s.Name)
		a := abilities(h)
		if a[28] != 0 || a[29] != 1 || a[30] != 0 || a[38] != 0 || a[4] != 1 || h.GeneMods != [NumBodyParts]int{2, 0, 1, 2, 2} {
			t.Errorf("%s: after swap/remove: gene mods %v, aUpgrades 28/29/30/38/4 = %d/%d/%d/%d/%d",
				path, h.GeneMods, a[28], a[29], a[30], a[38], a[4])
		}

		// Becoming a MEC removes every gene mod.
		h.Rank = max(h.Rank, 1)
		h.SetClass(MECIndex)
		m := soldierNamed(resave(t, hf), s.Name)
		if m.GeneMods != [NumBodyParts]int{} || m.HasGeneMods() {
			t.Errorf("%s: MEC gene mods %v", path, m.GeneMods)
		}
		for _, i := range geneModAbilities {
			if v := element(m.mec.abilities, i).Value; v != 0 {
				t.Errorf("%s: MEC keeps gene mod ability %d", path, i)
			}
		}
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- Game compatibility ----

func TestEnemyUnknownUnaffectedByEW(t *testing.T) {
	for _, path := range euSamples {
		f := loadSample(t, path)
		if f.EnemyWithin() || len(f.patients) != 0 || len(f.surgeries) != 0 || len(sampleRoles(f).mecs) != 0 {
			t.Fatalf("%s: Enemy Within data in an Enemy Unknown save", path)
		}
		f.Soldiers[0].SetClass(MECIndex)
		if f.Validate() == nil {
			t.Errorf("%s: a MEC class should not be allowed in an Enemy Unknown save", path)
		}

		// No gene mods: the same aUpgrades indexes are other abilities in
		// Enemy Unknown, so they are neither read nor written.
		f = loadSample(t, path)
		s := sampleRoles(f).varied[0]
		before := abilities(s)
		if s.HasGeneMods() || s.GeneMods != [NumBodyParts]int{} {
			t.Errorf("%s: gene mods %v editable %v", path, s.GeneMods, s.HasGeneMods())
		}
		s.GeneMods = [NumBodyParts]int{1, 1, 1, 1, 1}
		if got := abilities(soldierNamed(resave(t, f), s.Name)); !slices.Equal(got, before) {
			t.Errorf("%s: gene mods written to an Enemy Unknown save", path)
		}
	}
}

func TestSHIVType(t *testing.T) {
	f := loadSample(t, "../../example/eu-source")
	want := map[string]string{"GOLIATH-1": "Alloy S.H.I.V.", "LIGHTNING-1": "Hover S.H.I.V."}
	n := 0
	for _, s := range f.Soldiers {
		if !s.IsSHIV() {
			if s.SHIVType() != "" {
				t.Errorf("%s: soldier has S.H.I.V. type %q", s.LastName, s.SHIVType())
			}
			continue
		}
		n++
		if s.SHIVType() != want[s.LastName] || s.ClassLabel() != want[s.LastName] {
			t.Errorf("%s: type %q, label %q, want %q", s.LastName, s.SHIVType(), s.ClassLabel(), want[s.LastName])
		}
		if !slices.ContainsFunc(SHIVWeaponsFor(false), func(it LoadOutItem) bool { return it.Value == s.Weapon }) {
			t.Errorf("%s: weapon %d not a S.H.I.V. weapon", s.LastName, s.Weapon)
		}
	}
	if n != len(want) {
		t.Errorf("%d S.H.I.V.s, want %d", n, len(want))
	}
	// Enemy Within's numbers: the user's Alloy S.H.I.V. wears 114.
	s := &Soldier{enemyWithin: true}
	s.mec.charType = &xcomsave.IntProperty{Value: shivType}
	s.Armor = 114
	if s.SHIVType() != "Alloy S.H.I.V." {
		t.Errorf("Enemy Within deck 114: %q", s.SHIVType())
	}
}
