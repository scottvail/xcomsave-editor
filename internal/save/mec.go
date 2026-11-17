// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

import (
	"slices"

	"github.com/scottvail/go-xcomsave"
)

// MEC Troopers (Enemy Within). The values below are as the game set them
// when it augmented a soldier into a MEC in a real save.

// MECIndex is the Soldier.Class value of a MEC Trooper. It follows Classes
// rather than being one of them: MECs have no standard load out, so they
// are left out of the class lists used for load outs and God Squad.
const MECIndex = 5

// MECLabel is the MEC Trooper's class name (kClass.strName).
const MECLabel = "MEC Trooper"

const (
	mecArmor  = 192 // armor of a MEC wearing no MEC suit
	mecWeapon = 28  // a new MEC's weapon, the Minigun (its only large item; see MECWeaponItems)
)

// mecAbilities are the m_kChar.aUpgrades entries a MEC gains, and
// soldierAbilities those it loses (every other soldier has them). A MEC also
// loses every gene mod (geneModAbilities).
var (
	mecAbilities     = []int{121, 145} // ePerk_Foundry_MECCloseCombat, ePerk_BodyShield
	soldierAbilities = []int{165}      // ePerk_Foundry_TacticalRigging
)

// MECArmorItems are the armor a MEC Trooper can wear: Base Augments (no
// suit) and the MEC suits, by frame and modules (Kinetic Strike,
// Flamethrower, Grenade Launcher, Restorative Mist, Electro Pulse, Proximity
// Mine Launcher). Numbers are the game's
// EItemType (eItem_MecCivvies, eItem_MecArmor1_Kinetic, ...), which are also
// their XGStorage element indexes; eItem_MecArmor1-3 (193-195), frames with
// no modules, are not worn.
var MECArmorItems = []LoadOutItem{
	{192, "Base Augments (no suit)"},
	{196, "MEC-1 Warden: Kinetic"},
	{197, "MEC-1 Warden: Flame"},
	{198, "MEC-2 Sentinel: Kinetic, Grenade"},
	{199, "MEC-2 Sentinel: Kinetic, Mist"},
	{200, "MEC-2 Sentinel: Flame, Grenade"},
	{201, "MEC-2 Sentinel: Flame, Mist"},
	{202, "MEC-3 Paladin: Kinetic, Grenade, Electro"},
	{203, "MEC-3 Paladin: Kinetic, Grenade, Mines"},
	{204, "MEC-3 Paladin: Kinetic, Mist, Electro"},
	{205, "MEC-3 Paladin: Kinetic, Mist, Mines"},
	{206, "MEC-3 Paladin: Flame, Grenade, Electro"},
	{207, "MEC-3 Paladin: Flame, Grenade, Mines"},
	{208, "MEC-3 Paladin: Flame, Mist, Electro"},
	{209, "MEC-3 Paladin: Flame, Mist, Mines"},
}

// isMECArmor reports whether armor is one of MECArmorItems.
func isMECArmor(armor int) bool {
	return slices.ContainsFunc(MECArmorItems, func(it LoadOutItem) bool { return it.Value == armor })
}

// isMECSuit reports whether armor is a MEC suit (a storage item), not Base
// Augments.
func isMECSuit(armor int) bool { return armor != mecArmor && isMECArmor(armor) }

func init() {
	if MECIndex != len(Classes) {
		panic("save: MECIndex must follow Classes")
	}
}

// mecProps are the soldier properties involved in becoming, or ceasing to
// be, a MEC, beyond those in specs.
type mecProps struct {
	charType  *xcomsave.IntProperty         // m_kChar.iType: 2 a soldier, 3 a S.H.I.V.
	charClass *xcomsave.ByteProperty        // m_kChar.eClass
	abilities *xcomsave.StaticArrayProperty // m_kChar.aUpgrades
	backup    inventory                     // m_kBackedUpLoadout
	numLarge  *xcomsave.IntProperty         // m_kChar.kInventory.iNumLargeItems
	numSmall  *xcomsave.IntProperty         // m_kChar.kInventory.iNumSmallItems
	claimed   int                           // the MEC suit this soldier's claim in XGStorage is for (0 none); see applyMECSuits
}

// inventory is a TInventory struct's load out properties.
type inventory struct {
	armor, pistol, numLarge, numSmall *xcomsave.IntProperty
	large, small                      *xcomsave.StaticArrayProperty
}

func readInventory(props []xcomsave.Property) inventory {
	i := func(name string) *xcomsave.IntProperty {
		p, _ := xcomsave.Find(props, name).(*xcomsave.IntProperty)
		return p
	}
	a := func(name string) *xcomsave.StaticArrayProperty {
		p, _ := xcomsave.Find(props, name).(*xcomsave.StaticArrayProperty)
		return p
	}
	return inventory{
		armor: i("iArmor"), pistol: i("iPistol"), numLarge: i("iNumLargeItems"), numSmall: i("iNumSmallItems"),
		large: a("arrLargeItems"), small: a("arrSmallItems"),
	}
}

// element returns a's i'th element, or nil.
func element(a *xcomsave.StaticArrayProperty, i int) *xcomsave.IntProperty {
	if a == nil || i < 0 || i >= len(a.Properties) {
		return nil
	}
	p, _ := a.Properties[i].(*xcomsave.IntProperty)
	return p
}

func readMECProps(c *xcomsave.Checkpoint) mecProps {
	var m mecProps
	if char, ok := xcomsave.Find(c.Properties, "m_kChar").(*xcomsave.StructProperty); ok {
		m.charType, _ = xcomsave.Find(char.Properties, "iType").(*xcomsave.IntProperty)
		m.charClass, _ = xcomsave.Find(char.Properties, "eClass").(*xcomsave.ByteProperty)
		m.abilities, _ = xcomsave.Find(char.Properties, "aUpgrades").(*xcomsave.StaticArrayProperty)
		if inv, ok := xcomsave.Find(char.Properties, "kInventory").(*xcomsave.StructProperty); ok {
			cur := readInventory(inv.Properties)
			m.numLarge, m.numSmall = cur.numLarge, cur.numSmall
		}
	}
	if b, ok := xcomsave.Find(c.Properties, "m_kBackedUpLoadout").(*xcomsave.StructProperty); ok {
		m.backup = readInventory(b.Properties)
	}
	return m
}

// IsMEC reports whether the soldier is (or has been made) an Enemy Within
// MEC Trooper. MECs cannot gain psi abilities or gene mods.
func (s *Soldier) IsMEC() bool { return s.Class == MECIndex }

// shivType is m_kChar.iType for a S.H.I.V. (soldiers are 2).
const shivType = 3

// IsSHIV reports whether the entry is a S.H.I.V. rather than a soldier: the
// game keeps its robot tanks as XGStrategySoldier entries (with rank -1 and
// no class). Their rank, class, psi and load out are not editable.
func (s *Soldier) IsSHIV() bool { return s.mec.charType != nil && s.mec.charType.Value == shivType }

// S.H.I.V. decks (worn as armor, m_kChar.kInventory.iArmor) and weapons
// (arrLargeItems[0]): EItemType values, which differ between Enemy Unknown
// and Enemy Within. Names from XComGame.int's m_aItemNames.
var (
	euSHIVDecks   = []LoadOutItem{{117, "Tread Deck"}, {118, "Heavy Alloy Deck"}, {119, "Hover Deck"}}
	ewSHIVDecks   = []LoadOutItem{{113, "Tread Deck"}, {114, "Heavy Alloy Deck"}, {115, "Hover Deck"}}
	euSHIVWeapons = []LoadOutItem{{113, "Minigun"}, {114, "Sentry"}, {115, "Laser Cannon"}, {116, "Plasma Cannon"}}
	ewSHIVWeapons = []LoadOutItem{{109, "Minigun"}, {110, "Sentry"}, {111, "Laser Cannon"}, {112, "Plasma Cannon"}}
)

// shivTypes are the S.H.I.V. type names (m_aItemNames[eItem_SHIV],
// [eItem_SHIV_Alloy], [eItem_SHIV_Hover]) by deck.
var shivTypes = []string{"S.H.I.V.", "Alloy S.H.I.V.", "Hover S.H.I.V."}

// SHIVDecksFor and SHIVWeaponsFor return the S.H.I.V. decks and weapons
// for Enemy Within (ew) or Enemy Unknown saves.
func SHIVDecksFor(ew bool) []LoadOutItem {
	if ew {
		return ewSHIVDecks
	}
	return euSHIVDecks
}

func SHIVWeaponsFor(ew bool) []LoadOutItem {
	if ew {
		return ewSHIVWeapons
	}
	return euSHIVWeapons
}

// SHIVType returns a S.H.I.V.'s type, e.g. "Alloy S.H.I.V.", from its deck
// (plain "S.H.I.V." if the deck is not known), or "" for a soldier.
func (s *Soldier) SHIVType() string {
	if !s.IsSHIV() {
		return ""
	}
	for i, d := range SHIVDecksFor(s.enemyWithin) {
		if d.Value == s.Armor {
			return shivTypes[i]
		}
	}
	return shivTypes[0]
}

// SetClass changes the soldier's class to class (an index into Classes, or
// MECIndex). Becoming a MEC gives the soldier a MEC's load out (no suit, the
// MEC weapon, no pistol or small items) and clears their psi and gene mods. Ceasing to be
// a MEC takes off any MEC suit (it stays in storage) and restores the load
// out the game backed up, adjusted to the new class, or else the class's
// basic one. Other class changes keep the load out, swapping the weapon for
// the class's equivalent.
func (s *Soldier) SetClass(class int) {
	was := s.IsMEC()
	s.Class = class
	switch {
	case s.IsMEC() && !was:
		s.Armor, s.Pistol, s.Weapon, s.Rocket, s.Item1, s.Item2 = mecArmor, 0, mecWeapon, 0, 0, 0
		s.HasPsiGift, s.PsiTested, s.ForcePsiGift, s.PsiRank, s.PsiXP = false, true, false, 0, 0
		s.GeneMods = [NumBodyParts]int{}
	case was && !s.IsMEC():
		s.restoreLoadOut()
	default:
		s.Weapon = ClassWeapon(s.Class, s.Weapon)
	}
}

// restoreLoadOut gives a soldier leaving the MECs a load out for their new
// class: the backed-up one if the game kept it, else the class's basics.
func (s *Soldier) restoreLoadOut() {
	b := s.mec.backup
	val := func(p *xcomsave.IntProperty) int {
		if p == nil {
			return 0
		}
		return int(p.Value)
	}
	armor, pistol := val(b.armor), val(b.pistol)
	weapon, rocket := val(element(b.large, 0)), val(element(b.large, 1))
	item1, item2 := val(element(b.small, 0)), val(element(b.small, 1))
	if armor == 0 || weapon == 0 || isMECArmor(armor) || isMECWeapon(weapon) {
		// No usable backup: Body Armor and the class's conventional gear.
		armor = ArmorFor(s.enemyWithin)[0].Value
		pistol, weapon, rocket, item1, item2 = 2, 3, 0, 0, 0 // Pistol, Assault Rifle
		if s.classKnown() {
			weapon = classStandard[Classes[s.Class].Type][0]
		}
	}
	s.Armor, s.Pistol, s.Weapon, s.Item1, s.Item2 = armor, pistol, ClassWeapon(s.Class, weapon), item1, item2
	s.Rocket = 0
	if s.IsHeavy() {
		s.Pistol, s.Rocket = 0, rocket
		if s.Rocket == 0 {
			s.Rocket = 7 // Rocket Launcher
		}
	}
}

// setAbility sets m_kChar.aUpgrades[i] to v.
func (s *Soldier) setAbility(i, v int) {
	if p := element(s.mec.abilities, i); p != nil {
		p.Value = int32(v)
	}
}

// applyMEC writes what becoming, or ceasing to be, a MEC changes beyond the
// fields in specs: the class, abilities, item counts, and the backed-up load
// out. (m_kChar.eClass follows the class: see syncCharClass.)
func (s *Soldier) applyMEC() {
	was, is := s.orig.Class == MECIndex, s.IsMEC()
	if is {
		// A MEC carries just its weapon.
		if s.mec.numLarge != nil {
			s.mec.numLarge.Value = 1
		}
		if s.mec.numSmall != nil {
			s.mec.numSmall.Value = 0
		}
	}
	if was == is {
		return
	}
	if is {
		if s.Present(Class) {
			setStrProp(s.class.name, MECLabel)
			s.class.typ.Value = MECClass // eWeaponType is kept, as the game does
		}
		for _, i := range mecAbilities {
			s.setAbility(i, 1)
		}
		for _, i := range slices.Concat(soldierAbilities, geneModAbilities) {
			s.setAbility(i, 0)
		}
		s.backUpLoadOut()
		return
	}
	for _, i := range mecAbilities {
		s.setAbility(i, 0)
	}
	for _, i := range soldierAbilities {
		s.setAbility(i, 1)
	}
}

// backUpLoadOut stores the soldier's load out before becoming a MEC in
// m_kBackedUpLoadout, as the game does.
func (s *Soldier) backUpLoadOut() {
	b, o := s.mec.backup, s.orig
	set := func(p *xcomsave.IntProperty, v int) {
		if p != nil {
			p.Value = int32(v)
		}
	}
	set(b.armor, o.Armor)
	set(b.pistol, o.Pistol)
	set(element(b.large, 0), o.Weapon)
	set(element(b.large, 1), o.Rocket)
	set(element(b.small, 0), o.Item1)
	set(element(b.small, 1), o.Item2)
	set(b.numLarge, 1+min(o.Rocket, 1))
	set(b.numSmall, 1+min(o.Item2, 1))
}

// applyMECSuits keeps XGStorage in step with the MEC suits soldiers wear. A
// suit is a storage item: the game counts it in m_arrItems while it is worn,
// and also in m_arrClaims (items in use). Taking a suit off (changing it, or
// leaving the MECs) releases its claim, leaving the suit in storage; putting
// one on claims it, and if none is free in storage one is added, as building
// it would (m_arrItems, and m_arrItemArchives, the count ever had). Each
// soldier's claim is tracked, so writing the save again changes nothing more.
func (f *File) applyMECSuits() {
	if f.claimsProp == nil {
		return
	}
	claims := f.claimsProp.Elements
	inRange := func(i int) bool { return i > 0 && i < len(claims) }
	want := func(s *Soldier) int {
		if s.IsMEC() && isMECSuit(s.Armor) {
			return s.Armor
		}
		return 0
	}
	// Release every changed suit first, so a suit one soldier takes off is
	// free for another to put on.
	for _, s := range f.Soldiers {
		if c := s.mec.claimed; c != want(s) && inRange(c) && claims[c] > 0 {
			claims[c]--
		}
	}
	for _, s := range f.Soldiers {
		w := want(s)
		if w == s.mec.claimed {
			continue
		}
		if inRange(w) {
			if f.itemsProp != nil && w < len(f.Items) && f.Items[w] <= int(claims[w]) {
				f.Items[w]++
				if f.archivesProp != nil && w < len(f.archivesProp.Elements) {
					f.archivesProp.Elements[w]++
				}
			}
			claims[w]++
		}
		s.mec.claimed = w
	}
}

// SuitAdded reports whether saving will add a MEC suit armor to storage for
// the soldier to wear: none is in storage that isn't worn by someone else
// (counting one the soldier wore when the save was last written). False for
// Base Augments and other armor, or with no storage to check.
func (f *File) SuitAdded(s *Soldier, armor int) bool {
	if !isMECSuit(armor) || f.claimsProp == nil || f.itemsProp == nil || armor >= len(f.Items) || armor >= len(f.claimsProp.Elements) {
		return false
	}
	free := f.Items[armor] - int(f.claimsProp.Elements[armor])
	if s.mec.claimed == armor {
		free++
	}
	return free < 1
}
