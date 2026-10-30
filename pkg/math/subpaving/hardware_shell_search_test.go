package subpaving

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestHardwareShellRetainedClauseAndSearchRoute(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	qm, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{PageSize: 2, Budget: budget},
		Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewHardwareContext(m, &control.Reslimit{Limit: 1000}, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewHardwareShellCoefficients(c, qm)
	if err != nil {
		t.Fatal(err)
	}
	values := [3]numeral.Rational{numeral.NewRational(), numeral.NewRational(), numeral.NewRational()}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
		for i := range values {
			if err := qm.Del(&values[i]); err != nil {
				t.Error(err)
			}
		}
		if err := qm.CloseChecked(); err != nil {
			t.Error(err)
		}
		if budget.Used() != 0 {
			t.Error("rational wrapper roots", budget.Used())
		}
	})
	for i := range values {
		if err := qm.SetFractionSigned(&values[i], int32(i), 1); err != nil {
			t.Fatal(err)
		}
	}
	x, err := w.MkVar(false)
	if err != nil {
		t.Fatal(err)
	}
	y, err := w.MkVar(true)
	if err != nil {
		t.Fatal(err)
	}
	if c.isInt[x] || !c.isInt[y] {
		t.Fatal("wrapper must preserve each variable's integer flag")
	}
	z, err := w.MkMonomial([]HardwarePower{{Variable: x, Degree: 2}})
	if err != nil {
		t.Fatal(err)
	}
	xUpperZero, err := w.MkInequality(x, &values[0], false, false)
	if err != nil {
		t.Fatal(err)
	}
	xLowerOne, err := w.MkInequality(x, &values[1], true, false)
	if err != nil {
		t.Fatal(err)
	}
	yLowerTwo, err := w.MkInequality(y, &values[2], true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.AddClause([]*HardwareInequality{xUpperZero}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddClause([]*HardwareInequality{yLowerTwo, xLowerOne}); err != nil {
		t.Fatal(err)
	}
	c.maxDepth = 0
	if err := w.Search(); err != nil {
		t.Fatal(err)
	}
	root := c.root
	if root == nil || root.lower[z] == nil || root.upper[z] == nil ||
		m.ToFloat64(&root.lower[z].value) != 0 || m.ToFloat64(&root.upper[z].value) != 0 ||
		root.lower[y] == nil || m.ToFloat64(&root.lower[y].value) != 2 ||
		root.lower[y].reason != hardwareReasonClause || root.lower[y].reasonClause != c.clauses[0] ||
		c.leafHead != nil || c.numSplits != 0 {
		t.Fatal("wrapper unit, monomial, disjunction, and search route")
	}
}
