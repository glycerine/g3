package simplex

import (
	"sync"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var (
	testNumeralsOnce sync.Once
	testNumeralsS    *numeral.RationalService
)

// testNumerals is the tests' numeral service; optQ is its manager.
func testNumerals() *numeral.RationalService {
	testNumeralsOnce.Do(func() {
		budget := arena.NewBudget(^uint64(0))
		s, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
		if err != nil {
			panic(err)
		}
		testNumeralsS = s
	})
	return testNumeralsS
}

// optQ is the tests' rational manager.
func optQ() *numeral.RationalManager { return testNumerals().Manager }

// optRat is a fresh rational(n) of optQ.
func optRat(n int64) *numeral.Rational {
	r := numeral.NewRational()
	if err := optQ().SetInt64(&r, n); err != nil {
		panic(err)
	}
	return &r
}

func optCmp(a *numeral.Rational, n int64) int {
	c, err := optQ().Compare(a, optRat(n))
	if err != nil {
		panic(err)
	}
	return c
}

func optSign(a *numeral.Rational) int {
	s, err := optQ().Sign(a)
	if err != nil {
		panic(err)
	}
	return int(s)
}

func optIsInt(a *numeral.Rational) bool {
	v, err := optQ().IsInt(a)
	if err != nil {
		panic(err)
	}
	return v
}

func optInt64(a *numeral.Rational) int64 {
	v, err := optQ().Int64(a)
	if err != nil {
		panic(err)
	}
	return v
}

func optValue(opt *ModelBasedOpt, v uint32) (*numeral.Rational, error) {
	r := numeral.NewRational()
	return &r, opt.Value(v, &r)
}

func addOptVar(t *testing.T, opt *ModelBasedOpt, value int64, integer bool) uint32 {
	t.Helper()
	id, err := opt.AddVariable(optRat(value), integer)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func optLiveRows(t *testing.T, opt *ModelBasedOpt) []OptRow {
	t.Helper()
	rows, err := opt.LiveRows()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func optTerm(variable uint32, coefficient int64) LinearTerm {
	term, err := NewLinearTerm(optQ(), variable, optRat(coefficient))
	if err != nil {
		panic(err)
	}
	return term
}

func addOptConstraint(t *testing.T, opt *ModelBasedOpt, terms []LinearTerm, constant int64, relation Relation) {
	t.Helper()
	if err := opt.AddConstraint(terms, optRat(constant), relation); err != nil {
		t.Fatal(err)
	}
}

func TestModelBasedOptMaximizeSourceExamples(t *testing.T) {
	opt := NewModelBasedOpt(optQ())
	x := addOptVar(t, opt, 2, false)
	y := addOptVar(t, opt, 3, false)
	z := addOptVar(t, opt, 4, false)
	u := addOptVar(t, opt, 5, false)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(y, -1)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(z, -1)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(y, 1), optTerm(u, -1)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(z, 1), optTerm(u, -1)}, 1, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(u, 1)}, -6, LessEqual)
	if err := opt.SetObjective([]LinearTerm{optTerm(x, 2)}, optRat(0)); err != nil {
		t.Fatal(err)
	}
	maximum, err := opt.Maximize()
	if err != nil {
		t.Fatal(err)
	}
	if maximum.Infinite || optCmp(&maximum.Value, 10) != 0 || optSign(&maximum.Epsilon) != 0 {
		t.Fatal("maximum", maximum)
	}
	value, err := optValue(opt, x)
	if err != nil || optCmp(value, 5) != 0 {
		t.Fatal("updated x", value, err)
	}
}

func TestModelBasedOptIntegerEqualityPreservesIntegralRowsAndModulus(t *testing.T) {
	opt := NewModelBasedOpt(optQ())
	x := addOptVar(t, opt, 2, true)
	y := addOptVar(t, opt, 4, true)
	z := addOptVar(t, opt, 1, true)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 2), optTerm(y, -1)}, 0, Equal)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, -3), optTerm(z, 1)}, 0, LessEqual)
	if err := opt.AddDivides([]LinearTerm{optTerm(x, 1)}, optRat(-2), optRat(3)); err != nil {
		t.Fatal(err)
	}
	if err := opt.Project(x); err != nil {
		t.Fatal(err)
	}
	bound, modulus := false, false
	for _, r := range optLiveRows(t, opt) {
		for _, v := range r.Terms {
			if !optIsInt(&v.Coefficient) || v.Variable == x {
				t.Fatal("integer row lost its domain", r)
			}
		}
		if r.Relation == LessEqual && len(r.Terms) == 2 {
			bound = r.Terms[0].Variable == y && optCmp(&r.Terms[0].Coefficient, -3) == 0 && r.Terms[1].Variable == z && optCmp(&r.Terms[1].Coefficient, 2) == 0 && optSign(&r.Constant) == 0
		}
		if r.Relation == Divides && optCmp(&r.Modulus, 6) == 0 {
			modulus = len(r.Terms) == 1 && r.Terms[0].Variable == y && optCmp(&r.Terms[0].Coefficient, 1) == 0 && optCmp(&r.Constant, -4) == 0
		}
	}
	if !bound || !modulus {
		t.Fatal("expected 2z <= 3y and 6 divides y-4", optLiveRows(t, opt))
	}
}

func TestModelBasedOptIntegerUnitBoundsRetainSourceRepresentative(t *testing.T) {
	opt := NewModelBasedOpt(optQ())
	x := addOptVar(t, opt, 5, true)
	for _, value := range []int64{10, 11} {
		u := addOptVar(t, opt, value, true)
		addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(u, -1)}, 0, LessEqual)
	}
	for _, value := range []int64{0, 1, 2} {
		l := addOptVar(t, opt, value, true)
		addOptConstraint(t, opt, []LinearTerm{optTerm(x, -1), optTerm(l, 1)}, 0, LessEqual)
	}
	if err := opt.Project(x); err != nil {
		t.Fatal(err)
	}
	// Source picks the tight upper representative. Its order constraint and
	// three lower/upper resolvents give four rows, versus six FM pairs.
	if got := len(optLiveRows(t, opt)); got != 4 {
		t.Fatal("integer source bound strategy", got, optLiveRows(t, opt))
	}
}

func TestModelBasedOptStrictAndUnbounded(t *testing.T) {
	strict := NewModelBasedOpt(optQ())
	x := addOptVar(t, strict, 2, false)
	addOptConstraint(t, strict, []LinearTerm{optTerm(x, 1)}, -5, Less)
	if err := strict.SetObjective([]LinearTerm{optTerm(x, 2)}, optRat(0)); err != nil {
		t.Fatal(err)
	}
	maximum, err := strict.Maximize()
	if err != nil {
		t.Fatal(err)
	}
	if maximum.Infinite || optCmp(&maximum.Value, 10) != 0 || optCmp(&maximum.Epsilon, -1) != 0 {
		t.Fatal("strict maximum", maximum)
	}
	value, _ := optValue(strict, x)
	if optCmp(value, 5) >= 0 {
		t.Fatal("strict bound was not restored inside the feasible region", value)
	}

	unbounded := NewModelBasedOpt(optQ())
	x = addOptVar(t, unbounded, 2, false)
	if err := unbounded.SetObjective([]LinearTerm{optTerm(x, 1)}, optRat(0)); err != nil {
		t.Fatal(err)
	}
	maximum, err = unbounded.Maximize()
	if err != nil || !maximum.Infinite {
		t.Fatal("unbounded maximum", maximum, err)
	}
}

func TestModelBasedOptProjectRealBounds(t *testing.T) {
	opt := NewModelBasedOpt(optQ())
	x := addOptVar(t, opt, 2, false)
	y := addOptVar(t, opt, 3, false)
	z := addOptVar(t, opt, 1, false)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(y, -1)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(z, 1), optTerm(x, -1)}, 0, LessEqual)
	if err := opt.Project(x); err != nil {
		t.Fatal(err)
	}
	rows := optLiveRows(t, opt)
	if len(rows) != 1 || len(rows[0].Terms) != 2 {
		t.Fatal("projected rows", rows)
	}
	if rows[0].Terms[0].Variable != y || optCmp(&rows[0].Terms[0].Coefficient, -1) != 0 ||
		rows[0].Terms[1].Variable != z || optCmp(&rows[0].Terms[1].Coefficient, 1) != 0 {
		t.Fatal("expected z - y <= 0", rows[0])
	}
}

func TestModelBasedOptProjectIntegerEqualityAndModulo(t *testing.T) {
	equality := NewModelBasedOpt(optQ())
	x := addOptVar(t, equality, 4, true)
	y := addOptVar(t, equality, 2, true)
	z := addOptVar(t, equality, 5, true)
	addOptConstraint(t, equality, []LinearTerm{optTerm(x, 1), optTerm(y, -2)}, 0, Equal)
	addOptConstraint(t, equality, []LinearTerm{optTerm(x, 1), optTerm(z, -1)}, 0, LessEqual)
	if err := equality.Project(x); err != nil {
		t.Fatal(err)
	}
	for _, row := range optLiveRows(t, equality) {
		for _, term := range row.Terms {
			if term.Variable == x {
				t.Fatal("equality projection retained x", row)
			}
		}
	}

	modular := NewModelBasedOpt(optQ())
	x = addOptVar(t, modular, 5, true)
	y = addOptVar(t, modular, 6, true)
	if err := modular.AddDivides([]LinearTerm{optTerm(x, 1)}, optRat(1), optRat(2)); err != nil {
		t.Fatal(err)
	}
	addOptConstraint(t, modular, []LinearTerm{optTerm(x, 1), optTerm(y, -1)}, 0, LessEqual)
	if err := modular.Project(x); err != nil {
		t.Fatal(err)
	}
	for _, row := range optLiveRows(t, modular) {
		for _, term := range row.Terms {
			if term.Variable == x {
				t.Fatal("modular projection retained x", row)
			}
		}
	}
}

func TestModelBasedOptProjectIntegerNonUnitBounds(t *testing.T) {
	opt := NewModelBasedOpt(optQ())
	x0 := addOptVar(t, opt, 2, true)
	x := addOptVar(t, opt, 1, true)
	y := addOptVar(t, opt, 3, true)
	z := addOptVar(t, opt, 4, true)
	u := addOptVar(t, opt, 5, true)
	v := addOptVar(t, opt, 6, true)
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

// AUDIT.md math_misc_02-F1: model_based_opt::resolve normalizes row_dst after
// the opposite-sign integer mul_add as well as after the same-sign path.
func TestModelBasedOptIntegerOppositeSignResolveNormalizes(t *testing.T) {
	opt := NewModelBasedOpt(optQ())
	x := addOptVar(t, opt, 0, true)
	y := addOptVar(t, opt, 0, true)
	z := addOptVar(t, opt, 0, true)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, 1), optTerm(y, -2)}, 0, LessEqual)
	addOptConstraint(t, opt, []LinearTerm{optTerm(x, -1), optTerm(z, 2)}, 0, LessEqual)
	if err := opt.Project(x); err != nil {
		t.Fatal(err)
	}
	found := optLiveRows(t, opt)
	if len(found) != 1 {
		t.Fatalf("live rows = %+v", found)
	}
	row := found[0]
	coeffs := map[uint32]int64{}
	for _, term := range row.Terms {
		if !optIsInt(&term.Coefficient) {
			t.Fatalf("non-integral coefficient in %+v", row)
		}
		coeffs[term.Variable] = optInt64(&term.Coefficient)
	}
	if len(coeffs) != 2 || coeffs[y] != -1 || coeffs[z] != 1 || optSign(&row.Constant) != 0 || row.Relation != LessEqual {
		t.Fatalf("resolvent = %+v, want z - y <= 0", row)
	}
}
