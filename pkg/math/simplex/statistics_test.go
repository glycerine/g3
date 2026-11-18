package simplex

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSimplexStatisticsUnsignedWrapPreservesPivotAndWitness(t *testing.T) {
	s := newTestSolver(nil)
	r, err := s.AddRationalRow(1, []uint32{1, 0}, integerRow(1, -1))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetLower(1, testRI(rat(1, 1))); err != nil {
		t.Fatal(err)
	}
	s.Stats = Stats{NumPivots: math.MaxUint32, NumChecks: math.MaxUint32, NumInfeasible: math.MaxUint32}
	status, err := s.MakeFeasible()
	if err != nil || status != lbool.True || s.Stats.NumPivots != 0 || s.Stats.NumChecks != 0 {
		t.Fatal("source feasible pivot counter wrap", status, err, s.Stats)
	}
	base, err := s.BaseVariable(r)
	if err != nil {
		t.Fatal(err)
	}
	if base != 0 {
		t.Fatal("wrapped counter changed the entering variable", base)
	}
	if err = s.SetUpper(0, testRI(rat(0, 1))); err != nil {
		t.Fatal(err)
	}
	status, err = s.MakeFeasible()
	if err != nil || status != lbool.False || s.Stats.NumChecks != 1 || s.Stats.NumInfeasible != 0 {
		t.Fatal("source infeasible check counter wrap", status, err, s.Stats)
	}
	witness, err := s.InfeasibleRow()
	if err != nil || witness != r {
		t.Fatal("counter wrap lost the actual infeasible row", witness, err)
	}
}

func TestSparseMatrixRowAdditionStatisticUnsignedWrap(t *testing.T) {
	matrix := testMatrix()
	a, b := matrix.NewRow(), matrix.NewRow()
	if err := matrix.AddRational(a, rat(1, 1), 0); err != nil {
		t.Fatal(err)
	}
	if err := matrix.AddRational(b, rat(1, 1), 0); err != nil {
		t.Fatal(err)
	}
	matrix.addRows = math.MaxUint32
	if err := matrix.Add(a, rat(1, 1), b); err != nil {
		t.Fatal(err)
	}
	entries, err := matrix.Entries(a)
	if err != nil || matrix.AddRowCount() != 0 || len(entries) != 1 || qcmp(entries[0].Coefficient, rat(2, 1)) != 0 || !matrix.WellFormed() {
		t.Fatal("source matrix statistic wrap changed row addition", entries, err, matrix.AddRowCount())
	}
}
