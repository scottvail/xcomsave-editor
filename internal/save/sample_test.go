// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

import (
	"testing"

	"github.com/scottvail/go-xcomsave"
)

// The example saves the tests use. The sources are geoscape saves as the
// game wrote them; the samples are built from them by gen_samples_test.go
// (run `make samples`), which sets the values described below so the tests
// know exactly what each sample holds.
const (
	euSource     = "../../example/eu-source" // Enemy Unknown (Jan 2016, never edited)
	ewSource     = "../../example/ew-source" // Enemy Within (with a suited MEC and a gene mod)
	euSample     = "../../example/eu-sample"
	euSampleJSON = "../../example/eu-sample.json"
	ewSample     = "../../example/ew-sample"
	ewSampleJSON = "../../example/ew-sample.json"
)

// samples are the sample files, each as native and JSON.
var samples = []struct {
	path        string
	enemyWithin bool
}{
	{euSample, false}, {euSampleJSON, false}, {ewSample, true}, {ewSampleJSON, true},
}

// euSamples and ewSamples are the sample files of each game.
var (
	euSamples = []string{euSample, euSampleJSON}
	ewSamples = []string{ewSample, ewSampleJSON}
)

// Values the generator sets in both samples.
const (
	sampleCash          = 123456
	sampleDifficulty    = 2
	sampleLowest        = 1
	sampleScientists    = 42
	sampleEngineers     = 37
	sampleResearchHours = 200 // m_kProject.iActualHoursLeft

	sampleConstructionHours = 120 // m_arrConstructionProjects
	sampleFacilityHours     = 96  // m_arrFacilityProjects
	sampleItemHours         = 240 // m_arrItemProjects

	sampleAugmentHours = 72 // Enemy Within: the soldier in the Cybernetics Lab
	sampleGeneModHours = 48 // Enemy Within: the soldier in the Genetics Lab
)

// sampleOrders are the pending interceptor orders (m_akInterceptorOrders).
// The second leaves iDestinationContinent out, as the game does for 0.
var sampleOrders = []InterceptorOrder{
	{NumInterceptors: 1, DestinationContinent: 2, Hours: 66},
	{NumInterceptors: 2, Hours: 30},
}

// sampleSatelliteHours are the iTravelTime of the first satellites in
// m_arrSatellites: launched, not yet in place.
var sampleSatelliteHours = []int{72, 96, 120}

// sampleStorage are storage counts set by name, per game.
var sampleStorage = map[bool]map[string]int{
	false: {"Elerium": 500, "Alien Alloys": 700, "Weapon Fragments": 900, "Carapace Armor": 3},
	true:  {"Elerium": 600, "Alien Alloys": 800, "Meld": 120, "Mechtoid Core": 2},
}

// Soldier roles. Every soldier but MECs and S.H.I.V.s is a "varied"
// soldier given sampleSoldier values, except the last ones, which play
// special parts: the last is dead; in Enemy Within the one before is being
// augmented and the one before that gene modded. MECs and S.H.I.V.s are left
// as the source has them.
type soldierRoles struct {
	varied           []*Soldier
	dead             *Soldier
	augment, geneMod *Soldier // Enemy Within only
	mecs, shivs      []*Soldier
}

func sampleRoles(f *File) soldierRoles {
	var r soldierRoles
	var others []*Soldier
	for _, s := range f.Soldiers {
		switch {
		case s.IsMEC():
			r.mecs = append(r.mecs, s)
		case s.IsSHIV():
			r.shivs = append(r.shivs, s)
		default:
			others = append(others, s)
		}
	}
	r.dead, others = others[len(others)-1], others[:len(others)-1]
	if f.EnemyWithin() {
		r.augment, r.geneMod = others[len(others)-1], others[len(others)-2]
		others = others[:len(others)-2]
	}
	r.varied = others
	return r
}

// sampleSoldier is what the generator gives the i'th varied soldier: every
// rank in turn, each class, gear of the rank's tech level (conventional,
// laser, plasma), one or two small items, some with the psi gift, some
// wounded.
func sampleSoldier(i int, ew bool) soldierValues {
	rank := i % 8
	tier := 0
	switch {
	case rank >= 6:
		tier = 2
	case rank >= 3:
		tier = 1
	}
	v := soldierValues{Rank: rank, XP: rank*400 + i}
	if rank > 0 {
		v.Class = 1 + i%4 // Sniper, Assault, Support, Heavy
	}
	typ := Classes[v.Class].Type
	v.Weapon = classStandard[typ][tier]
	if typ == "eSC_HeavyWeapons" {
		v.Rocket = []int{7, 7, 19}[tier] // Rocket Launcher, Blaster Launcher
	} else {
		v.Pistol = []int{2, 8, 13}[tier] // Pistol, Laser Pistol, Plasma Pistol
	}
	armor := ArmorFor(ew)
	v.Armor = armor[min(tier*2+i%2, len(armor)-1)].Value
	small := SmallItemsFor(ew)
	v.Item1 = small[i%len(small)].Value
	if i%3 == 0 {
		v.Item2 = small[(i+4)%len(small)].Value
	}
	v.HasPsiGift = rank > 0 && i%5 == 0
	v.PsiTested = i%5 <= 1
	if v.HasPsiGift {
		v.PsiRank = 1 + i%3
		v.PsiXP = v.PsiRank * 120
	}
	if i%6 == 4 {
		v.TurnsOut = 3 + i
	}
	v.Health, v.Aim, v.Defense, v.Mobility, v.Will = 3+i%9, 55+i, i%3*5, 12+i%3, 30+2*i
	return v
}

// sampleShip is what the generator gives the i'th interceptor: alternately
// an Interceptor and a Firestorm, each weapon in turn, some grounded or
// damaged, on different continents.
type sampleShipValues struct {
	typeName      string
	weapon        string
	hoursDown     int
	damaged       bool // CurrentHP is half its HP
	homeContinent int
}

func sampleShip(i int) sampleShipValues {
	return sampleShipValues{
		typeName:      ShipNames[i%2],
		weapon:        ShipWeapons[i%len(ShipWeapons)].Value,
		hoursDown:     map[bool]int{true: 24 * (1 + i%2)}[i%3 == 1],
		damaged:       i%4 == 2,
		homeContinent: i % 5,
	}
}

// loadSample loads a sample, skipping the test if it is missing.
func loadSample(t *testing.T, path string) *File {
	t.Helper()
	f, err := Load(path)
	if err != nil {
		t.Skipf("%s: %v (run `make samples`)", path, err)
	}
	return f
}

// checkpoint returns f's first checkpoint of class, or nil.
func checkpoint(f *File, class string) *xcomsave.Checkpoint {
	for ci := range f.save.Checkpoints {
		for i := range f.save.Checkpoints[ci].Checkpoints {
			if c := &f.save.Checkpoints[ci].Checkpoints[i]; c.ClassName == class {
				return c
			}
		}
	}
	return nil
}

// soldierCheckpoint returns the checkpoint of soldier s.
func soldierCheckpoint(f *File, s *Soldier) *xcomsave.Checkpoint {
	for ci := range f.save.Checkpoints {
		for i := range f.save.Checkpoints[ci].Checkpoints {
			if c := &f.save.Checkpoints[ci].Checkpoints[i]; c.Name == s.Name {
				return c
			}
		}
	}
	return nil
}

// setProp replaces the property of p's name in props, or appends p.
func setProp(props *[]xcomsave.Property, p xcomsave.Property) {
	for i, q := range *props {
		if q.PropertyName() == p.PropertyName() {
			(*props)[i] = p
			return
		}
	}
	*props = append(*props, p)
}

// actorIndex returns the actor table index of the entry with instance name
// inst.
func actorIndex(f *File, inst string) int32 {
	for i, a := range f.save.Actors {
		if len(a) > len(inst) && a[len(a)-len(inst)-1:] == "."+inst {
			return int32(i)
		}
	}
	return -1
}
