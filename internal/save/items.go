// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

import "slices"

// StorageItem is an item kept in XGStorage's m_arrItems, which holds the
// number of each item type in storage. The numbers are Enemy Unknown's
// (version 0x0f); Enemy Within saves number their items differently.
type StorageItem struct {
	Element int // 1-based element number; the item's count is m_arrItems[Element-1]
	Name    string
	Group   string
}

// Index is the item's position in m_arrItems (its item type number).
func (it StorageItem) Index() int { return it.Element - 1 }

// Storage item groups, in display order.
const (
	GroupWeapons   = "Weapons"
	GroupArmor     = "Armor"
	GroupMECSuits  = "MEC Suits"
	GroupEquipment = "Equipment"
	GroupShivs     = "S.H.I.V.s"
	GroupAircraft  = "Aircraft & Satellites"
	GroupCorpses   = "Corpses"
	GroupResources = "Resources"
	GroupArtifacts = "Alien Artifacts"
)

// StorageGroups lists the groups in display order.
var StorageGroups = []string{
	GroupResources, GroupWeapons, GroupArmor, GroupMECSuits, GroupEquipment, GroupShivs,
	GroupAircraft, GroupArtifacts, GroupCorpses,
}

// StorageItems are the editable storage items of an Enemy Unknown save.
var StorageItems = []StorageItem{
	{9, "Laser Pistol", GroupWeapons},
	{10, "Laser Rifle", GroupWeapons},
	{11, "Scatter Laser", GroupWeapons},
	{12, "Heavy Laser", GroupWeapons},
	{13, "Laser Sniper Rifle", GroupWeapons},
	{14, "Plasma Pistol", GroupWeapons},
	{15, "Light Plasma Rifle", GroupWeapons},
	{16, "Plasma Rifle", GroupWeapons},
	{17, "Alloy Cannon", GroupWeapons},
	{18, "Heavy Plasma", GroupWeapons},
	{19, "Plasma Sniper Rifle", GroupWeapons},
	{20, "Blaster Launcher", GroupWeapons},

	{59, "Carapace Armor", GroupArmor},
	{60, "Skeleton Suit", GroupArmor},
	{61, "Titan Armor", GroupArmor},
	{62, "Archangel Armor", GroupArmor},
	{63, "Ghost Armor", GroupArmor},
	{64, "Psi Armor", GroupArmor},

	{77, "Medikit", GroupEquipment},
	{78, "Combat Stims", GroupEquipment},
	{79, "Mind Shield", GroupEquipment},
	{80, "Chitin Plating", GroupEquipment},
	{81, "Arc Thrower", GroupEquipment},
	{82, "S.C.O.P.E.", GroupEquipment},
	{83, "Nano-Fiber Vest", GroupEquipment},
	{89, "Alien Grenade", GroupEquipment},
	{100, "Battle Scanner", GroupEquipment},

	{103, "S.H.I.V.", GroupShivs},
	{104, "Alloy S.H.I.V.", GroupShivs},
	{105, "Hover S.H.I.V.", GroupShivs},

	{107, "Firestorm", GroupAircraft},
	{109, "Satellite", GroupAircraft},
	{124, "Phoenix Cannon", GroupAircraft},
	{126, "Laser Cannon", GroupAircraft},
	{127, "Plasma Cannon", GroupAircraft},
	{128, "EMP Cannon", GroupAircraft},
	{129, "Fusion Lance", GroupAircraft},
	{134, "Defense Matrix (Dodge)", GroupAircraft},
	{135, "UFO Tracking (Boost)", GroupAircraft},
	{136, "Uplink Targeting (Aim)", GroupAircraft},

	{145, "Sectoid Corpse", GroupCorpses},
	{146, "Sectoid Commander Corpse", GroupCorpses},
	{147, "Floater Corpse", GroupCorpses},
	{148, "Heavy Floater Corpse", GroupCorpses},
	{149, "Thin Man Corpse", GroupCorpses},
	{150, "Muton Corpse", GroupCorpses},
	{151, "Muton Elite Corpse", GroupCorpses},
	{152, "Berserker Corpse", GroupCorpses},
	{153, "Cyberdisc Wreck", GroupCorpses},
	{154, "Ethereal Corpse", GroupCorpses},
	{155, "Chryssalid Corpse", GroupCorpses},
	{157, "Sectopod Wreck", GroupCorpses},
	{158, "Drone Wreck", GroupCorpses},

	{172, "Elerium", GroupResources},
	{173, "Alien Alloys", GroupResources},
	{174, "Weapon Fragments", GroupResources},
	{189, "Fusion Core", GroupResources},
	{190, "Ethereal Device", GroupResources},
	{192, "Outsider Shard", GroupResources},
	{193, "Skeleton Key", GroupResources},

	{175, "Alien Entertainment", GroupArtifacts},
	{176, "Alien Food", GroupArtifacts},
	{177, "Alien Stasis Tank", GroupArtifacts},
	{178, "UFO Flight Computer", GroupArtifacts},
	{179, "Alien Surgery", GroupArtifacts},
	{180, "UFO Power Source", GroupArtifacts},
	{181, "Hyperwave Beacon", GroupArtifacts},
	{182, "Alien Entertainment (Damaged)", GroupArtifacts},
	{183, "Alien Food (Damaged)", GroupArtifacts},
	{184, "Alien Stasis Tank (Damaged)", GroupArtifacts},
	{185, "UFO Flight Computer (Damaged)", GroupArtifacts},
	{186, "Alien Surgery (Damaged)", GroupArtifacts},
	{187, "UFO Power Source (Damaged)", GroupArtifacts},
	{188, "Hyperwave Beacon (Damaged)", GroupArtifacts},
}

// EWStorageLen is the length of m_arrItems in the Enemy Within saves
// EWStorageItems was built from. Storage in an Enemy Within save with any
// other length is not editable.
const EWStorageLen = 226

// EWStorageItems are the editable storage items of an Enemy Within save.
// Enemy Within inserts, removes and reorders item types, so the numbers
// differ from Enemy Unknown's: weapons are unchanged, armor and equipment
// match the Enemy Within load out numbers (EWArmorItems, EWSmallItems),
// corpses through Weapon Fragments sit 10 lower, Meld is new, the artifacts
// through Ethereal Device sit 9 lower, Outsider Shard sits 8 lower, and
// Enemy Within's own items follow from element 187. Only item types
// identified in a real save are listed.
var EWStorageItems = []StorageItem{
	{9, "Laser Pistol", GroupWeapons},
	{10, "Laser Rifle", GroupWeapons},
	{11, "Scatter Laser", GroupWeapons},
	{12, "Heavy Laser", GroupWeapons},
	{13, "Laser Sniper Rifle", GroupWeapons},
	{14, "Plasma Pistol", GroupWeapons},
	{15, "Light Plasma Rifle", GroupWeapons},
	{16, "Plasma Rifle", GroupWeapons},
	{17, "Alloy Cannon", GroupWeapons},
	{18, "Heavy Plasma", GroupWeapons},
	{19, "Plasma Sniper Rifle", GroupWeapons},
	{20, "Blaster Launcher", GroupWeapons},
	{213, "EXALT Assault Rifle", GroupWeapons},
	{214, "EXALT Sniper Rifle", GroupWeapons},
	{215, "EXALT LMG", GroupWeapons},
	{216, "EXALT Laser Assault Rifle", GroupWeapons},
	{217, "EXALT Laser Sniper Rifle", GroupWeapons},
	{218, "EXALT Heavy Laser", GroupWeapons},
	{219, "EXALT Rocket Launcher", GroupWeapons},

	{61, "Carapace Armor", GroupArmor},
	{62, "Skeleton Suit", GroupArmor},
	{63, "Titan Armor", GroupArmor},
	{64, "Archangel Armor", GroupArmor},
	{65, "Ghost Armor", GroupArmor},
	{66, "Psi Armor", GroupArmor},

	{70, "Medikit", GroupEquipment},
	{72, "Mind Shield", GroupEquipment},
	{73, "Chitin Plating", GroupEquipment},
	{74, "Arc Thrower", GroupEquipment},
	{75, "S.C.O.P.E.", GroupEquipment},
	{76, "Nano-Fiber Vest", GroupEquipment},
	{77, "Respirator Implant", GroupEquipment},
	{79, "Reaper Rounds", GroupEquipment},
	{84, "Flashbang Grenade", GroupEquipment},
	{85, "Alien Grenade", GroupEquipment},
	{86, "Ghost Grenade", GroupEquipment},
	{87, "Gas Grenade", GroupEquipment},
	{88, "Needle Grenade", GroupEquipment},
	{89, "Mimic Beacon", GroupEquipment},

	{135, "Sectoid Corpse", GroupCorpses},
	{136, "Sectoid Commander Corpse", GroupCorpses},
	{137, "Floater Corpse", GroupCorpses},
	{138, "Heavy Floater Corpse", GroupCorpses},
	{139, "Thin Man Corpse", GroupCorpses},
	{140, "Muton Corpse", GroupCorpses},
	{141, "Muton Elite Corpse", GroupCorpses},
	{142, "Berserker Corpse", GroupCorpses},
	{143, "Cyberdisc Wreck", GroupCorpses},
	{144, "Ethereal Corpse", GroupCorpses},
	{145, "Chryssalid Corpse", GroupCorpses},
	{147, "Sectopod Wreck", GroupCorpses},
	{148, "Drone Wreck", GroupCorpses},
	{188, "Mechtoid Core", GroupCorpses},
	{189, "Seeker Wreck", GroupCorpses},

	{162, "Elerium", GroupResources},
	{163, "Alien Alloys", GroupResources},
	{164, "Weapon Fragments", GroupResources},
	{165, "Meld", GroupResources},
	{180, "Fusion Core", GroupResources},
	{181, "Ethereal Device", GroupResources},
	{184, "Outsider Shard", GroupResources},
	{185, "Skeleton Key", GroupResources},
	// Element 183 (Enemy Unknown's Outsider Shard, 192, less 9) is the
	// eItem_END_COLLECTIBLES marker and shows up in the game as an unknown
	// object.

	{166, "Alien Entertainment", GroupArtifacts},
	{167, "Alien Food", GroupArtifacts},
	{168, "Alien Stasis Tank", GroupArtifacts},
	{169, "UFO Flight Computer", GroupArtifacts},
	{170, "Alien Surgery", GroupArtifacts},
	{171, "UFO Power Source", GroupArtifacts},
	{172, "Hyperwave Beacon", GroupArtifacts},
	{173, "Alien Entertainment (Damaged)", GroupArtifacts},
	{174, "Alien Food (Damaged)", GroupArtifacts},
	{175, "Alien Stasis Tank (Damaged)", GroupArtifacts},
	{176, "UFO Flight Computer (Damaged)", GroupArtifacts},
	{177, "Alien Surgery (Damaged)", GroupArtifacts},
	{178, "UFO Power Source (Damaged)", GroupArtifacts},
	{179, "Hyperwave Beacon (Damaged)", GroupArtifacts},
	{182, "EXALT Intelligence", GroupArtifacts},
	{223, "Recovered Art", GroupArtifacts},
	{224, "EXALT Artifact", GroupArtifacts},
	{225, "EXALT Technology", GroupArtifacts},
	// MEC suits: element = the game's item number (MECArmorItems) + 1.
	{197, "Mk1-Kinetic", GroupMECSuits},
	{198, "Mk1-Flamethrower", GroupMECSuits},
	{199, "Mk2-Kinetic-Grenade", GroupMECSuits},
	{200, "Mk2-Kinetic-Mist", GroupMECSuits},
	{201, "Mk2-Flame-Grenade", GroupMECSuits},
	{202, "Mk2-Flame-Mist", GroupMECSuits},
	{203, "Mk3-Kinetic-Grenade-Electro", GroupMECSuits},
	{204, "Mk3-Kinetic-Grenade-Mine", GroupMECSuits},
	{205, "Mk3-Kinetic-Mist-Electro", GroupMECSuits},
	{206, "Mk3-Kinetic-Mist-Mine", GroupMECSuits},
	{207, "Mk3-Flame-Grenade-Electro", GroupMECSuits},
	{208, "Mk3-Flame-Grenade-Mine", GroupMECSuits},
	{209, "Mk3-Flame-Mist-Electro", GroupMECSuits},
	{210, "Mk3-Flame-Mist-Mine", GroupMECSuits},
}

// StorageItemsFor returns the storage items for a save (Enemy Within or
// not).
func StorageItemsFor(enemyWithin bool) []StorageItem {
	if enemyWithin {
		return EWStorageItems
	}
	return StorageItems
}

// LoadOutItem is an item type a soldier can carry in m_kChar.kInventory.
type LoadOutItem struct {
	Value int // item type number
	Name  string
}

// Load out items for each inventory slot. Pistols, weapons and rockets are
// numbered the same in Enemy Unknown (version 0x0f) and Enemy Within
// (0x10) saves; armor and small items are not.
var (
	PistolItems = []LoadOutItem{
		{2, "Pistol"},
		{8, "Laser Pistol"},
		{13, "Plasma Pistol"},
	}
	WeaponItems = []LoadOutItem{
		{3, "Assault Rifle"},
		{4, "Shotgun"},
		{5, "LMG"},
		{6, "Sniper Rifle"},
		{9, "Laser Rifle"},
		{10, "Scatter Laser"},
		{11, "Heavy Laser"},
		{12, "Laser Sniper Rifle"},
		{14, "Light Plasma Rifle"},
		{15, "Plasma Rifle"},
		{16, "Alloy Cannon"},
		{17, "Heavy Plasma"},
		{18, "Plasma Sniper Rifle"},
	}
	RocketItems = []LoadOutItem{
		{7, "Rocket Launcher"},
		{19, "Blaster Launcher"},
	}
)

// Enemy Unknown (version 0x0f) armor and small items: the storage item
// numbers in StorageItems.
var (
	EUArmorItems = []LoadOutItem{
		{57, "Body Armor"},
		{58, "Carapace Armor"},
		{59, "Skeleton Armor"},
		{60, "Titan Armor"},
		{61, "Archangel Armor"},
		{62, "Ghost Armor"},
		{63, "Psi Armor"},
	}
	EUSmallItems = []LoadOutItem{
		{76, "Medikit"},
		{77, "Combat Stims"},
		{78, "Mind Shield"},
		{79, "Chitin Plating"},
		{80, "Arc Thrower"},
		{81, "S.C.O.P.E."},
		{82, "Nano-Fiber Vest"},
		{85, "Frag Grenade"},
		{88, "Alien Grenade"},
		{99, "Battle Scanner"},
	}
)

// Enemy Within (version 0x10 and Android's 0x13) armor and small items, and
// the weapons and rockets that only exist in Enemy Within.
var (
	EWArmorItems = []LoadOutItem{
		{59, "Body Armor"},
		{60, "Carapace Armor"},
		{61, "Skeleton Armor"},
		{62, "Titan Armor"},
		{63, "Archangel Armor"},
		{64, "Ghost Armor"},
		{65, "Psi Armor"},
	}
	EWSmallItems = []LoadOutItem{
		{69, "Medkit"},
		{71, "Mind Shield"},
		{72, "Chitin Plating"},
		{73, "Arc Thrower"},
		{74, "S.C.O.P.E."},
		{75, "Nano-fiber Vest"},
		{76, "Respirator Implant"},
		{78, "Reaper Rounds"},
		{81, "Frag Grenade"},
		{83, "Flashbang Grenade"},
		{84, "Alien Grenade"},
		{85, "Ghost Grenade"},
		{86, "Gas Grenade"},
		{87, "Needle Grenade"},
		{88, "Mimic Beacon"},
	}
	EWWeaponItems = []LoadOutItem{
		{212, "EXALT Assault Rifle"},
		{213, "EXALT Sniper Rifle"},
		{214, "EXALT LMG"},
		{215, "EXALT Laser Assault Rifle"},
		{216, "EXALT Laser Sniper Rifle"},
		{217, "EXALT Heavy Laser"},
	}
	EWRocketItems = []LoadOutItem{
		{218, "EXALT Rocket Launcher"},
	}
)

// ArmorFor returns the armor for a save (Enemy Within or not).
func ArmorFor(enemyWithin bool) []LoadOutItem {
	if enemyWithin {
		return EWArmorItems
	}
	return EUArmorItems
}

// SmallItemsFor returns the small items for a save (Enemy Within or not).
func SmallItemsFor(enemyWithin bool) []LoadOutItem {
	if enemyWithin {
		return EWSmallItems
	}
	return EUSmallItems
}

// withEW returns base, plus ew if the save is from Enemy Within, ordered by
// item number.
func withEW(base, ew []LoadOutItem, enemyWithin bool) []LoadOutItem {
	if !enemyWithin {
		return base
	}
	out := append(slices.Clone(base), ew...)
	slices.SortStableFunc(out, func(a, b LoadOutItem) int { return a.Value - b.Value })
	return out
}

// RocketsFor returns the rockets for a save (Enemy Within or not).
func RocketsFor(enemyWithin bool) []LoadOutItem {
	return withEW(RocketItems, EWRocketItems, enemyWithin)
}

// weaponTier is each weapon's tech level: 0 conventional, 1 laser, 2 plasma.
var weaponTier = map[int]int{
	3: 0, 4: 0, 5: 0, 6: 0, 212: 0, 213: 0, 214: 0,
	9: 1, 10: 1, 11: 1, 12: 1, 215: 1, 216: 1, 217: 1,
	14: 2, 15: 2, 16: 2, 17: 2, 18: 2,
}

// classWeapons are the weapons each class (by kClass.eType) can carry, and
// classStandard its standard weapon of each tier.
var (
	classWeapons = map[string][]int{
		"eSC_None":         {3, 9, 14, 15, 212, 215},
		"eSC_Assault":      {3, 4, 9, 10, 14, 15, 16, 212, 215},
		"eSC_Support":      {3, 9, 14, 15, 212, 215},
		"eSC_Sniper":       {6, 12, 18, 213, 216},
		"eSC_HeavyWeapons": {5, 11, 17, 214, 217},
	}
	classStandard = map[string][3]int{
		"eSC_None":         {3, 9, 15},
		"eSC_Assault":      {3, 9, 15},
		"eSC_Support":      {3, 9, 15},
		"eSC_Sniper":       {6, 12, 18},
		"eSC_HeavyWeapons": {5, 11, 17},
	}
)

// MECWeaponItems are the weapons of Enemy Within's MEC Troopers (numbers and
// names from the game's EItemType and localization: eItem_Chaingun,
// eItem_Railgun, eItem_ParticleBeam).
var MECWeaponItems = []LoadOutItem{
	{28, "Minigun"},
	{29, "Railgun"},
	{30, "Particle Cannon"},
}

// isMECWeapon reports whether weapon is one of MECWeaponItems.
func isMECWeapon(weapon int) bool {
	return slices.ContainsFunc(MECWeaponItems, func(it LoadOutItem) bool { return it.Value == weapon })
}

// WeaponsFor returns the weapons a soldier of class (an index into Classes,
// or MECIndex) can carry, including Enemy Within's if enemyWithin: every
// weapon for UnknownClass.
func WeaponsFor(class int, enemyWithin bool) []LoadOutItem {
	if class == MECIndex && enemyWithin {
		return MECWeaponItems
	}
	all := withEW(WeaponItems, EWWeaponItems, enemyWithin)
	if class < 0 || class >= len(Classes) {
		return all
	}
	var out []LoadOutItem
	for _, it := range all {
		if slices.Contains(classWeapons[Classes[class].Type], it.Value) {
			out = append(out, it)
		}
	}
	return out
}

// ClassWeapon returns the weapon a soldier of class should carry in place
// of weapon: weapon itself if the class can carry it (or either is unknown),
// otherwise the class's standard weapon of the same tier.
func ClassWeapon(class, weapon int) int {
	tier, known := weaponTier[weapon]
	if class < 0 || class >= len(Classes) || !known {
		return weapon
	}
	t := Classes[class].Type
	if slices.Contains(classWeapons[t], weapon) {
		return weapon
	}
	return classStandard[t][tier]
}
