package rel

// Iteration orders for the compiler's per-variable column maps. Go map order
// is random; these reproduce the source containers' orders.

import "slices"

// variablesByFirstColumn lists the variables of a variable -> columns map in
// ascending order of their first column: the order in which a column scan
// (variable_intersection::populate, or building int2ints var_indexes from
// the tail arguments) first meets them.
func variablesByFirstColumn(columns map[uint32][]int) []uint32 {
	keys := make([]uint32, 0, len(columns))
	for variable := range columns {
		keys = append(keys, variable)
	}
	first := func(v uint32) int {
		if c := columns[v]; len(c) != 0 {
			return c[0]
		}
		return -1
	}
	slices.SortFunc(keys, func(a, b uint32) int {
		if fa, fb := first(a), first(b); fa != fb {
			return fa - fb
		}
		return int(a) - int(b)
	})
	return keys
}

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
