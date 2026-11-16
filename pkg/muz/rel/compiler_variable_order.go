package rel

// Iteration orders for the compiler's per-variable column maps. Go map order
// is random; these reproduce the source containers' orders.

import "slices"

// uMapOrder is the iteration order of a u_map<V> (core_hashtable with
// u_hash(k) == k, initial capacity 8, linear probing, growth when
// (size+deleted)*4 > capacity*3) after inserting keys, in the given order and
// without removals.
func uMapOrder(keys []uint32) []uint32 {
	type slot struct {
		used bool
		key  uint32
	}
	place := func(table []slot, key uint32) {
		mask := uint32(len(table) - 1)
		begin := int(key & mask)
		for i := begin; i < len(table); i++ {
			if !table[i].used {
				table[i] = slot{true, key}
				return
			}
			if table[i].key == key {
				return
			}
		}
		for i := 0; i < begin; i++ {
			if !table[i].used {
				table[i] = slot{true, key}
				return
			}
			if table[i].key == key {
				return
			}
		}
	}
	table := make([]slot, 8)
	size := 0
	present := make(map[uint32]bool, len(keys))
	for _, key := range keys {
		if size<<2 > len(table)*3 {
			grown := make([]slot, len(table)<<1)
			for _, s := range table {
				if s.used {
					place(grown, s.key)
				}
			}
			table = grown
		}
		if present[key] {
			continue
		}
		present[key] = true
		place(table, key)
		size++
	}
	result := make([]uint32, 0, size)
	for _, s := range table {
		if s.used {
			result = append(result, s.key)
		}
	}
	return result
}

// sortedBoundIndices lists a bound column set ascending (uint_set order).
func sortedBoundIndices(set map[uint32]bool) []uint32 {
	keys := make([]uint32, 0, len(set))
	for index := range set {
		keys = append(keys, index)
	}
	slices.Sort(keys)
	return keys
}
