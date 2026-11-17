// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

import (
	"slices"
	"strings"

	"github.com/scottvail/go-xcomsave"
)

// hoursLeft is an hours-remaining value of a project: an int property, or
// one element of a number array (arr[idx]). A project whose hours property
// the save leaves out has the default, 0, and is not tracked.
type hoursLeft struct {
	prop      *xcomsave.IntProperty
	arr       *xcomsave.NumberArrayProperty
	idx       int
	hours     int
	origHours int
}

func newHoursLeft(props []xcomsave.Property, name string) *hoursLeft {
	p, ok := xcomsave.Find(props, name).(*xcomsave.IntProperty)
	if !ok {
		return nil
	}
	return &hoursLeft{prop: p, hours: int(p.Value), origHours: int(p.Value)}
}

// arrayHoursLeft returns an hoursLeft for each element of props' number
// array property name.
func arrayHoursLeft(props []xcomsave.Property, name string) []*hoursLeft {
	a, ok := xcomsave.Find(props, name).(*xcomsave.NumberArrayProperty)
	if !ok {
		return nil
	}
	out := make([]*hoursLeft, len(a.Elements))
	for i, v := range a.Elements {
		out[i] = &hoursLeft{arr: a, idx: i, hours: int(v), origHours: int(v)}
	}
	return out
}

func (h *hoursLeft) modified() bool { return h != nil && h.hours != h.origHours }

func (h *hoursLeft) apply() {
	if !h.modified() {
		return
	}
	if h.arr != nil {
		h.arr.Elements[h.idx] = int32(h.hours)
	} else {
		h.prop.Value = int32(h.hours)
	}
}

// readConstruction records the iHoursLeft of every project in
// XGFacility_Engineering's m_arrConstructionProjects and
// m_arrFacilityProjects (construction), and m_arrItemProjects and
// m_arrFoundryProjects (item and Foundry projects).
func (f *File) readConstruction(c *xcomsave.Checkpoint) {
	f.construction = append(f.construction, readHoursLeft(c, "m_arrConstructionProjects", "m_arrFacilityProjects")...)
	f.itemProjects = append(f.itemProjects, readHoursLeft(c, "m_arrItemProjects", "m_arrFoundryProjects")...)
}

// readHoursLeft returns the iHoursLeft of every struct in c's struct array
// properties names.
func readHoursLeft(c *xcomsave.Checkpoint, names ...string) []*hoursLeft {
	var out []*hoursLeft
	for _, name := range names {
		a, ok := xcomsave.Find(c.Properties, name).(*xcomsave.StructArrayProperty)
		if !ok {
			continue
		}
		for _, st := range a.Structs {
			if h := newHoursLeft(st, "iHoursLeft"); h != nil {
				out = append(out, h)
			}
		}
	}
	return out
}

// pending returns the number of hs above 0 hours.
func pending(hs []*hoursLeft) int {
	n := 0
	for _, h := range hs {
		if h.hours > 0 {
			n++
		}
	}
	return n
}

// PendingItemProjects returns the number of item and Foundry projects
// (XGFacility_Engineering's m_arrItemProjects and m_arrFoundryProjects) with
// iHoursLeft above 0.
func (f *File) PendingItemProjects() int { return pending(f.itemProjects) }

// CompleteProjects sets every item and Foundry project's iHoursLeft to 0.
func (f *File) CompleteProjects() {
	for _, h := range f.itemProjects {
		h.hours = 0
	}
}

// readResearch records XGFacility_Labs' m_kProject.iActualHoursLeft.
func (f *File) readResearch(c *xcomsave.Checkpoint) {
	if p, ok := xcomsave.Find(c.Properties, "m_kProject").(*xcomsave.StructProperty); ok {
		f.research = newHoursLeft(p.Properties, "iActualHoursLeft")
	}
}

// PendingConstruction returns the number of construction and facility
// projects (XGFacility_Engineering's m_arrConstructionProjects and
// m_arrFacilityProjects) with iHoursLeft above 0.
func (f *File) PendingConstruction() int { return pending(f.construction) }

// FinishConstruction sets every construction and facility project's
// iHoursLeft to 0.
func (f *File) FinishConstruction() {
	for _, h := range f.construction {
		h.hours = 0
	}
}

// ResearchHoursLeft returns the current research project's hours left
// (XGFacility_Labs' m_kProject.iActualHoursLeft), or 0 if there is none.
func (f *File) ResearchHoursLeft() int {
	if f.research == nil {
		return 0
	}
	return f.research.hours
}

// FinishResearch sets the current research project's iActualHoursLeft to 0.
func (f *File) FinishResearch() {
	if f.research != nil {
		f.research.hours = 0
	}
}

func (f *File) projectsModified() bool {
	for _, h := range slices.Concat(f.construction, f.itemProjects, f.satellites, f.surgeries) {
		if h.modified() {
			return true
		}
	}
	return f.research.modified()
}

func (f *File) applyProjects() {
	for _, h := range slices.Concat(f.construction, f.itemProjects, f.satellites, f.surgeries) {
		h.apply()
	}
	f.research.apply()
}

// readSatellites records the iTravelTime of every satellite in
// XGHeadQuarters' m_arrSatellites: hours until a launched satellite is in
// place. An operational satellite has none (0).
func (f *File) readSatellites(c *xcomsave.Checkpoint) {
	a, ok := xcomsave.Find(c.Properties, "m_arrSatellites").(*xcomsave.StructArrayProperty)
	if !ok {
		return
	}
	for _, st := range a.Structs {
		if h := newHoursLeft(st, "iTravelTime"); h != nil {
			f.satellites = append(f.satellites, h)
		}
	}
}

// PendingSatellites returns the number of satellites still on their way
// (iTravelTime above 0).
func (f *File) PendingSatellites() int { return pending(f.satellites) }

// DeploySatellites sets every satellite's iTravelTime to 0, so all are in
// place.
func (f *File) DeploySatellites() {
	for _, h := range f.satellites {
		h.hours = 0
	}
}

// CyberneticsLabClass is the class of the checkpoint entry for Enemy
// Within's Cybernetics Lab, where soldiers are augmented into MECs.
const CyberneticsLabClass = "XComStrategyGame.XGFacility_CyberneticsLab"

// GeneLabsClass is the class of the checkpoint entry for Enemy Within's
// Genetics Lab, where soldiers are given gene mods.
const GeneLabsClass = "XComStrategyGame.XGFacility_GeneLabs"

// patient is a soldier in surgery: an m_arrPatients entry's m_kSoldier
// actor, and its hours left: one value (the Cybernetics Lab's m_iHoursLeft)
// or one per pending gene mod (the Genetics Lab's m_arrHoursLeft).
type patient struct {
	actor int32
	hours []*hoursLeft
	gene  bool
}

// readPatients records a lab's m_arrPatients: soldiers being augmented
// (Cybernetics Lab) or gene modded (Genetics Lab), with the hours left. The
// soldier's m_iTurnsOut counts down alongside.
func (f *File) readPatients(c *xcomsave.Checkpoint) {
	a, ok := xcomsave.Find(c.Properties, "m_arrPatients").(*xcomsave.StructArrayProperty)
	if !ok {
		return
	}
	gene := c.ClassName == GeneLabsClass
	for _, st := range a.Structs {
		s, ok := xcomsave.Find(st, "m_kSoldier").(*xcomsave.ObjectProperty)
		if !ok {
			continue
		}
		var hours []*hoursLeft
		if gene {
			hours = arrayHoursLeft(st, "m_arrHoursLeft")
		} else if h := newHoursLeft(st, "m_iHoursLeft"); h != nil {
			hours = []*hoursLeft{h}
		}
		if len(hours) > 0 {
			f.patients = append(f.patients, patient{actor: s.Actor, hours: hours, gene: gene})
			f.surgeries = append(f.surgeries, hours...)
		}
	}
}

// linkPatients attaches each patient's hours left to its soldier (by the
// actor table name, e.g. Command1.XGStrategySoldier_1).
func (f *File) linkPatients() {
	for _, p := range f.patients {
		if p.actor < 0 || int(p.actor) >= len(f.save.Actors) {
			continue
		}
		a := f.save.Actors[p.actor]
		inst := a[strings.LastIndexByte(a, '.')+1:]
		for _, s := range f.Soldiers {
			if s.InstanceName != inst {
				continue
			}
			if p.gene {
				s.geneMods = append(s.geneMods, p.hours...)
			} else {
				s.augment = append(s.augment, p.hours...)
			}
		}
	}
}

// maxHours returns the most hours left among hs.
func maxHours(hs []*hoursLeft) int {
	n := 0
	for _, h := range hs {
		n = max(n, h.hours)
	}
	return n
}

// AugmentHoursLeft returns the hours left on the soldier's augmentation in
// the Cybernetics Lab (m_iHoursLeft), or 0 if they are not being augmented.
func (s *Soldier) AugmentHoursLeft() int { return maxHours(s.augment) }

// GeneModHoursLeft returns the hours left on the soldier's gene mods in the
// Genetics Lab (the most of m_arrHoursLeft), or 0 if they have none pending.
func (s *Soldier) GeneModHoursLeft() int { return maxHours(s.geneMods) }

// FinishSurgery sets the hours left on the soldier's augmentation and gene
// mods to 0. Their m_iTurnsOut, which also counts down, is set separately.
func (s *Soldier) FinishSurgery() {
	for _, h := range slices.Concat(s.augment, s.geneMods) {
		h.hours = 0
	}
}
