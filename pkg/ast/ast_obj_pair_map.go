package ast

// Source: src/util/obj_pair_hashtable.h obj_pair_map<T1, T2, V>, a
// core_hashtable whose entries hash combine_hash(k1->hash(), k2->hash()).
// An ObjMap stores such an entry with Key = k1 and Value[0] = k2; callers
// supply the pair hash (PairHash) captured at insertion, so a pair can be
// erased after its keys died.

// CombineHash is combine_hash (src/util/hash.h).
func CombineHash(h1, h2 uint32) uint32 {
	h2 -= h1
	h2 ^= h1 << 8
	h1 -= h2
	h2 ^= h1 << 16
	h2 -= h1
	h2 ^= h1 << 10
	return h2
}

// PairHash is the obj_pair_map hash of (k1, k2).
func (o *ObjMap) PairHash(k1, k2 ASTHandle) (uint32, error) {
	h1, err := o.keyHash(k1)
	if err != nil {
		return 0, err
	}
	h2, err := o.keyHash(k2)
	if err != nil {
		return 0, err
	}
	return CombineHash(h1, h2), nil
}

// InsertPairKeys is obj_pair_map::insert(k1, k2, v) for a new pair (an
// existing pair keeps its entry).
func (o *ObjMap) InsertPairKeys(k1, k2 ASTHandle, hash uint32) {
	e := o.insertIfNotThereCore(k1, 0, hash, func(e *ObjMapEntry) bool {
		return e.Key == k1 && e.Value[0] == k2
	})
	e.Value[0] = k2
}

// ErasePairKeys is obj_pair_map::erase(k1, k2).
func (o *ObjMap) ErasePairKeys(k1, k2 ASTHandle, hash uint32) {
	mask := uint32(len(o.table) - 1)
	begin := int(hash & mask)
	for _, r := range [2][2]int{{begin, len(o.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &o.table[i]
			if curr.state == objMapUsed {
				if curr.hash == hash && curr.Key == k1 && curr.Value[0] == k2 {
					o.removeAt(i)
					return
				}
			} else if curr.state == objMapFree {
				return
			}
		}
	}
}
