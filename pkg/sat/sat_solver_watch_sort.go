package sat

import "sort"

type satWatchSortSlice struct {
	b      *satWatchBacking
	binary bool
}

func (v *satWatchSortSlice) Len() int { return len(v.b.data) }
func (v *satWatchSortSlice) Less(i, j int) bool {
	a, b := v.b.data[i], v.b.data[j]
	x, y := satWatched{val1: a.val1, val2: a.val2}, satWatched{val1: b.val1, val2: b.val2}
	if v.binary {
		return satSimplifierBinaryLess(x, y)
	}
	return satWatchedLess(x, y)
}
func (v *satWatchSortSlice) Swap(i, j int) { v.b.swap(i, j) }

// watched_lt orders three source watch classes and treats entries within a
// class as equivalent. Go's stable sort keeps their original insertion order.
func (s *satSolverWatchStore) SortByKind(r satSolverWatchListRef) error {
	x, e := s.record(r)
	if e != nil {
		return e
	}
	// The store owns the interface adapter, so sorting each watch list does
	// not allocate a fresh boxed slice during the solver's outer loop.
	if len(x.values()) < 2 {
		return nil
	}
	s.sorter.b = x.backing
	sort.Stable(&s.sorter)
	s.sorter.b = nil
	return nil
}
