package simplex

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func integerRow(values ...int64) []big.Rat {
	r := make([]big.Rat, len(values))
	for i, v := range values {
		r[i].SetInt64(v)
	}
	return r
}

func TestSimplexFeasiblePivotAndValues(t *testing.T) {
	s := NewSolver(nil)
	// z - x = 0, z >= 1. The base z starts at zero and pivots with x.
	r, err := s.AddRationalRow(1, []uint32{1, 0}, integerRow(1, -1))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetLower(1, RationalInfinitesimal(rat(1, 1))); err != nil {
		t.Fatal(err)
	}
	result, err := s.MakeFeasible()
	if err != nil || result != lbool.True {
		t.Fatal("feasible", result, err)
	}
	v0, v1 := s.Value(0), s.Value(1)
	if v0.Main.Cmp(rat(1, 1)) != 0 || v1.Main.Cmp(rat(1, 1)) != 0 {
		t.Fatal("pivot values", v0, v1)
	}
	base, err := s.BaseVariable(r)
	if err != nil {
		t.Fatal(err)
	}
	if base != 0 || s.Stats.NumPivots != 1 || s.Stats.NumChecks != 1 {
		t.Fatal("pivot metadata", base, s.Stats)
	}
}

func TestSimplexInfeasibleFixedNonbasicAndWitnessRow(t *testing.T) {
	s := NewSolver(nil)
	r, err := s.AddRationalRow(1, []uint32{1, 0}, integerRow(1, -1))
	if err != nil {
		t.Fatal(err)
	}
	zero := RationalInfinitesimal(rat(0, 1))
	if err = s.SetLower(0, zero); err != nil {
		t.Fatal(err)
	}
	if err = s.SetUpper(0, zero); err != nil {
		t.Fatal(err)
	}
	if err = s.SetLower(1, RationalInfinitesimal(rat(1, 1))); err != nil {
		t.Fatal(err)
	}
	result, err := s.MakeFeasible()
	if err != nil || result != lbool.False {
		t.Fatal("expected infeasible", result, err)
	}
	witness, err := s.InfeasibleRow()
	if err != nil || witness != r {
		t.Fatal("witness", witness, err)
	}
	if s.Stats.NumInfeasible != 1 {
		t.Fatal("statistics", s.Stats)
	}
}

func TestSimplexInfinitesimalStrictBounds(t *testing.T) {
	s := NewSolver(nil)
	_, err := s.AddRationalRow(1, []uint32{1, 0}, integerRow(1, -1))
	if err != nil {
		t.Fatal(err)
	}
	zero := RationalInfinitesimal(rat(0, 1))
	strict := NewInfinitesimal(rat(0, 1), rat(1, 1))
	if strict.Cmp(&zero) <= 0 {
		t.Fatal("epsilon ordering")
	}
	if err = s.SetUpper(0, zero); err != nil {
		t.Fatal(err)
	}
	if err = s.SetLower(1, strict); err != nil {
		t.Fatal(err)
	}
	result, err := s.MakeFeasible()
	if err != nil || result != lbool.False {
		t.Fatal("strict infeasibility", result, err)
	}
}

func TestSimplexIntegerRowFacadeAndReset(t *testing.T) {
	s := NewSolver(nil)
	coeff := []big.Int{*big.NewInt(2), *big.NewInt(-2)}
	if _, err := s.AddIntegerRow(2, []uint32{2, 0}, coeff); err != nil {
		t.Fatal(err)
	}
	if uint32(len(s.vars)) != 3 || !s.matrix.WellFormed() {
		t.Fatal("integer row")
	}
	s.Reset()
	if uint32(len(s.vars)) != 0 || len(s.matrix.rows) != 0 || len(s.patch.values) != 0 {
		t.Fatal("reset")
	}
}

func TestSimplexMinimizeBoundedAndUnbounded(t *testing.T) {
	bounded := NewSolver(nil)
	_, err := bounded.AddRationalRow(1, []uint32{1, 0}, integerRow(1, 1)) // objective + x = 0
	if err != nil {
		t.Fatal(err)
	}
	if err = bounded.SetLower(0, RationalInfinitesimal(rat(0, 1))); err != nil {
		t.Fatal(err)
	}
	if err = bounded.SetUpper(0, RationalInfinitesimal(rat(5, 1))); err != nil {
		t.Fatal(err)
	}
	result, err := bounded.Minimize(1)
	if err != nil || result != lbool.True {
		t.Fatal("bounded minimum", result, err)
	}
	x, objective := bounded.Value(0), bounded.Value(1)
	if x.Main.Cmp(rat(5, 1)) != 0 || objective.Main.Cmp(rat(-5, 1)) != 0 {
		t.Fatal("minimum values", x, objective)
	}

	unbounded := NewSolver(nil)
	_, err = unbounded.AddRationalRow(1, []uint32{1, 0}, integerRow(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	result, err = unbounded.Minimize(1)
	if err != nil || result != lbool.False {
		t.Fatal("unbounded minimum", result, err)
	}
}

func TestSimplexDeleteBaseAndNonbasicRows(t *testing.T) {
	s := NewSolver(nil)
	r, err := s.AddRationalRow(1, []uint32{1, 0}, integerRow(1, -1))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteRow(0); err != nil {
		t.Fatal(err)
	} // pivot x into the row, then delete it.
	if s.matrix.valid(r) || s.matrix.ColumnSize(0) != 0 || s.matrix.ColumnSize(1) != 0 {
		t.Fatal("nonbasic deletion retained row links")
	}
	r, err = s.AddRationalRow(2, []uint32{2, 1}, integerRow(1, -1))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteRow(2); err != nil {
		t.Fatal(err)
	}
	if s.matrix.valid(r) {
		t.Fatal("basic deletion retained row")
	}
}

func addSourceInequality(t *testing.T, s *Solver, a, b, bound int64, equality bool) {
	t.Helper()
	slack := uint32(len(s.vars))
	one := slack + 1
	if slack < 2 {
		slack = 2
		one = 3
	}
	zero := RationalInfinitesimal(rat(0, 1))
	oneValue := RationalInfinitesimal(rat(1, 1))
	if err := s.SetLower(slack, zero); err != nil {
		t.Fatal(err)
	}
	if equality {
		if err := s.SetUpper(slack, zero); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetLower(one, oneValue); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUpper(one, oneValue); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIntegerRow(slack, []uint32{0, 1, slack, one}, []big.Int{*big.NewInt(a), *big.NewInt(b), *big.NewInt(-1), *big.NewInt(-bound)}); err != nil {
		t.Fatal(err)
	}
}

func TestSimplexRetainedSourceFeasibilityCases(t *testing.T) {
	tests := []struct {
		name string
		rows [][4]int64
		want lbool.LBool
	}{
		{"lower-bounds", [][4]int64{{1, 0, 1, 0}, {0, 1, 1, 0}, {1, 1, 1, 0}}, lbool.True},
		{"inconsistent-equality", [][4]int64{{1, 0, 1, 0}, {0, 1, 1, 0}, {1, 1, 1, 1}}, lbool.False},
		{"upper-bounds-equality", [][4]int64{{-1, 0, -1, 0}, {0, -1, -1, 0}, {1, 1, 1, 1}}, lbool.True},
		{"mixed-bounds-equality", [][4]int64{{1, 0, 1, 0}, {0, -1, -1, 0}, {1, 1, 1, 1}}, lbool.True},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := NewSolver(nil)
			s.EnsureVariable(1)
			for _, row := range test.rows {
				addSourceInequality(t, s, row[0], row[1], row[2], row[3] != 0)
			}
			got, err := s.MakeFeasible()
			if err != nil || got != test.want {
				t.Fatal("feasibility", got, err)
			}
		})
	}
}

// AUDIT.md math_misc_02-F2: select_smallest_var is a bare erase_min, so a
// patch entry that is already back in bounds still costs one m_limit.inc().
func TestSimplexMakeFeasibleCountsStalePatchEntries(t *testing.T) {
	lim := &control.Reslimit{}
	s := NewSolver(lim)
	if _, err := s.AddRationalRow(1, []uint32{1, 0}, integerRow(1, -1)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRationalRow(2, []uint32{2, 0}, integerRow(1, -1)); err != nil {
		t.Fatal(err)
	}
	for _, v := range []uint32{1, 2} {
		if err := s.SetLower(v, RationalInfinitesimal(rat(1, 1))); err != nil {
			t.Fatal(err)
		}
	}
	result, err := s.MakeFeasible()
	if err != nil || result != lbool.True {
		t.Fatal("feasible", result, err)
	}
	if lim.Count() != 2 || s.Stats.NumPivots != 1 {
		t.Fatalf("rlimit count = %d, pivots = %d; want 2 and 1", lim.Count(), s.Stats.NumPivots)
	}
}

// AUDIT.md math_misc_02-F2: check_blands_rule runs for the default policy
// and switches to Bland's rule after m_blands_rule_threshold repeats.
func TestSimplexMakeFeasibleBlandAfterThreshold(t *testing.T) {
	s := NewSolver(nil)
	left := make(map[uint32]bool)
	var repeated uint32
	for i := 0; i <= blandsRuleThreshold; i++ {
		s.checkBlandsRule(7, left, &repeated, blandsRuleThreshold)
		if s.arithmeticBland {
			t.Fatalf("Bland's rule after %d selections", i+1)
		}
	}
	s.checkBlandsRule(7, left, &repeated, blandsRuleThreshold)
	if !s.arithmeticBland || repeated != blandsRuleThreshold+1 {
		t.Fatalf("Bland's rule not enabled: repeated=%d", repeated)
	}
}

// AUDIT.md smtcore_12-F4: select_pivot_core breaks (dependencies, column
// size) ties with m_random()%n (random_gen, seed 0). Two rows, each with four
// equally good free candidates; a harness linked against the reference
// build's libz3.a (simplex<mpq_ext>, same rows and bounds) printed
// "status l_true row1 base v3 row2 base v6" with 2 pivots.
func TestSelectPivotRandomTie(t *testing.T) {
	s := NewSolver(nil)
	r1, err := s.AddRationalRow(0, []uint32{0, 1, 2, 3, 4}, integerRow(-1, 1, 1, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := s.AddRationalRow(5, []uint32{5, 6, 7, 8, 9}, integerRow(-1, 1, 1, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	one := RationalInfinitesimal(rat(1, 1))
	if err = s.SetLower(0, one); err != nil {
		t.Fatal(err)
	}
	if err = s.SetLower(5, one); err != nil {
		t.Fatal(err)
	}
	status, err := s.MakeFeasible()
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	b1, _ := s.BaseVariable(r1)
	b2, _ := s.BaseVariable(r2)
	if b1 != 3 || b2 != 6 || s.Stats.NumPivots != 2 || s.Stats.NumChecks != 1 {
		t.Fatalf("row1 base v%d row2 base v%d pivots %d checks %d; want v3 v6 2 1", b1, b2, s.Stats.NumPivots, s.Stats.NumChecks)
	}
}
