package smtcore

// Typed wrappers for the approved libstdc++ std::sort port. The algorithm,
// and the WARNING that governs it, is in pkg/util/sorting/libstdcxx_sort.go:
// a deliberate, user-approved port of STL internals; do not replace these
// calls with sort.Slice or any other sort.

import "github.com/glycerine/g3/pkg/util/sorting"

// libstdcxxAtomSlice adapts []*smtArithmeticAtom with compare_atoms.
type libstdcxxAtomSlice struct {
	a    []*smtArithmeticAtom
	less func(x, y *smtArithmeticAtom) bool
}

func (s libstdcxxAtomSlice) Len() int           { return len(s.a) }
func (s libstdcxxAtomSlice) Less(i, j int) bool { return s.less(s.a[i], s.a[j]) }
func (s libstdcxxAtomSlice) Swap(i, j int)      { s.a[i], s.a[j] = s.a[j], s.a[i] }

// libstdcxxSortAtoms is std::sort(a.begin(), a.end(), less) for
// theory_arith's flush_bound_axioms. See the WARNING above.
func libstdcxxSortAtoms(a []*smtArithmeticAtom, less func(x, y *smtArithmeticAtom) bool) {
	sorting.LibstdcxxSort(libstdcxxAtomSlice{a: a, less: less})
}

// libstdcxxLRABoundsByValue adapts []*smtLRABound with theory_lra's
// compare_bounds: a1->get_value() < a2->get_value() (value only).
type libstdcxxLRABoundsByValue []*smtLRABound

func (s libstdcxxLRABoundsByValue) Len() int           { return len(s) }
func (s libstdcxxLRABoundsByValue) Less(i, j int) bool { return s[i].value.Cmp(&s[j].value) < 0 }
func (s libstdcxxLRABoundsByValue) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// libstdcxxSortLRABounds is std::sort(a.begin(), a.end(), compare_bounds())
// for theory_lra's flush_bound_axioms. See the WARNING above.
func libstdcxxSortLRABounds(a []*smtLRABound) {
	sorting.LibstdcxxSort(libstdcxxLRABoundsByValue(a))
}
