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

type diffObjectiveFixture struct {
	t       *testing.T
	m       *ast.ASTManager
	s       *SMTInternalizerState
	i       *SMTIntegerDiffTheory
	r       *SMTRealDiffTheory
	family  ast.FamilyID
	sort    ast.ASTHandle
	integer bool
}

func newDiffObjectiveFixture(t *testing.T, integer bool) *diffObjectiveFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	f := &diffObjectiveFixture{t: t, m: m, s: s, family: family, integer: integer}
	kind := int32(0)
	if integer {
		f.r, err = NewSMTMixedDiffTheory(family)
		kind = 1
	} else {
		f.r, err = NewSMTRealDiffTheory(family)
	}
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(f.r))
	f.i = f.r.SMTIntegerDiffTheory
	f.sort, err = m.MkFamilySort(family, kind, nil)
	astTestMust(t, err)
	return f
}
func (f *diffObjectiveFixture) pin(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	astTestMust(f.t, err)
	ref, err := f.m.NewRef(h)
	astTestMust(f.t, err)
	f.t.Cleanup(func() { astTestMust(f.t, f.m.DelRef(&ref)) })
	return h
}
func (f *diffObjectiveFixture) variable(name string) ast.ASTHandle {
	return f.pin(f.m.MkConst(astTestSymbol(f.t, f.m, name), f.sort))
}
func (f *diffObjectiveFixture) number(text string) ast.ASTHandle {
	if f.integer {
		return f.pin(f.m.ArithmeticIntegerNumeralText(text))
	}
	return f.pin(f.m.ArithmeticRealNumeralText(text))
}
func (f *diffObjectiveFixture) app(op int32, args ...ast.ASTHandle) ast.ASTHandle {
	return f.pin(f.m.MkFamilyApp(f.family, op, nil, args))
}
func (f *diffObjectiveFixture) assert(h ast.ASTHandle) {
	f.t.Helper()
	astTestMust(f.t, f.s.Assert(h, ast.ASTHandle{}, 0))
	consistent, err := f.s.PropagateCore()
	astTestMust(f.t, err)
	if !consistent {
		f.t.Fatal("unexpected inconsistent difference fixture")
	}
}
func (f *diffObjectiveFixture) maximize(v SMTTheoryVar) (SMTArithmeticObjectiveValue, ast.ASTHandle) {
	f.t.Helper()
	var result SMTArithmeticObjectiveValue
	var blocker ast.ASTHandle
	var shared bool
	var err error
	result, blocker, shared, err = f.r.MaximizeObjective(v)
	astTestMust(f.t, err)
	if shared {
		f.t.Fatal("difference optimizer reported shared")
	}
	return result, f.pin(blocker, nil)
}
func TestSMTDiffObjectiveBoundsBlockersAndRepeatedPop(t *testing.T) {
	for _, integer := range []bool{true, false} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			f := newDiffObjectiveFixture(t, integer)
			x := f.variable("diff-objective-x")
			y := f.variable("diff-objective-y")
			sum := f.app(smtArithOpAdd, x, f.app(smtArithOpMul, f.number("-1"), y))
			f.assert(f.app(smtArithOpLE, sum, f.number("5")))
			v, err := f.r.AddObjective(sum)
			astTestMust(t, err)
			if v != 0 {
				t.Fatal(v)
			}
			for repeat := 0; repeat < 2; repeat++ {
				astTestMust(t, f.s.PushScope())
				f.assert(f.app(smtArithOpLE, sum, f.number("3")))
				value, _ := f.maximize(v)
				checkArithmeticObjective(t, value, "3", "0")
				if len(f.i.optimization.objectives[v].core) == 0 {
					t.Fatal("missing objective row core")
				}
				astTestMust(t, f.s.PopScopes(1))
				if f.i.optimization.solver != nil || f.i.optimization.edges != 0 || len(f.i.optimization.rows) != 0 {
					t.Fatal("pop did not reset removed edge rows")
				}
				value, _ = f.maximize(v)
				checkArithmeticObjective(t, value, "5", "0")
			}
		})
	}
}
func TestSMTDiffObjectiveWeightedCoreAndConstants(t *testing.T) {
	for _, integer := range []bool{true, false} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			f := newDiffObjectiveFixture(t, integer)
			x := f.variable("diff-weight-x")
			y := f.variable("diff-weight-y")
			f.assert(f.app(smtArithOpLE, x, f.number("2")))
			f.assert(f.app(smtArithOpLE, y, f.number("3")))
			term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number("2"), x), f.app(smtArithOpMul, y, f.number("3")), f.number("7"))
			v, err := f.r.AddObjective(term)
			astTestMust(t, err)
			value, blocker := f.maximize(v)
			checkArithmeticObjective(t, value, "20", "0")
			op, _, err := f.s.basicOperation(blocker)
			astTestMust(t, err)
			if op != ast.ASTOpNot {
				t.Fatal("weighted objective did not use the native core blocker")
			}
			core := f.i.optimization.objectives[v].core
			if len(core) != 2 {
				t.Fatal("weighted core", len(core))
			}
			ge, err := f.r.MakeObjectiveLowerBound(nil, v, value.Finite)
			astTestMust(t, err)
			ge = f.pin(ge, nil)
			op, _, err = f.s.basicOperation(ge)
			astTestMust(t, err)
			if op != ast.ASTOpAnd {
				t.Fatal("weighted lower bound did not preserve the core")
			}
			// value() uses the objective index as graph node in this source revision.
			var current SMTArithmeticObjectiveValue
			current, err = f.r.ObjectiveValue(v)
			astTestMust(t, err)
			checkArithmeticObjective(t, current, "7", "0")
		})
	}
}
func TestSMTDiffObjectiveStrictRealAndUnbounded(t *testing.T) {
	f := newDiffObjectiveFixture(t, false)
	x := f.variable("diff-strict-x")
	y := f.variable("diff-unbounded-y")
	f.assert(f.pin(f.m.MkNot(f.app(smtArithOpGE, x, f.number("5")))))
	v, err := f.r.AddObjective(x)
	astTestMust(t, err)
	w, err := f.r.AddObjective(y)
	astTestMust(t, err)
	value, blocker := f.maximize(v)
	checkArithmeticObjective(t, value, "5", "-1")
	op, own, err := f.i.operation(blocker)
	astTestMust(t, err)
	if !own || op != smtArithOpGE {
		t.Fatal("strict supremum blocker", op)
	}
	ge, err := f.r.MakeObjectiveLowerBound(nil, v, value.Finite)
	astTestMust(t, err)
	ge = f.pin(ge, nil)
	if len(f.i.optimization.objectives[v].core) != 1 {
		t.Fatal("missing strict bound core")
	}
	assignment, err := f.r.realGraph.Assignment(int32(f.s.context.enodes.get(x).TheoryVar(f.family)))
	astTestMust(t, err)
	if assignment.rational.Cmp(big.NewRat(5, 1)) >= 0 || assignment.infinitesimal.Sign() != 0 {
		t.Fatal("optimized graph has no concrete strict witness", assignment)
	}
	value, blocker = f.maximize(w)
	if !value.Unbounded || !f.m.IsFalse(blocker) {
		t.Fatal("unbounded objective", value)
	}
}
func TestSMTDiffObjectiveCompilerSourceDomainAndValueCopies(t *testing.T) {
	f := newDiffObjectiveFixture(t, false)
	x := f.variable("diff-compiler-x")
	product := f.app(smtArithOpMul, f.number("2"), f.app(smtArithOpAdd, x, f.number("3")))
	v, err := f.r.AddObjective(product)
	astTestMust(t, err)
	o := &f.i.optimization.objectives[v]
	if len(o.terms) != 1 || o.terms[0].coefficient.Cmp(big.NewRat(2, 1)) != 0 || o.constant.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("source objective compilation changed")
	}
	// Subtraction/unary minus/casts pass is_linear but the native objective
	// compiler refuses arithmetic-family leaves beyond numeral/add/mul.
	for _, h := range []ast.ASTHandle{f.app(smtArithOpUMinus, x), f.app(smtArithOpSub, x, f.number("1")), f.app(smtArithOpMul, x, x)} {
		before := len(f.i.optimization.objectives)
		got, err := f.r.AddObjective(h)
		astTestMust(t, err)
		if got != SMTNullTheoryVar || len(f.i.optimization.objectives) != before {
			t.Fatal("rejected objective was published")
		}
	}
	value, err := f.r.ObjectiveValue(v)
	astTestMust(t, err)
	value.Finite.Main.SetInt64(99)
	again, err := f.r.ObjectiveValue(v)
	astTestMust(t, err)
	checkArithmeticObjective(t, again, "3", "0")
	if _, err := f.r.ObjectiveValue(SMTNullTheoryVar); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if _, _, _, err := f.r.MaximizeObjective(SMTNullTheoryVar); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if _, err := f.r.MakeObjectiveLowerBound(nil, SMTNullTheoryVar, simplex.Infinitesimal{}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
}

func TestSMTDiffObjectiveConfiguredSolverPublishesModelAndBlocks(t *testing.T) {
	for _, integer := range []bool{false, true} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			p, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
			key := "auto_config"
			astTestMust(t, cfg.Context.SetBoolName(&p, &key, false))
			key = "arith.solver"
			astTestMust(t, cfg.Context.SetUIntName(&p, &key, 1))
			logic := "QF_UFLRA"
			if integer {
				logic = "QF_LIA"
			}
			s, err := NewSMTSolver(m, &p, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			astTestMust(t, s.SetProduceModels(true))
			astTestMust(t, s.Push())
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			a, ok := s.kernel.internalizer.theories[family].(*SMTRealDiffTheory)
			if !ok {
				t.Fatalf("mode1 selected %T", s.kernel.internalizer.theories[family])
			}
			kind := int32(0)
			if integer {
				kind = 1
			}
			sort, err := m.MkFamilySort(family, kind, nil)
			astTestMust(t, err)
			f := &diffObjectiveFixture{t: t, m: m, s: s.kernel.internalizer, i: a.SMTIntegerDiffTheory, r: a, family: family, sort: sort, integer: integer}
			x := f.variable("diff-native-x")
			bound := f.app(smtArithOpLE, x, f.number("5"))
			if !integer {
				bound = f.pin(m.MkNot(f.app(smtArithOpGE, x, f.number("5"))))
			}
			astTestMust(t, s.Assert(bound))
			v, err := a.AddObjective(x)
			astTestMust(t, err)
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			old := acquireUpdateModel(t, s)
			oldValue := modelUpdateNumber(t, m, old, x)
			value, blocker, shared, err := a.MaximizeObjective(v)
			astTestMust(t, err)
			blocker = f.pin(blocker, nil)
			if shared {
				t.Fatal("unexpected shared difference objective")
			}
			epsilon := "-1"
			wantModel := "4"
			if integer {
				epsilon = "0"
				wantModel = "5"
			}
			checkArithmeticObjective(t, value, "5", epsilon)
			updated, err := s.UpdateModel(false)
			astTestMust(t, err)
			if !updated {
				t.Fatal("difference model refresh failed")
			}
			fresh := acquireUpdateModel(t, s)
			if fresh == old || modelUpdateNumber(t, m, fresh, x) != wantModel || modelUpdateNumber(t, m, old, x) != oldValue {
				t.Fatal("difference assignment/model ownership mismatch")
			}
			astTestMust(t, s.Push())
			astTestMust(t, s.Assert(blocker))
			status, err = s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("improvement past optimum must be UNSAT", status)
			}
			astTestMust(t, s.Pop(1))
			status, err = s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("pop failed to restore original difference problem", status)
			}
		})
	}
}
func TestSMTDiffObjectiveResourceStopPreservesNativeInfinityConvention(t *testing.T) {
	f := newDiffObjectiveFixture(t, false)
	x := f.variable("diff-stop-x")
	f.assert(f.app(smtArithOpLE, x, f.number("5")))
	v, err := f.r.AddObjective(x)
	astTestMust(t, err)
	f.m.Limit.Cancel()
	value, blocker, shared, err := f.r.MaximizeObjective(v)
	f.m.Limit.ResetCancel()
	astTestMust(t, err)
	blocker = f.pin(blocker, nil)
	if !value.Unbounded || !f.m.IsFalse(blocker) || shared {
		t.Fatal("native resource-stop convention changed")
	}
	value, _ = f.maximize(v)
	checkArithmeticObjective(t, value, "5", "0")
}

func TestSMTDiffObjectiveCoreOwnsScopedExpressions(t *testing.T) {
	for _, integer := range []bool{true, false} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			f := newDiffObjectiveFixture(t, integer)
			x := f.variable("diff-owned-core-x")
			v, err := f.r.AddObjective(f.app(smtArithOpMul, f.number("2"), x))
			astTestMust(t, err)
			astTestMust(t, f.s.PushScope())
			bound, err := f.m.MkFamilyApp(f.family, smtArithOpLE, nil, []ast.ASTHandle{x, f.number("7")})
			astTestMust(t, err)
			f.assert(bound)
			var blocker ast.ASTHandle
			_, blocker, _, err = f.r.MaximizeObjective(v)
			astTestMust(t, err)
			// Retire the borrowed blocker and its conjunction so they cannot keep the
			// original bound alive and conceal a missing core-vector reference.
			ref, err := f.m.NewRef(blocker)
			astTestMust(t, err)
			astTestMust(t, f.m.DelRef(&ref))
			if len(f.i.optimization.objectives[v].core) != 1 || f.i.optimization.objectives[v].core[0] != bound {
				t.Fatal("unexpected saved optimization core")
			}
			astTestMust(t, f.s.PopScopes(1))
			if _, found := f.s.BoolVar(bound); found {
				t.Fatal("scoped Boolean binding survived pop")
			}
			refs, err := f.m.GetRefCount(bound)
			astTestMust(t, err)
			if refs != 1 {
				t.Fatal("saved core should own exactly one remaining bound reference", refs)
			}
			astTestMust(t, f.r.Close())
			if f.m.IsLiveHandle(bound) {
				t.Fatal("saved core declaration survived its last owner")
			}
			astTestMust(t, f.r.Close())
		})
	}
}
func TestSMTDiffObjectiveNegativeAndReversedDifferenceBlockers(t *testing.T) {
	for _, integer := range []bool{true, false} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			f := newDiffObjectiveFixture(t, integer)
			x := f.variable("diff-negative-x")
			y := f.variable("diff-negative-y")
			minus := f.app(smtArithOpMul, f.number("-1"), x)
			reversed := f.app(smtArithOpAdd, minus, y)
			v, err := f.r.AddObjective(minus)
			astTestMust(t, err)
			w, err := f.r.AddObjective(reversed)
			astTestMust(t, err)
			f.assert(f.app(smtArithOpGE, x, f.number("-2")))
			f.assert(f.app(smtArithOpLE, y, f.number("3")))
			value, blocker := f.maximize(v)
			checkArithmeticObjective(t, value, "2", "0")
			left, err := f.m.GetArg(blocker, 0)
			astTestMust(t, err)
			op, own, err := f.i.operation(left)
			astTestMust(t, err)
			if !own || op != smtArithOpUMinus {
				t.Fatal("native negative objective blocker changed")
			}
			value, blocker = f.maximize(w)
			checkArithmeticObjective(t, value, "5", "0")
			left, err = f.m.GetArg(blocker, 0)
			astTestMust(t, err)
			op, own, err = f.i.operation(left)
			astTestMust(t, err)
			if !own || op != smtArithOpSub {
				t.Fatal("native reversed difference blocker changed")
			}
			first, err := f.m.GetArg(left, 0)
			astTestMust(t, err)
			second, err := f.m.GetArg(left, 1)
			astTestMust(t, err)
			if first != y || second != x {
				t.Fatal("difference blocker used the wrong orientation")
			}
		})
	}
}

func TestSMTDiffObjectiveIncrementalEdgesReuseCanonicalRows(t *testing.T) {
	for _, integer := range []bool{true, false} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			f := newDiffObjectiveFixture(t, integer)
			x := f.variable("diff-incremental-x")
			v, err := f.r.AddObjective(x)
			astTestMust(t, err)
			f.assert(f.app(smtArithOpLE, x, f.number("7")))
			value, _ := f.maximize(v)
			checkArithmeticObjective(t, value, "7", "0")
			solver := f.i.optimization.solver
			f.assert(f.app(smtArithOpLE, x, f.number("3")))
			value, _ = f.maximize(v)
			checkArithmeticObjective(t, value, "3", "0")
			if f.i.optimization.solver != solver {
				t.Fatal("incremental edges discarded the source simplex cache")
			}
			variable := f.s.context.enodes.get(x).TheoryVar(f.family)
			var point *big.Rat
			a, err := f.r.realGraph.Assignment(int32(variable))
			astTestMust(t, err)
			point = &a.rational
			if point.Cmp(big.NewRat(3, 1)) != 0 {
				t.Fatal("incremental optimized assignment does not match its value", point)
			}
		})
	}
}
