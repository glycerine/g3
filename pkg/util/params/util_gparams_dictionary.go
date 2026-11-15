package params

// Source: src/util/gparams.cpp; src/util/dictionary.h; src/util/map.h;
// src/util/hashtable.h core_hashtable.

import (
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// DEFAULT_HASHTABLE_INITIAL_CAPACITY
const gparamsDictionaryInitialCapacity = 8

type gparamsDictionaryState uint8

const (
	gparamsDictionaryFree gparamsDictionaryState = iota
	gparamsDictionaryDeleted
	gparamsDictionaryUsed
)

type gparamsDescriptorEntry struct {
	state gparamsDictionaryState
	hash  uint32
	key   usymbol.Symbol
	value uint32
}

// gparamsDescriptorDictionary is m_module_param_descrs, a
// dictionary<param_descrs*> over core_hashtable: display and display_modules
// walk it in table-slot order, which follows symbol::hash() and the
// open-addressing layout (initial capacity 8, doubling at 3/4 load, linear
// probing). Callers pass symbol::hash() of the key (Manager.Hash).
type gparamsDescriptorDictionary struct {
	table      []gparamsDescriptorEntry
	size_      int
	numDeleted int
}

type gparamsRefDictionary struct {
	values map[usymbol.Symbol]uint32
}

type gparamsDescriptionDictionary struct {
	values map[usymbol.Symbol]utext.RegionCString
}

func (d *gparamsDescriptorDictionary) init() {
	d.table = make([]gparamsDescriptorEntry, gparamsDictionaryInitialCapacity)
	d.size_, d.numDeleted = 0, 0
}

// find is core_hashtable::find_core.
func (d *gparamsDescriptorDictionary) find(k usymbol.Symbol, hash uint32) *gparamsDescriptorEntry {
	if len(d.table) == 0 {
		return nil
	}
	mask := uint32(len(d.table) - 1)
	begin := int(hash & mask)
	for _, r := range [2][2]int{{begin, len(d.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &d.table[i]
			switch curr.state {
			case gparamsDictionaryUsed:
				if curr.hash == hash && curr.key == k {
					return curr
				}
			case gparamsDictionaryFree:
				return nil
			}
		}
	}
	return nil
}

func (d *gparamsDescriptorDictionary) get(k usymbol.Symbol, hash uint32) (uint32, bool) {
	if e := d.find(k, hash); e != nil {
		return e.value, true
	}
	return 0, false
}

// moveTable is core_hashtable::move_table: used entries are re-placed in
// source slot order with linear probing.
func gparamsDescriptorMoveTable(source, target []gparamsDescriptorEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if entry.state != gparamsDictionaryUsed {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if target[i].state == gparamsDictionaryFree {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if target[i].state == gparamsDictionaryFree {
				target[i], placed = entry, true
			}
		}
	}
}

// set is map::insert(k, v), core_hashtable::insert: an existing key is
// overwritten in place.
func (d *gparamsDescriptorDictionary) set(k usymbol.Symbol, hash uint32, descriptor uint32) {
	if len(d.table) == 0 {
		d.init()
	}
	if (d.size_+d.numDeleted)<<2 > len(d.table)*3 {
		target := make([]gparamsDescriptorEntry, len(d.table)<<1)
		gparamsDescriptorMoveTable(d.table, target)
		d.table, d.numDeleted = target, 0
	}
	mask := uint32(len(d.table) - 1)
	begin := int(hash & mask)
	deleted := -1
	for _, r := range [2][2]int{{begin, len(d.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &d.table[i]
			switch curr.state {
			case gparamsDictionaryUsed:
				if curr.hash == hash && curr.key == k {
					curr.value = descriptor
					return
				}
			case gparamsDictionaryFree:
				target := i
				if deleted >= 0 {
					target = deleted
					d.numDeleted--
				}
				d.table[target] = gparamsDescriptorEntry{state: gparamsDictionaryUsed, hash: hash, key: k, value: descriptor}
				d.size_++
				return
			default:
				if deleted < 0 {
					deleted = i
				}
			}
		}
	}
	// core_hashtable::insert: UNREACHABLE (the 3/4 load rule keeps a free slot).
	panic("gparams dictionary: no free slot")
}

// all walks the table slots in order, yielding used entries (iterator).
func (d *gparamsDescriptorDictionary) all() func(yield func(usymbol.Symbol, uint32) bool) {
	return func(yield func(usymbol.Symbol, uint32) bool) {
		for i := range d.table {
			if d.table[i].state == gparamsDictionaryUsed {
				if !yield(d.table[i].key, d.table[i].value) {
					return
				}
			}
		}
	}
}

// reset is core_hashtable::reset: every slot becomes free, and a table
// whose free overhead exceeds 3/4 of a capacity above 16 halves.
func (d *gparamsDescriptorDictionary) reset() {
	if d.size_ == 0 && d.numDeleted == 0 {
		return
	}
	overhead := 0
	for i := range d.table {
		if d.table[i].state != gparamsDictionaryFree {
			d.table[i] = gparamsDescriptorEntry{}
		} else {
			overhead++
		}
	}
	if len(d.table) > 16 && overhead<<2 > len(d.table)*3 {
		d.table = make([]gparamsDescriptorEntry, len(d.table)>>1)
	}
	d.size_, d.numDeleted = 0, 0
}
func (d *gparamsDescriptorDictionary) close() { d.table, d.size_, d.numDeleted = nil, 0, 0 }
func (d *gparamsDescriptorDictionary) size() uint32 {
	return uint32(d.size_)
}

func (d *gparamsRefDictionary) init() { d.values = make(map[usymbol.Symbol]uint32) }
func (d *gparamsRefDictionary) get(k usymbol.Symbol) (uint32, bool) {
	v, ok := d.values[k]
	return v, ok
}
func (d *gparamsRefDictionary) set(k usymbol.Symbol, cell uint32) {
	if d.values == nil {
		d.init()
	}
	d.values[k] = cell
}
func (d *gparamsRefDictionary) reset() { clear(d.values) }
func (d *gparamsRefDictionary) close() { d.values = nil }
func (d *gparamsRefDictionary) size() uint32 {
	return uint32(len(d.values))
}

func (d *gparamsDescriptionDictionary) init() {
	d.values = make(map[usymbol.Symbol]utext.RegionCString)
}
func (d *gparamsDescriptionDictionary) get(k usymbol.Symbol) (utext.RegionCString, bool) {
	v, ok := d.values[k]
	return v, ok
}
func (d *gparamsDescriptionDictionary) set(k usymbol.Symbol, text utext.RegionCString) {
	if d.values == nil {
		d.init()
	}
	d.values[k] = text
}
func (d *gparamsDescriptionDictionary) reset() { clear(d.values) }
func (d *gparamsDescriptionDictionary) close() { d.values = nil }
func (d *gparamsDescriptionDictionary) size() uint32 {
	return uint32(len(d.values))
}
