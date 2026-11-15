package nlsat

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func evaluatorFixture(t *testing.T) *Evaluator {
	t.Helper()
	am, _ := assignmentFixture(t)
	q, err := am.RationalManager()
	assignmentMust(t, err)
	pm, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{Core: polynomial.MultivariatePolynomialConfig{Integer: q.Integers(), Rationals: q, Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }}})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, pm.CloseChecked()) })
	core, err := pm.ArithmeticCore()
	assignmentMust(t, err)
	for i := uint32(0); i < 2; i++ {
		x, err := core.MkVar()
		assignmentMust(t, err)
		if x != i {
			t.Fatal("variable sequence", x)
		}
	}
	a, err := NewAssignment(am)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, a.Close()) })
	allocator, err := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: q.Integers().RuntimeOwner(), Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: diagnostic.DebugBuild}})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, allocator.Close()) })
	e, err := NewEvaluator(a, pm, allocator)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, e.Close()) })
	return e
}

type evaluatorTerm struct {
	coefficient int32
	y, x        uint32
}

func evaluatorPolynomial(t *testing.T, e *Evaluator, terms ...evaluatorTerm) polynomial.Polynomial {
	t.Helper()
	mm, err := e.core.MonomialManager()
	assignmentMust(t, err)
	coefficients := make([]unumeral.Int, len(terms))
	monomials := make([]polynomial.Monomial, len(terms))
	for i, term := range terms {
		coefficients[i] = unumeral.NewInt(term.coefficient)
		var powers []polynomial.MonomialPower
		if term.y != 0 {
			powers = append(powers, polynomial.MonomialPower{Var: 0, Degree: term.y})
		}
		if term.x != 0 {
			powers = append(powers, polynomial.MonomialPower{Var: 1, Degree: term.x})
		}
		monomials[i], err = mm.MkMonomial(powers)
		assignmentMust(t, err)
	}
	p, err := e.core.MkTerms(coefficients, monomials)
	assignmentMust(t, err)
	assignmentMust(t, e.core.IncRef(p))
	t.Cleanup(func() { assignmentMust(t, e.core.DecRef(p)) })
	return p
}
func evaluatorSetFraction(t *testing.T, e *Evaluator, x uint32, n int64, d uint64) {
	t.Helper()
	q, err := e.am.RationalManager()
	assignmentMust(t, err)
	r := unumeral.NewRational()
	assignmentMust(t, q.SetFraction(&r, n, d))
	var v polynomial.Algebraic
	assignmentMust(t, e.am.SetRational(&v, &r))
	assignmentMust(t, e.assignment.Set(x, &v))
	assignmentMust(t, e.am.Del(&v))
	assignmentMust(t, q.Del(&r))
}
func evaluatorTruth(sign int, k AtomKind, neg bool) bool {
	var value bool
	switch k {
	case AtomEQ, AtomRootEQ:
		value = sign == 0
	case AtomLT, AtomRootLT:
		value = sign < 0
	case AtomGT, AtomRootGT:
		value = sign > 0
	case AtomRootLE:
		value = sign <= 0
	case AtomRootGE:
		value = sign >= 0
	}
	if neg {
		return !value
	}
	return value
}
func evaluatorCheckPoint(t *testing.T, e *Evaluator, s *IntervalSet, xnum int, y int, k AtomKind, neg bool, product int, jst sat.Literal, a ArithmeticAtom, eval bool) {
	t.Helper()
	q, err := e.am.RationalManager()
	assignmentMust(t, err)
	point := unumeral.NewRational()
	assignmentMust(t, q.SetFraction(&point, int64(xnum), 2))
	defer func() { assignmentMust(t, q.Del(&point)) }()
	count, why := intervalActualAt(t, e.ism, s, &point)
	want := evaluatorTruth(product, k, neg)
	if count > 1 || (count == 1) == want || count == 1 && why != jst {
		t.Fatalf("membership x=%d/2 y=%d kind=%v neg=%v sign=%d count=%d why=%v", xnum, y, k, neg, product, count, why)
	}
	if eval {
		evaluatorSetFraction(t, e, 1, int64(xnum), 2)
		got, err := e.Eval(a, neg)
		assignmentMust(t, err)
		if got != want {
			t.Fatal("literal evaluation", xnum, y, k, neg, product, got, want)
		}
	}
}

func TestEvaluatorInequalityProjectionAndEvaluationIndependentModels(t *testing.T) {
	e := evaluatorFixture(t)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{-1, 1, 0}) // x^2-y
	q := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{-1, 0, 0}) // x-1
	r := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 0}, evaluatorTerm{-1, 0, 0}) // y-1
	for _, y := range []int{-1, 1, 4} {
		evaluatorSetFraction(t, e, 0, int64(y), 1)
		for parity := 0; parity < 8; parity++ {
			for _, k := range []AtomKind{AtomEQ, AtomLT, AtomGT} {
				var a InequalityAtom
				assignmentMust(t, a.initialize(k, []polynomial.Polynomial{p, q, r}, []bool{parity&1 != 0, parity&2 != 0, parity&4 != 0}, 1, make([]InequalityFactor, 3)))
				a.boolVar = 19
				for _, neg := range []bool{false, true} {
					// A currently assigned maximal variable is deliberately masked
					// during projection; lower variables stay fixed.
					evaluatorSetFraction(t, e, 1, 101, 1)
					s, err := e.InfeasibleIntervals(&a, neg)
					assignmentMust(t, err)
					for x := -6; x <= 6; x++ {
						factors := []int{x*x - 4*y, x - 2, y - 1}
						product := 1
						for i, f := range factors {
							if parity&(1<<i) != 0 && f < 0 {
								f = -f
							}
							product *= f
						}
						evaluatorCheckPoint(t, e, s, x, y, k, neg, product, sat.MakeLiteral(19, neg), &a, true)
					}
					assignmentMust(t, e.ism.DecRef(s))
				}
			}
		}
	}
}

func TestEvaluatorRootProjectionAndEvaluationIndependentModels(t *testing.T) {
	e := evaluatorFixture(t)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{-1, 1, 0})
	for _, y := range []int{-1, 0, 2, 4} {
		evaluatorSetFraction(t, e, 0, int64(y), 1)
		for _, k := range []AtomKind{AtomRootEQ, AtomRootLT, AtomRootGT, AtomRootLE, AtomRootGE} {
			for index := uint32(1); index <= 3; index++ {
				var a RootAtom
				assignmentMust(t, a.initialize(k, 1, index, p))
				a.boolVar = 23
				for _, neg := range []bool{false, true} {
					evaluatorSetFraction(t, e, 1, 17, 1)
					s, err := e.InfeasibleIntervals(&a, neg)
					assignmentMust(t, err)
					for x := -6; x <= 6; x++ {
						numRoots := 2
						if y < 0 {
							numRoots = 0
						} else if y == 0 {
							numRoots = 1
						}
						if int(index) > numRoots {
							full, err := e.ism.IsFull(s)
							assignmentMust(t, err)
							if full == neg || (s == nil) != neg {
								t.Fatal("missing root", y, index, neg, full)
							}
							if x%3 == 0 {
								evaluatorSetFraction(t, e, 1, int64(x), 2)
								got, err := e.Eval(&a, neg)
								assignmentMust(t, err)
								if got != neg {
									t.Fatal("missing-root evaluation")
								}
							}
							continue
						}
						root := math.Sqrt(float64(y))
						if index == 1 {
							root = -root
						}
						compare := 0
						if float64(x)/2 < root {
							compare = -1
						} else if float64(x)/2 > root {
							compare = 1
						}
						evaluatorCheckPoint(t, e, s, x, y, k, neg, compare, sat.MakeLiteral(23, neg), &a, x%3 == 0)
					}
					assignmentMust(t, e.ism.DecRef(s))
				}
			}
		}
	}
	for i := range e.locals {
		if e.locals[i].value != nil || e.locals[i].accepted != nil || e.locals[i].pending {
			t.Fatal("local reference retained", i)
		}
	}
}

// z3 4.7.1 multiplies the raw numerators of the factor values in a 32-bit
// int, so (x0)(x1) > 0 evaluates to false at x0 = x1 = 50000 (AUDIT.md
// nlsat_01-F2). g3 normalizes the signs as upstream does; without that,
// three factors of 2^22 would wrap a 64-bit product to 0.
func TestEvaluatorInequalitySignProductOverflow(t *testing.T) {
	e := evaluatorFixture(t)
	x0 := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 0})
	x1 := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1})
	for _, value := range []int64{50000, 1 << 22} {
		evaluatorSetFraction(t, e, 0, value, 1)
		evaluatorSetFraction(t, e, 1, value, 1)
		for _, factors := range [][]polynomial.Polynomial{{x0, x1}, {x0, x1, x0}} {
			for _, tc := range []struct {
				kind AtomKind
				want bool
			}{{AtomGT, true}, {AtomLT, false}} {
				var a InequalityAtom
				even := make([]bool, len(factors))
				assignmentMust(t, a.initialize(tc.kind, factors, even, 1, make([]InequalityFactor, len(factors))))
				got, err := e.Eval(&a, false)
				assignmentMust(t, err)
				if got != tc.want {
					t.Fatal("product of", len(factors), "factors at", value, tc.kind, got)
				}
			}
		}
	}
}
