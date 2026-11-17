package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func lraObjectiveBlockerShape(t *testing.T, f *lraFrontendFixture, h ast.ASTHandle, v SMTTheoryVar, op int32, r *big.Rat) {
	t.Helper()
	got, err := f.m.GetDeclKind(h)
	astTestMust(t, err)
	lhs, err := f.m.GetArg(h, 0)
	astTestMust(t, err)
	rhs, err := f.m.GetArg(h, 1)
	astTestMust(t, err)
	value, matched, err := f.a.numeralBig(rhs)
	astTestMust(t, err)
	node, err := f.a.Enode(v)
	astTestMust(t, err)
	if got != op || lhs != node.owner || !matched || value.Cmp(r) != 0 {
		t.Fatal("source objective blocker", got, lhs, value, op, node.owner, r)
	}
}

func TestSMTLRAObjectiveStrictValueAndSourceBlocker(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-objective-x"), f.variable("lra-objective-y")
	f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
	f.assert(f.app(smtArithOpGE, x, f.number(3)), false)
	f.assert(f.app(smtArithOpGE, y, f.number(0)), true)
	f.assert(f.app(smtArithOpLE, y, f.number(2)), true)
	term := f.app(smtArithOpAdd, f.number(3), f.app(smtArithOpMul, f.number(2), x), f.app(smtArithOpUMinus, y))
	v, err := f.a.AddObjective(term)
	astTestMust(t, err)
	f.propagate(true)
	value, h, shared, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	h = f.pin(h, nil)
	if value.Unbounded || shared {
		t.Fatal("strict bounded callback", value, shared)
	}
	checkInfiniteArithmetic(t, value.Finite, 0, 9, -2)
	lraObjectiveBlockerShape(t, f, h, v, smtArithOpGE, big.NewRat(9, 1))
	current, err := f.a.ObjectiveValue(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, current.Finite, 0, 9, -2)
	// Mutating the caller's point must not mutate LAR's owned assignment.
	current.Finite.Main.SetInt64(99)
	current, err = f.a.ObjectiveValue(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, current.Finite, 0, 9, -2)
	checkNativeLRANumeric(t)
}

func TestSMTLRAObjectiveRetainsLocalImprovementAndScope(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-local-objective-x"), f.variable("lra-local-objective-y")
	sum := f.app(smtArithOpAdd, x, y)
	objective := f.app(smtArithOpAdd, f.number(2), f.app(smtArithOpMul, f.number(5), x), f.app(smtArithOpMul, f.number(3), y))
	for _, h := range []ast.ASTHandle{f.app(smtArithOpGE, x, f.number(0)), f.app(smtArithOpLE, x, f.number(3)), f.app(smtArithOpGE, y, f.number(0)), f.app(smtArithOpLE, y, f.number(2)), f.app(smtArithOpLE, sum, f.number(4))} {
		f.assert(h, true)
	}
	v, err := f.a.AddObjective(objective)
	astTestMust(t, err)
	f.propagate(true)
	for _, want := range []int64{8, 18, 20} {
		value, h, shared, err := f.a.MaximizeObjective(v)
		astTestMust(t, err)
		h = f.pin(h, nil)
		if value.Unbounded || shared {
			t.Fatal("coupled callback status")
		}
		checkInfiniteArithmetic(t, value.Finite, 0, want, 0)
		lraObjectiveBlockerShape(t, f, h, v, smtArithOpGT, big.NewRat(want, 1))
	}
	astTestMust(t, f.s.PushScope())
	f.assert(f.app(smtArithOpGE, sum, f.number(4)), false)
	f.propagate(true)
	value, h, _, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	h = f.pin(h, nil)
	checkInfiniteArithmetic(t, value.Finite, 0, 20, -3)
	lraObjectiveBlockerShape(t, f, h, v, smtArithOpGE, big.NewRat(20, 1))
	astTestMust(t, f.s.PopScopes(1))
	f.propagate(true)
	value, _, _, err = f.a.MaximizeObjective(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, value.Finite, 0, 20, 0)
	checkNativeLRACoupledProgression(t)
}

func TestSMTLRAObjectiveUnmappedAndUnsupportedRegistration(t *testing.T) {
	d, _ := newLRASourceAssertionContext(t)
	// This test retains the native/disabled Go domain-error contract. The
	// enabled checked source assertion is covered by the dedicated gate test.
	astTestMust(t, d.EnableAssertions(false))
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-objective-unmapped")
	v, err := f.a.AddObjective(x)
	astTestMust(t, err)
	value, h, shared, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	if !value.Unbounded || !f.m.IsFalse(h) || shared {
		t.Fatal("missing-index infinity branch", value, h, shared)
	}
	if _, err = f.a.ObjectiveValue(v); err != ast.ErrASTDomain {
		t.Fatal("unregistered value read", err)
	}
	product := f.app(smtArithOpMul, x, x)
	w, err := f.a.AddObjective(product)
	astTestMust(t, err)
	if w == SMTNullTheoryVar || f.a.notHandled != product {
		t.Fatal("LRA objective added a foreign linearity gate", w)
	}
	boolean := f.pin(f.m.MkTrue())
	if _, err = f.a.AddObjective(boolean); err != ast.ErrASTDomain {
		t.Fatal("Boolean objective accepted", err)
	}
}

func TestSMTLRAObjectiveBlockerRoundingAndEpsilon(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for _, r := range []string{"-2", "-3/2", "3/2", "2"} {
			for _, eps := range []int64{-1, 0, 1} {
				t.Run(fmt.Sprint(integer, "/", r, "/", eps), func(t *testing.T) {
					f := newLRAFrontendFixture(t, true)
					if integer {
						var err error
						f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
						astTestMust(t, err)
					}
					x := f.variable("lra-blocker-rounded")
					v, err := f.a.AddObjective(x)
					astTestMust(t, err)
					rational, _ := new(big.Rat).SetString(r)
					point := simplex.NewInfinitesimal(rational, big.NewRat(eps, 1))
					h := f.pin(f.a.objectiveBlocker(v, point))
					want, op := new(big.Rat).Set(rational), smtArithOpGT
					if integer {
						// Independently ceil the fractional input, or advance one whole integer.
						q, rem := new(big.Int), new(big.Int)
						q.QuoRem(rational.Num(), rational.Denom(), rem)
						if rem.Sign() == 0 || rational.Sign() > 0 {
							q.Add(q, big.NewInt(1))
						}
						want.SetInt(q)
						op = smtArithOpGE
					} else if eps < 0 {
						op = smtArithOpGE
					}
					lraObjectiveBlockerShape(t, f, h, v, op, want)
				})
			}
		}
	}
}

func TestSMTLRAObjectiveLowerBoundReconstructionAndBooleanReuse(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-lower-objective-x"), f.variable("lra-lower-objective-y")
	term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(2), x), f.app(smtArithOpMul, f.number(3), y), f.number(5))
	v, err := f.a.AddObjective(term)
	astTestMust(t, err)
	filter := objectiveBoundFilter(t, f.m)
	astTestMust(t, f.s.PushScope())
	point := simplex.NewInfinitesimal(big.NewRat(7, 1), big.NewRat(1, 1))
	h := f.pin(f.a.MakeObjectiveLowerBound(filter, v, point))
	isNot, _, err := f.m.IsNot(h)
	astTestMust(t, err)
	if !isNot {
		t.Fatal("positive epsilon did not negate an upper-bound atom")
	}
	base, err := f.m.GetArg(h, 0)
	astTestMust(t, err)
	op, err := f.m.GetDeclKind(base)
	astTestMust(t, err)
	if op != smtArithOpLE {
		t.Fatal("strict lower-bound base polarity", op)
	}
	lhs, err := f.m.GetArg(base, 0)
	astTestMust(t, err)
	count, err := f.m.GetNumArgs(lhs)
	astTestMust(t, err)
	if count != 3 {
		t.Fatal("reconstructed coefficient/constant count", count)
	}
	constant, err := f.m.GetArg(lhs, 2)
	astTestMust(t, err)
	cv, matched, err := f.a.numeralBig(constant)
	astTestMust(t, err)
	if !matched || cv.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("objective constant reconstruction", cv)
	}
	seen := make(map[ast.ASTHandle]string)
	for i := uint32(0); i < 2; i++ {
		product, err := f.m.GetArg(lhs, i)
		astTestMust(t, err)
		kind, err := f.m.GetDeclKind(product)
		astTestMust(t, err)
		if kind != smtArithOpMul {
			t.Fatal("reconstructed product missing", kind)
		}
		coefficient, err := f.m.GetArg(product, 0)
		astTestMust(t, err)
		owner, err := f.m.GetArg(product, 1)
		astTestMust(t, err)
		r, matched, err := f.a.numeralBig(coefficient)
		astTestMust(t, err)
		if !matched {
			t.Fatal("coefficient is not a numeral")
		}
		seen[owner] = r.RatString()
	}
	if seen[x] != "2" || seen[y] != "3" {
		t.Fatal("objective coefficient ownership", seen)
	}
	bv, found := f.s.BoolVar(base)
	b := f.a.boundByBoolean[bv]
	if !found || b == nil || b.variable != v || !b.upper || testQCmp(&b.value, big.NewRat(7, 1)) != 0 {
		t.Fatal("native bound not attached to original objective", found, b)
	}
	decl, err := f.m.GetDecl(base)
	astTestMust(t, err)
	refs, err := f.m.GetRefCount(decl)
	astTestMust(t, err)
	before := len(f.a.boundTrail)
	again := f.pin(f.a.MakeObjectiveLowerBound(filter, v, point))
	afterRefs, err := f.m.GetRefCount(decl)
	astTestMust(t, err)
	if again != h || before != len(f.a.boundTrail) || afterRefs != refs {
		t.Fatal("existing Boolean repeated registration/filter insertion")
	}
	astTestMust(t, f.s.PopScopes(1))
	if _, found = f.s.BoolVar(base); found || len(f.a.boundTrail) != 0 || f.a.boundByBoolean[bv] != nil {
		t.Fatal("objective bound survived scope pop")
	}
}

func TestSMTLRAObjectiveLowerBoundActualConstraintsAndDomains(t *testing.T) {
	for _, eps := range []int64{-1, 0, 1} {
		t.Run(fmt.Sprint(eps), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			x, y := f.variable("lra-objective-bound-x"), f.variable("lra-objective-bound-y")
			lraFix(t, f, x, 0)
			lraFix(t, f, y, 0)
			term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(2), x), f.app(smtArithOpMul, f.number(3), y), f.number(5))
			v, err := f.a.AddObjective(term)
			astTestMust(t, err)
			f.propagate(true)
			h := f.pin(f.a.MakeObjectiveLowerBound(objectiveBoundFilter(t, f.m), v, simplex.NewInfinitesimal(big.NewRat(5, 1), big.NewRat(eps, 1))))
			f.assert(h, true)
			f.propagate(eps <= 0)
		})
	}
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-objective-domain")
	v, err := f.a.AddObjective(f.app(smtArithOpAdd, x, f.number(2)))
	astTestMust(t, err)
	if _, err = f.a.MakeObjectiveLowerBound(nil, v, simplex.Infinitesimal{}); err != ast.ErrASTHandle {
		t.Fatal("nil filter", err)
	}
	point := simplex.Infinitesimal{}
	point.Infinity.SetInt64(1)
	if _, err = f.a.MakeObjectiveLowerBound(objectiveBoundFilter(t, f.m), v, point); err != ast.ErrASTDomain {
		t.Fatal("infinity accepted in inf_rational input", err)
	}
}

func TestSMTLRAObjectivePhysicalRefusalAndOwnerCleanup(t *testing.T) {
	refusals, successes := 0, 0
	for _, stop := range []int64{-1, 0, 1, 2, 4, 8, 16, 32} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m := astTestManager(t)
			// Registered before context cleanup, so this census runs after
			// context-owned and caller-owned references have both retired.
			t.Cleanup(func() {
				if m.Stats().LiveReferences != 0 {
					t.Fatal("objective AST references leaked", m.Stats().LiveReferences)
				}
			})
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, _, s := smtInternalizerFixture(t, m, nil)
			a, err := NewSMTLRATheory(family)
			astTestMust(t, err)
			astTestMust(t, s.RegisterTheory(a))
			sort, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			f := &lraFrontendFixture{t, m, s, a, sort}
			x := f.variable("lra-objective-refusal-x")
			objective := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(2), x), f.number(3))
			v, err := f.a.AddObjective(objective)
			astTestMust(t, err)
			filter := objectiveBoundFilter(t, m)
			var padding []ast.ASTRef
			t.Cleanup(func() {
				f.m.Budget().FailAfter(-1)
				for i := range padding {
					astTestMust(t, f.m.DelRef(&padding[i]))
				}
			})
			f.m.Budget().FailAfter(0)
			cold := false
			for i := 0; i < 8192; i++ {
				ref, err := f.m.NewRef(objective)
				if err != nil {
					cold = errors.Is(err, arena.ErrInjectedFailure)
					break
				}
				padding = append(padding, ref)
			}
			f.m.Budget().FailAfter(-1)
			if !cold {
				t.Fatal("did not reach a cold AST reference frontier")
			}
			f.m.Budget().FailAfter(stop)
			result, next := f.a.MakeObjectiveLowerBound(filter, v, simplex.NewInfinitesimal(big.NewRat(5, 1), big.NewRat(1, 1)))
			err = next
			f.m.Budget().FailAfter(-1)
			if err == nil {
				f.pin(result, nil)
			}
			if err != nil {
				var failure umemory.Failure
				memout := errors.As(err, &failure) && failure.Kind == umemory.FailureMemout && failure.ErrorCode() == 101 && stop >= 0
				if !errors.Is(err, arena.ErrInjectedFailure) && !memout {
					t.Fatal(err)
				}
				refusals++
			} else {
				successes++
			}
		})
	}
	if refusals == 0 || successes == 0 {
		t.Fatal("objective samples require physical refusal and success", refusals, successes)
	}
}

func TestSMTLRAObjectiveMappedRayAndConstant(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-mapped-ray")
	f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
	v, err := f.a.AddObjective(x)
	astTestMust(t, err)
	f.propagate(true)
	value, blocker, shared, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	if !value.Unbounded || !f.m.IsFalse(blocker) || shared {
		t.Fatal("mapped unbounded ray", value, blocker, shared)
	}
	g := newLRAFrontendFixture(t, true)
	w, err := g.a.AddObjective(g.number(4))
	astTestMust(t, err)
	value, h, shared, err := g.a.MaximizeObjective(w)
	astTestMust(t, err)
	h = g.pin(h, nil)
	if value.Unbounded || shared {
		t.Fatal("constant objective", value, shared)
	}
	checkInfiniteArithmetic(t, value.Finite, 0, 4, 0)
	lraObjectiveBlockerShape(t, g, h, w, smtArithOpGT, big.NewRat(4, 1))
	lower := g.pin(g.a.MakeObjectiveLowerBound(objectiveBoundFilter(t, g.m), w, simplex.RationalInfinitesimal(big.NewRat(3, 1))))
	if lower.IsNull() {
		t.Fatal("constant lower bound missing")
	}
}
