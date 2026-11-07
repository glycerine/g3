package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func infiniteArithmeticFixture(t *testing.T) *arithmeticObjectiveFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a, err := NewSMTInfiniteArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(a))
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	return &arithmeticObjectiveFixture{t: t, m: m, s: s, a: a, sort: sort}
}

func TestSMTInfiniteArithmeticConfiguredSolverModelAndPop(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	p, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
	key := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&p, &key, false))
	key = "arith.solver"
	astTestMust(t, cfg.Context.SetUIntName(&p, &key, 5))
	s, err := NewSMTSolver(m, &p, "", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	s.kernel.preprocess = false
	astTestMust(t, s.SetProduceModels(true))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	f := &arithmeticObjectiveFixture{t: t, m: m, s: s.kernel.internalizer, sort: sort}
	astTestMust(t, s.Push())
	f.a, _ = f.s.theories[family].(*SMTLinearArithmeticTheory)
	if f.a == nil || !f.a.infinite {
		t.Fatal("first push did not select inf_ext")
	}
	astTestMust(t, s.Pop(1))
	x := f.variable("configured-infinite-x")
	astTestMust(t, s.Assert(f.app(smtArithOpGE, x, f.number("0"))))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	f.a, _ = f.s.theories[family].(*SMTLinearArithmeticTheory)
	if f.a == nil || !f.a.infinite {
		t.Fatal("mode 5 did not select inf_ext")
	}
	v, err := f.a.AddObjective(x)
	astTestMust(t, err)
	old := acquireUpdateModel(t, s)
	oldValue := modelUpdateNumber(t, m, old, x)
	astTestMust(t, s.Push())
	h := f.pin(f.a.MakeObjectiveLowerBound(objectiveBoundFilter(t, m), v, infiniteArithmeticValue(1, 7, -2)))
	astTestMust(t, s.Assert(h))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("nonstandard child", status)
	}
	value, err := f.a.ObjectiveValue(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, value.Finite, 1, 7, -2)
	// Native mk_value projects away infinity and substitutes delta=1 for
	// epsilon here. This checks its publication behavior, not a finite witness
	// for the opaque nonstandard bound.
	fresh := acquireUpdateModel(t, s)
	if modelUpdateNumber(t, m, fresh, x) != "5" || modelUpdateNumber(t, m, old, x) != oldValue {
		t.Fatal("published projection or retained model changed")
	}
	astTestMust(t, s.Pop(1))
	if bound, ok := f.a.solver.Lower(f.a.simplexVariables[v]); ok && bound.Infinity.Sign() != 0 {
		t.Fatal("nonstandard bound survived pop")
	}
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("parent recovery", status)
	}
}
func infiniteArithmeticValue(i, r, e int64) simplex.Infinitesimal {
	return simplex.NewInfiniteInfinitesimal(big.NewRat(i, 1), big.NewRat(r, 1), big.NewRat(e, 1))
}
func checkInfiniteArithmetic(t *testing.T, got simplex.Infinitesimal, i, r, e int64) {
	t.Helper()
	want := infiniteArithmeticValue(i, r, e)
	if got.Cmp(&want) != 0 {
		t.Fatal(got.String(), "want", want.String())
	}
}
func assignInfiniteBound(t *testing.T, f *arithmeticObjectiveFixture, v SMTTheoryVar, value simplex.Infinitesimal, truth bool) ast.ASTHandle {
	t.Helper()
	filter := objectiveBoundFilter(t, f.m)
	h := f.pin(f.a.MakeObjectiveLowerBound(filter, v, value))
	bv, ok := f.s.BoolVar(h)
	if !ok {
		t.Fatal("unregistered infinite atom")
	}
	astTestMust(t, f.s.AssignLiteral(NewSMTLiteral(bv, !truth), NewSMTBooleanAxiom()))
	consistent, err := f.s.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected infinite-bound conflict")
	}
	return h
}
func TestSMTInfiniteArithmeticThresholdNameComplementAndCopies(t *testing.T) {
	for _, truth := range []bool{true, false} {
		t.Run(fmt.Sprint(truth), func(t *testing.T) {
			f := infiniteArithmeticFixture(t)
			x := f.variable("infinite-bound-x")
			v := f.objective(x)
			value := simplex.NewInfiniteInfinitesimal(big.NewRat(2, 3), big.NewRat(3, 1), big.NewRat(-2, 1))
			h := assignInfiniteBound(t, f, v, value, truth)
			decl, err := f.m.GetDecl(h)
			astTestMust(t, err)
			name, err := f.m.GetASTName(decl)
			astTestMust(t, err)
			text, err := f.m.Symbols.Display(name)
			astTestMust(t, err)
			if text != "(2/3*oo + (3 -e*2)) <= infinite-bound-x" {
				t.Fatal("source infinite name", text)
			}
			value.Infinity.SetInt64(99)
			value.Main.SetInt64(99)
			value.Epsilon.SetInt64(99)
			column := f.a.simplexVariables[v]
			bound, ok := f.a.solver.Lower(column)
			epsilon := int64(-2)
			if !truth {
				bound, ok = f.a.solver.Upper(column)
				epsilon = -3
			}
			if !ok || bound.Infinity.Cmp(big.NewRat(2, 3)) != 0 || bound.Main.Cmp(big.NewRat(3, 1)) != 0 || bound.Epsilon.Cmp(big.NewRat(epsilon, 1)) != 0 {
				t.Fatal("infinite threshold or complement lost a coordinate", ok, bound.String())
			}
		})
	}
	for _, test := range []struct {
		value simplex.Infinitesimal
		name  string
	}{{infiniteArithmeticValue(1, 0, 0), "oo"}, {infiniteArithmeticValue(-1, 0, 0), "-oo"}, {infiniteArithmeticValue(-2, -3, 2), "(-2*oo + (-3 +e*2))"}} {
		if got := smtArithmeticBoundName(test.value); got != test.name {
			t.Fatal(got, test.name)
		}
	}
}
func TestSMTInfiniteArithmeticMaximizeKeepsAllCoordinatesAndPop(t *testing.T) {
	f := infiniteArithmeticFixture(t)
	x := f.variable("infinite-max-x")
	v := f.objective(x)
	assignInfiniteBound(t, f, v, infiniteArithmeticValue(1, 3, 0), true)
	astTestMust(t, f.s.PushScope())
	assignInfiniteBound(t, f, v, infiniteArithmeticValue(2, 3, -1), false) // x <=2*oo+3-2*epsilon
	value, blocker, shared, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	f.pin(blocker, nil)
	if value.Unbounded || shared {
		t.Fatal("exact infinite endpoint collapsed to unbounded/shared")
	}
	checkInfiniteArithmetic(t, value.Finite, 2, 3, -2)
	current, err := f.a.ObjectiveValue(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, current.Finite, 2, 3, -2)
	// Source mk_gt extracts only the rational/epsilon parts even for inf_ext.
	op, own, err := f.a.operation(blocker)
	astTestMust(t, err)
	if !own || op != smtArithOpGE {
		t.Fatal("source infinite blocker shape", op)
	}
	astTestMust(t, f.s.PopScopes(1))
	current, err = f.a.ObjectiveValue(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, current.Finite, 2, 3, -2)
	if _, ok := f.a.solver.Upper(f.a.simplexVariables[v]); ok {
		t.Fatal("removed infinite upper endpoint remained in the parent")
	}
	value, blocker, shared, err = f.a.MaximizeObjective(v)
	astTestMust(t, err)
	f.pin(blocker, nil)
	if !value.Unbounded || !f.m.IsFalse(blocker) || shared {
		t.Fatal("removed infinite upper endpoint remained active")
	}
}
func TestSMTInfiniteArithmeticEqualityDoesNotProjectInfinity(t *testing.T) {
	f := infiniteArithmeticFixture(t)
	x := f.variable("infinite-equality-x")
	y := f.variable("infinite-equality-y")
	v := f.objective(x)
	assignInfiniteBound(t, f, v, infiniteArithmeticValue(1, 3, 0), true)
	assignInfiniteBound(t, f, v, infiniteArithmeticValue(2, 3, 1), false) // x <=2*oo+3
	for _, op := range []int32{smtArithOpLE, smtArithOpGE} {
		h := f.app(op, y, f.number("3"))
		astTestMust(t, f.s.Assert(h, ast.ASTHandle{}, 0))
	}
	consistent, err := f.s.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("infinite and rational points incorrectly conflicted")
	}
	_, err = f.a.propagateImpliedEqualities()
	astTestMust(t, err)
	if f.s.context.enodes.get(x).Root() == f.s.context.enodes.get(y).Root() {
		t.Fatal("rational projection invented x=y")
	}
	xv := f.a.simplexVariables[v]
	if _, fixed := f.a.fixedLinearAssignments()[xv]; fixed {
		t.Fatal("rational polynomial shortcut fixed an infinite range")
	}
	// Both variables are shared through an uninterpreted function. Their
	// rational/epsilon projections match, but full arithmetic values differ.
	decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "infinite-shared-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{x, y} {
		app := f.pin(f.m.MkApp(decl, []ast.ASTHandle{h}))
		astTestMust(t, f.s.Internalize(app, false))
	}
	astTestMust(t, f.a.Propagate())
	f.a.liberalFinalCheck = false
	_, err = f.a.assumeSharedEqualities()
	astTestMust(t, err)
	for _, pair := range f.a.assumeEqualityCandidates {
		if pair.left == v || pair.right == v {
			t.Fatal("shared-value hash discarded infinity")
		}
	}
}

func TestSMTInfiniteArithmeticCompoundBoundAndRationalRounding(t *testing.T) {
	f := infiniteArithmeticFixture(t)
	x := f.variable("infinite-compound-x")
	term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number("2"), x), f.number("1"))
	v := f.objective(term)
	assignInfiniteBound(t, f, v, infiniteArithmeticValue(2, 7, -2), true)
	xcol, err := f.a.ensureVariable(x)
	astTestMust(t, err)
	bound, ok := f.a.solver.Lower(xcol)
	if !ok {
		t.Fatal("missing scaled compound bound")
	}
	checkInfiniteArithmetic(t, bound, 1, 3, -1)
	f.a.integerColumns[xcol] = true
	rounded := f.a.normalizeOptimizedBound(xcol, true, bound)
	checkInfiniteArithmetic(t, rounded, 0, 3, 0)
}

func TestSMTInfiniteArithmeticSharedModelDeltaSkipsInfinity(t *testing.T) {
	f := infiniteArithmeticFixture(t)
	x, y := f.variable("infinite-delta-x"), f.variable("infinite-delta-y")
	decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "infinite-delta-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{x, y} {
		app := f.pin(f.m.MkApp(decl, []ast.ASTHandle{h}))
		astTestMust(t, f.s.Internalize(app, false))
	}
	for i, h := range []ast.ASTHandle{x, y} {
		column, err := f.a.ensureVariable(h)
		astTestMust(t, err)
		// Equal projections with different infinity coefficients cannot be
		// separated by halving epsilon. Native refine_epsilon skips them.
		f.a.solver.SetValue(column, infiniteArithmeticValue(int64(i+1), 3, 0))
	}
	f.a.delta.SetInt64(1)
	astTestMust(t, f.a.refineModelDelta())
	if f.a.delta.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("infinite projection changed model delta")
	}
}
