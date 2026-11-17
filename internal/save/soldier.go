// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/scottvail/go-xcomsave"
)

// SoldierPrefix identifies soldier entries in a checkpoint table.
const SoldierPrefix = "Command1.TheWorld:PersistentLevel.XGStrategySoldier_"

// Field identifies one editable soldier property.
type Field int

const (
	FirstName Field = iota
	LastName
	NickName
	Rank
	PsiRank
	XP
	PsiXP
	HasPsiGift
	PsiTested
	ForcePsiGift
	Class // kClass.strName + eType + eWeaponType, edited together
	Health
	Aim
	Defense
	Mobility // shown as "Travel"
	Will
	Armor // load out: m_kChar.kInventory
	Pistol
	Weapon   // arrLargeItems[0]
	Rocket   // arrLargeItems[1], Heavy only
	Item1    // arrSmallItems[0]
	Item2    // arrSmallItems[1]
	TurnsOut // m_iTurnsOut: time left recovering from wounds
	// Item counts, kept consistent with the load out; not edited directly.
	NumLargeItems
	NumSmallItems
	numFields
)

type fieldSpec struct {
	parent string // dotted struct path, e.g. "m_kChar.kInventory", or "" for the soldier itself
	prop   string
	index  int // element of a static array property, or -1
}

// Indexes into m_kChar.aStats (the game's ECharacterStat enum).
const (
	statHP       = 0
	statOffense  = 1
	statDefense  = 2
	statMobility = 3
	statWill     = 7
)

var specs = [numFields]fieldSpec{
	FirstName:    {"m_kSoldier", "strFirstName", -1},
	LastName:     {"m_kSoldier", "strLastName", -1},
	NickName:     {"m_kSoldier", "strNickName", -1},
	Rank:         {"m_kSoldier", "iRank", -1},
	PsiRank:      {"m_kSoldier", "iPsiRank", -1},
	XP:           {"m_kSoldier", "iXP", -1},
	PsiXP:        {"m_kSoldier", "iPsiXP", -1},
	HasPsiGift:   {"m_kChar", "bHasPsiGift", -1},
	PsiTested:    {"", "m_bPsiTested", -1},
	ForcePsiGift: {"", "bForcePsiGift", -1},
	Class:        {"m_kSoldier", "kClass", -1},
	Health:       {"m_kChar", "aStats", statHP},
	Aim:          {"m_kChar", "aStats", statOffense},
	Defense:      {"m_kChar", "aStats", statDefense},
	Mobility:     {"m_kChar", "aStats", statMobility},
	Will:         {"m_kChar", "aStats", statWill},

	Armor:         {"m_kChar.kInventory", "iArmor", -1},
	Pistol:        {"m_kChar.kInventory", "iPistol", -1},
	Weapon:        {"m_kChar.kInventory", "arrLargeItems", 0},
	Rocket:        {"m_kChar.kInventory", "arrLargeItems", 1},
	Item1:         {"m_kChar.kInventory", "arrSmallItems", 0},
	Item2:         {"m_kChar.kInventory", "arrSmallItems", 1},
	NumLargeItems: {"m_kChar.kInventory", "iNumLargeItems", -1},
	NumSmallItems: {"m_kChar.kInventory", "iNumSmallItems", -1},
	TurnsOut:      {"", "m_iTurnsOut", -1},
}

// SoldierClass is a soldier class. In the save it is three properties of
// the m_kSoldier.kClass struct (TClass), which must be changed together.
type SoldierClass struct {
	Name   string // kClass.strName
	Type   string // kClass.eType (ESoldierClass)
	Weapon string // kClass.eWeaponType (EWeaponProperty)
}

// Label is the class's display name.
func (c SoldierClass) Label() string {
	if c.Name == "" {
		return "None"
	}
	return c.Name
}

// Classes are the classes a soldier can be given. Soldier.Class is an index
// into this list.
var Classes = []SoldierClass{
	{"", "eSC_None", "eWP_None"},
	{"Sniper", "eSC_Sniper", "eWP_Sniper"},
	{"Assault", "eSC_Assault", "eWP_Assault"},
	{"Support", "eSC_Support", "eWP_Support"},
	{"Heavy", "eSC_HeavyWeapons", "eWP_Heavy"},
}

// UnknownClass is the Soldier.Class value of a class not in Classes.
const UnknownClass = -1

func classIndex(eType string) int {
	for i, c := range Classes {
		if c.Type == eType {
			return i
		}
	}
	return UnknownClass
}

// classProps are the kClass properties that make up a soldier's class.
type classProps struct {
	name        *xcomsave.StrProperty
	typ, weapon *xcomsave.ByteProperty
}

func findClassProps(kClass *xcomsave.StructProperty) (classProps, error) {
	var c classProps
	var ok bool
	if p := xcomsave.Find(kClass.Properties, "strName"); p != nil {
		if c.name, ok = p.(*xcomsave.StrProperty); !ok {
			return c, fmt.Errorf("kClass.strName has kind %s", p.Kind())
		}
	}
	for _, b := range []struct {
		name string
		dst  **xcomsave.ByteProperty
	}{{"eType", &c.typ}, {"eWeaponType", &c.weapon}} {
		if p := xcomsave.Find(kClass.Properties, b.name); p != nil {
			if *b.dst, ok = p.(*xcomsave.ByteProperty); !ok {
				return c, fmt.Errorf("kClass.%s has kind %s", b.name, p.Kind())
			}
		}
	}
	return c, nil
}

// PropName returns the save-file property name of a field.
func (f Field) PropName() string {
	if sp := specs[f]; sp.index >= 0 {
		return fmt.Sprintf("%s[%d]", sp.prop, sp.index)
	}
	return specs[f].prop
}

// soldierValues holds a soldier's editable values.
type soldierValues struct {
	FirstName, LastName, NickName string
	Rank, PsiRank, XP, PsiXP      int
	HasPsiGift                    bool
	PsiTested, ForcePsiGift       bool
	Class                         int // index into Classes, or UnknownClass
	// Base stats from m_kChar.aStats.
	Health, Aim, Defense, Mobility, Will int
	// Load out from m_kChar.kInventory (item type numbers; 0 is empty).
	Armor, Pistol, Weapon, Rocket, Item1, Item2 int
	// Wounded: time left out of action (m_iTurnsOut); 0 when healthy.
	TurnsOut int
	// Enemy Within gene mods, per BodyParts: 0 none, else 1 + the index
	// into that part's Mods (see genemods.go).
	GeneMods [NumBodyParts]int
}

// Soldier is one XGStrategySoldier entry. Edit the exported value fields;
// they are written into the save by File.SaveAs.
type Soldier struct {
	Name         string // full entry name, e.g. Command1...XGStrategySoldier_3
	InstanceName string // e.g. XGStrategySoldier_3
	Status       string // m_eStatus (ESoldierStatus), e.g. "eStatus_Active"; read only

	soldierValues

	strs  [numFields]*xcomsave.StrProperty
	ints  [numFields]*xcomsave.IntProperty
	bools [numFields]*xcomsave.BoolProperty
	class classProps
	orig  soldierValues

	mec         mecProps // what changes on becoming, or ceasing to be, a MEC (see mec.go)
	enemyWithin bool     // the save is Enemy Within

	augment  []*hoursLeft // Cybernetics Lab hours left, if being augmented
	geneMods []*hoursLeft // Genetics Lab hours left, one per pending gene mod
}

// Present reports whether the field exists in the save and can be edited.
// Missing fields cannot be edited, and neither can a MEC Trooper's psi
// fields: MECs cannot gain psi abilities.
func (s *Soldier) Present(f Field) bool {
	if s.IsSHIV() && shivFixed(f) {
		return false
	}
	if f == Class {
		return s.class.name != nil && s.class.typ != nil && s.class.weapon != nil
	}
	if s.IsMEC() && isPsiField(f) {
		return false
	}
	return s.strs[f] != nil || s.ints[f] != nil || s.bools[f] != nil
}

// MECClass is the class type (kClass.eType) of Enemy Within's MEC Troopers.
const MECClass = "eSC_Mec"

// shivFixed reports whether f cannot be edited for a S.H.I.V.: rank, class,
// psi and load out.
func shivFixed(f Field) bool {
	switch f {
	case Rank, Class, Armor, Pistol, Weapon, Rocket, Item1, Item2:
		return true
	}
	return isPsiField(f)
}

// isPsiField reports whether f is one of the psi fields.
func isPsiField(f Field) bool {
	switch f {
	case PsiRank, PsiXP, HasPsiGift, PsiTested, ForcePsiGift:
		return true
	}
	return false
}

// ClassLabel describes the soldier's class as loaded, including classes
// not in Classes (shown by their strName, or eType if that is empty).
func (s *Soldier) ClassLabel() string {
	if s.IsSHIV() {
		return s.SHIVType() // S.H.I.V.s have no class
	}
	if s.Class >= 0 && s.Class < len(Classes) {
		return Classes[s.Class].Label()
	}
	if s.IsMEC() {
		return MECLabel
	}
	if s.class.name != nil && s.class.name.Value.Str != "" {
		return s.class.name.Value.Str
	}
	if s.class.typ != nil {
		return s.class.typ.Value
	}
	return "?"
}

// Modified reports whether any value differs from the loaded value.
func (s *Soldier) Modified() bool { return s.soldierValues != s.orig }

func soldierNum(name string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(name, SoldierPrefix))
	if err != nil {
		return 1 << 30
	}
	return n
}

func newSoldier(c *xcomsave.Checkpoint) (*Soldier, error) {
	s := &Soldier{Name: c.Name, InstanceName: c.InstanceName}
	if p, ok := xcomsave.Find(c.Properties, "m_eStatus").(*xcomsave.ByteProperty); ok {
		s.Status = p.Value
	}
	for fld := Field(0); fld < numFields; fld++ {
		spec := specs[fld]
		props := c.Properties
		if spec.parent != "" {
			for _, name := range strings.Split(spec.parent, ".") {
				parent, _ := xcomsave.Find(props, name).(*xcomsave.StructProperty)
				if parent == nil {
					props = nil
					break
				}
				props = parent.Properties
			}
		}
		p := xcomsave.Find(props, spec.prop)
		if p != nil && spec.index >= 0 {
			// Static arrays are stored densely, so the element at position
			// i has array index i. A one-element array is a plain property.
			switch a := p.(type) {
			case *xcomsave.StaticArrayProperty:
				p = nil
				if spec.index < len(a.Properties) {
					p = a.Properties[spec.index]
				}
			default:
				if spec.index != 0 {
					p = nil
				}
			}
		}
		if p == nil {
			continue
		}
		ok := false
		switch fld {
		case FirstName, LastName, NickName:
			s.strs[fld], ok = p.(*xcomsave.StrProperty)
		case HasPsiGift, PsiTested, ForcePsiGift:
			s.bools[fld], ok = p.(*xcomsave.BoolProperty)
		case Class:
			var kClass *xcomsave.StructProperty
			if kClass, ok = p.(*xcomsave.StructProperty); ok {
				var err error
				if s.class, err = findClassProps(kClass); err != nil {
					return nil, err
				}
			}
		default:
			s.ints[fld], ok = p.(*xcomsave.IntProperty)
		}
		if !ok {
			return nil, fmt.Errorf("%s has kind %s", fld.PropName(), p.Kind())
		}
	}
	str := func(f Field) string {
		if p := s.strs[f]; p != nil {
			return p.Value.Str
		}
		return ""
	}
	num := func(f Field) int {
		if p := s.ints[f]; p != nil {
			return int(p.Value)
		}
		return 0
	}
	s.FirstName, s.LastName, s.NickName = str(FirstName), str(LastName), str(NickName)
	s.Rank, s.PsiRank, s.XP, s.PsiXP = num(Rank), num(PsiRank), num(XP), num(PsiXP)
	s.Health, s.Aim, s.Defense, s.Mobility, s.Will = num(Health), num(Aim), num(Defense), num(Mobility), num(Will)
	s.Armor, s.Pistol, s.Weapon, s.Rocket = num(Armor), num(Pistol), num(Weapon), num(Rocket)
	s.Item1, s.Item2 = num(Item1), num(Item2)
	s.TurnsOut = num(TurnsOut)
	flag := func(f Field) bool {
		if p := s.bools[f]; p != nil {
			return p.Value
		}
		return false
	}
	s.HasPsiGift, s.PsiTested, s.ForcePsiGift = flag(HasPsiGift), flag(PsiTested), flag(ForcePsiGift)
	// The class is identified by eType alone: strName is localized in
	// non-English saves.
	s.Class = UnknownClass
	if s.class.typ != nil {
		s.Class = classIndex(s.class.typ.Value)
		if s.class.typ.Value == MECClass {
			s.Class = MECIndex
		}
	}
	s.mec = readMECProps(c)
	if s.IsMEC() && isMECSuit(s.Armor) {
		s.mec.claimed = s.Armor
	}
	s.orig = s.soldierValues
	return s, nil
}

// apply copies edited values into the underlying save model.
func (s *Soldier) apply() {
	setStr := func(f Field, v string) {
		if p := s.strs[f]; p != nil {
			setStrProp(p, v)
		}
	}
	setInt := func(f Field, v int) {
		if p := s.ints[f]; p != nil {
			p.Value = int32(v)
		}
	}
	setStr(FirstName, s.FirstName)
	setStr(LastName, s.LastName)
	setStr(NickName, s.NickName)
	setInt(Rank, s.Rank)
	setInt(PsiRank, s.PsiRank)
	setInt(XP, s.XP)
	setInt(PsiXP, s.PsiXP)
	setInt(Health, s.Health)
	setInt(Aim, s.Aim)
	setInt(Defense, s.Defense)
	setInt(Mobility, s.Mobility)
	setInt(Will, s.Will)
	s.applyLoadOut(setInt)
	setInt(TurnsOut, s.TurnsOut)
	setBool := func(f Field, v bool) {
		if p := s.bools[f]; p != nil {
			p.Value = v
		}
	}
	setBool(HasPsiGift, s.HasPsiGift)
	setBool(PsiTested, s.PsiTested)
	setBool(ForcePsiGift, s.ForcePsiGift)
	// A MEC Trooper cannot gain psi: the game marks a soldier it augments as
	// psi tested without the gift. Keep any MEC in that state.
	if s.IsMEC() {
		setBool(PsiTested, true)
		setBool(HasPsiGift, false)
		setBool(ForcePsiGift, false)
		setInt(PsiRank, 0)
		setInt(PsiXP, 0)
	}
	// Only rewrite the class if it was changed, so a localized strName is
	// kept otherwise.
	if s.Class != s.orig.Class && s.classKnown() && s.Present(Class) {
		c := Classes[s.Class]
		setStrProp(s.class.name, c.Name)
		s.class.typ.Value = c.Type
		s.class.weapon.Value = c.Weapon
	}
	s.applyMEC()
	s.applyGeneMods()
	s.syncCharClass()
}

// syncCharClass sets m_kChar.eClass to the soldier's class (kClass.eType).
// The game sets both when it promotes a Rookie (or makes a MEC) and checks
// m_kChar.eClass for some things, such as which soldiers the Genetics Lab
// will modify. Every soldier is synced, not just edited ones, which repairs
// soldiers given a class by earlier versions of the editor; a save the game
// wrote already matches, so it is unchanged. S.H.I.V.s and unknown (e.g.
// modded) classes are left alone.
func (s *Soldier) syncCharClass() {
	if s.mec.charClass == nil || s.class.typ == nil || s.IsSHIV() || !(s.classKnown() || s.IsMEC()) {
		return
	}
	s.mec.charClass.Value = s.class.typ.Value
}

// IsDead reports whether the soldier is dead (m_eStatus eStatus_Dead).
func (s *Soldier) IsDead() bool { return s.Status == "eStatus_Dead" }

// IsHeavy reports whether the soldier's (possibly edited) class is Heavy,
// the only class that carries a rocket, and the only one with no pistol.
func (s *Soldier) IsHeavy() bool {
	return s.Class >= 0 && s.Class < len(Classes) && Classes[s.Class].Type == "eSC_HeavyWeapons"
}

// classKnown reports whether the soldier's class is one of Classes.
func (s *Soldier) classKnown() bool { return s.Class >= 0 && s.Class < len(Classes) }

// applyLoadOut writes the load out and keeps the item counts consistent
// with it. Nothing is changed unless the load out or class was edited, so
// unusual load outs in a save survive untouched.
func (s *Soldier) applyLoadOut(setInt func(Field, int)) {
	v, o := s.soldierValues, s.orig
	if v.Armor == o.Armor && v.Pistol == o.Pistol && v.Weapon == o.Weapon && v.Rocket == o.Rocket &&
		v.Item1 == o.Item1 && v.Item2 == o.Item2 && v.Class == o.Class {
		return
	}
	setInt(Armor, v.Armor)
	if s.IsHeavy() {
		setInt(Pistol, 0) // Heavies carry a rocket instead of a pistol
	} else {
		setInt(Pistol, v.Pistol)
	}
	setInt(Weapon, v.Weapon)
	setInt(Item1, v.Item1)
	setInt(Item2, v.Item2)
	if s.classKnown() {
		// Only Heavies carry a rocket (large item slot 1).
		if s.IsHeavy() {
			setInt(Rocket, v.Rocket)
			setInt(NumLargeItems, 2)
		} else {
			setInt(Rocket, 0)
			setInt(NumLargeItems, 1)
		}
	} else {
		setInt(Rocket, v.Rocket)
	}
	if v.Item2 != 0 {
		setInt(NumSmallItems, 2)
	} else {
		setInt(NumSmallItems, 1)
	}
}

// validate checks the soldier's edited values against simple sanity limits.
func (s *Soldier) validate() error {
	const maxInt32 = 1<<31 - 1
	who := s.InstanceName
	// Values are only checked if edited: the game keeps some outside the
	// editable ranges (a S.H.I.V.'s rank is -1).
	if s.Rank != s.orig.Rank && (s.Rank < 0 || s.Rank > 7) {
		return fmt.Errorf("%s: rank must be 0-7 (got %d)", who, s.Rank)
	}
	v, o := s.soldierValues, s.orig
	// below reports whether any pair (value, original) was edited to a
	// value below min.
	below := func(min int, pairs ...[2]int) bool {
		for _, p := range pairs {
			if p[0] != p[1] && p[0] < min {
				return true
			}
		}
		return false
	}
	if below(0, [2]int{v.PsiRank, o.PsiRank}, [2]int{v.XP, o.XP}, [2]int{v.PsiXP, o.PsiXP}) {
		return fmt.Errorf("%s: psi rank and experience cannot be negative", who)
	}
	if s.Class != s.orig.Class && !s.classKnown() && !(s.IsMEC() && s.enemyWithin) {
		return fmt.Errorf("%s: invalid class %d", who, s.Class)
	}
	name := strings.TrimSpace(s.FirstName + " " + s.LastName)
	if name == "" {
		name = who
	}
	// A rookie promoted past Rookie must also be given a class.
	if s.orig.Rank == 0 && s.Rank > 0 && s.Class == 0 && s.Present(Class) {
		return fmt.Errorf("%s was promoted from Rookie: choose a class (not None) before saving", name)
	}
	// A MEC wears Base Augments or a MEC suit; other soldiers cannot.
	if s.Armor != s.orig.Armor && s.IsMEC() != isMECArmor(s.Armor) {
		if s.IsMEC() {
			return fmt.Errorf("%s is a MEC Trooper: armor must be Base Augments or a MEC suit", name)
		}
		return fmt.Errorf("%s: only MEC Troopers can wear MEC armor", name)
	}
	// A MEC Trooper cannot be a Rookie: a soldier made a MEC needs a rank.
	if s.IsMEC() && s.Class != s.orig.Class && s.Rank == 0 {
		return fmt.Errorf("%s was made a MEC Trooper: choose a rank above Rookie before saving", name)
	}
	if below(1, [2]int{v.Health, o.Health}) {
		return fmt.Errorf("%s: health must be at least 1", who)
	}
	if below(0, [2]int{v.Aim, o.Aim}, [2]int{v.Defense, o.Defense}, [2]int{v.Mobility, o.Mobility}, [2]int{v.Will, o.Will}) {
		return fmt.Errorf("%s: stats cannot be negative", who)
	}
	if below(0, [2]int{v.TurnsOut, o.TurnsOut}) {
		return fmt.Errorf("%s: turns out cannot be negative", who)
	}
	if below(0, [2]int{v.Armor, o.Armor}, [2]int{v.Pistol, o.Pistol}, [2]int{v.Weapon, o.Weapon},
		[2]int{v.Rocket, o.Rocket}, [2]int{v.Item1, o.Item1}, [2]int{v.Item2, o.Item2}) {
		return fmt.Errorf("%s: load out items cannot be negative", who)
	}
	for _, v := range []int{s.PsiRank, s.XP, s.PsiXP, s.Health, s.Aim, s.Defense, s.Mobility, s.Will,
		s.Armor, s.Pistol, s.Weapon, s.Rocket, s.Item1, s.Item2, s.TurnsOut} {
		if v > maxInt32 {
			return fmt.Errorf("%s: value too large", who)
		}
	}
	return nil
}
