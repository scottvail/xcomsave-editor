// SPDX-License-Identifier: GPL-2.0-or-later
// Copyright (C) 2026 Scott Vail

package save

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/scottvail/go-xcomsave"
)

// InterceptorClass is the class of the checkpoint entries holding XCOM's
// interceptors (XGShip_Interceptor_<n>).
const InterceptorClass = "XComStrategyGame.XGShip_Interceptor"

// ShipNames are the values an interceptor's m_kTShip.strName may take.
var ShipNames = []string{"Interceptor", "Firestorm"}

// ShipTypes are the values an interceptor's m_kTShip.eType (EShipType) may
// take: ShipTypes[i] goes with ShipNames[i].
var ShipTypes = []string{"eShip_Interceptor", "eShip_Firestorm"}

// ShipGraphics are the values the m_eEntityGraphic (EEntityGraphic) of an
// interceptor's XGShipEntity may take: ShipGraphics[i] goes with
// ShipNames[i]. The saves seen so far only hold eEntityGraphic_Firestorm;
// eEntityGraphic_Interceptor follows the same naming.
var ShipGraphics = []string{"eEntityGraphic_Interceptor", "eEntityGraphic_Firestorm"}

// ShipEntityClass is the class of the checkpoint entry an interceptor's
// m_kEntity refers to: its model on the Geoscape.
const ShipEntityClass = "XComStrategyGame.XGShipEntity"

// shipStats are a ship type's m_kTShip stats.
type shipStats struct {
	Speed, EngagementSpeed, HP, AP, Armor int
}

// ewShipDefaults are the stats of each ship type in Enemy Within
// (ewShipDefaults[i] goes with ShipNames[i]), as found in real saves: the
// Interceptor's from a save before any upgrades, the Firestorm's from one
// with working Firestorms. The game judges a ship's health against its
// type's hit points, so a ship changed to a tougher type keeping its old
// m_iHP shows as damaged.
var ewShipDefaults = []shipStats{
	{Speed: 1500, EngagementSpeed: 10, HP: 2500, AP: 0, Armor: 5},
	{Speed: 3500, EngagementSpeed: 15, HP: 6500, AP: 16, Armor: 25},
}

// euShipDefaults are the Enemy Unknown equivalents, from unedited Enemy
// Unknown saves: the Interceptor's are the same as in Enemy Within, the
// Firestorm's engagement speed and armor penetration are higher.
var euShipDefaults = []shipStats{
	{Speed: 1500, EngagementSpeed: 10, HP: 2500, AP: 0, Armor: 5},
	{Speed: 3500, EngagementSpeed: 20, HP: 6500, AP: 34, Armor: 25},
}

// shipDefaults returns the ship type stats for a save's game.
func shipDefaults(enemyWithin bool) []shipStats {
	if enemyWithin {
		return ewShipDefaults
	}
	return euShipDefaults
}

// SetName sets m_kTShip.strName to name (one of ShipNames), and eType and
// the ship entity's m_eEntityGraphic to the matching ShipTypes and
// ShipGraphics values. If the name changes, it also sets the m_kTShip stats
// to that type's in the save's game (shipDefaults) and m_iHP to its full hit
// points. Other names are ignored.
func (s *Ship) SetName(name string) {
	i := slices.Index(ShipNames, name)
	if i < 0 {
		return
	}
	changed := name != s.TypeName
	s.TypeName, s.Type = name, ShipTypes[i]
	if s.graphic != nil {
		s.Graphic = ShipGraphics[i]
	}
	if !changed {
		return
	}
	d := shipDefaults(s.enemyWithin)[i]
	for stat, v := range map[string]int{
		"iSpeed": d.Speed, "iEngagementSpeed": d.EngagementSpeed, "iHP": d.HP, "iAP": d.AP, "iArmor": d.Armor,
	} {
		if s.HasStat(stat) {
			*s.Stat(stat) = v
		}
	}
	if s.HasCurrentHP() {
		s.CurrentHP = d.HP
	}
}

// ShipWeapon is an interceptor weapon: m_eWeapon's EItemType value, and the
// number m_kTShip.arrWeapons holds for it.
type ShipWeapon struct {
	Value  string
	Name   string
	ArrNum int
	Slots  int // the length of arrWeapons with this weapon
}

// ShipWeapons are the interceptor weapons. The arrWeapons numbers and slot
// counts are as found in real Enemy Unknown and Enemy Within saves: I is
// [1], II is [3, 0], III [4], V [6] and VI [7] (IV, never seen, is taken to
// be [5]). The names are inferred from the order of the Enemy Unknown
// storage items and have not all been confirmed in game.
var ShipWeapons = []ShipWeapon{
	{"eItem_IntWeap_I", "Phoenix Cannon", 1, 1},
	{"eItem_IntWeap_II", "Avalanche Missiles", 3, 2},
	{"eItem_IntWeap_III", "Laser Cannon", 4, 1},
	{"eItem_IntWeap_IV", "Plasma Cannon", 5, 1},
	{"eItem_IntWeap_V", "EMP Cannon", 6, 1},
	{"eItem_IntWeap_VI", "Fusion Lance", 7, 1},
}

// shipWeapon returns the ShipWeapon for an m_eWeapon value.
func shipWeapon(value string) (ShipWeapon, bool) {
	i := slices.IndexFunc(ShipWeapons, func(w ShipWeapon) bool { return w.Value == value })
	if i < 0 {
		return ShipWeapon{}, false
	}
	return ShipWeapons[i], true
}

// shipValues are an interceptor's editable values.
type shipValues struct {
	Callsign        string // m_strCallsign
	Weapon          string // m_eWeapon (EItemType)
	TypeName        string // m_kTShip.strName
	Type            string // m_kTShip.eType (EShipType)
	Speed           int    // m_kTShip.iSpeed
	EngagementSpeed int    // m_kTShip.iEngagementSpeed
	HP              int    // m_kTShip.iHP
	AP              int    // m_kTShip.iAP
	Armor           int    // m_kTShip.iArmor
	Graphic         string // the XGShipEntity's m_eEntityGraphic (EEntityGraphic)
	HoursDown       int    // m_iHoursDown: hours left out of action
	HomeContinent   int    // m_iHomeContinent: EContinent index (see File.ContinentLabels)
	CurrentHP       int    // m_iHP: current hit points (m_kTShip.iHP is the maximum)
}

// Ship is one of XCOM's interceptors. Edit the exported values in place;
// a value whose property is missing from the save is left alone.
type Ship struct {
	InstanceName string // e.g. "XGShip_Interceptor_2"
	shipValues
	orig shipValues

	callsign    *xcomsave.StrProperty
	weapon      *xcomsave.ByteProperty
	typeName    *xcomsave.StrProperty
	typ         *xcomsave.ByteProperty
	arrWeapons  *xcomsave.NumberArrayProperty
	ints        map[string]*xcomsave.IntProperty // by m_kTShip property name
	hoursDown   *xcomsave.IntProperty
	homeCont    *xcomsave.IntProperty
	currentHP   *xcomsave.IntProperty
	entity      int32                  // m_kEntity actor index, or -1
	enemyWithin bool                   // the save is Enemy Within (for shipDefaults)
	graphic     *xcomsave.ByteProperty // the entity's m_eEntityGraphic
}

// HasHoursDown and HasHomeContinent report whether m_iHoursDown and
// m_iHomeContinent are in the save.
func (s *Ship) HasHoursDown() bool     { return s.hoursDown != nil }
func (s *Ship) HasHomeContinent() bool { return s.homeCont != nil }

// HasCurrentHP reports whether m_iHP is in the save.
func (s *Ship) HasCurrentHP() bool { return s.currentHP != nil }

// HasGraphic reports whether the interceptor's ship entity, and its
// m_eEntityGraphic, were found.
func (s *Ship) HasGraphic() bool { return s.graphic != nil }

// linkEntity finds the interceptor's XGShipEntity through its m_kEntity
// actor reference (actors is the save's actor table, entities the
// XGShipEntity entries by instance name) and records its m_eEntityGraphic.
func (s *Ship) linkEntity(actors []string, entities map[string]*xcomsave.Checkpoint) {
	if s.entity < 0 || int(s.entity) >= len(actors) {
		return
	}
	a := actors[s.entity]
	c := entities[a[strings.LastIndexByte(a, '.')+1:]]
	if c == nil {
		return
	}
	if p, ok := xcomsave.Find(c.Properties, "m_eEntityGraphic").(*xcomsave.ByteProperty); ok && p.Type != "None" {
		s.graphic, s.Graphic, s.orig.Graphic = p, p.Value, p.Value
	}
}

// ShipStats are the m_kTShip integer properties the hangar edits, in display
// order.
var ShipStats = []string{"iSpeed", "iEngagementSpeed", "iHP", "iAP", "iArmor"}

// Stat returns a pointer to the value of m_kTShip integer property name (one
// of ShipStats).
func (s *Ship) Stat(name string) *int {
	switch name {
	case "iSpeed":
		return &s.Speed
	case "iEngagementSpeed":
		return &s.EngagementSpeed
	case "iHP":
		return &s.HP
	case "iAP":
		return &s.AP
	case "iArmor":
		return &s.Armor
	}
	return nil
}

// HasStat reports whether m_kTShip integer property name is in the save.
func (s *Ship) HasStat(name string) bool { return s.ints[name] != nil }

// HasCallsign, HasWeapon, HasTypeName and HasType report whether the
// corresponding property is in the save.
func (s *Ship) HasCallsign() bool { return s.callsign != nil }
func (s *Ship) HasWeapon() bool   { return s.weapon != nil }
func (s *Ship) HasTypeName() bool { return s.typeName != nil }
func (s *Ship) HasType() bool     { return s.typ != nil }

// Modified reports whether the interceptor has unsaved changes.
func (s *Ship) Modified() bool { return s.shipValues != s.orig }

// shipNum is the number at the end of an instance name, for sorting.
func shipNum(name string) int {
	n, _ := strconv.Atoi(name[strings.LastIndexByte(name, '_')+1:])
	return n
}

func newShip(c *xcomsave.Checkpoint) *Ship {
	s := &Ship{InstanceName: c.InstanceName, ints: map[string]*xcomsave.IntProperty{}, entity: -1}
	if p, ok := xcomsave.Find(c.Properties, "m_kEntity").(*xcomsave.ObjectProperty); ok {
		s.entity = p.Actor
	}
	if p, ok := xcomsave.Find(c.Properties, "m_iHoursDown").(*xcomsave.IntProperty); ok {
		s.hoursDown, s.HoursDown = p, int(p.Value)
	}
	if p, ok := xcomsave.Find(c.Properties, "m_iHomeContinent").(*xcomsave.IntProperty); ok {
		s.homeCont, s.HomeContinent = p, int(p.Value)
	}
	if p, ok := xcomsave.Find(c.Properties, "m_iHP").(*xcomsave.IntProperty); ok {
		s.currentHP, s.CurrentHP = p, int(p.Value)
	}
	if s.InstanceName == "" {
		s.InstanceName = c.Name[strings.LastIndexByte(c.Name, '.')+1:]
	}
	if p, ok := xcomsave.Find(c.Properties, "m_strCallsign").(*xcomsave.StrProperty); ok {
		s.callsign, s.Callsign = p, p.Value.Str
	}
	if p, ok := xcomsave.Find(c.Properties, "m_eWeapon").(*xcomsave.ByteProperty); ok && p.Type != "None" {
		s.weapon, s.Weapon = p, p.Value
	}
	if t, ok := xcomsave.Find(c.Properties, "m_kTShip").(*xcomsave.StructProperty); ok {
		if p, ok := xcomsave.Find(t.Properties, "strName").(*xcomsave.StrProperty); ok {
			s.typeName, s.TypeName = p, p.Value.Str
		}
		if p, ok := xcomsave.Find(t.Properties, "eType").(*xcomsave.ByteProperty); ok && p.Type != "None" {
			s.typ, s.Type = p, p.Value
		}
		s.arrWeapons, _ = xcomsave.Find(t.Properties, "arrWeapons").(*xcomsave.NumberArrayProperty)
		for _, name := range ShipStats {
			if p, ok := xcomsave.Find(t.Properties, name).(*xcomsave.IntProperty); ok {
				s.ints[name] = p
				*s.Stat(name) = int(p.Value)
			}
		}
	}
	s.orig = s.shipValues
	return s
}

// validate checks the interceptor's edited values.
func (s *Ship) validate() error {
	const maxInt32 = 1<<31 - 1
	who := s.InstanceName
	if s.TypeName != s.orig.TypeName && !slices.Contains(ShipNames, s.TypeName) {
		return fmt.Errorf("%s: name must be one of %s (got %q)", who, strings.Join(ShipNames, ", "), s.TypeName)
	}
	if s.Type != s.orig.Type && !slices.Contains(ShipTypes, s.Type) {
		return fmt.Errorf("%s: type must be one of %s (got %q)", who, strings.Join(ShipTypes, ", "), s.Type)
	}
	if s.Graphic != s.orig.Graphic && !slices.Contains(ShipGraphics, s.Graphic) {
		return fmt.Errorf("%s: entity graphic must be one of %s (got %q)", who, strings.Join(ShipGraphics, ", "), s.Graphic)
	}
	if _, ok := shipWeapon(s.Weapon); s.Weapon != s.orig.Weapon && !ok {
		return fmt.Errorf("%s: unknown weapon %q", who, s.Weapon)
	}
	if s.CurrentHP < 0 || s.CurrentHP > maxInt32 {
		return fmt.Errorf("%s: current hit points must be 0-%d (got %d)", who, maxInt32, s.CurrentHP)
	}
	if s.HoursDown < 0 || s.HoursDown > maxInt32 {
		return fmt.Errorf("%s: hours down must be 0-%d (got %d)", who, maxInt32, s.HoursDown)
	}
	if s.HomeContinent != s.orig.HomeContinent && (s.HomeContinent < 0 || s.HomeContinent >= len(continentEnum)) {
		return fmt.Errorf("%s: home continent must be 0-%d (got %d)", who, len(continentEnum)-1, s.HomeContinent)
	}
	for _, name := range ShipStats {
		if v := *s.Stat(name); v < 0 || v > maxInt32 {
			return fmt.Errorf("%s: %s must be 0-%d (got %d)", who, name, maxInt32, v)
		}
	}
	return nil
}

// apply writes the interceptor's edited values back to its properties.
func (s *Ship) apply() {
	if s.callsign != nil && s.Callsign != s.orig.Callsign {
		setStrProp(s.callsign, s.Callsign)
	}
	if s.weapon != nil && s.Weapon != s.orig.Weapon {
		s.weapon.Value = s.Weapon
	}
	// Keep arrWeapons in step with the weapon: its number, and its slot
	// count.
	if a := s.arrWeapons; a != nil && (s.Weapon != s.orig.Weapon || s.Type != s.orig.Type) {
		var first int32
		if len(a.Elements) > 0 {
			first = a.Elements[0]
		}
		slots := max(len(a.Elements), 1)
		if w, ok := shipWeapon(s.Weapon); ok {
			first, slots = int32(w.ArrNum), w.Slots
		}
		a.Elements = append([]int32{first}, make([]int32, slots-1)...)
	}
	if s.currentHP != nil && s.CurrentHP != s.orig.CurrentHP {
		s.currentHP.Value = int32(s.CurrentHP)
	}
	if s.typeName != nil && s.TypeName != s.orig.TypeName {
		setStrProp(s.typeName, s.TypeName)
	}
	if s.typ != nil && s.Type != s.orig.Type {
		s.typ.Value = s.Type
	}
	if s.graphic != nil && s.Graphic != s.orig.Graphic {
		s.graphic.Value = s.Graphic
	}
	if s.hoursDown != nil && s.HoursDown != s.orig.HoursDown {
		s.hoursDown.Value = int32(s.HoursDown)
	}
	if s.homeCont != nil && s.HomeContinent != s.orig.HomeContinent {
		s.homeCont.Value = int32(s.HomeContinent)
	}
	for name, p := range s.ints {
		if v := *s.Stat(name); int32(v) != p.Value {
			p.Value = int32(v)
		}
	}
}

// InterceptorOrder is one entry of XGHeadQuarters' m_akInterceptorOrders:
// interceptors bought and on their way. A property the save leaves out has
// its default value, 0.
type InterceptorOrder struct {
	NumInterceptors      int // iNumInterceptors
	DestinationContinent int // iDestinationContinent: EContinent index
	Hours                int // iHours: hours until delivery

	hours *xcomsave.IntProperty // nil if the save leaves iHours out (0)
	orig  int
}

// newOrders reads m_akInterceptorOrders.
func newOrders(p *xcomsave.StructArrayProperty) []*InterceptorOrder {
	var out []*InterceptorOrder
	for _, st := range p.Structs {
		o := &InterceptorOrder{}
		for name, v := range map[string]*int{
			"iNumInterceptors": &o.NumInterceptors, "iDestinationContinent": &o.DestinationContinent, "iHours": &o.Hours,
		} {
			if ip, ok := xcomsave.Find(st, name).(*xcomsave.IntProperty); ok {
				*v = int(ip.Value)
				if name == "iHours" {
					o.hours = ip
				}
			}
		}
		o.orig = o.Hours
		out = append(out, o)
	}
	return out
}

// PendingOrders returns the number of interceptor orders not yet due.
func (f *File) PendingOrders() int {
	n := 0
	for _, o := range f.Orders {
		if o.Hours > 0 {
			n++
		}
	}
	return n
}

// FulfillOrders sets every interceptor order's iHours to 0, so the
// interceptors arrive at the next opportunity.
func (f *File) FulfillOrders() {
	for _, o := range f.Orders {
		o.Hours = 0
	}
}

func (f *File) ordersModified() bool {
	return slices.ContainsFunc(f.Orders, func(o *InterceptorOrder) bool { return o.Hours != o.orig })
}

func (f *File) applyOrders() {
	for _, o := range f.Orders {
		if o.hours != nil && o.Hours != o.orig {
			o.hours.Value = int32(o.Hours)
		}
	}
}

// ShipsDown returns the number of interceptors with m_iHoursDown above 0.
func (f *File) ShipsDown() int {
	n := 0
	for _, s := range f.Ships {
		if s.HasHoursDown() && s.HoursDown > 0 {
			n++
		}
	}
	return n
}

// RepairAll sets every interceptor's m_iHoursDown to 0.
func (f *File) RepairAll() {
	for _, s := range f.Ships {
		if s.HasHoursDown() {
			s.HoursDown = 0
		}
	}
}

// WeaponName returns the name of interceptor weapon value (an m_eWeapon
// EItemType value), e.g. "EMP Cannon", or value itself if it is not one of
// ShipWeapons.
func WeaponName(value string) string {
	if w, ok := shipWeapon(value); ok {
		return w.Name
	}
	return value
}
