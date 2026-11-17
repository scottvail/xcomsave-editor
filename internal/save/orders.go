package save

import (
	"fmt"

	"github.com/scottvail/go-xcomsave"
)

// MaxInterceptorsPerContinent is the number of interceptors a continent's
// hangar holds.
const MaxInterceptorsPerContinent = 4

// The number of soldiers the barracks holds (confirmed in game).
const (
	MaxSoldiersEU = 99 // Enemy Unknown
	MaxSoldiersEW = 70 // Enemy Within
)

// MaxSoldiers returns the number of soldiers the save's barracks holds.
func (f *File) MaxSoldiers() int {
	if f.EnemyWithin() {
		return MaxSoldiersEW
	}
	return MaxSoldiersEU
}

// BarracksClass is the class of the checkpoint entry holding XCOM's
// soldier list.
const BarracksClass = "XComStrategyGame.XGFacility_Barracks"

// staffSoldier is THireOrder.iStaffType for a soldier.
const staffSoldier = 1

// Buying an interceptor or hiring a soldier in game creates no entry: it
// adds an order to XGHeadQuarters (m_akInterceptorOrders or
// m_arrHiringOrders) with iHours until delivery, and the game creates the
// interceptor or soldier when the order is due. New orders made here are
// due at once (iHours 0, so the property is left out, as the game leaves out
// properties at their default value).

// newOrder is an order added in the editor.
type newOrder struct {
	count     int
	continent int // interceptors only
}

// readHQOrders reads the orders in XGHeadQuarters entry c.
func (f *File) readHQOrders(c *xcomsave.Checkpoint) {
	f.hq = c
	if p, ok := xcomsave.Find(c.Properties, "m_akInterceptorOrders").(*xcomsave.StructArrayProperty); ok {
		f.Orders = newOrders(p)
	}
	if p, ok := xcomsave.Find(c.Properties, "m_arrHiringOrders").(*xcomsave.StructArrayProperty); ok {
		for _, st := range p.Structs {
			if intField(st, "iStaffType") != staffSoldier {
				continue
			}
			f.hiring += intField(st, "iNumStaff")
			if h, ok := xcomsave.Find(st, "iHours").(*xcomsave.IntProperty); ok && h.Value > 0 {
				f.hireOrders = append(f.hireOrders, hireOrder{h, intField(st, "iNumStaff")})
			}
		}
	}
}

// hireOrder is a soldier hiring order not yet due.
type hireOrder struct {
	hours *xcomsave.IntProperty // iHours
	count int                   // iNumStaff
}

// PendingHires returns the number of soldiers ordered and not yet due.
func (f *File) PendingHires() int {
	if f.rushHires {
		return 0
	}
	n := 0
	for _, o := range f.hireOrders {
		n += o.count
	}
	return n
}

// RushHires sets the iHours of every soldier hiring order to 0, so the
// soldiers arrive at the next opportunity.
func (f *File) RushHires() { f.rushHires = len(f.hireOrders) > 0 }

func (f *File) applyRushHires() {
	if f.rushHires {
		for _, o := range f.hireOrders {
			o.hours.Value = 0
		}
	}
}

// intField returns the value of IntProperty name in props, or 0 (the
// default) if it is missing.
func intField(props []xcomsave.Property, name string) int {
	if p, ok := xcomsave.Find(props, name).(*xcomsave.IntProperty); ok {
		return int(p.Value)
	}
	return 0
}

// orderArray returns XGHeadQuarters' order array name as a struct array, or
// nil if the save has none. An empty array is stored without its element
// type (an ArrayProperty with no data); it is replaced by an empty struct
// array.
func (f *File) orderArray(name string) *xcomsave.StructArrayProperty {
	if f.hq == nil {
		return nil
	}
	for i, p := range f.hq.Properties {
		if p.PropertyName() != name {
			continue
		}
		switch p := p.(type) {
		case *xcomsave.StructArrayProperty:
			return p
		case *xcomsave.ArrayProperty:
			if p.ArrayBound == 0 && len(p.Data) == 0 {
				sp := &xcomsave.StructArrayProperty{Name: name}
				f.hq.Properties[i] = sp
				return sp
			}
		}
		return nil
	}
	return nil
}

// hasOrderArray reports whether XGHeadQuarters has order array name in a
// form orderArray can add to, without changing it.
func (f *File) hasOrderArray(name string) bool {
	if f.hq == nil {
		return false
	}
	switch p := xcomsave.Find(f.hq.Properties, name).(type) {
	case *xcomsave.StructArrayProperty:
		return true
	case *xcomsave.ArrayProperty:
		return p.ArrayBound == 0 && len(p.Data) == 0
	}
	return false
}

// CanOrderInterceptors reports whether interceptors can be ordered.
func (f *File) CanOrderInterceptors() bool { return f.hasOrderArray("m_akInterceptorOrders") }

// InterceptorsOn returns the number of interceptors based on continent
// (an EContinent index), on order to it, or ordered in the editor.
func (f *File) InterceptorsOn(continent int) int {
	n := 0
	for _, s := range f.Ships {
		if s.HomeContinent == continent { // 0 if the save leaves it out
			n++
		}
	}
	for _, o := range f.Orders {
		if o.DestinationContinent == continent {
			n += o.NumInterceptors
		}
	}
	for _, o := range f.newShips {
		if o.continent == continent {
			n += o.count
		}
	}
	return n
}

// FreeHangarBays returns how many more interceptors continent can take.
func (f *File) FreeHangarBays(continent int) int {
	return max(0, MaxInterceptorsPerContinent-f.InterceptorsOn(continent))
}

// OrderInterceptors orders n interceptors for continent, due at once.
func (f *File) OrderInterceptors(continent, n int) error {
	if !f.CanOrderInterceptors() {
		return fmt.Errorf("this save has no interceptor orders")
	}
	if continent < 0 || continent >= len(continentEnum) {
		return fmt.Errorf("continent must be 0-%d (got %d)", len(continentEnum)-1, continent)
	}
	if free := f.FreeHangarBays(continent); n < 1 || n > free {
		return fmt.Errorf("%s has room for %d more interceptor(s) (got %d)", f.ContinentLabels()[continent], free, n)
	}
	f.newShips = append(f.newShips, newOrder{count: n, continent: continent})
	return nil
}

// NewInterceptors returns the number of interceptors ordered in the editor.
func (f *File) NewInterceptors() int {
	n := 0
	for _, o := range f.newShips {
		n += o.count
	}
	return n
}

// CanHireSoldiers reports whether soldiers can be hired.
func (f *File) CanHireSoldiers() bool {
	return f.hasBarracks && f.hasOrderArray("m_arrHiringOrders")
}

// BarracksCount returns the number of soldiers in the barracks
// (XGFacility_Barracks' m_arrSoldiers), on their way, or hired in the
// editor.
func (f *File) BarracksCount() int {
	return f.barracksSize + f.hiring + f.newHires
}

// readBarracks reads the size of XGFacility_Barracks entry c's soldier
// list. Enemy Within stores it as actor references; Enemy Unknown's single
// int references read as a number array.
func (f *File) readBarracks(c *xcomsave.Checkpoint) {
	switch p := xcomsave.Find(c.Properties, "m_arrSoldiers").(type) {
	case *xcomsave.ObjectArrayProperty:
		f.barracksSize, f.hasBarracks = len(p.Actors), true
	case *xcomsave.NumberArrayProperty:
		f.barracksSize, f.hasBarracks = len(p.Elements), true
	}
}

// FreeBarracks returns how many more soldiers can be hired.
func (f *File) FreeBarracks() int { return max(0, f.MaxSoldiers()-f.BarracksCount()) }

// HireSoldiers hires n soldiers, due at once. The game creates them as
// rookies when the order is delivered.
func (f *File) HireSoldiers(n int) error {
	if !f.CanHireSoldiers() {
		return fmt.Errorf("this save has no hiring orders")
	}
	if free := f.FreeBarracks(); n < 1 || n > free {
		return fmt.Errorf("the barracks has room for %d more soldier(s) (got %d)", free, n)
	}
	f.newHires += n
	return nil
}

// NewSoldiers returns the number of soldiers hired in the editor.
func (f *File) NewSoldiers() int { return f.newHires }

func (f *File) newOrdersModified() bool { return len(f.newShips) > 0 || f.newHires > 0 || f.rushHires }

// applyNewOrders adds the editor's orders to XGHeadQuarters. It can run
// more than once (each run replaces what the previous one added).
func (f *File) applyNewOrders() {
	f.applyRushHires()
	if len(f.newShips) > 0 {
		if p := f.orderArray("m_akInterceptorOrders"); p != nil {
			if f.shipBase < 0 {
				f.shipBase = len(p.Structs)
			}
			p.Structs = p.Structs[:f.shipBase:f.shipBase]
			for _, o := range f.newShips {
				st := []xcomsave.Property{intProp("iNumInterceptors", o.count)}
				if o.continent != 0 {
					st = append(st, intProp("iDestinationContinent", o.continent))
				}
				p.Structs = append(p.Structs, st)
			}
		}
	}
	if f.newHires > 0 {
		if p := f.orderArray("m_arrHiringOrders"); p != nil {
			if f.hireBase < 0 {
				f.hireBase = len(p.Structs)
			}
			p.Structs = append(p.Structs[:f.hireBase:f.hireBase], []xcomsave.Property{
				intProp("iNumStaff", f.newHires), intProp("iStaffType", staffSoldier),
			})
		}
	}
}

func intProp(name string, v int) *xcomsave.IntProperty {
	return &xcomsave.IntProperty{Name: name, Value: int32(v)}
}
