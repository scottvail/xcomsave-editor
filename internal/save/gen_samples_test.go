// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

//go:build samples

package save

// Builds example/eu-sample and example/ew-sample (each native and JSON) from
// the source saves, setting the values sample_test.go describes. Run with
// `make samples` (go test -tags samples -run TestGenerateSamples).

import (
	"os"
	"testing"

	"github.com/scottvail/go-xcomsave"
)

func TestGenerateSamples(t *testing.T) {
	for _, g := range []struct{ source, native, json string }{
		{euSource, euSample, euSampleJSON},
		{ewSource, ewSample, ewSampleJSON},
	} {
		f := load(t, g.source)
		generateSample(t, f)
		data, err := f.Bytes(Native)
		if err != nil {
			t.Fatalf("%s: %v", g.source, err)
		}
		if err := os.WriteFile(g.native, data, 0o644); err != nil {
			t.Fatal(err)
		}
		// The JSON is the same save, as xcom2json would write it.
		s, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		js, err := s.Bytes(JSON)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(g.json, js, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d soldiers, %d interceptors -> %s, %s", g.source, len(f.Soldiers), len(f.Ships), g.native, g.json)
	}
}

func generateSample(t *testing.T, f *File) {
	ew := f.EnemyWithin()
	ip := func(name string, v int32) xcomsave.Property { return &xcomsave.IntProperty{Name: name, Value: v} }

	// Campaign values.
	f.Cash, f.Difficulty, f.LowestDifficulty = sampleCash, sampleDifficulty, sampleLowest
	f.Scientists, f.Engineers = sampleScientists, sampleEngineers
	for _, it := range f.StorageItems() {
		if n, ok := sampleStorage[ew][it.Name]; ok {
			f.Items[it.Index()] = n
		}
	}

	// Soldiers.
	roles := sampleRoles(f)
	for i, s := range roles.varied {
		v := sampleSoldier(i, ew)
		// Keep names and anything the generator does not set.
		v.FirstName, v.LastName, v.NickName, v.ForcePsiGift = s.FirstName, s.LastName, s.NickName, false
		s.soldierValues = v
	}
	status := func(s *Soldier, value string) {
		c := soldierCheckpoint(f, s)
		setProp(&c.Properties, &xcomsave.ByteProperty{Name: "m_eStatus", Type: "ESoldierStatus", Value: value})
	}
	status(roles.dead, "eStatus_Dead")
	if ew {
		roles.augment.TurnsOut = sampleAugmentHours
		status(roles.augment, "eStatus_Augmenting")
		lab := checkpoint(f, CyberneticsLabClass)
		setProp(&lab.Properties, &xcomsave.StructArrayProperty{Name: "m_arrPatients", Structs: [][]xcomsave.Property{{
			&xcomsave.ObjectProperty{Name: "m_kSoldier", Actor: actorIndex(f, roles.augment.InstanceName)},
			ip("m_iHoursLeft", sampleAugmentHours),
		}}})
		roles.geneMod.TurnsOut = sampleGeneModHours
		status(roles.geneMod, "eStatus_GeneMod")
		genes := checkpoint(f, GeneLabsClass)
		setProp(&genes.Properties, &xcomsave.StructArrayProperty{Name: "m_arrPatients", Structs: [][]xcomsave.Property{{
			&xcomsave.ObjectProperty{Name: "m_kSoldier", Actor: actorIndex(f, roles.geneMod.InstanceName)},
			&xcomsave.NumberArrayProperty{Name: "m_arrHoursLeft", Elements: []int32{sampleGeneModHours}},
			&xcomsave.EnumArrayProperty{Name: "m_arrPendingGeneMods", Values: []xcomsave.EnumValue{{Value: "ePerk_GeneMod_DepthPerception"}}},
		}}})
	}

	// Interceptors.
	for i, s := range f.Ships {
		v := sampleShip(i)
		s.SetName(v.typeName)
		s.Weapon, s.HoursDown, s.HomeContinent = v.weapon, v.hoursDown, v.homeContinent
		s.CurrentHP = s.HP
		if v.damaged {
			s.CurrentHP = s.HP / 2
		}
	}

	// HQ: interceptor orders and satellites on their way.
	hq := checkpoint(f, HeadquartersClass)
	orders := &xcomsave.StructArrayProperty{Name: "m_akInterceptorOrders"}
	for _, o := range sampleOrders {
		st := []xcomsave.Property{ip("iNumInterceptors", int32(o.NumInterceptors))}
		if o.DestinationContinent != 0 {
			st = append(st, ip("iDestinationContinent", int32(o.DestinationContinent)))
		}
		orders.Structs = append(orders.Structs, append(st, ip("iHours", int32(o.Hours))))
	}
	setProp(&hq.Properties, orders)
	sats := xcomsave.Find(hq.Properties, "m_arrSatellites").(*xcomsave.StructArrayProperty)
	for i, h := range sampleSatelliteHours {
		setProp(&sats.Structs[i], ip("iTravelTime", int32(h)))
	}

	// Engineering: construction, a facility and an item project under way.
	eng := checkpoint(f, EngineeringClass)
	setProp(&eng.Properties, &xcomsave.StructArrayProperty{Name: "m_arrConstructionProjects", Structs: [][]xcomsave.Property{
		{ip("iHoursLeft", sampleConstructionHours), ip("X", 4), ip("Y", 1)},
	}})
	setProp(&eng.Properties, &xcomsave.StructArrayProperty{Name: "m_arrFacilityProjects", Structs: [][]xcomsave.Property{{
		&xcomsave.ByteProperty{Name: "eFacility", Type: "EFacilityType", Value: "eFacility_AccessLift"},
		ip("iHoursLeft", sampleFacilityHours), &xcomsave.BoolProperty{Name: "bNotify", Value: true}, ip("X", 3), ip("Y", 2),
	}}})
	setProp(&eng.Properties, &xcomsave.StructArrayProperty{Name: "m_arrItemProjects", Structs: [][]xcomsave.Property{{
		&xcomsave.ByteProperty{Name: "eItem", Type: "EItemType", Value: "eItem_Satellite"},
		ip("iEngineers", 5), ip("iMaxEngineers", 5), ip("iQuantity", 3), ip("iQuantityLeft", 3),
		ip("iHoursLeft", sampleItemHours), &xcomsave.BoolProperty{Name: "bNotify", Value: true},
	}}})

	// Research under way.
	labs := checkpoint(f, LabsClass)
	project := xcomsave.Find(labs.Properties, "m_kProject").(*xcomsave.StructProperty)
	setProp(&project.Properties, ip("iActualHoursLeft", sampleResearchHours))
}
