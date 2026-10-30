package polynomial

// Selected registered algebraic.cpp inputs, through actual resource services.

import (
	"sort"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

type algebraicSourceCohort struct {
	service                         *unumeral.RationalService
	polynomial                      *UPolynomialManager
	core                            *AlgebraicRootCore
	binary                          *unumeral.BinaryRationalManager
	budget                          *arena.Budget
	limit                           *control.Reslimit
	algebraicTicks, polynomialTicks uint64
}

func algebraicSourceMust(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func algebraicSourceFixture(t *testing.T) *algebraicSourceCohort {
	t.Helper()
	b := arena.NewBudget(1 << 27)
	s, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{PageSize: 8, Budget: b}, Words: arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 8, Budget: b}}}})
	algebraicSourceMust(t, e)
	limit, e := control.NewReslimit(b)
	algebraicSourceMust(t, e)
	cooperation, e := cooperate.NewCooperation(b)
	algebraicSourceMust(t, e)
	f := &algebraicSourceCohort{service: s, budget: b, limit: limit}
	p, e := NewUPolynomialManagerFromRationalService(s, UPolynomialConfig{Checkpoint: func() bool { f.polynomialTicks++; return limit.Inc() }, Cooperation: func() error { return cooperation.Cooperate("upolynomial") }})
	algebraicSourceMust(t, e)
	f.polynomial = p
	primes, e := primegen.NewGenerator(b)
	algebraicSourceMust(t, e)
	factors, e := NewUPolynomialFactorizationService(p, UPolynomialFactorizationConfig{Primes: primes})
	algebraicSourceMust(t, e)
	binary, e := unumeral.NewBinaryRationalManager(p.IntegerManager(), unumeral.BinaryRationalConfig{Diagnostics: s})
	algebraicSourceMust(t, e)
	f.binary = binary
	intervals, e := unumeral.NewBinaryIntervalManager(binary)
	algebraicSourceMust(t, e)
	allocator, e := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: s.RuntimeOwner(), Profile: umemory.SmallAllocatorProfile{PointerBits: s.RuntimeOwner().SizeTBits(), Debug: polynomialSourceDebug}})
	algebraicSourceMust(t, e)
	core, e := numericRationalService_NewAlgebraicRootCore(s, AlgebraicRootConfig{Polynomials: p, Factors: factors, Binary: binary, Intervals: intervals, Allocator: allocator, Checkpoint: func() bool { f.algebraicTicks++; return limit.Inc() }, Cooperation: func() error { return cooperation.Cooperate("algebraic") }})
	algebraicSourceMust(t, e)
	f.core = core
	t.Cleanup(func() {
		b.FailAfter(-1)
		for _, close := range []func() error{core.Close, allocator.Close, func() error { intervals.Close(); return nil }, binary.CloseChecked, factors.Close, primes.Close, p.Close, cooperation.Close, s.Close} {
			if e := close(); e != nil {
				t.Error(e)
			}
		}
		limit.Close()
		if b.Used() != 0 {
			t.Error("remaining actual owner bytes", b.Used())
		}
	})
	return f
}
func algebraicSourceView(t *testing.T, p *UPolynomialManager, v UPolynomial) UPolynomialView {
	t.Helper()
	n, e := p.Size(v)
	algebraicSourceMust(t, e)
	view, e := p.View(v, 0, n)
	algebraicSourceMust(t, e)
	return view
}

// TestAlgebraicRegisteredEvalSignCases ports the active numeric assertions in
// src/test/algebraic.cpp::tst_eval_sign through the live algebraic evaluator.
func TestAlgebraicRegisteredEvalSignCases(t *testing.T) {
	f := algebraicSourceFixture(t)
	terms := []polynomialEvalTerm{
		{coefficient: 1, degrees: [3]uint32{1, 1, 0}},
		{coefficient: 1, degrees: [3]uint32{0, 2, 0}},
		{coefficient: 1, degrees: [3]uint32{0, 0, 1}},
		{coefficient: 2},
	}
	pm, poly, _ := polynomialEvalFixtureWithRationals(t, f.service.Manager, terms)
	values := algebraicSignTestValues{}
	q := unumeral.NewRational()
	setRational := func(i int, n int64) {
		t.Helper()
		algebraicSourceMust(t, f.service.Manager.SetFraction(&q, n, 1))
		algebraicSourceMust(t, f.core.SetRational(&values.values[i], &q))
	}
	check := func(want int) {
		t.Helper()
		got, e := f.core.EvalSign(pm, poly, &values)
		algebraicSourceMust(t, e)
		if (got == 0) != (want == 0) || (got < 0) != (want < 0) || (got > 0) != (want > 0) {
			t.Fatalf("registered algebraic evaluation sign=%d, want polarity %d", got, want)
		}
	}
	setRational(0, 0)
	setRational(1, 0)
	setRational(2, 0)
	check(1)
	setRational(2, -2)
	check(0)
	setRational(1, 1)
	setRational(0, -3)
	check(-1)

	root := algebraicCompareRoot(t, f.core, []int32{-2, 0, 1}, 1, 0, 2, 0, true)
	algebraicSourceMust(t, f.core.Del(&values.values[0]))
	algebraicSourceMust(t, f.core.Copy(&values.values[0], &root))
	algebraicSourceMust(t, f.core.Del(&root))
	setRational(1, 0)
	check(0)
	setRational(2, 1)
	check(1)
	setRational(2, -3)
	check(-1)
	setRational(1, 1)
	check(1)
	setRational(2, -4)
	check(1)
	setRational(2, -5)
	check(-1)
	setRational(2, -2)
	algebraicSourceMust(t, f.core.Del(&values.values[1]))
	algebraicSourceMust(t, f.core.Copy(&values.values[1], &values.values[0]))
	algebraicSourceMust(t, f.core.Neg(&values.values[1]))
	check(0)
	setRational(2, -3)
	check(-1)

	negatedTerms := []polynomialEvalTerm{
		{coefficient: 1, degrees: [3]uint32{1, 1, 0}},
		{coefficient: 1, degrees: [3]uint32{0, 2, 0}},
		{coefficient: -1, degrees: [3]uint32{0, 0, 1}},
		{coefficient: 2},
	}
	negatedManager, negatedPoly, _ := polynomialEvalFixtureWithRationals(t, f.service.Manager, negatedTerms)
	got, e := f.core.EvalSign(negatedManager, negatedPoly, &values)
	algebraicSourceMust(t, e)
	if got <= 0 {
		t.Fatalf("registered negated-x2 polynomial sign=%d, want positive", got)
	}
	for i := range values.values {
		algebraicSourceMust(t, f.core.Del(&values.values[i]))
	}
	algebraicSourceMust(t, f.service.Manager.Del(&q))
}

// TestAlgebraicRegisteredDejanRadicalSum ports the active radical construction
// and addition sequence in src/test/algebraic.cpp::tst_dejan.
func TestAlgebraicRegisteredDejanRadicalSum(t *testing.T) {
	f := algebraicSourceFixture(t)
	q := unumeral.NewRational()
	var a, b, sum Algebraic
	algebraicSourceMust(t, f.service.Manager.SetFraction(&q, 2, 1))
	algebraicSourceMust(t, f.core.SetRational(&a, &q))
	algebraicSourceMust(t, f.core.Root(&a, 11, &a))
	algebraicSourceMust(t, f.core.SetRational(&b, &q))
	algebraicSourceMust(t, f.core.Root(&b, 7, &b))
	for _, endpoint := range []int64{1, 2} {
		algebraicSourceMust(t, f.service.Manager.SetFraction(&q, endpoint, 1))
		for _, root := range []*Algebraic{&a, &b} {
			cmp, e := f.core.CompareRational(root, &q)
			algebraicSourceMust(t, e)
			if (endpoint == 1 && cmp <= 0) || (endpoint == 2 && cmp >= 0) {
				t.Fatalf("2-root is not between 1 and 2: endpoint=%d comparison=%d", endpoint, cmp)
			}
		}
	}
	algebraicSourceMust(t, f.core.Add(&b, &a, &sum))
	for _, endpoint := range []int64{2, 3} {
		algebraicSourceMust(t, f.service.Manager.SetFraction(&q, endpoint, 1))
		cmp, e := f.core.CompareRational(&sum, &q)
		algebraicSourceMust(t, e)
		if (endpoint == 2 && cmp <= 0) || (endpoint == 3 && cmp >= 0) {
			t.Fatalf("radical sum is not between 2 and 3: endpoint=%d comparison=%d", endpoint, cmp)
		}
	}
	algebraicSourceMust(t, f.core.Del(&sum))
	algebraicSourceMust(t, f.core.Del(&b))
	algebraicSourceMust(t, f.core.Del(&a))
	algebraicSourceMust(t, f.service.Manager.Del(&q))
}

// TestAlgebraicRegisteredBasicRootDegrees ports the active root-degree
// sequence in src/test/algebraic.cpp::tst_root.
func TestAlgebraicRegisteredBasicRootDegrees(t *testing.T) {
	f := algebraicSourceFixture(t)
	q := unumeral.NewRational()
	var input, root Algebraic
	algebraicSourceMust(t, f.service.Manager.SetFraction(&q, 4, 1))
	algebraicSourceMust(t, f.core.SetRational(&input, &q))
	algebraicSourceMust(t, f.core.Root(&input, 2, &root))
	algebraicSourceMust(t, f.service.Manager.SetFraction(&q, 2, 1))
	cmp, e := f.core.CompareRational(&root, &q)
	algebraicSourceMust(t, e)
	if cmp != 0 {
		t.Fatal("sqrt(4)", cmp)
	}
	algebraicSourceMust(t, f.service.Manager.SetFraction(&q, 4, 1))
	algebraicSourceMust(t, f.core.SetRational(&input, &q))
	algebraicSourceMust(t, f.core.Root(&input, 4, &root))
	isRational, e := f.core.IsRational(&root)
	algebraicSourceMust(t, e)
	if isRational {
		t.Fatal("fourth root of four unexpectedly rational")
	}
	for _, endpoint := range []int64{1, 2} {
		algebraicSourceMust(t, f.service.Manager.SetFraction(&q, endpoint, 1))
		cmp, e = f.core.CompareRational(&root, &q)
		algebraicSourceMust(t, e)
		if (endpoint == 1 && cmp <= 0) || (endpoint == 2 && cmp >= 0) {
			t.Fatalf("fourth root of four outside (1,2): comparison at %d = %d", endpoint, cmp)
		}
	}
	algebraicSourceMust(t, f.core.Del(&root))
	algebraicSourceMust(t, f.core.Del(&input))
	algebraicSourceMust(t, f.service.Manager.Del(&q))
}

// TestAlgebraicRegisteredSquareRootFourthPower ports the active power ENSUREs
// in src/test/algebraic.cpp::tst1.
func TestAlgebraicRegisteredSquareRootFourthPower(t *testing.T) {
	f := algebraicSourceFixture(t)
	sqrt2 := algebraicCompareRoot(t, f.core, []int32{-2, 0, 1}, 1, 0, 2, 0, true)
	var power Algebraic
	algebraicSourceMust(t, f.core.Power(&sqrt2, 4, &power))
	isInt, e := f.core.IsInt(&power)
	algebraicSourceMust(t, e)
	if !isInt {
		t.Fatal("sqrt(2)^4 is not an integer")
	}
	q := unumeral.NewRational()
	algebraicSourceMust(t, f.service.Manager.SetFraction(&q, 4, 1))
	cmp, e := f.core.CompareRational(&power, &q)
	algebraicSourceMust(t, e)
	if cmp != 0 {
		t.Fatal("sqrt(2)^4", cmp)
	}
	algebraicSourceMust(t, f.service.Manager.Del(&q))
	algebraicSourceMust(t, f.core.Del(&power))
	algebraicSourceMust(t, f.core.Del(&sqrt2))
}

// TestAlgebraicRegisteredEx1PseudoRemainderSigns ports the executed ex1
// pseudo-remainder chain and its algebraic sign evaluations.
func TestAlgebraicRegisteredEx1PseudoRemainderSigns(t *testing.T) {
	f := algebraicSourceFixture(t)
	terms := []polynomialEvalTerm{
		{coefficient: 1, degrees: [3]uint32{3, 1, 0}},
		{coefficient: 2, degrees: [3]uint32{3, 0, 1}},
		{coefficient: 1, degrees: [3]uint32{1, 1, 0}},
		{coefficient: 1, degrees: [3]uint32{0, 0, 2}},
	}
	pm, p, _ := polynomialEvalFixtureWithRationals(t, f.service.Manager, terms)
	p1, e := pm.Derivative(p, 0)
	algebraicSourceMust(t, e)
	algebraicSourceMust(t, pm.core.IncRef(p1))
	t.Cleanup(func() { algebraicSourceMust(t, pm.core.DecRef(p1)) })
	_, h2Ref := polynomialPseudoOutputs(t, pm.core)
	_, h3Ref := polynomialPseudoOutputs(t, pm.core)
	d2, d3 := uint32(0), uint32(0)
	algebraicSourceMust(t, pm.core.PseudoRemainder(p, p1, 0, &d2, h2Ref))
	h2, e := h2Ref.Get()
	algebraicSourceMust(t, e)
	algebraicSourceMust(t, pm.core.PseudoRemainder(p1, h2, 0, &d3, h3Ref))
	h3, e := h3Ref.Get()
	algebraicSourceMust(t, e)
	if d2 == 0 || d3 == 0 {
		t.Fatalf("source pseudo-remainder scaling degrees: %d, %d", d2, d3)
	}

	values := &algebraicIsolationAssignments{assigned: [3]bool{false, true, true}}
	values.values[1] = algebraicCompareRoot(t, f.core, []int32{-2, 0, 0, 1}, 1, 0, 2, 0, true)
	values.values[2] = algebraicCompareRoot(t, f.core, []int32{-3, 0, 0, 1}, 1, 0, 2, 0, true)
	eval := func(poly Polynomial, want int) {
		t.Helper()
		sign, e := f.core.EvalSign(pm, poly, values)
		algebraicSourceMust(t, e)
		if (sign < 0) != (want < 0) || (sign == 0) != (want == 0) {
			t.Fatalf("source ex1 sign=%d want polarity %d", sign, want)
		}
	}
	// h3 is independent of x0, so the first source assignment map has only
	// the two algebraic cube-root parameters.
	eval(h3, 1)
	q := unumeral.NewRational()
	for _, x := range []struct {
		n, d int64
		p    int
	}{{-1, 1, -1}, {-1, 2, 1}} {
		algebraicSourceMust(t, f.service.Manager.SetFraction(&q, x.n, uint64(x.d)))
		algebraicSourceMust(t, f.core.SetRational(&values.values[0], &q))
		values.assigned[0] = true
		eval(h2, 1)
		eval(p1, 1)
		eval(p, x.p)
	}
	algebraicSourceMust(t, f.service.Manager.Del(&q))
	for i := range values.values {
		algebraicSourceMust(t, f.core.Del(&values.values[i]))
	}
}

// TestAlgebraicRegisteredTst1ArithmeticRoutes covers the executed arithmetic
// and root operations in tst1 before its source-level return.
func TestAlgebraicRegisteredTst1ArithmeticRoutes(t *testing.T) {
	f := algebraicSourceFixture(t)
	q := unumeral.NewRational()
	set := func(a *Algebraic, n int64, d uint64) {
		t.Helper()
		algebraicSourceMust(t, f.service.Manager.SetFraction(&q, n, d))
		algebraicSourceMust(t, f.core.SetRational(a, &q))
	}
	compare := func(a *Algebraic, n int64, d uint64) int {
		t.Helper()
		algebraicSourceMust(t, f.service.Manager.SetFraction(&q, n, d))
		cmp, e := f.core.CompareRational(a, &q)
		algebraicSourceMust(t, e)
		return cmp
	}
	between := func(a *Algebraic, lo, hi int64) {
		t.Helper()
		if compare(a, lo, 1) <= 0 || compare(a, hi, 1) >= 0 {
			t.Fatalf("value not strictly between %d and %d", lo, hi)
		}
	}
	var sqrt2, negativeSqrt2, aq, gauss, sum, product Algebraic
	sqrt2 = algebraicCompareRoot(t, f.core, []int32{-2, 0, 1}, 1, 0, 2, 0, true)
	negativeSqrt2 = algebraicCompareRoot(t, f.core, []int32{-2, 0, 1}, -2, 0, -1, 0, true)
	set(&aq, 1, 3)
	algebraicSourceMust(t, f.core.Add(&sqrt2, &aq, &aq))
	between(&aq, 1, 2)
	set(&aq, 1, 3)
	algebraicSourceMust(t, f.core.Add(&negativeSqrt2, &aq, &aq))
	between(&aq, -2, -1)

	gauss = algebraicCompareRoot(t, f.core, []int32{-1, -1, 0, 0, 0, 1}, 1, 0, 2, 0, true)
	algebraicSourceMust(t, f.core.Add(&sqrt2, &gauss, &sum))
	between(&sum, 2, 3)
	algebraicSourceMust(t, f.core.Mul(&sqrt2, &sqrt2, &product))
	if compare(&product, 2, 1) != 0 {
		t.Fatal("sqrt(2)*sqrt(2) != 2")
	}

	var minusThree, fifthRootNegative, cubeRootSqrt2, one, cubeRootSum, fifthRootSum Algebraic
	set(&minusThree, -3, 1)
	algebraicSourceMust(t, f.core.Root(&minusThree, 5, &fifthRootNegative))
	between(&fifthRootNegative, -2, -1)
	algebraicSourceMust(t, f.core.Root(&sqrt2, 3, &cubeRootSqrt2))
	between(&cubeRootSqrt2, 1, 2)
	set(&one, 1, 1)
	algebraicSourceMust(t, f.core.Add(&sqrt2, &one, &sum))
	algebraicSourceMust(t, f.core.Root(&sum, 3, &cubeRootSum))
	between(&cubeRootSum, 1, 2)
	algebraicSourceMust(t, f.core.Root(&sum, 5, &fifthRootSum))
	between(&fifthRootSum, 1, 2)

	var hiddenSqrt2, quotientOne, quotientGauss Algebraic
	hiddenSqrt2 = algebraicCompareRoot(t, f.core, []int32{-2, 0, 1}, 1, 0, 2, 0, true)
	algebraicSourceMust(t, f.core.Div(&sqrt2, &hiddenSqrt2, &quotientOne))
	if compare(&quotientOne, 1, 1) != 0 {
		t.Fatal("sqrt(2)/sqrt(2) != 1")
	}
	algebraicSourceMust(t, f.core.Div(&sqrt2, &gauss, &quotientGauss))
	between(&quotientGauss, 1, 2)

	var inverse, four, tmp Algebraic
	algebraicSourceMust(t, f.core.Copy(&inverse, &sqrt2))
	algebraicSourceMust(t, f.core.Inv(&inverse))
	set(&four, 4, 1)
	algebraicSourceMust(t, f.core.Mul(&inverse, &four, &tmp))
	algebraicSourceMust(t, f.core.Mul(&tmp, &sqrt2, &tmp))
	isInt, e := f.core.IsInt(&tmp)
	algebraicSourceMust(t, e)
	if !isInt || compare(&tmp, 4, 1) != 0 {
		t.Fatal("sqrt(2)*4*(1/sqrt(2)) != 4")
	}

	// The final source polynomial has roots -sqrt(15), 707/499, sqrt(15);
	// its middle root is rational, as exercised by the source is_rational call.
	polynomial, e := f.polynomial.NewPolynomial()
	algebraicSourceMust(t, e)
	t.Cleanup(func() { algebraicSourceMust(t, f.polynomial.DelPolynomial(&polynomial)) })
	algebraicSourceMust(t, f.polynomial.SetInts(polynomial, []unumeral.Int{unumeral.NewInt(21210), unumeral.NewInt(-14970), unumeral.NewInt(-1414), unumeral.NewInt(998)}))
	view := algebraicSourceView(t, f.polynomial, polynomial)
	roots, e := f.core.NewVector()
	algebraicSourceMust(t, e)
	algebraicSourceMust(t, f.core.IsolateRoots(view, roots))
	if roots.Len() != 3 {
		t.Fatal("source polynomial roots", roots.Len())
	}
	middle, e := roots.Get(1)
	algebraicSourceMust(t, e)
	isRational, e := f.core.IsRational(&middle)
	algebraicSourceMust(t, e)
	if !isRational {
		t.Fatal("source middle root is not rational")
	}
	algebraicSourceMust(t, f.service.Manager.SetFraction(&q, 707, 499))
	if cmp, e := f.core.CompareRational(&middle, &q); e != nil || cmp != 0 {
		t.Fatalf("source middle root comparison=%d, err=%v", cmp, e)
	}
	algebraicSourceMust(t, roots.Close())
	algebraicSourceMust(t, f.service.Manager.Del(&q))
	for _, a := range []*Algebraic{&sqrt2, &negativeSqrt2, &aq, &gauss, &sum, &product, &minusThree, &fifthRootNegative, &cubeRootSqrt2, &one, &cubeRootSum, &fifthRootSum, &hiddenSqrt2, &quotientOne, &quotientGauss, &inverse, &four, &tmp} {
		algebraicSourceMust(t, f.core.Del(a))
	}
}

type algebraicSourceSparseTerm struct {
	powers [4]uint32
	coeff  int32
}

// algebraicSourceProduct expands a product of factors of the form
// x3 - (a sparse polynomial in x0..x2), matching the source isolation inputs.
func algebraicSourceProduct(factors ...[]algebraicSourceSparseTerm) ([]int32, [][]MonomialPower) {
	terms := map[[4]uint32]int32{[4]uint32{}: 1}
	for _, value := range factors {
		next := make(map[[4]uint32]int32)
		for powers, coeff := range terms {
			y := powers
			y[3]++
			next[y] += coeff
			for _, term := range value {
				product := powers
				for i := range product {
					product[i] += term.powers[i]
				}
				next[product] -= coeff * term.coeff
			}
		}
		for powers, coeff := range next {
			if coeff == 0 {
				delete(next, powers)
			}
		}
		terms = next
	}
	ordered := make([][4]uint32, 0, len(terms))
	for powers := range terms {
		ordered = append(ordered, powers)
	}
	sort.Slice(ordered, func(i, j int) bool {
		for k := range ordered[i] {
			if ordered[i][k] != ordered[j][k] {
				return ordered[i][k] < ordered[j][k]
			}
		}
		return false
	})
	coeffs, monomials := make([]int32, len(ordered)), make([][]MonomialPower, len(ordered))
	for i, powers := range ordered {
		coeffs[i] = terms[powers]
		for variable, degree := range powers {
			if degree != 0 {
				monomials[i] = append(monomials[i], MonomialPower{Var: uint32(variable), Degree: degree})
			}
		}
	}
	return coeffs, monomials
}

func TestAlgebraicRegisteredMultivariateIsolationInputs(t *testing.T) {
	f := algebraicSourceFixture(t)
	pm, _, _ := polynomialEvalFixtureWithRationals(t, f.service.Manager, nil)
	variable, e := pm.core.monomials.MkVar()
	algebraicSourceMust(t, e)
	if variable != 3 {
		t.Fatalf("source varying variable index=%d", variable)
	}
	values := &algebraicIsolationAssignments{assigned: [3]bool{true, true, true}}
	q := unumeral.NewRational()
	setRational := func(i int, n int64) {
		t.Helper()
		algebraicSourceMust(t, f.service.Manager.SetFraction(&q, n, 1))
		algebraicSourceMust(t, f.core.SetRational(&values.values[i], &q))
	}
	setRoot := func(i int, root *Algebraic) {
		t.Helper()
		algebraicSourceMust(t, f.core.Del(&values.values[i]))
		algebraicSourceMust(t, f.core.Copy(&values.values[i], root))
	}
	for i := range values.values {
		setRational(i, 0)
	}
	x0 := []algebraicSourceSparseTerm{{powers: [4]uint32{1, 0, 0, 0}, coeff: 1}}
	x1 := []algebraicSourceSparseTerm{{powers: [4]uint32{0, 1, 0, 0}, coeff: 1}}
	one := []algebraicSourceSparseTerm{{coeff: 1}}
	two := []algebraicSourceSparseTerm{{coeff: 2}}
	check := func(name string, coeffs []int32, monomials [][]MonomialPower, want int) {
		t.Helper()
		p := polynomialQueryTerms(t, pm.core, coeffs, monomials)
		roots, e := f.core.NewVector()
		algebraicSourceMust(t, e)
		var signs []int
		e = f.core.IsolateMultivariateRootsWithSigns(pm, p, values, roots, &signs)
		algebraicSourceMust(t, e)
		if len(roots.values) != want || len(signs) != len(roots.values)+1 {
			t.Fatalf("%s root/sign counts: roots=%d signs=%v want roots=%d", name, len(roots.values), signs, want)
		}
		algebraicSourceMust(t, roots.Close())
	}
	checkProduct := func(name string, factors [][]algebraicSourceSparseTerm, want int) {
		t.Helper()
		coeffs, monomials := algebraicSourceProduct(factors...)
		check(name, coeffs, monomials, want)
	}

	// Source x3*x1+1: first x1=0 (no roots), then x1=1 and sqrt(2).
	check("x3*x1+1 at zero", []int32{1, 1}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 1}}, nil}, 0)
	setRational(1, 1)
	check("x3*x1+1 at one", []int32{1, 1}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 1}}, nil}, 1)
	sqrt2 := algebraicCompareRoot(t, f.core, []int32{-2, 0, 1}, 1, 0, 2, 0, true)
	setRoot(1, &sqrt2)
	check("x3*x1+1 at sqrt2", []int32{1, 1}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 1}}, nil}, 1)
	algebraicSourceMust(t, f.core.Copy(&values.values[2], &sqrt2))
	check("(x1+x2)*x3+1", []int32{1, 1, 1}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 1}}, {{Var: 2, Degree: 1}, {Var: 3, Degree: 1}}, nil}, 1)
	check("(x1+x2)*x3+x1*x2+2", []int32{1, 1, 1, 2}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 1}}, {{Var: 2, Degree: 1}, {Var: 3, Degree: 1}}, {{Var: 1, Degree: 1}, {Var: 2, Degree: 1}}, nil}, 1)
	check("(x1+x2)*x3^3+x1*x2+2", []int32{1, 1, 1, 2}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 3}}, {{Var: 2, Degree: 1}, {Var: 3, Degree: 3}}, {{Var: 1, Degree: 1}, {Var: 2, Degree: 1}}, nil}, 1)
	check("(x1+x2)*x3^2-x1*x2-2", []int32{1, 1, -1, -2}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 2}}, {{Var: 2, Degree: 1}, {Var: 3, Degree: 2}}, {{Var: 1, Degree: 1}, {Var: 2, Degree: 1}}, nil}, 2)
	check("x0*(x1+x2)*x3^2-x0*x1*x2-2", []int32{1, 1, -1, -2}, [][]MonomialPower{{{Var: 0, Degree: 1}, {Var: 1, Degree: 1}, {Var: 3, Degree: 2}}, {{Var: 0, Degree: 1}, {Var: 2, Degree: 1}, {Var: 3, Degree: 2}}, {{Var: 0, Degree: 1}, {Var: 1, Degree: 1}, {Var: 2, Degree: 1}}, nil}, 0)
	// Equal algebraic coefficients cancel, leaving the identically zero
	// polynomial in x3, as in the two source difference cases.
	check("(x1-x2)*x3+x1*x2-2", []int32{1, -1, 1, -2}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 1}}, {{Var: 2, Degree: 1}, {Var: 3, Degree: 1}}, {{Var: 1, Degree: 1}, {Var: 2, Degree: 1}}, nil}, 0)
	check("(x1-x2)*x3^3+x1*x2-2", []int32{1, -1, 1, -2}, [][]MonomialPower{{{Var: 1, Degree: 1}, {Var: 3, Degree: 3}}, {{Var: 2, Degree: 1}, {Var: 3, Degree: 3}}, {{Var: 1, Degree: 1}, {Var: 2, Degree: 1}}, nil}, 0)

	// Final source block uses sqrt(2), sqrt(3), and reset x2=0.
	sqrt3 := algebraicCompareRoot(t, f.core, []int32{-3, 0, 1}, 1, 0, 2, 0, true)
	setRoot(0, &sqrt2)
	setRoot(1, &sqrt3)
	setRational(2, 0)
	checkProduct("(x3-x0)*(x3-x0-x1)", [][]algebraicSourceSparseTerm{x0, append(x0, x1...)}, 2)
	checkProduct("(x3-x0)*(x3-x0-x1)^2", [][]algebraicSourceSparseTerm{x0, append(x0, x1...), append(x0, x1...)}, 2)
	checkProduct("(x3-x0)*(x3-2)*(x3-1)^2*(x3-x1)", [][]algebraicSourceSparseTerm{x0, two, one, one, x1}, 4)
	algebraicSourceMust(t, f.core.Del(&sqrt2))
	algebraicSourceMust(t, f.core.Del(&sqrt3))
	for i := range values.values {
		algebraicSourceMust(t, f.core.Del(&values.values[i]))
	}
	algebraicSourceMust(t, f.service.Manager.Del(&q))
}
func TestAlgebraicRegisteredUnivariateRootCounts(t *testing.T) {
	f := algebraicSourceFixture(t)
	// algebraic.cpp:57,65,91,107. The third row is independently expanded from
	// (x^5-x-1)*(x-1)*(x-2). Full tst1 arithmetic/printers remain separate.
	cases := []struct {
		coeff []int32
		count uint32
	}{{[]int32{-2, 3}, 1}, {[]int32{-2, 0, 1}, 2}, {[]int32{-2, 1, 2, -1, 0, 2, -3, 1}, 3}, {[]int32{6, 0, -5, 0, 1}, 4}}
	for _, x := range cases {
		p := upolySourceNew(t, f.polynomial, x.coeff)
		roots, e := (f.core).NewVector()
		algebraicSourceMust(t, e)
		algebraicSourceMust(t, f.core.IsolateRoots(algebraicSourceView(t, f.polynomial, p), roots))
		if roots.Len() != x.count {
			t.Fatalf("%v: roots %d want %d", x.coeff, roots.Len(), x.count)
		}
		for i := uint32(1); i < roots.Len(); i++ {
			cmp, e := roots.Compare(i-1, i)
			algebraicSourceMust(t, e)
			if cmp >= 0 {
				t.Fatal("actual root order", i, cmp)
			}
		}
		algebraicSourceMust(t, roots.Close())
	}
	if f.polynomialTicks == 0 || f.limit.Count() != f.polynomialTicks+f.algebraicTicks {
		t.Fatal("actual resource consumer", f.limit.Count(), f.polynomialTicks, f.algebraicTicks)
	}
}
func TestAlgebraicRegisteredWilkinsonTwentyIntegerRoots(t *testing.T) {
	f := algebraicSourceFixture(t)
	// algebraic.cpp:224-248: build the same twenty factors using actual dense
	// polynomial multiplication, then retain the count and all twenty IsInt ENSUREs.
	p := upolySourceNew(t, f.polynomial, []int32{-1, 1})
	factor := upolySourceNew(t, f.polynomial, []int32{0, 1})
	for i := int32(2); i <= 20; i++ {
		z := unumeral.NewInt(-i)
		algebraicSourceMust(t, f.polynomial.SetCoefficient(factor, 0, &z))
		algebraicSourceMust(t, f.polynomial.Mul(p, factor, p))
	}
	roots, e := (f.core).NewVector()
	algebraicSourceMust(t, e)
	algebraicSourceMust(t, f.core.IsolateRoots(algebraicSourceView(t, f.polynomial, p), roots))
	if roots.Len() != 20 {
		t.Fatal("Wilkinson root count", roots.Len())
	}
	for i := uint32(0); i < 20; i++ {
		integer, e := roots.IsInt(i)
		algebraicSourceMust(t, e)
		if !integer {
			t.Fatal("Wilkinson integer root", i)
		}
	}
	algebraicSourceMust(t, roots.Close())
	if f.limit.Count() == 0 || f.limit.Count() != f.polynomialTicks+f.algebraicTicks {
		t.Fatal("actual Wilkinson resource calls")
	}
}
func TestAlgebraicRealResourceCancellationBeforeMinimalRefine(t *testing.T) {
	f := algebraicSourceFixture(t)
	p := upolySourceNew(t, f.polynomial, []int32{-2, 0, 1})
	q := upolySourceNew(t, f.polynomial, []int32{-3, 0, 1})
	lo, hi := unumeral.NewBinaryInt(1), unumeral.NewBinaryInt(2)
	var a, b Algebraic
	algebraicSourceMust(t, f.core.SetRoot(&a, algebraicSourceView(t, f.polynomial, p), &lo, &hi, true))
	algebraicSourceMust(t, f.core.SetRoot(&b, algebraicSourceView(t, f.polynomial, q), &lo, &hi, true))
	t.Cleanup(func() { algebraicSourceMust(t, f.core.Del(&a)); algebraicSourceMust(t, f.core.Del(&b)) })
	before := f.limit.Count()
	ticks := f.algebraicTicks
	f.limit.Cancel()
	_, e := f.core.Compare(&a, &b)
	if e == nil {
		t.Fatal("actual canceled source checkpoint accepted")
	}
	if f.limit.Count() != before+1 || f.algebraicTicks != ticks+1 {
		t.Fatal("minimal comparison cancellation order", before, f.limit.Count(), ticks, f.algebraicTicks)
	}
	f.limit.ResetCancel()
	cmp, e := f.core.Compare(&a, &b)
	algebraicSourceMust(t, e)
	if cmp != -1 {
		t.Fatal("sqrt2/sqrt3 after actual cancellation", cmp)
	}
}
