package rel

// Source: src/muz/rel/dl_table.h hashtable_table::storage, which is
// hashtable<table_fact, svector_hash_proc<table_element_hash>,
// vector_eq_proc<table_fact>> over src/util/hashtable.h core_hashtable with
// default_hash_entry. Iteration walks the slots in index order, so the order
// depends on the fact hashes, the capacity and the history of insertions and
// removals; this type reproduces that layout.

import "github.com/glycerine/g3/pkg/util/hash"

const (
	factHashInitialCapacity = 8  // DEFAULT_HASHTABLE_INITIAL_CAPACITY
	factHashSmallCapacity   = 64 // SMALL_TABLE_CAPACITY
)

type factHashState uint8

const (
	factHashFree factHashState = iota
	factHashDeleted
	factHashUsed
)

type factHashEntry struct {
	fact  Fact
	hash  uint32
	state factHashState
}

type factHashSet struct {
	table      []factHashEntry
	size       int
	numDeleted int
}

func newFactHashSet() *factHashSet {
	return &factHashSet{table: make([]factHashEntry, factHashInitialCapacity)}
}

// factHash is svector_hash<uint64_hash>: 778 for the empty vector, otherwise
// get_composite_hash with kind hash 17 and child hash (unsigned)x.
func factHash(fact Fact) uint32 {
	if len(fact) == 0 {
		return 778
	}
	return hash.Composite(17, uint32(len(fact)), func(i uint32) uint32 { return uint32(fact[i]) })
}

func factsEqual(a, b Fact) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func factHashMoveTable(source, target []factHashEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if entry.state != factHashUsed {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if target[i].state == factHashFree {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if target[i].state == factHashFree {
				target[i], placed = entry, true
			}
		}
	}
}

func (s *factHashSet) Len() int { return s.size }

// insert is core_hashtable::insert; it reports whether fact was absent. The
// set keeps its own copy of an inserted fact.
func (s *factHashSet) insert(fact Fact) bool {
	if (s.size+s.numDeleted)<<2 > len(s.table)*3 {
		target := make([]factHashEntry, len(s.table)<<1)
		factHashMoveTable(s.table, target)
		s.table, s.numDeleted = target, 0
	}
	h := factHash(fact)
	mask := uint32(len(s.table) - 1)
	begin := int(h & mask)
	deleted := -1
	visit := func(i int) (done, inserted bool) {
		curr := &s.table[i]
		switch curr.state {
		case factHashUsed:
			if curr.hash == h && factsEqual(curr.fact, fact) {
				return true, false
			}
		case factHashFree:
			n := i
			if deleted >= 0 {
				n = deleted
				s.numDeleted--
			}
			s.table[n] = factHashEntry{fact: cloneFact(fact), hash: h, state: factHashUsed}
			s.size++
			return true, true
		default:
			deleted = i
		}
		return false, false
	}
	for i := begin; i < len(s.table); i++ {
		if done, inserted := visit(i); done {
			return inserted
		}
	}
	for i := 0; i < begin; i++ {
		if done, inserted := visit(i); done {
			return inserted
		}
	}
	panic("muz/rel: fact hashtable has no free slot") // UNREACHABLE
}

func (s *factHashSet) index(fact Fact) int {
	h := factHash(fact)
	mask := uint32(len(s.table) - 1)
	begin := int(h & mask)
	for _, r := range [2][2]int{{begin, len(s.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &s.table[i]
			if curr.state == factHashUsed {
				if curr.hash == h && factsEqual(curr.fact, fact) {
					return i
				}
			} else if curr.state == factHashFree {
				return -1
			}
		}
	}
	return -1
}

func (s *factHashSet) contains(fact Fact) bool { return s.index(fact) >= 0 }

// remove is core_hashtable::remove; it reports whether fact was present.
func (s *factHashSet) remove(fact Fact) bool {
	i := s.index(fact)
	if i < 0 {
		return false
	}
	next := i + 1
	if next == len(s.table) {
		next = 0
	}
	if s.table[next].state == factHashFree {
		s.table[i] = factHashEntry{}
		s.size--
		return true
	}
	s.table[i] = factHashEntry{state: factHashDeleted}
	s.numDeleted++
	s.size--
	if s.numDeleted > s.size && s.numDeleted > factHashSmallCapacity {
		target := make([]factHashEntry, len(s.table))
		factHashMoveTable(s.table, target)
		s.table, s.numDeleted = target, 0
	}
	return true
}

// facts lists the stored facts (borrowed) in slot order, as the
// hashtable_table iterator visits them.
func (s *factHashSet) facts() []Fact {
	out := make([]Fact, 0, s.size)
	for i := range s.table {
		if s.table[i].state == factHashUsed {
			out = append(out, s.table[i].fact)
		}
	}
	return out
}
