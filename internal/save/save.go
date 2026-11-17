// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

// Package save exposes an XCOM: Enemy Unknown / Enemy Within saved game for
// editing: its soldiers, interceptors, storage, projects and campaign
// settings. Files may be native saves or JSON produced by
// xcom2json; both are handled by the xcomsave package.
package save

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/scottvail/go-xcomsave"
)

// StrategyClass is the class of the checkpoint entry holding campaign-wide
// settings. Its name varies (XGStrategy_0 in Enemy Unknown saves,
// XGStrategy_1 in Enemy Within ones).
const StrategyClass = "XComStrategyGame.XGStrategy"

// HeadquartersClass is the class of the checkpoint entry holding XCOM HQ's
// state. Its name varies (XGHeadQuarters_0 in Enemy Unknown saves,
// XGHeadQuarters_1 in Enemy Within ones).
const HeadquartersClass = "XComStrategyGame.XGHeadQuarters"

// StorageClass is the class of the checkpoint entry holding XCOM's stored
// items. Its name varies (XGStorage_0 in Enemy Unknown saves, XGStorage_1 in
// Enemy Within ones).
const StorageClass = "XComStrategyGame.XGStorage"

// LabsClass is the class of the checkpoint entry holding XCOM's research
// labs. Its name varies (XGFacility_Labs_0 in Enemy Unknown saves,
// XGFacility_Labs_1 in Enemy Within ones).
const LabsClass = "XComStrategyGame.XGFacility_Labs"

// EngineeringClass is the class of the checkpoint entry holding XCOM's
// engineering (the Workshop and Foundry). Its name varies
// (XGFacility_Engineering_0 in Enemy Unknown saves, _1 in Enemy Within ones).
const EngineeringClass = "XComStrategyGame.XGFacility_Engineering"

// ContinentClass is the class of the checkpoint entries describing each
// continent (its m_strName and m_eContinent).
const ContinentClass = "XComStrategyGame.XGContinent"

// continentEnum is the game's EContinent enum in order: XGHeadQuarters'
// m_iContinent is an index into it.
var continentEnum = []string{
	"eContinent_NorthAmerica",
	"eContinent_SouthAmerica",
	"eContinent_Europe",
	"eContinent_Asia",
	"eContinent_Africa",
}

// Difficulties are the campaign difficulty levels; File.Difficulty is an
// index into this list.
var Difficulties = []string{"Easy", "Normal", "Classic", "Impossible"}

// Format is a save file format.
type Format int

const (
	Native Format = iota // the game's own compressed save format
	JSON                 // xcom2json's JSON format
)

func (f Format) String() string {
	if f == JSON {
		return "JSON"
	}
	return "XCOM save"
}

// FormatForPath picks the format for a new file name: JSON for names ending
// in ".json", otherwise the native save format.
func FormatForPath(path string) Format {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return JSON
	}
	return Native
}

// DetectFormat reports whether data is JSON or a native save.
func DetectFormat(data []byte) Format {
	if t := bytes.TrimLeft(data, " \t\r\n\xef\xbb\xbf"); len(t) > 0 && t[0] == '{' {
		return JSON
	}
	return Native
}

// File is a loaded save.
type File struct {
	Path     string
	Format   Format
	Soldiers []*Soldier

	// Ships are XCOM's interceptors (XGShip_Interceptor_<n> entries), in
	// instance number order.
	Ships []*Ship

	// Scientists is XGFacility_Labs' m_iNumScientists.
	Scientists     int
	origScientists int
	scientistsProp *xcomsave.IntProperty

	// Engineers is XGFacility_Engineering's m_iNumEngineers.
	Engineers     int
	origEngineers int
	engineersProp *xcomsave.IntProperty

	construction []*hoursLeft // construction and facility projects (see projects.go)
	itemProjects []*hoursLeft // item and Foundry projects
	satellites   []*hoursLeft // satellites on their way
	patients     []patient    // Cybernetics Lab patients (see projects.go)
	surgeries    []*hoursLeft // their hours left
	research     *hoursLeft   // the current research project

	// Orders are XGHeadQuarters' m_akInterceptorOrders: interceptors bought
	// and not yet delivered.
	Orders []*InterceptorOrder

	// Orders made in the editor (see orders.go).
	hq                 *xcomsave.Checkpoint // XGHeadQuarters
	barracksSize       int                  // XGFacility_Barracks' m_arrSoldiers length
	hasBarracks        bool
	hireOrders         []hireOrder // soldier hiring orders not yet due
	rushHires          bool        // set their iHours to 0 on save
	hiring             int         // soldiers on their way (m_arrHiringOrders)
	newShips           []newOrder  // interceptors ordered in the editor
	newHires           int         // soldiers hired in the editor
	shipBase, hireBase int         // order counts before the editor's (-1 until applied)

	// Ironman is the campaign's Ironman setting. It is stored both in the
	// header and as XGStrategy_0's m_bIronMan; saving sets both.
	Ironman     bool
	origIronman bool
	ironmanProp *xcomsave.BoolProperty

	// Difficulty is XGStrategy_0's m_iDifficulty (see Difficulties).
	Difficulty     int
	origDifficulty int
	difficultyProp *xcomsave.IntProperty

	// LowestDifficulty is XGStrategy_0's m_iLowestDifficulty: the lowest
	// difficulty the campaign has been played on (see Difficulties).
	LowestDifficulty int
	origLowest       int
	lowestProp       *xcomsave.IntProperty

	// Cash is XGHeadQuarters_0's m_iCash: XCOM's funds.
	Cash     int
	origCash int
	cashProp *xcomsave.IntProperty

	// Continent is XGHeadQuarters' m_iContinent: the continent XCOM HQ is
	// on, as an EContinent index (read only).
	Continent      int
	hasContinent   bool
	continentNames map[string]string // EContinent value -> XGContinent m_strName

	// Items is XGStorage_0's m_arrItems: the number of each item type in
	// storage, indexed by item type (see StorageItemsFor). Edit the values in
	// place; the length cannot change.
	Items     []int
	origItems []int
	itemsProp *xcomsave.NumberArrayProperty

	// claimsProp is XGStorage's m_arrClaims: how many of each item are in
	// use (equipped through the game).
	claimsProp *xcomsave.NumberArrayProperty
	// archivesProp is XGStorage's m_arrItemArchives: how many of each item
	// XCOM has ever had (a suit the editor creates is counted, as when built).
	archivesProp *xcomsave.NumberArrayProperty

	save *xcomsave.Save
	crlf bool   // JSON line endings
	src  []byte // file contents as loaded or last saved
}

// Version describes the game version of the save, e.g. "Enemy Within".
func (f *File) Version() string { return f.save.Header.Version.String() }

// Header returns a copy of the save's header (description, time, flags).
func (f *File) Header() xcomsave.Header { return f.save.Header }

// EnemyWithin reports whether the save is from Enemy Within (PC or
// Android), whose saves have items Enemy Unknown's do not.
func (f *File) EnemyWithin() bool {
	v := f.save.Header.Version
	return v == xcomsave.EnemyWithin || v == xcomsave.EnemyWithinAndroid
}

// Load reads a native save or xcom2json JSON file.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := Parse(data)
	if err != nil {
		return nil, err
	}
	f.Path = path
	return f, nil
}

// Parse extracts soldiers from save data in either format.
func Parse(data []byte) (*File, error) {
	f := &File{Format: DetectFormat(data), src: data, shipBase: -1, hireBase: -1}
	var err error
	if f.Format == JSON {
		f.save, err = xcomsave.UnmarshalXComJSON(data)
		f.crlf = bytes.Contains(data, []byte("\r\n"))
	} else {
		f.save, err = xcomsave.Read(data)
	}
	if err != nil {
		return nil, err
	}
	f.Ironman = f.save.Header.Ironman
	f.origIronman = f.Ironman
	for ci := range f.save.Checkpoints {
		for i := range f.save.Checkpoints[ci].Checkpoints {
			c := &f.save.Checkpoints[ci].Checkpoints[i]
			if c.ClassName == HeadquartersClass && f.cashProp == nil {
				f.cashProp, _ = xcomsave.Find(c.Properties, "m_iCash").(*xcomsave.IntProperty)
				if f.cashProp != nil {
					f.Cash = int(f.cashProp.Value)
					f.origCash = f.Cash
				}
				if p, ok := xcomsave.Find(c.Properties, "m_iContinent").(*xcomsave.IntProperty); ok {
					f.Continent, f.hasContinent = int(p.Value), true
				}
				f.readSatellites(c)
				f.readHQOrders(c)
			}
			if c.ClassName == BarracksClass && !f.hasBarracks {
				f.readBarracks(c)
			}
			if c.ClassName == ContinentClass {
				e, _ := xcomsave.Find(c.Properties, "m_eContinent").(*xcomsave.ByteProperty)
				n, _ := xcomsave.Find(c.Properties, "m_strName").(*xcomsave.StrProperty)
				if e != nil && n != nil {
					if f.continentNames == nil {
						f.continentNames = map[string]string{}
					}
					f.continentNames[e.Value] = n.Value.Str
				}
			}
			// EWStorageItems was built from Enemy Within saves with
			// EWStorageLen item types, so storage in an Enemy Within save is
			// only editable when its m_arrItems has that length.
			if c.ClassName == StorageClass && f.itemsProp == nil {
				f.claimsProp, _ = xcomsave.Find(c.Properties, "m_arrClaims").(*xcomsave.NumberArrayProperty)
				f.archivesProp, _ = xcomsave.Find(c.Properties, "m_arrItemArchives").(*xcomsave.NumberArrayProperty)
				f.itemsProp, _ = xcomsave.Find(c.Properties, "m_arrItems").(*xcomsave.NumberArrayProperty)
				if f.itemsProp != nil && f.EnemyWithin() && len(f.itemsProp.Elements) != EWStorageLen {
					f.itemsProp = nil
				}
				if f.itemsProp != nil {
					f.Items = make([]int, len(f.itemsProp.Elements))
					for i, v := range f.itemsProp.Elements {
						f.Items[i] = int(v)
					}
					f.origItems = slices.Clone(f.Items)
				}
			}
			if c.ClassName == StrategyClass && f.difficultyProp == nil {
				f.ironmanProp, _ = xcomsave.Find(c.Properties, "m_bIronMan").(*xcomsave.BoolProperty)
				f.difficultyProp, _ = xcomsave.Find(c.Properties, "m_iDifficulty").(*xcomsave.IntProperty)
				if f.difficultyProp != nil {
					f.Difficulty = int(f.difficultyProp.Value)
					f.origDifficulty = f.Difficulty
				}
				f.lowestProp, _ = xcomsave.Find(c.Properties, "m_iLowestDifficulty").(*xcomsave.IntProperty)
				if f.lowestProp != nil {
					f.LowestDifficulty = int(f.lowestProp.Value)
					f.origLowest = f.LowestDifficulty
				}
			}
			if c.ClassName == EngineeringClass && f.engineersProp == nil {
				f.readConstruction(c)
				f.engineersProp, _ = xcomsave.Find(c.Properties, "m_iNumEngineers").(*xcomsave.IntProperty)
				if f.engineersProp != nil {
					f.Engineers = int(f.engineersProp.Value)
					f.origEngineers = f.Engineers
				}
			}
			if c.ClassName == LabsClass && f.scientistsProp == nil {
				f.readResearch(c)
				f.scientistsProp, _ = xcomsave.Find(c.Properties, "m_iNumScientists").(*xcomsave.IntProperty)
				if f.scientistsProp != nil {
					f.Scientists = int(f.scientistsProp.Value)
					f.origScientists = f.Scientists
				}
			}
			// Augmentation (MECs) and gene mods exist only in Enemy Within.
			if (c.ClassName == CyberneticsLabClass || c.ClassName == GeneLabsClass) && f.EnemyWithin() {
				f.readPatients(c)
			}
			if c.ClassName == InterceptorClass {
				s := newShip(c)
				s.enemyWithin = f.EnemyWithin()
				f.Ships = append(f.Ships, s)
			}
			if !strings.HasPrefix(c.Name, SoldierPrefix) {
				continue
			}
			s, err := newSoldier(c)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", c.Name, err)
			}
			s.enemyWithin = f.EnemyWithin()
			s.readGeneMods()
			f.Soldiers = append(f.Soldiers, s)
		}
	}
	sort.SliceStable(f.Soldiers, func(i, j int) bool {
		return soldierNum(f.Soldiers[i].Name) < soldierNum(f.Soldiers[j].Name)
	})
	f.linkPatients()
	if len(f.Ships) > 0 {
		entities := map[string]*xcomsave.Checkpoint{}
		for ci := range f.save.Checkpoints {
			for i := range f.save.Checkpoints[ci].Checkpoints {
				if c := &f.save.Checkpoints[ci].Checkpoints[i]; c.ClassName == ShipEntityClass {
					entities[c.InstanceName] = c
				}
			}
		}
		for _, s := range f.Ships {
			s.linkEntity(f.save.Actors, entities)
		}
	}
	sort.SliceStable(f.Ships, func(i, j int) bool {
		return shipNum(f.Ships[i].InstanceName) < shipNum(f.Ships[j].InstanceName)
	})
	return f, nil
}

func setStrProp(p *xcomsave.StrProperty, v string) {
	if p.Value.Str != v {
		p.Value.Str = v
		// Narrow strings are Latin-1; anything else must be stored wide.
		if needsWide(v) {
			p.Value.IsWide = true
		}
	}
}

func needsWide(s string) bool {
	for _, r := range s {
		if r > 0xff {
			return true
		}
	}
	return false
}

// HasDifficulty reports whether the save has a difficulty setting to edit.
func (f *File) HasDifficulty() bool { return f.difficultyProp != nil }

// HasCash reports whether the save has a cash value to edit.
func (f *File) HasCash() bool { return f.cashProp != nil }

// HasScientists reports whether the save has a lab scientist count to edit.
func (f *File) HasScientists() bool { return f.scientistsProp != nil }

// HasEngineers reports whether the save has an engineer count to edit.
func (f *File) HasEngineers() bool { return f.engineersProp != nil }

// HasContinent reports whether the save records XCOM HQ's continent.
func (f *File) HasContinent() bool { return f.hasContinent }

// ContinentName returns the name of XCOM HQ's continent as the save gives
// it (e.g. "Africa"), or "" if the save doesn't name it.
func (f *File) ContinentName() string {
	if f.Continent < 0 || f.Continent >= len(continentEnum) {
		return ""
	}
	return f.continentNames[continentEnum[f.Continent]]
}

// ContinentLabels returns a label for each EContinent index: the name the
// save gives the continent, or the enum value's name if it gives none.
func (f *File) ContinentLabels() []string {
	out := make([]string, len(continentEnum))
	for i, e := range continentEnum {
		out[i] = f.continentNames[e]
		if out[i] == "" {
			out[i] = strings.TrimPrefix(e, "eContinent_")
		}
	}
	return out
}

// HasStorage reports whether the save has a storage item list to edit: any
// Enemy Unknown save, or an Enemy Within save whose list has EWStorageLen
// items.
func (f *File) HasStorage() bool { return f.itemsProp != nil }

// StorageItems returns the storage items for the save's game version.
func (f *File) StorageItems() []StorageItem { return StorageItemsFor(f.EnemyWithin()) }

// HasItem reports whether storage item it exists in the save.
func (f *File) HasItem(it StorageItem) bool {
	return it.Index() >= 0 && it.Index() < len(f.Items)
}

// HasLowestDifficulty reports whether the save has a lowest difficulty
// setting to edit.
func (f *File) HasLowestDifficulty() bool { return f.lowestProp != nil }

// Modified reports whether anything in the save has unsaved changes.
func (f *File) Modified() bool {
	if f.Ironman != f.origIronman || f.Difficulty != f.origDifficulty || f.LowestDifficulty != f.origLowest ||
		f.Cash != f.origCash || f.Scientists != f.origScientists || f.Engineers != f.origEngineers ||
		!slices.Equal(f.Items, f.origItems) {
		return true
	}
	for _, s := range f.Soldiers {
		if s.Modified() {
			return true
		}
	}
	for _, s := range f.Ships {
		if s.Modified() {
			return true
		}
	}
	return f.ordersModified() || f.newOrdersModified() || f.projectsModified()
}

// Validate checks values against simple sanity limits.
func (f *File) Validate() error {
	const maxInt32 = 1<<31 - 1
	if f.Difficulty != f.origDifficulty && (f.Difficulty < 0 || f.Difficulty >= len(Difficulties)) {
		return fmt.Errorf("difficulty must be 0-%d (got %d)", len(Difficulties)-1, f.Difficulty)
	}
	if f.LowestDifficulty != f.origLowest && (f.LowestDifficulty < 0 || f.LowestDifficulty >= len(Difficulties)) {
		return fmt.Errorf("lowest difficulty must be 0-%d (got %d)", len(Difficulties)-1, f.LowestDifficulty)
	}
	if f.Cash != f.origCash && (f.Cash < 0 || f.Cash > maxInt32) {
		return fmt.Errorf("cash must be 0-%d (got %d)", maxInt32, f.Cash)
	}
	if f.Scientists != f.origScientists && (f.Scientists < 0 || f.Scientists > maxInt32) {
		return fmt.Errorf("scientists must be 0-%d (got %d)", maxInt32, f.Scientists)
	}
	if f.Engineers != f.origEngineers && (f.Engineers < 0 || f.Engineers > maxInt32) {
		return fmt.Errorf("engineers must be 0-%d (got %d)", maxInt32, f.Engineers)
	}
	if len(f.Items) != len(f.origItems) {
		return fmt.Errorf("the number of storage item types cannot change")
	}
	for i, v := range f.Items {
		if v != f.origItems[i] && (v < 0 || v > maxInt32) {
			return fmt.Errorf("storage item %d: count must be 0-%d (got %d)", i, maxInt32, v)
		}
	}
	for _, s := range f.Soldiers {
		if err := s.validate(); err != nil {
			return err
		}
	}
	for _, s := range f.Ships {
		if err := s.validate(); err != nil {
			return err
		}
	}
	return nil
}

// Bytes serializes the save, with all edits applied, in the given format.
// JSON is written losslessly (full-precision floats) with the same line
// endings as the loaded file; for files produced by xcom2json the output
// differs from the input only where values were edited.
func (f *File) Bytes(format Format) ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	for _, s := range f.Soldiers {
		s.apply()
	}
	f.applyMECSuits()
	for _, s := range f.Ships {
		s.apply()
	}
	f.applyOrders()
	f.applyNewOrders()
	f.applyProjects()
	if f.Ironman != f.origIronman {
		f.save.Header.Ironman = f.Ironman
		if f.ironmanProp != nil {
			f.ironmanProp.Value = f.Ironman
		}
	}
	if f.Difficulty != f.origDifficulty && f.difficultyProp != nil {
		f.difficultyProp.Value = int32(f.Difficulty)
	}
	if f.LowestDifficulty != f.origLowest && f.lowestProp != nil {
		f.lowestProp.Value = int32(f.LowestDifficulty)
	}
	if f.Cash != f.origCash && f.cashProp != nil {
		f.cashProp.Value = int32(f.Cash)
	}
	if f.Scientists != f.origScientists && f.scientistsProp != nil {
		f.scientistsProp.Value = int32(f.Scientists)
	}
	if f.Engineers != f.origEngineers && f.engineersProp != nil {
		f.engineersProp.Value = int32(f.Engineers)
	}
	if f.itemsProp != nil && len(f.Items) == len(f.itemsProp.Elements) {
		for i, v := range f.Items {
			f.itemsProp.Elements[i] = int32(v)
		}
	}
	if format == JSON {
		return f.save.MarshalXComJSON(xcomsave.JSONOptions{CRLF: f.crlf, ExactFloats: true}), nil
	}
	return f.save.Bytes()
}

// SaveAs writes the edited save to path: in the file's own format if path
// is the file it was loaded from, otherwise in FormatForPath(path). It then
// reloads from the written data so further edits are tracked against it.
//
// If backup is true, the previous contents of path are first copied to
// path+".bak" (only if no backup exists yet). When path is the loaded file,
// the backup is taken from the contents as loaded rather than from disk, so
// it is correct even if the file was truncated in the meantime (as GUI
// "save as" dialogs do when they open the chosen file for writing).
func (f *File) SaveAs(path string, backup bool) error {
	format := f.Format
	if path != f.Path {
		format = FormatForPath(path)
	}
	data, err := f.Bytes(format)
	if err != nil {
		return err
	}
	// Re-parse before touching the disk as a safety check.
	nf, err := Parse(data)
	if err != nil {
		return fmt.Errorf("internal error, edited save is invalid: %w", err)
	}
	if backup {
		bak := path + ".bak"
		if _, err := os.Stat(bak); errors.Is(err, os.ErrNotExist) {
			orig, err := os.ReadFile(path)
			if path == f.Path {
				orig, err = f.src, nil
			}
			if err == nil {
				if err := os.WriteFile(bak, orig, 0o644); err != nil {
					return fmt.Errorf("creating backup: %w", err)
				}
			}
		}
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	nf.Path = path
	*f = *nf
	return nil
}
