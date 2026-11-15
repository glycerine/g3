package hilbert

import "github.com/glycerine/g3/pkg/util/sorting"

func (b *Basis) resolveOrdered(left, right *vector) (*vector, error) {
	return b.resolve(left, right)
}

// basisByMagnitude is vector_lt_t (hilbert_basis.cpp:478-485, 895-904):
// sum_abs(idx1) < sum_abs(idx2). The first magnitude error is kept.
type basisByMagnitude struct {
	values []*vector
	cause  error
}

func (s *basisByMagnitude) Len() int      { return len(s.values) }
func (s *basisByMagnitude) Swap(i, j int) { s.values[i], s.values[j] = s.values[j], s.values[i] }
func (s *basisByMagnitude) Less(i, j int) bool {
	left, leftErr := vectorMagnitude(s.values[i])
	right, rightErr := vectorMagnitude(s.values[j])
	if leftErr != nil {
		s.cause = leftErr
	}
	if rightErr != nil {
		s.cause = rightErr
	}
	return left < right
}

// orderBasis is saturate's
//
//	std::sort(m_basis.begin(), m_basis.end(), vector_lt_t(*this));
//
// (hilbert_basis.cpp:981). USER-APPROVED libstdc++ std::sort CALL SITE
// (owner, 2026-11-14, AUDIT.md math_misc_01-F4): equal magnitudes tie, and
// with more than 16 basis vectors libstdc++'s introsort permutes them; Karr
// dualization follows basis order. It must use
// pkg/util/sorting/libstdcxx_sort.go, never sort.Slice/sort.SliceStable.
func orderBasis(values []*vector) error {
	s := &basisByMagnitude{values: values}
	sorting.LibstdcxxSort(s)
	return s.cause
}
