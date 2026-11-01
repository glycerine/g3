package simplex

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestArithmeticPivotPolicyChangesFeasiblePath(t *testing.T) {
	for strategy := uint32(0); strategy <= 2; strategy++ {
		for _, upper := range []bool{false, true} {
			t.Run(fmt.Sprintf("strategy%d/upper%v", strategy, upper), func(t *testing.T) {
				s := NewSolver(nil)
				s.SetArithmeticPivotPolicy(strategy, 1000)
				// a=x+y, b=x+2*y. Repairing b first satisfies both bounds
				// in one pivot; repairing a first requires a second pivot.
				for _, row := range []struct {
					base   uint32
					values []int64
				}{{2, []int64{1, -1, -1}}, {3, []int64{1, -1, -2}}} {
					if _, err := s.AddRationalRow(row.base, []uint32{row.base, 0, 1}, integerRow(row.values...)); err != nil {
						t.Fatal(err)
					}
				}
				for _, b := range []struct {
					v     uint32
					value int64
				}{{2, 1}, {3, 3}} {
					var err error
					if upper {
						err = s.SetUpper(b.v, RationalInfinitesimal(rat(-b.value, 1)))
					} else {
						err = s.SetLower(b.v, RationalInfinitesimal(rat(b.value, 1)))
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				status, err := s.MakeFeasible()
				if err != nil || status != lbool.True {
					t.Fatal(status, err)
				}
				for v := uint32(0); v < s.NumVariables(); v++ {
					if s.outside(v) {
						t.Fatal("unrepaired variable", v)
					}
				}
				want := uint32(2)
				if strategy == 1 {
					want = 1
				}
				if s.Stats.NumPivots != want {
					t.Fatal("policy did not change repair path", s.Stats.NumPivots, "want", want)
				}
			})
		}
	}
}

func TestArithmeticPivotStrictErrorAndStalePatch(t *testing.T) {
	for _, strategy := range []uint32{1, 2} {
		s := NewSolver(nil)
		s.SetArithmeticPivotPolicy(strategy, 1000)
		for _, v := range []uint32{0, 1, 2} {
			s.EnsureVariable(v)
		}
		// Equal rational errors, distinguished by epsilon; the upper-bound
		// violation must use the upper bound even when no lower bound exists.
		if err := s.SetLower(0, NewInfinitesimal(rat(1, 1), rat(1, 1))); err != nil {
			t.Fatal(err)
		}
		if err := s.SetUpper(1, NewInfinitesimal(rat(-1, 1), rat(-2, 1))); err != nil {
			t.Fatal(err)
		}
		// Nonbasic bounds repair values immediately, so explicitly stage the
		// same pending violations that tableau propagation queues for bases.
		s.vars[0].value = RationalInfinitesimal(rat(0, 1))
		s.vars[1].value = RationalInfinitesimal(rat(0, 1))
		s.patch.insert(0)
		s.patch.insert(1)
		s.patch.insert(2)
		want := uint32(1)
		if strategy == 2 {
			want = 0
		}
		if v := s.selectArithmeticPatch(); v != want {
			t.Fatal("strict error selection", strategy, v, want)
		}
		if v := s.selectArithmeticPatch(); v != 1-want {
			t.Fatal("remaining violation", v)
		}
		if v := s.selectArithmeticPatch(); v != NullVariable || len(s.patch.values) != 0 {
			t.Fatal("stale feasible patch", v, s.patch)
		}
	}
}

func TestArithmeticBlandThresholdAndEligiblePivot(t *testing.T) {
	s := NewSolver(nil)
	s.SetArithmeticPivotPolicy(1, 1)
	if _, err := s.AddRationalRow(2, []uint32{2, 0, 1}, integerRow(1, -1, -1)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLower(0, RationalInfinitesimal(rat(0, 1))); err != nil {
		t.Fatal(err)
	}
	// The normal dependency heuristic prefers free y; Bland chooses the
	// smaller eligible x even though x has a bound.
	if v, _ := s.selectPivot(2, true); v != 1 {
		t.Fatal("normal pivot", v)
	}
	left := make(map[uint32]bool)
	var repeated uint32
	s.checkArithmeticBlands(2, left, &repeated)
	s.checkArithmeticBlands(2, left, &repeated)
	if s.arithmeticBland {
		t.Fatal("Bland enabled at rather than above threshold")
	}
	s.checkArithmeticBlands(2, left, &repeated)
	if !s.arithmeticBland {
		t.Fatal("repeated leaving variable did not activate Bland")
	}
	if v, _ := s.selectPivot(2, true); v != 0 {
		t.Fatal("Bland pivot", v)
	}
	if err := s.SetUpper(0, RationalInfinitesimal(rat(0, 1))); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.selectPivot(2, true); v != 1 {
		t.Fatal("Bland selected fixed variable", v)
	}
	if status, err := s.MakeFeasible(); err != nil || status != lbool.True || s.arithmeticBland {
		t.Fatal("new feasibility search did not reset Bland", status, err)
	}
}
