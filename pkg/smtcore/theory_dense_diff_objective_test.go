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

type denseObjectiveFixture struct {
	*diffObjectiveFixture
	d *SMTDenseDiffTheory
}

func newDenseObjectiveFixture(t *testing.T, integer bool) *denseObjectiveFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	d, err := NewSMTDenseDiffTheory(family)
	astTestMust(t, err)
	astTestMust(t, d.ConfigureArithmetic(SMTArithState{Reflect: true}))
	astTestMust(t, s.RegisterTheory(d))
	kind := int32(0)
	if integer {
		kind = 1
	}
	sort, err := m.MkFamilySort(family, kind, nil)
	astTestMust(t, err)
	return &denseObjectiveFixture{&diffObjectiveFixture{t: t, m: m, s: s, i: &d.syntax, family: family, sort: sort, integer: integer}, d}
}
func (f *denseObjectiveFixture) prepare() {
	f.t.Helper()
	status, err := f.d.FinalCheck()
	astTestMust(f.t, err)
	if status != SMTFinalCheckDone {
		f.t.Fatal("incomplete dense objective fixture", status)
	}
}
func (f *denseObjectiveFixture) maximize(v SMTTheoryVar) (SMTArithmeticObjectiveValue, ast.ASTHandle) {
	f.t.Helper()
	value, blocker, shared, err := f.d.MaximizeObjective(v)
	astTestMust(f.t, err)
	if shared {
		f.t.Fatal("dense source must report has_shared=false")
	}
	return value, f.pin(blocker, nil)
}
func TestSMTDenseObjectiveWeightedScopesAndNullCore(t *testing.T) {
	for _, integer := range []bool{true, false} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			f := newDenseObjectiveFixture(t, integer)
			x := f.variable("dense-weight-x")
			y := f.variable("dense-weight-y")
			f.assert(f.app(smtArithOpLE, x, f.number("2")))
			f.assert(f.app(smtArithOpLE, y, f.number("3")))
			v, err := f.d.AddObjective(f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number("2"), x), f.app(smtArithOpMul, y, f.number("3")), f.number("7")))
			astTestMust(t, err)
			f.prepare()
			value, blocker := f.maximize(v)
			checkArithmeticObjective(t, value, "20", "0")
			op, _, err := f.s.basicOperation(blocker)
			astTestMust(t, err)
			if op != ast.ASTOpNot {
				t.Fatal("weighted blocker did not use its core")
			}
			core := f.d.syntax.optimization.objectives[v].core
			if len(core) != 2 {
				t.Fatal("weighted core", len(core))
			}
			ge, err := f.d.MakeObjectiveLowerBound(nil, v, value.Finite)
			astTestMust(t, err)
			ge = f.pin(ge, nil)
			op, _, err = f.s.basicOperation(ge)
			astTestMust(t, err)
			if op != ast.ASTOpAnd {
				t.Fatal("weighted lower bound did not use its core")
			}
			for repeat := 0; repeat < 2; repeat++ {
				astTestMust(t, f.s.PushScope())
				f.assert(f.app(smtArithOpLE, x, f.number("1")))
				f.prepare()
				value, _ = f.maximize(v)
				checkArithmeticObjective(t, value, "18", "0")
				astTestMust(t, f.s.PopScopes(1))
				f.prepare()
				value, _ = f.maximize(v)
				checkArithmeticObjective(t, value, "20", "0")
			}
			// Dense definition edges have null justifications and must not enter the
			// saved literal core, unlike a Boolean bound edge.
			zero, err := f.d.term(f.number("0"))
			astTestMust(t, err)
			xv := f.s.context.enodes.get(x).TheoryVar(f.family)
			offset := newSMTInfRational(big.NewRat(-1, 1), nil)
			astTestMust(t, f.d.addEdge(int(zero), int(xv), &offset, SMTNullLiteral))
			f.prepare()
			value, _ = f.maximize(v)
			checkArithmeticObjective(t, value, "14", "0")
			for _, h := range f.d.syntax.optimization.objectives[v].core {
				if h.IsNull() {
					t.Fatal("null edge became a core expression")
				}
			}
		})
	}
}
func TestSMTDenseObjectiveStrictSourceRationalProjection(t *testing.T) {
	f := newDenseObjectiveFixture(t, false)
	x := f.variable("dense-strict-x")
	f.assert(f.pin(f.m.MkNot(f.app(smtArithOpGE, x, f.number("5")))))
	v, err := f.d.AddObjective(x)
	astTestMust(t, err)
	f.prepare()
	value, blocker := f.maximize(v)
	checkArithmeticObjective(t, value, "5", "-1")
	op, own, err := f.d.syntax.operation(blocker)
	astTestMust(t, err)
	if !own || op != smtArithOpGE {
		t.Fatal("strict dense supremum blocker", op)
	}
	xv := f.s.context.enodes.get(x).TheoryVar(f.family)
	if f.d.values[xv].rational.Cmp(big.NewRat(5, 1)) != 0 || f.d.values[xv].infinitesimal.Sign() != 0 {
		t.Fatal("source rational-only assignment projection changed")
	}
	ge, err := f.d.MakeObjectiveLowerBound(nil, v, value.Finite)
	astTestMust(t, err)
	ge = f.pin(ge, nil)
	if len(f.d.syntax.optimization.objectives[v].core) != 1 {
		t.Fatal("missing strict bound core")
	}
	op, _, err = f.s.basicOperation(ge)
	astTestMust(t, err)
	if op != ast.ASTOpAnd {
		t.Fatal("non-strict epsilon bound did not use the source core")
	}
}
func TestSMTDenseObjectiveUnboundedAndResourceStop(t *testing.T) {
	f := newDenseObjectiveFixture(t, false)
	x := f.variable("dense-unbounded-x")
	v, err := f.d.AddObjective(x)
	astTestMust(t, err)
	f.prepare()
	value, blocker := f.maximize(v)
	if !value.Unbounded || !f.m.IsFalse(blocker) {
		t.Fatal("unbounded dense objective")
	}
	f.assert(f.app(smtArithOpLE, x, f.number("7")))
	f.prepare()
	f.m.Limit.Cancel()
	value, blocker, shared, err := f.d.MaximizeObjective(v)
	f.m.Limit.ResetCancel()
	astTestMust(t, err)
	blocker = f.pin(blocker, nil)
	if !value.Unbounded || !f.m.IsFalse(blocker) || shared {
		t.Fatal("native dense resource stop convention changed")
	}
	// x was interned before zero, so source's objective-index anchor clamps x.
	value, _ = f.maximize(v)
	checkArithmeticObjective(t, value, "0", "0")
}
func TestSMTDenseObjectiveSourceIndexAndCompilerDomains(t *testing.T) {
	f := newDenseObjectiveFixture(t, false)
	x := f.variable("dense-source-index-x")
	f.assert(f.app(smtArithOpLE, x, f.number("7")))
	v, err := f.d.AddObjective(x)
	astTestMust(t, err)
	w, err := f.d.AddObjective(x)
	astTestMust(t, err)
	f.prepare()
	first, _ := f.maximize(v)
	checkArithmeticObjective(t, first, "7", "0")
	second, _ := f.maximize(w)
	checkArithmeticObjective(t, second, "0", "0") // Native anchor uses objective index1.
	f.d.values[0] = newSMTInfRational(big.NewRat(2, 1), big.NewRat(-3, 1))
	f.d.values[1] = newSMTInfRational(big.NewRat(11, 1), big.NewRat(4, 1))
	current, err := f.d.ObjectiveValue(v)
	astTestMust(t, err)
	checkArithmeticObjective(t, current, "2", "-3")
	current.Finite.Main.SetInt64(99)
	current, err = f.d.ObjectiveValue(v)
	astTestMust(t, err)
	checkArithmeticObjective(t, current, "2", "-3")
	for _, h := range []ast.ASTHandle{f.app(smtArithOpUMinus, x), f.app(smtArithOpMul, x, x)} {
		got, err := f.d.AddObjective(h)
		astTestMust(t, err)
		if got != SMTNullTheoryVar {
			t.Fatal("source compiler accepted an unsupported objective")
		}
	}
	if _, err := f.d.ObjectiveValue(SMTNullTheoryVar); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if _, _, _, err := f.d.MaximizeObjective(SMTNullTheoryVar); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if _, err := f.d.MakeObjectiveLowerBound(nil, SMTNullTheoryVar, simplex.Infinitesimal{}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
}
func TestSMTDenseObjectiveCoreOwnsScopedExpressions(t *testing.T) {
	f := newDenseObjectiveFixture(t, false)
	x := f.variable("dense-owned-core-x")
	f.assert(f.app(smtArithOpLE, x, f.number("9")))
	v, err := f.d.AddObjective(f.app(smtArithOpMul, f.number("2"), x))
	astTestMust(t, err)
	astTestMust(t, f.s.PushScope())
	bound, err := f.m.MkFamilyApp(f.family, smtArithOpLE, nil, []ast.ASTHandle{x, f.number("7")})
	astTestMust(t, err)
	f.assert(bound)
	f.prepare()
	_, blocker, _, err := f.d.MaximizeObjective(v)
	astTestMust(t, err)
	ref, err := f.m.NewRef(blocker)
	astTestMust(t, err)
	astTestMust(t, f.m.DelRef(&ref))
	astTestMust(t, f.s.PopScopes(1))
	refs, err := f.m.GetRefCount(bound)
	astTestMust(t, err)
	if refs != 1 {
		t.Fatal("saved core should be its scoped bound's only owner", refs)
	}
	astTestMust(t, f.d.Close())
	if _, err := f.m.GetKind(bound); err == nil {
		t.Fatal("last dense core owner did not release its bound")
	}
	astTestMust(t, f.d.Close())
}
func TestSMTDenseObjectiveConfiguredModelRefreshAndBlocker(t *testing.T) {
	for _, scenario := range [][2]bool{{true, false}, {false, false}, {false, true}} {
		integer, strict := scenario[0], scenario[1]
		t.Run(fmt.Sprint(integer, "/", strict), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			p, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
			key := "auto_config"
			astTestMust(t, cfg.Context.SetBoolName(&p, &key, false))
			key = "arith.solver"
			astTestMust(t, cfg.Context.SetUIntName(&p, &key, 3))
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
			d, ok := s.kernel.internalizer.theories[family].(*SMTDenseDiffTheory)
			if !ok {
				t.Fatalf("mode3 selected %T", s.kernel.internalizer.theories[family])
			}
			kind := int32(0)
			if integer {
				kind = 1
			}
			sort, err := m.MkFamilySort(family, kind, nil)
			astTestMust(t, err)
			f := &diffObjectiveFixture{t: t, m: m, s: s.kernel.internalizer, i: &d.syntax, family: family, sort: sort, integer: integer}
			x := f.variable("dense-native-x")
			bound := f.app(smtArithOpLE, x, f.number("7"))
			if strict {
				bound = f.pin(m.MkNot(f.app(smtArithOpGE, x, f.number("7"))))
			}
			astTestMust(t, s.Assert(bound))
			// optsmt::setup forces pending base assertions through an empty
			// push/pop before it registers the objective set.
			astTestMust(t, s.Push())
			astTestMust(t, s.Pop(1))
			v, err := d.AddObjective(x)
			astTestMust(t, err)
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			old := acquireUpdateModel(t, s)
			oldValue := modelUpdateNumber(t, m, old, x)
			value, blocker, shared, err := d.MaximizeObjective(v)
			astTestMust(t, err)
			blocker = f.pin(blocker, nil)
			epsilon := "0"
			if strict {
				epsilon = "-1"
			}
			checkArithmeticObjective(t, value, "7", epsilon)
			if shared {
				t.Fatal("dense source reported shared")
			}
			updated, err := s.UpdateModel(false)
			astTestMust(t, err)
			if !updated {
				t.Fatal("dense model refresh failed")
			}
			fresh := acquireUpdateModel(t, s)
			if strict {
				// The published rational-only point7 fails x<7. This matches
				// the explicit dense source TBD and the C++ strict vector;
				// it is a recorded source limitation, not a valid strict model.
				point, ok := new(big.Rat).SetString(modelUpdateNumber(t, m, fresh, x))
				if !ok || point.Cmp(big.NewRat(7, 1)) < 0 {
					t.Fatal("dense source rational projection changed")
				}
			}
			if fresh == old || modelUpdateNumber(t, m, fresh, x) != "7" || modelUpdateNumber(t, m, old, x) != oldValue {
				t.Fatal("dense assignment/model publication mismatch")
			}
			astTestMust(t, s.Push())
			astTestMust(t, s.Assert(blocker))
			status, err = s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("improvement past dense optimum must be UNSAT", status)
			}
			astTestMust(t, s.Pop(1))
			status, err = s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("dense scope did not recover SAT", status)
			}
		})
	}
}
