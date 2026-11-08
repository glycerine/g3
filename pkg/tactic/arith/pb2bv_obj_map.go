package arith

// Source: src/util/hashtable.h core_hashtable, concretely for pb2bv's
// obj_map<func_decl, expr*> m_const2bit and m_not_const2bit. ~imp releases
// their entries in table order, which decides the order in which the fresh
// bits and their negations die (and so which ids are recycled first). The
// maps only ever grow, so this keeps just the insertion-only layout.

import "github.com/glycerine/g3/pkg/ast"

type pbObjMapEntry struct {
	used  bool
	hash  uint32
	index int // position in PB2BVTactic.entries
}

// pbObjMapOrder returns the entries' indices in obj_map iteration order for
// keys inserted in the given order (DEFAULT_HASHTABLE_INITIAL_CAPACITY 8,
// expanded when size*4 > capacity*3, linear probing on the decl hash).
func pbObjMapOrder(m *ast.ASTManager, keys []ast.ASTHandle) ([]int, error) {
	table := make([]pbObjMapEntry, 8)
	size := 0
	place := func(t []pbObjMapEntry, e pbObjMapEntry) {
		mask := uint32(len(t) - 1)
		begin := int(e.hash & mask)
		for k := 0; k < len(t); k++ {
			i := (begin + k) % len(t)
			if !t[i].used {
				t[i] = e
				return
			}
		}
	}
	for index, key := range keys {
		if size<<2 > len(table)*3 {
			bigger := make([]pbObjMapEntry, len(table)<<1)
			for _, e := range table {
				if e.used {
					place(bigger, e)
				}
			}
			table = bigger
		}
		hash, err := m.GetHash(key)
		if err != nil {
			return nil, err
		}
		place(table, pbObjMapEntry{used: true, hash: hash, index: index})
		size++
	}
	order := make([]int, 0, size)
	for _, e := range table {
		if e.used {
			order = append(order, e.index)
		}
	}
	return order, nil
}
