package simplex

// Source: src/test/simplex.cpp tst_simplex initial solver checks.
import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestSimplexSourceInitialEmptyAndFixedVariable(t *testing.T) {
	s := NewSolver(nil)
	check := func(stage string) {
		t.Helper()
		status, err := s.MakeFeasible()
		if err != nil || status != lbool.True {
			t.Fatal(stage, status, err)
		}
	}
	check("empty")
	for i := uint32(0); i < 5; i++ {
		s.EnsureVariable(i)
	}
	check("unconstrained")
	one := RationalInfinitesimal(rat(1, 1))
	if err := s.SetLower(0, one); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUpper(0, one); err != nil {
		t.Fatal(err)
	}
	check("fixed")
	if got := s.Value(0); got.Main.Cmp(rat(1, 1)) != 0 {
		t.Fatal("fixed variable", got)
	}
}
