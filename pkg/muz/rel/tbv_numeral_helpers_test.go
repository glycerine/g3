package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// tbvTestService is a standalone rational service for tbv/doc rational
// allocation tests.
func tbvTestService(t testing.TB) *numeral.RationalService {
	t.Helper()
	budget := arena.NewBudget(^uint64(0))
	s, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// tbvTestRational is rational(v) of s.
func tbvTestRational(t testing.TB, s *numeral.RationalService, v int64) *numeral.Rational {
	t.Helper()
	r := numeral.NewRational()
	if err := s.Manager.SetInt64(&r, v); err != nil {
		t.Fatal(err)
	}
	return &r
}
