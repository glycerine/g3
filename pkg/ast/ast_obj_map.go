package ast

// Source: src/util/hashtable.h core_hashtable and src/util/obj_hashtable.h
// obj_map<T, V>, keyed by an AST with obj_ptr_hash (the AST's hash). Code
// that iterates an obj_map (to release its entries, or to build terms) does
// so in table order, which depends on the hashes and on the history of
// insertions and removals; this type reproduces that layout. Values are one
// or two handles (Value[1] for std::pair values), or a number (Num) for
// obj_map<T, unsigned>.

const (
	objMapInitialCapacity = 8  // DEFAULT_HASHTABLE_INITIAL_CAPACITY
	objMapSmallCapacity   = 64 // SMALL_TABLE_CAPACITY
)

type objMapState uint8

const (
	objMapFree objMapState = iota
	objMapDeleted
	objMapUsed
)

// ObjMapEntry is one key/value entry.
type ObjMapEntry struct {
	Key   ASTHandle
	Value [2]ASTHandle
	Num   uint64
	hash  uint32
	state objMapState
}

// ObjMap is obj_map<K*, V>. Its zero value is not usable; see NewObjMap.
type ObjMap struct {
	manager    *ASTManager
	table      []ObjMapEntry
	size       int
	numDeleted int
}

func NewObjMap(m *ASTManager) *ObjMap {
	return &ObjMap{manager: m, table: make([]ObjMapEntry, objMapInitialCapacity)}
}

func objMapMoveTable(source, target []ObjMapEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if entry.state != objMapUsed {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if target[i].state == objMapFree {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if target[i].state == objMapFree {
				target[i], placed = entry, true
			}
		}
	}
}

func (o *ObjMap) expandTable() {
	target := make([]ObjMapEntry, len(o.table)<<1)
	objMapMoveTable(o.table, target)
	o.table, o.numDeleted = target, 0
}

func (o *ObjMap) removeDeletedEntries() {
	target := make([]ObjMapEntry, len(o.table))
	objMapMoveTable(o.table, target)
	o.table, o.numDeleted = target, 0
}

// Len is the number of keys.
func (o *ObjMap) Len() int { return o.size }

// Insert is obj_map::insert(k, v): it adds k, or replaces its value.
func (o *ObjMap) Insert(key ASTHandle, value [2]ASTHandle) error {
	return o.insert(ObjMapEntry{Key: key, Value: value})
}

// InsertNum is obj_map<T, unsigned>::insert(k, n).
func (o *ObjMap) InsertNum(key ASTHandle, n uint64) error {
	return o.insert(ObjMapEntry{Key: key, Num: n})
}

func (o *ObjMap) insert(entry ObjMapEntry) error {
	key := entry.Key
	if (o.size+o.numDeleted)<<2 > len(o.table)*3 {
		o.expandTable()
	}
	hash, err := o.manager.GetHash(key)
	if err != nil {
		return err
	}
	mask := uint32(len(o.table) - 1)
	begin := int(hash & mask)
	deleted := -1
	visit := func(i int) bool {
		curr := &o.table[i]
		switch curr.state {
		case objMapUsed:
			if curr.hash == hash && curr.Key == key {
				curr.Value, curr.Num = entry.Value, entry.Num
				return true
			}
		case objMapFree:
			n := i
			if deleted >= 0 {
				n = deleted
				o.numDeleted--
			}
			entry.hash, entry.state = hash, objMapUsed
			o.table[n] = entry
			o.size++
			return true
		default:
			deleted = i
		}
		return false
	}
	for i := begin; i < len(o.table); i++ {
		if visit(i) {
			return nil
		}
	}
	for i := 0; i < begin; i++ {
		if visit(i) {
			return nil
		}
	}
	return ErrASTDomain // UNREACHABLE: the table always has a free slot
}

func (o *ObjMap) index(key ASTHandle) (int, error) {
	hash, err := o.manager.GetHash(key)
	if err != nil {
		return -1, err
	}
	mask := uint32(len(o.table) - 1)
	begin := int(hash & mask)
	for _, r := range [2][2]int{{begin, len(o.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &o.table[i]
			if curr.state == objMapUsed {
				if curr.hash == hash && curr.Key == key {
					return i, nil
				}
			} else if curr.state == objMapFree {
				return -1, nil
			}
		}
	}
	return -1, nil
}

// Find is obj_map::find.
func (o *ObjMap) Find(key ASTHandle) ([2]ASTHandle, bool, error) {
	i, err := o.index(key)
	if err != nil || i < 0 {
		return [2]ASTHandle{}, false, err
	}
	return o.table[i].Value, true, nil
}

// FindNum is obj_map<T, unsigned>::find.
func (o *ObjMap) FindNum(key ASTHandle) (uint64, bool, error) {
	i, err := o.index(key)
	if err != nil || i < 0 {
		return 0, false, err
	}
	return o.table[i].Num, true, nil
}

// Contains is obj_map::contains.
func (o *ObjMap) Contains(key ASTHandle) (bool, error) {
	i, err := o.index(key)
	return i >= 0, err
}

// Erase is obj_map::erase (core_hashtable::remove).
func (o *ObjMap) Erase(key ASTHandle) error {
	i, err := o.index(key)
	if err != nil || i < 0 {
		return err
	}
	next := i + 1
	if next == len(o.table) {
		next = 0
	}
	if o.table[next].state == objMapFree {
		o.table[i] = ObjMapEntry{}
		o.size--
		return nil
	}
	o.table[i] = ObjMapEntry{state: objMapDeleted}
	o.numDeleted++
	o.size--
	if o.numDeleted > o.size && o.numDeleted > objMapSmallCapacity {
		o.removeDeletedEntries()
	}
	return nil
}

// Entries lists the entries in iteration (table) order.
func (o *ObjMap) Entries() []ObjMapEntry {
	out := make([]ObjMapEntry, 0, o.size)
	for _, entry := range o.table {
		if entry.state == objMapUsed {
			out = append(out, entry)
		}
	}
	return out
}

// Reset is core_hashtable::reset, which halves a mostly free large table.
func (o *ObjMap) Reset() {
	if o.size == 0 && o.numDeleted == 0 {
		return
	}
	overhead := 0
	for i := range o.table {
		if o.table[i].state != objMapFree {
			o.table[i] = ObjMapEntry{}
		} else {
			overhead++
		}
	}
	if len(o.table) > 16 && overhead<<2 > len(o.table)*3 {
		o.table = make([]ObjMapEntry, len(o.table)>>1)
	}
	o.size, o.numDeleted = 0, 0
}
