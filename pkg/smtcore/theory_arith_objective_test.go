package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type arithmeticObjectiveFixture struct {
	t       *testing.T
	m       *ast.ASTManager
	s       *SMTInternalizerState
	a       *SMTLinearArithmeticTheory
	sort    ast.ASTHandle
	integer bool
}

func newArithmeticObjectiveFixture(t *testing.T, integer bool) *arithmeticObjectiveFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a, err := NewSMTLinearArithmeticTheory(family)
	if integer {
		a, err = NewSMTLinearIntegerArithmeticTheory(family)
	}
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(a))
	kind := int32(0)
	if integer {
		kind = 1
	}
	sort, err := m.MkFamilySort(family, kind, nil)
	astTestMust(t, err)
	return &arithmeticObjectiveFixture{t: t, m: m, s: s, a: a, sort: sort, integer: integer}
}

func (f *arithmeticObjectiveFixture) pin(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	astTestMust(f.t, err)
	r, err := f.m.NewRef(h)
	astTestMust(f.t, err)
	f.t.Cleanup(func() { astTestMust(f.t, f.m.DelRef(&r)) })
	return h
}

func (f *arithmeticObjectiveFixture) variable(name string) ast.ASTHandle {
	f.t.Helper()
	return f.pin(f.m.MkConst(astTestSymbol(f.t, f.m, name), f.sort))
}

func (f *arithmeticObjectiveFixture) number(text string) ast.ASTHandle {
	f.t.Helper()
	if f.integer {
		return f.pin(f.m.ArithmeticIntegerNumeralText(text))
	}
	return f.pin(f.m.ArithmeticRealNumeralText(text))
}

func (f *arithmeticObjectiveFixture) app(op int32, args ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	return f.pin(f.m.MkFamilyApp(f.a.FamilyID(), op, nil, args))
}

func (f *arithmeticObjectiveFixture) assert(h ast.ASTHandle) {
	f.t.Helper()
	astTestMust(f.t, f.s.Internalize(h, true))
	v, found := f.s.BoolVar(h)
	if !found {
		f.t.Fatal("arithmetic bound was not internalized")
	}
	astTestMust(f.t, f.a.Assign(v, true))
}

func (f *arithmeticObjectiveFixture) objective(h ast.ASTHandle) SMTTheoryVar {
	f.t.Helper()
	v, err := f.a.AddObjective(h)
	astTestMust(f.t, err)
	if v == SMTNullTheoryVar {
		f.t.Fatal("linear objective refused")
	}
	astTestMust(f.t, f.a.Propagate())
	if f.s.inconsistent {
		f.t.Fatal("objective fixture is inconsistent")
	}
	return v
}

func checkArithmeticObjective(t *testing.T, value SMTArithmeticObjectiveValue, main, epsilon string) {
	t.Helper()
	r, ok := new(big.Rat).SetString(main)
	if !ok {
		t.Fatal(main)
	}
	e, ok := new(big.Rat).SetString(epsilon)
	if !ok {
		t.Fatal(epsilon)
	}
	if value.Unbounded || value.Finite.Main.Cmp(r) != 0 || value.Finite.Epsilon.Cmp(e) != 0 {
		t.Fatal("objective value", value.Unbounded, value.Finite.String(), "want", main, epsilon)
	}
}

func checkArithmeticBlocker(t *testing.T, f *arithmeticObjectiveFixture, h, objective ast.ASTHandle, op int32, bound string) {
	t.Helper()
	kind, err := f.m.GetDeclKind(h)
	astTestMust(t, err)
	lhs, err := f.m.GetArg(h, 0)
	astTestMust(t, err)
	rhs, err := f.m.GetArg(h, 1)
	astTestMust(t, err)
	text, _, numeral, err := f.m.ArithmeticNumeralText(rhs)
	astTestMust(t, err)
	if kind != op || lhs != objective || !numeral || text != bound {
		t.Fatal("objective blocker", kind, lhs == objective, text, "want", op, bound)
	}
}

func TestSMTArithmeticObjectiveBoundedAndStrict(t *testing.T) {
	for _, test := range []struct{ integer, strict, sum bool }{{false, false, false}, {false, true, false}, {true, false, false}, {false, false, true}, {false, true, true}} {
		t.Run(fmt.Sprint(test), func(t *testing.T) {
			f := newArithmeticObjectiveFixture(t, test.integer)
			x := f.variable("objective-x")
			op := smtArithOpLE
			if test.strict {
				op = smtArithOpLT
			}
			f.assert(f.app(op, x, f.number("5")))
			obj := x
			if test.sum {
				obj = f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number("2"), x), f.number("1"))
			}
			v := f.objective(obj)
			value, blocker, shared, err := f.a.MaximizeObjective(v)
			astTestMust(t, err)
			f.pin(blocker, nil)
			main, eps, bound := "5", "0", "5"
			if test.sum {
				main, bound = "11", "11"
			}
			if test.strict {
				eps = "-1"
				if test.sum {
					eps = "-2"
				}
			}
			checkArithmeticObjective(t, value, main, eps)
			blockOp := smtArithOpGT
			if test.strict || test.integer {
				blockOp = smtArithOpGE
			}
			if test.integer {
				bound = "6"
			}
			checkArithmeticBlocker(t, f, blocker, obj, blockOp, bound)
			if shared {
				t.Fatal("unshared linear objective reported sharing")
			}
			read, err := f.a.ObjectiveValue(v)
			astTestMust(t, err)
			checkArithmeticObjective(t, read, main, eps)
			read.Finite.Main.SetInt64(-100)
			again, err := f.a.ObjectiveValue(v)
			astTestMust(t, err)
			checkArithmeticObjective(t, again, main, eps)
		})
	}
}

func TestSMTArithmeticObjectivePivotsDerivedBoundsAndPop(t *testing.T) {
	f := newArithmeticObjectiveFixture(t, false)
	x, y := f.variable("objective-pivot-x"), f.variable("objective-pivot-y")
	f.assert(f.app(smtArithOpGE, x, f.number("0")))
	f.assert(f.app(smtArithOpGE, y, f.number("0")))
	f.assert(f.app(smtArithOpLE, f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number("2"), x), y), f.number("10")))
	f.assert(f.app(smtArithOpLE, f.app(smtArithOpAdd, x, f.app(smtArithOpMul, f.number("2"), y)), f.number("10")))
	obj := f.app(smtArithOpAdd, x, y)
	v := f.objective(obj)
	for round := 0; round < 2; round++ {
		value, blocker, shared, err := f.a.MaximizeObjective(v)
		astTestMust(t, err)
		f.pin(blocker, nil)
		checkArithmeticObjective(t, value, "20/3", "0")
		if shared {
			t.Fatal("unshared coupled objective reports sharing")
		}
		if round == 0 {
			// A pivot round can return native BEST_EFFORT even at the optimum.
			// The next round certifies its row and publishes the derived bound.
			value, blocker, _, err = f.a.MaximizeObjective(v)
			astTestMust(t, err)
			f.pin(blocker, nil)
			checkArithmeticObjective(t, value, "20/3", "0")
			if len(f.a.pendingBounds) == 0 {
				t.Fatal("pivot optimum did not queue a derived upper bound")
			}
			bound := f.a.pendingBounds[len(f.a.pendingBounds)-1]
			if bound.lower || bound.column != f.a.simplexVariables[v] || bound.explanation == nil || len(bound.explanation.dependencies) == 0 {
				t.Fatal("missing optimum bound explanation", bound)
			}
			astTestMust(t, f.a.Propagate())
		}
		astTestMust(t, f.s.PushScope())
		f.assert(f.app(smtArithOpLE, x, f.number("1")))
		astTestMust(t, f.a.Propagate())
		value, blocker, _, err = f.a.MaximizeObjective(v)
		astTestMust(t, err)
		f.pin(blocker, nil)
		checkArithmeticObjective(t, value, "11/2", "0")
		astTestMust(t, f.a.Propagate())
		astTestMust(t, f.s.PopScopes(1))
	}
	checkNativeArithmeticObjective(t, "arithmetic_objective_coupled.smt2", `sat
(objectives ((+ x y) (/ 20.0 3.0)))
sat
(objectives ((+ x y) (/ 11.0 2.0)))
sat
(objectives ((+ x y) (/ 20.0 3.0)))`)
}

func TestSMTArithmeticObjectiveUnboundedAndAtBound(t *testing.T) {
	for _, bounded := range []bool{false, true} {
		t.Run(fmt.Sprint(bounded), func(t *testing.T) {
			f := newArithmeticObjectiveFixture(t, false)
			x := f.variable("objective-free")
			if bounded {
				f.assert(f.app(smtArithOpLE, x, f.number("0")))
			}
			v := f.objective(x)
			before, random := f.a.statistics.maxMin, f.s.random
			value, blocker, shared, err := f.a.MaximizeObjective(v)
			astTestMust(t, err)
			if bounded {
				checkArithmeticObjective(t, value, "0", "0")
				checkArithmeticBlocker(t, f, blocker, x, smtArithOpGT, "0")
				if f.a.statistics.maxMin != before || f.s.random.Next() != random.Next() {
					t.Fatal("AT_BOUND entered randomized optimizer")
				}
			} else if !value.Unbounded || !f.m.IsFalse(blocker) {
				t.Fatal("unbounded objective did not return infinity/false", value, blocker)
			}
			if shared {
				t.Fatal("unbounded or at-bound objective reports sharing")
			}
		})
	}
}

func TestSMTArithmeticObjectiveSharedAndNonlinearGuards(t *testing.T) {
	for _, nonlinear := range []bool{false, true} {
		t.Run(fmt.Sprint(nonlinear), func(t *testing.T) {
			f := newArithmeticObjectiveFixture(t, false)
			x := f.variable("objective-shared-x")
			f.assert(f.app(smtArithOpLE, x, f.number("5")))
			v := f.objective(x)
			if nonlinear {
				y := f.variable("objective-nonlinear-y")
				product := f.app(smtArithOpMul, x, y)
				bad, err := f.a.AddObjective(product)
				astTestMust(t, err)
				if bad != SMTNullTheoryVar || len(f.a.nonlinearColumns) == 0 {
					t.Fatal("nonlinear objective did not internalize then refuse")
				}
				astTestMust(t, f.a.Propagate())
			} else {
				decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "objective-shared-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				fx := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
				astTestMust(t, f.s.Internalize(fx, false))
			}
			before, err := f.a.ObjectiveValue(v)
			astTestMust(t, err)
			stats := f.a.statistics.maxMin
			value, _, shared, err := f.a.MaximizeObjective(v)
			astTestMust(t, err)
			if !shared {
				t.Fatal("missing combination revalidation request")
			}
			if nonlinear {
				if value.Finite.Cmp(&before.Finite) != 0 || f.a.statistics.maxMin != stats {
					t.Fatal("nonlinear guard optimized a local linear relaxation")
				}
			} else {
				checkArithmeticObjective(t, value, "5", "0")
			}
		})
	}
}

func TestSMTArithmeticObjectiveBlockerIntegerCeilAndRealCast(t *testing.T) {
	f := newArithmeticObjectiveFixture(t, true)
	x := f.variable("objective-round-x")
	v := f.objective(x)
	for _, test := range []struct{ rational, bound string }{{"3/2", "2"}, {"-3/2", "-1"}, {"-2", "-1"}} {
		r, _ := new(big.Rat).SetString(test.rational)
		h, err := f.a.objectiveBlocker(v, simplex.RationalInfinitesimal(r))
		astTestMust(t, err)
		checkArithmeticBlocker(t, f, h, x, smtArithOpGE, test.bound)
	}
	// An integral-valued Real expression still uses the real strict blocker.
	cast := f.app(smtArithOpToReal, x)
	astTestMust(t, f.s.Internalize(cast, false))
	node := f.s.context.enodes.get(cast)
	if node == nil {
		t.Fatal("missing cast enode")
	}
	h, err := f.a.objectiveBlocker(node.TheoryVar(f.a.FamilyID()), simplex.RationalInfinitesimal(big.NewRat(2, 1)))
	astTestMust(t, err)
	checkArithmeticBlocker(t, f, h, cast, smtArithOpGT, "2")
}

func TestSMTArithmeticObjectiveIntegerGainsAndCancellation(t *testing.T) {
	f := newArithmeticObjectiveFixture(t, true)
	x := f.variable("objective-integral-x")
	f.assert(f.app(smtArithOpLE, f.app(smtArithOpMul, f.number("2"), x), f.number("7")))
	v := f.objective(x)
	value, blocker, _, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	checkArithmeticObjective(t, value, "3", "0")
	checkArithmeticBlocker(t, f, blocker, x, smtArithOpGE, "4")

	g := newArithmeticObjectiveFixture(t, false)
	y := g.variable("objective-cancel-y")
	w := g.objective(y)
	before, err := g.a.ObjectiveValue(w)
	astTestMust(t, err)
	g.m.Limit.Cancel()
	value, blocker, _, err = g.a.MaximizeObjective(w)
	g.m.Limit.SetCancel(0)
	astTestMust(t, err)
	if value.Unbounded || value.Finite.Cmp(&before.Finite) != 0 || g.m.IsFalse(blocker) {
		t.Fatal("canceled optimization reported an unbounded result or changed the assignment")
	}
}

func TestSMTArithmeticObjectiveSolverBlockerAndScopeRecovery(t *testing.T) {
	for _, integer := range []bool{false, true} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			params, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
			key := "auto_config"
			astTestMust(t, cfg.Context.SetBoolName(&params, &key, false))
			key = "arith.solver"
			astTestMust(t, cfg.Context.SetUIntName(&params, &key, 2))
			logic := "QF_LRA"
			if integer {
				logic = "QF_LIA"
			}
			s, err := NewSMTSolver(m, &params, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			// Explicit legacy algorithm target; QF real automatic setup now uses LAR.
			if logic == "QF_LRA" || logic == "QF_UFLRA" {
				smtDisableAutoConfig(t, s)
			}
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			s.kernel.preprocess = false
			astTestMust(t, s.Push())
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			a, ok := s.kernel.internalizer.theories[family].(*SMTLinearArithmeticTheory)
			if !ok {
				t.Fatal("native arithmetic mode 2 did not select the linear theory")
			}
			kind := int32(0)
			if integer {
				kind = 1
			}
			sort, err := m.MkFamilySort(family, kind, nil)
			astTestMust(t, err)
			f := &arithmeticObjectiveFixture{t: t, m: m, s: s.kernel.internalizer, a: a, sort: sort, integer: integer}
			x := f.variable("objective-solver-x")
			obj := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number("2"), x), f.number("1"))
			bound := f.app(smtArithOpLT, x, f.number("5"))
			if integer {
				bound = f.app(smtArithOpLE, f.app(smtArithOpMul, f.number("2"), x), f.number("7"))
			}
			astTestMust(t, s.Assert(bound))
			v, err := a.AddObjective(obj)
			astTestMust(t, err)
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("objective solver initial check", status)
			}
			value, blocker, shared, err := a.MaximizeObjective(v)
			astTestMust(t, err)
			f.pin(blocker, nil)
			if integer {
				checkArithmeticObjective(t, value, "7", "0")
			} else {
				checkArithmeticObjective(t, value, "11", "-2")
			}
			if shared {
				t.Fatal("unshared objective requested combination revalidation")
			}
			astTestMust(t, s.Push())
			astTestMust(t, s.Assert(blocker))
			status, err = s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("objective blocker admitted a better value", status)
			}
			astTestMust(t, s.Pop(1))
			status, err = s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("objective blocker scope did not recover", status)
			}
			if integer {
				checkNativeArithmeticObjective(t, "arithmetic_objective_integer.smt2", `sat
(objectives ((+ (* 2 x) 1) 7))
unsat
sat`)
			} else {
				checkNativeArithmeticObjective(t, "arithmetic_objective_real.smt2", `sat
(objectives ((+ (* 2.0 x) 1.0) (+ 11.0 (* (- 2.0) epsilon))))
unsat
sat`)
			}
		})
	}
}

func TestSMTArithmeticObjectiveDomainsAndScopeLifetime(t *testing.T) {
	f := newArithmeticObjectiveFixture(t, false)
	x := f.variable("objective-refusal-x")
	f.assert(f.app(smtArithOpLE, x, f.number("5")))
	v := f.objective(x)
	if _, err := f.a.ObjectiveValue(SMTNullTheoryVar); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid objective accepted", err)
	}
	no := f.pin(f.m.MkFalse())
	if _, err := f.a.AddObjective(no); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("Boolean objective accepted", err)
	}
	value, blocker, _, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	checkArithmeticObjective(t, value, "5", "0")
	checkArithmeticBlocker(t, f, blocker, x, smtArithOpGT, "5")
	astTestMust(t, f.s.PushScope())
	y := f.variable("objective-child-y")
	w := f.objective(y)
	astTestMust(t, f.s.PopScopes(1))
	if _, err := f.a.ObjectiveValue(w); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("popped objective accepted", err)
	}
}
