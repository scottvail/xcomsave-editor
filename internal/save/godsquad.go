// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

import "fmt"

// God Squad values: every soldier's base stats and ranks.
const (
	godHealth   = 25
	godAim      = 225
	godDefense  = 25
	godMobility = 48
	godWill     = 225
	godRank     = 7 // Colonel
	// godPsiRank 3 means Enemy Within's Genetics Lab will refuse the soldier
	// (tested in game), so God Squad gives the gene mods itself (godGeneMods).
	godPsiRank = 3
)

// godGeneMods are the gene mods God Squad gives each class in Enemy Within,
// by name (see BodyParts); every class also gets godSkin. Other body parts
// are left as they are.
var godGeneMods = map[string]string{
	"eSC_Sniper":       "Depth Perception",
	"eSC_HeavyWeapons": "Adrenal Neurosympathy",
	"eSC_Assault":      "Neural Feedback",
	"eSC_Support":      "Muscle Fiber Density",
}

const godSkin = "Bioelectric Skin"

// setGeneMod gives the soldier the gene mod called name, replacing the other
// mod for that body part. It reports whether name is a gene mod.
func (s *Soldier) setGeneMod(name string) bool {
	for i, p := range BodyParts {
		for j, m := range p.Mods {
			if m.Name == name {
				s.GeneMods[i] = j + 1
				return true
			}
		}
	}
	return false
}

// godClasses are the classes a classless soldier may be given, in the order
// ties are broken.
var godClasses = []string{"eSC_Assault", "eSC_Support", "eSC_HeavyWeapons", "eSC_Sniper"}

// godLoadOut is a class's God Squad load out, by item name (looked up in
// the save's game version's tables, since armor and small item numbers
// differ between Enemy Unknown and Enemy Within).
type godLoadOut struct {
	weapon, rocket int // item numbers (the same in both games); rocket 0 for none
	armor          string
	items          []string // small items, at most two
}

var godLoadOuts = map[string]godLoadOut{
	"eSC_Assault":      {weapon: 16, armor: "Ghost Armor", items: []string{"Alien Grenade", "Arc Thrower"}}, // Alloy Cannon
	"eSC_Support":      {weapon: 15, armor: "Ghost Armor", items: []string{"Arc Thrower", "Medikit"}},       // Plasma Rifle
	"eSC_HeavyWeapons": {weapon: 17, rocket: 19, armor: "Titan Armor", items: []string{"Alien Grenade"}},    // Heavy Plasma, Blaster Launcher
	"eSC_Sniper":       {weapon: 18, armor: "Archangel Armor", items: []string{"Alien Grenade"}},            // Plasma Sniper Rifle
}

// godPistol is the Plasma Pistol, carried by every class but the Heavy.
const godPistol = 13

// itemNumber returns the number of the item called name in items; "Medikit"
// also matches Enemy Within's "Medkit".
func itemNumber(items []LoadOutItem, name string) (int, error) {
	for _, it := range items {
		if it.Name == name || (name == "Medikit" && it.Name == "Medkit") {
			return it.Value, nil
		}
	}
	return 0, fmt.Errorf("no item %q", name)
}

// GodSquad makes every living soldier a Colonel with psi rank 3 and the psi
// gift, in Enemy Within gives them their class's gene mods (godGeneMods) and
// Bioelectric Skin, gives each classless soldier a class (each to whichever of Assault,
// Support, Heavy and Sniper has fewest living soldiers), gives every soldier
// the best weapons, armor and items for their class, and sets every
// soldier's health, aim, defense, mobility and will. Enemy Within's MEC
// Troopers only get the stats (they cannot gain psi, and their rank and load
// out are their own). Dead soldiers, S.H.I.V.s, and soldiers of any other
// class, are left alone. It returns the number of soldiers upgraded (MECs included), of
// those the number given a class, and the number left alone.
func (f *File) GodSquad() (upgraded, assigned, skipped int, err error) {
	ew := f.EnemyWithin()
	armor, small := ArmorFor(ew), SmallItemsFor(ew)

	counts := f.classCounts()
	for _, s := range f.Soldiers {
		if s.IsMEC() && !s.IsDead() {
			s.Health, s.Aim, s.Defense, s.Mobility, s.Will = godHealth, godAim, godDefense, godMobility, godWill
			upgraded++
			continue
		}
		if s.IsDead() || s.IsSHIV() || !s.classKnown() {
			skipped++
			continue
		}
		lo, ok := godLoadOuts[Classes[s.Class].Type]
		if s.Class == 0 { // eSC_None: give the soldier a class
			if !s.Present(Class) {
				skipped++
				continue
			}
			best := godClasses[0]
			for _, c := range godClasses[1:] {
				if counts[c] < counts[best] {
					best = c
				}
			}
			s.Class = classIndex(best)
			counts[best]++
			assigned++
			lo, ok = godLoadOuts[best]
		}
		if !ok {
			skipped++
			continue
		}
		upgraded++

		s.Rank, s.PsiRank = godRank, godPsiRank
		// Missing properties are ignored when saving. Force Psi Gift only
		// guarantees the gift at testing, which is already done.
		s.HasPsiGift, s.PsiTested, s.ForcePsiGift = true, true, false
		s.Health, s.Aim, s.Defense, s.Mobility, s.Will = godHealth, godAim, godDefense, godMobility, godWill
		if s.HasGeneMods() { // Enemy Within soldiers only
			for _, name := range []string{godGeneMods[Classes[s.Class].Type], godSkin} {
				if !s.setGeneMod(name) {
					return upgraded, assigned, skipped, fmt.Errorf("no gene mod %q", name)
				}
			}
		}

		if s.Armor, err = itemNumber(armor, lo.armor); err != nil {
			return upgraded, assigned, skipped, err
		}
		s.Weapon, s.Rocket, s.Pistol = lo.weapon, lo.rocket, godPistol
		if s.IsHeavy() {
			s.Pistol = 0
		}
		// Two small items need iNumSmallItems 2, which saving sets whenever
		// the second slot is filled.
		items := make([]int, 2)
		for i, name := range lo.items {
			if items[i], err = itemNumber(small, name); err != nil {
				return upgraded, assigned, skipped, err
			}
		}
		s.Item1, s.Item2 = items[0], items[1]
	}
	return upgraded, assigned, skipped, nil
}

// classCounts returns how many living soldiers have each class type.
func (f *File) classCounts() map[string]int {
	counts := map[string]int{}
	for _, s := range f.Soldiers {
		if s.classKnown() && !s.IsDead() && !s.IsSHIV() {
			counts[Classes[s.Class].Type]++
		}
	}
	return counts
}
