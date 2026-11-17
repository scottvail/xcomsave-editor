// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

// Gene mods (Enemy Within). A completed gene mod is a flag in the soldier's
// m_kChar.aUpgrades, indexed by the game's EPerkType enum (read from the
// game's XComGame.upk; ePerk_GeneMod_DepthPerception = 38 matches what the
// game wrote in a real save). A soldier has at most one of the two mods for
// each body part.

// GeneMod is one gene mod: its name and its m_kChar.aUpgrades index.
type GeneMod struct {
	Name    string
	Ability int
}

// BodyPart is a body part and the two gene mods it can take.
type BodyPart struct {
	Name string
	Mods [2]GeneMod
}

// NumBodyParts is the number of body parts that can be gene modded.
const NumBodyParts = 5

// BodyParts are the gene mods, by body part, as the Genetics Lab lists them.
var BodyParts = [NumBodyParts]BodyPart{
	{"Brain", [2]GeneMod{{"Neural Damping", 28}, {"Neural Feedback", 29}}},
	{"Eyes", [2]GeneMod{{"Hyper-Reactive Pupils", 30}, {"Depth Perception", 38}}},
	{"Chest", [2]GeneMod{{"Secondary Heart", 4}, {"Adrenal Neurosympathy", 20}}},
	{"Skin", [2]GeneMod{{"Bioelectric Skin", 42}, {"Mimetic Skin", 95}}},
	{"Legs", [2]GeneMod{{"Muscle Fiber Density", 50}, {"Adaptive Bone Marrow", 45}}},
}

// geneModAbilities are every gene mod's m_kChar.aUpgrades index.
var geneModAbilities = func() []int {
	var a []int
	for _, p := range BodyParts {
		for _, m := range p.Mods {
			a = append(a, m.Ability)
		}
	}
	return a
}()

// HasGeneMods reports whether the soldier's gene mods can be edited: in
// Enemy Within saves only (Enemy Unknown has no gene mods, and the same
// aUpgrades indexes are other abilities there), and not for MEC Troopers or
// S.H.I.V.s, who cannot be modded.
func (s *Soldier) HasGeneMods() bool {
	return s.enemyWithin && s.mec.abilities != nil && !s.IsMEC() && !s.IsSHIV()
}

// GeneModCount returns how many body parts have a gene mod.
func (s *Soldier) GeneModCount() int {
	n := 0
	for _, m := range s.GeneMods {
		if m != 0 {
			n++
		}
	}
	return n
}

// readGeneMods sets GeneMods from m_kChar.aUpgrades. Called once the save's
// game is known. GeneMods[i] is 0 for none, else 1 + the index into
// BodyParts[i].Mods.
func (s *Soldier) readGeneMods() {
	if !s.enemyWithin || s.mec.abilities == nil {
		return
	}
	for i, p := range BodyParts {
		s.GeneMods[i] = 0
		for j, m := range p.Mods {
			if v := element(s.mec.abilities, m.Ability); v != nil && v.Value != 0 && s.GeneMods[i] == 0 {
				s.GeneMods[i] = j + 1
			}
		}
	}
	s.orig.GeneMods = s.GeneMods
}

// applyGeneMods writes edited gene mods into m_kChar.aUpgrades. Body parts
// that were not edited are left as they are.
func (s *Soldier) applyGeneMods() {
	if !s.enemyWithin {
		return
	}
	for i, p := range BodyParts {
		if s.GeneMods[i] == s.orig.GeneMods[i] {
			continue
		}
		for j, m := range p.Mods {
			v := 0
			if s.GeneMods[i] == j+1 {
				v = 1
			}
			s.setAbility(m.Ability, v)
		}
	}
}
