package simplex

import (
	"math/big"
	"testing"
)

func optTerm(variable uint32, coefficient int64) LinearTerm {
	return NewLinearTerm(variable, big.NewRat(coefficient, 1))
}

func addOptConstraint(t *testing.T, opt *ModelBasedOpt, terms []LinearTerm, constant int64, relation Relation) {
	t.Helper()
	if err := opt.AddConstraint(terms, big.NewRat(constant, 1), relation); err != nil {
		t.Fatal(err)
	}
}

func TestModelBasedOptMaximizeSourceExamples(t *testing.T) {
	opt := NewModelBasedOpt()
	x := opt.AddVariable(big.NewRat(2, 1), false)
	y := opt.AddVariable(big.NewRat(3, 1), false)
	z := opt.AddVariable(big.NewRat(4, 1), false)
	u := opt.AddVariable(big.NewRat(5, 1), false)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(y, -1)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(z, -1)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(y, 1), optTerm(u, -1)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(z, 1), optTerm(u, -1)}, 1, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(u, 1)}, -6, LessEqual)
	if err := opt.SetObjective([]LinearTerm{optTerm(x, 2)}, big.NewRat(0, 1)); err != nil {
		t.Fatal(err)
	}
	maximum, err := opt.Maximize()
	if err != nil {
		t.Fatal(err)
	}
	if maximum.Infinite || maximum.Value.Cmp(big.NewRat(10, 1)) != 0 || maximum.Epsilon.Sign() != 0 {
		t.Fatal("maximum", maximum)
	}
	value, err := opt.Value(x)
	if err != nil || value.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("updated x", value, err)
	}
}

func TestModelBasedOptIntegerEqualityPreservesIntegralRowsAndModulus(t *testing.T) {
	opt := NewModelBasedOpt()
	x := opt.AddVariable(big.NewRat(2, 1), true)
	y := opt.AddVariable(big.NewRat(4, 1), true)
	z := opt.AddVariable(big.NewRat(1, 1), true)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 2), optTerm(y, -1)}, 0, Equal)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, -3), optTerm(z, 1)}, 0, LessEqual)
	if err := opt.AddDivides([]LinearTerm{optTerm(x, 1)}, big.NewRat(-2, 1), big.NewRat(3, 1)); err != nil {
		t.Fatal(err)
	}
	if err := opt.Project(x); err != nil {
		t.Fatal(err)
	}
	bound, modulus := false, false
	for _, r := range opt.LiveRows() {
		for _, v := range r.Terms {
			if !v.Coefficient.IsInt() || v.Variable == x {
				t.Fatal("integer row lost its domain", r)
			}
		}
		if r.Relation == LessEqual && len(r.Terms) == 2 {
			bound = r.Terms[0].Variable == y && r.Terms[0].Coefficient.Cmp(big.NewRat(-3, 1)) == 0 && r.Terms[1].Variable == z && r.Terms[1].Coefficient.Cmp(big.NewRat(2, 1)) == 0 && r.Constant.Sign() == 0
		}
		if r.Relation == Divides && r.Modulus.Cmp(big.NewRat(6, 1)) == 0 {
			modulus = len(r.Terms) == 1 && r.Terms[0].Variable == y && r.Terms[0].Coefficient.Cmp(big.NewRat(1, 1)) == 0 && r.Constant.Cmp(big.NewRat(-4, 1)) == 0
		}
	}
	if !bound || !modulus {
		t.Fatal("expected 2z <= 3y and 6 divides y-4", opt.LiveRows())
	}
}

func TestModelBasedOptIntegerUnitBoundsRetainSourceRepresentative(t *testing.T) {
	opt := NewModelBasedOpt()
	x := opt.AddVariable(big.NewRat(5, 1), true)
	for _, value := range []int64{10, 11} {
		u := opt.AddVariable(big.NewRat(value, 1), true)
		addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(u, -1)}, 0, LessEqual)
	}
	for _, value := range []int64{0, 1, 2} {
		l := opt.AddVariable(big.NewRat(value, 1), true)
		addOptConstraint(t, opt, []LinearTerm{optTerm(x, -1), optTerm(l, 1)}, 0, LessEqual)
	}
	if err := opt.Project(x); err != nil {
		t.Fatal(err)
	}
	// Source picks the tight upper representative. Its order constraint and
	// three lower/upper resolvents give four rows, versus six FM pairs.
	if got := len(opt.LiveRows()); got != 4 {
		t.Fatal("integer source bound strategy", got, opt.LiveRows())
	}
}

func TestModelBasedOptStrictAndUnbounded(t *testing.T) {
	strict := NewModelBasedOpt()
	x := strict.AddVariable(big.NewRat(2, 1), false)
	addOptConstraint(t, strict, []LinearTerm{optTerm(x, 1)}, -5, Less)
	if err := strict.SetObjective([]LinearTerm{optTerm(x, 2)}, big.NewRat(0, 1)); err != nil {
		t.Fatal(err)
	}
	maximum, err := strict.Maximize()
	if err != nil {
		t.Fatal(err)
	}
	if maximum.Infinite || maximum.Value.Cmp(big.NewRat(10, 1)) != 0 || maximum.Epsilon.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("strict maximum", maximum)
	}
	value, _ := strict.Value(x)
	if value.Cmp(big.NewRat(5, 1)) >= 0 {
		t.Fatal("strict bound was not restored inside the feasible region", value)
	}

	unbounded := NewModelBasedOpt()
	x = unbounded.AddVariable(big.NewRat(2, 1), false)
	if err := unbounded.SetObjective([]LinearTerm{optTerm(x, 1)}, big.NewRat(0, 1)); err != nil {
		t.Fatal(err)
	}
	maximum, err = unbounded.Maximize()
	if err != nil || !maximum.Infinite {
		t.Fatal("unbounded maximum", maximum, err)
	}
}

func TestModelBasedOptProjectRealBounds(t *testing.T) {
	opt := NewModelBasedOpt()
	x := opt.AddVariable(big.NewRat(2, 1), false)
	y := opt.AddVariable(big.NewRat(3, 1), false)
	z := opt.AddVariable(big.NewRat(1, 1), false)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(y, -1)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(z, 1), optTerm(x, -1)}, 0, LessEqual)
	if err := opt.Project(x); err != nil {
		t.Fatal(err)
	}
	rows := opt.LiveRows()
	if len(rows) != 1 || len(rows[0].Terms) != 2 {
		t.Fatal("projected rows", rows)
	}
	if rows[0].Terms[0].Variable != y || rows[0].Terms[0].Coefficient.Cmp(big.NewRat(-1, 1)) != 0 ||
		rows[0].Terms[1].Variable != z || rows[0].Terms[1].Coefficient.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("expected z - y <= 0", rows[0])
	}
}

func TestModelBasedOptProjectIntegerEqualityAndModulo(t *testing.T) {
	equality := NewModelBasedOpt()
	x := equality.AddVariable(big.NewRat(4, 1), true)
	y := equality.AddVariable(big.NewRat(2, 1), true)
	z := equality.AddVariable(big.NewRat(5, 1), true)
	addOptConstraint(t, equality, []LinearTerm{optTerm(x, 1), optTerm(y, -2)}, 0, Equal)
	addOptConstraint(t, equality, []LinearTerm{optTerm(x, 1), optTerm(z, -1)}, 0, LessEqual)
	if err := equality.Project(x); err != nil {
		t.Fatal(err)
	}
	for _, row := range equality.LiveRows() {
		for _, term := range row.Terms {
			if term.Variable == x {
				t.Fatal("equality projection retained x", row)
			}
		}
	}

	modular := NewModelBasedOpt()
	x = modular.AddVariable(big.NewRat(5, 1), true)
	y = modular.AddVariable(big.NewRat(6, 1), true)
	if err := modular.AddDivides([]LinearTerm{optTerm(x, 1)}, big.NewRat(1, 1), big.NewRat(2, 1)); err != nil {
		t.Fatal(err)
	}
	addOptConstraint(t, modular, []LinearTerm{optTerm(x, 1), optTerm(y, -1)}, 0, LessEqual)
	if err := modular.Project(x); err != nil {
		t.Fatal(err)
	}
	for _, row := range modular.LiveRows() {
		for _, term := range row.Terms {
			if term.Variable == x {
				t.Fatal("modular projection retained x", row)
			}
		}
	}
}

func TestModelBasedOptProjectIntegerNonUnitBounds(t *testing.T) {
	opt := NewModelBasedOpt()
	x0 := opt.AddVariable(big.NewRat(2, 1), true)
	x := opt.AddVariable(big.NewRat(1, 1), true)
	y := opt.AddVariable(big.NewRat(3, 1), true)
	z := opt.AddVariable(big.NewRat(4, 1), true)
	u := opt.AddVariable(big.NewRat(5, 1), true)
	v := opt.AddVariable(big.NewRat(6, 1), true)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x0, 1), optTerm(y, -2)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(y, -2)}, 0, Less)
	addOptConstraint(t, opt, []LinearTerm{optTerm(y, 3), optTerm(u, -4)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(y, 3), optTerm(z, -5)}, 1, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(y, 3), optTerm(v, -6)}, 1, LessEqual)
	for _, eliminated := range [][]uint32{{y}, {y, x0}} {
		if err := opt.Project(eliminated[len(eliminated)-1]); err != nil {
			t.Fatal(err)
		}
		for id := range opt.rows {
			if !opt.rows[id].Alive {
				continue
			}
			if err := opt.checkRow(uint32(id)); err != nil {
				t.Fatalf("row %d violates the selected model: %v", id, err)
			}
			for _, term := range opt.rows[id].Terms {
				for _, variable := range eliminated {
					if term.Variable == variable {
						t.Fatalf("projection left variable in row %d: %+v", id, opt.rows[id])
					}
				}
			}
		}
	}
}
