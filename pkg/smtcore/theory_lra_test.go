package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lp"
)

type lraFrontendFixture struct {
	t    *testing.T
	m    *ast.ASTManager
	s    *SMTInternalizerState
	a    *SMTLRATheory
	sort ast.ASTHandle
}

func newLRAFrontendFixture(t *testing.T, reflect bool) *lraFrontendFixture {
	t.Helper()
	return newLRAFrontendFixtureWithPropagation(t, reflect, 2, ^uint32(0))
}

func newLRAFrontendFixtureWithPropagation(t *testing.T, reflect bool, mode, threshold uint32) *lraFrontendFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a, err := NewSMTLRATheory(family)
	astTestMust(t, err)
	config := a.config
	config.Reflect = reflect
	config.BoundProp = mode
	config.PropagationThreshold = threshold
	astTestMust(t, a.ConfigureArithmetic(config))
	astTestMust(t, s.RegisterTheory(a))
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	return &lraFrontendFixture{t, m, s, a, sort}
}
func (f *lraFrontendFixture) pin(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	astTestMust(f.t, err)
	r, err := f.m.NewRef(h)
	astTestMust(f.t, err)
	f.t.Cleanup(func() { astTestMust(f.t, f.m.DelRef(&r)) })
	return h
}
func (f *lraFrontendFixture) variable(name string) ast.ASTHandle {
	f.t.Helper()
	return f.pin(f.m.MkConst(astTestSymbol(f.t, f.m, name), f.sort))
}
func (f *lraFrontendFixture) number(n int64) ast.ASTHandle {
	f.t.Helper()
	return f.pin(f.m.ArithmeticNumeralRat(big.NewRat(n, 1), false))
}
func (f *lraFrontendFixture) app(op int32, args ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	return f.pin(f.m.MkFamilyApp(f.a.FamilyID(), op, nil, args))
}
func (f *lraFrontendFixture) assert(h ast.ASTHandle, truth bool) {
	f.t.Helper()
	astTestMust(f.t, f.s.Internalize(h, false))
	if !truth {
		h = f.pin(f.m.MkNot(h))
	}
	astTestMust(f.t, f.s.Assert(h, ast.ASTHandle{}, 0))
}
func (f *lraFrontendFixture) propagate(want bool) {
	f.t.Helper()
	consistent, err := f.s.PropagateCore()
	astTestMust(f.t, err)
	if consistent != want {
		f.t.Fatal("LRA propagation", consistent, "want", want)
	}
}

func TestSMTLRAFrontendWorkStackCompactionAndUnitVariable(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-stack-x"), f.variable("lra-stack-y")
	astTestMust(t, f.s.Internalize(x, false))
	if f.a.IsAttachedToVariable(f.s.context.enodes.get(x)) {
		t.Fatal("source LRA has no sort-constraint attachment")
	}
	product := f.app(smtArithOpMul, f.number(2), x)
	negative := f.app(smtArithOpUMinus, y)
	term := f.app(smtArithOpAdd, product, negative, f.number(7), x)
	astTestMust(t, f.s.Internalize(term, false))
	if f.a.NumVariables() != 3 {
		t.Fatal("leaf/term variable count", f.a.NumVariables())
	}
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	yv := f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
	v := f.s.context.enodes.get(term).TheoryVar(f.a.FamilyID())
	if xv != 0 || yv != 1 || v != 2 {
		t.Fatal("native swap-last visitation order", xv, yv, v)
	}
	index, ok := f.a.numeric.index(v)
	if !ok {
		t.Fatal("missing actual LAR term")
	}
	lpterm := f.a.numeric.solver.Terms[index-lp.LARTermStartIndex]
	for column, coefficient := range lpterm.Coefficients().All() {
		value, err := f.a.numeric.rational(coefficient)
		astTestMust(t, err)
		want := int64(3)
		if column == 1 {
			want = -1
		}
		if value.Cmp(big.NewRat(want, 1)) != 0 {
			t.Fatal(column, value, want)
		}
	}
	constant, err := f.a.numeric.rational(&lpterm.Value)
	astTestMust(t, err)
	if constant.Cmp(big.NewRat(7, 1)) != 0 || len(f.a.numeric.solver.Terms) != 1 {
		t.Fatal("compacted constant/term", constant)
	}
	if f.s.context.enodes.get(term).CongruenceEnabled() || f.s.context.enodes.get(product).CongruenceEnabled() || f.s.context.enodes.get(term).NumArgs() != 4 {
		t.Fatal("native arithmetic enode policy")
	}
	unit := f.app(smtArithOpMul, f.number(1), x)
	got, err := f.a.internalizeDef(unit)
	astTestMust(t, err)
	if got != xv || len(f.a.numeric.solver.Terms) != 1 {
		t.Fatal("unit form created a slack", got)
	}
}

func TestSMTLRAFrontendBooleanBoundsConflictAndScopeReplay(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-scope-x")
	f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
	// Push before propagation: root assignments are reached inside the child
	// and must be replayed after the numerical scope is removed.
	astTestMust(t, f.s.PushScope())
	// Distinct axes force a numerical contradiction; same-axis bound clauses
	// could otherwise conflict before the root assignment reaches LAR.
	y := f.variable("lra-scope-y")
	f.assert(f.app(smtArithOpGE, y, f.number(0)), true)
	child := f.app(smtArithOpLE, f.app(smtArithOpAdd, x, y), f.number(-1))
	f.assert(child, true)
	f.propagate(false)
	astTestMust(t, f.s.PopScopes(1))
	f.propagate(true)
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	index, ok := f.a.numeric.index(xv)
	if !ok || len(f.a.numeric.solver.Constraints) != 1 {
		t.Fatal("parent bound was not replayed", ok, len(f.a.numeric.solver.Constraints))
	}
	if f.a.numeric.solver.ColumnsToExternal[index] != uint32(xv) {
		t.Fatal("replayed variable mapping")
	}
	astTestMust(t, f.s.PushScope())
	opaque := f.app(smtArithOpMul, x, x)
	astTestMust(t, f.s.Internalize(opaque, false))
	if f.a.notHandled != opaque {
		t.Fatal("nonlinear source refusal")
	}
	astTestMust(t, f.s.PopScopes(1))
	if !f.a.notHandled.IsNull() {
		t.Fatal("child refusal survived pop")
	}
	status, err := f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("feasible parent retained a popped unsupported term", status)
	}
}

func TestSMTLRAFrontendStrictBoxThroughActualLAR(t *testing.T) {
	f := newLRAFrontendFixture(t, false)
	x, y := f.variable("lra-ast-x"), f.variable("lra-ast-y")
	f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
	f.assert(f.app(smtArithOpGE, x, f.number(3)), false)
	f.assert(f.app(smtArithOpGE, y, f.number(0)), true)
	f.assert(f.app(smtArithOpLE, y, f.number(2)), true)
	objective := f.app(smtArithOpAdd, f.number(3), f.app(smtArithOpMul, f.number(2), x), f.app(smtArithOpUMinus, y))
	v, err := f.a.internalizeDef(objective)
	astTestMust(t, err)
	f.propagate(true)
	value, err := f.a.numeric.maximize(v)
	astTestMust(t, err)
	if value.Unbounded {
		t.Fatal("bounded source objective")
	}
	checkInfiniteArithmetic(t, value.Finite, 0, 9, -2)
	astTestMust(t, f.s.PushScope())
	f.assert(f.app(smtArithOpLE, x, f.number(2)), true)
	f.propagate(true)
	value, err = f.a.numeric.maximize(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, value.Finite, 0, 7, 0)
	astTestMust(t, f.s.PopScopes(1))
	f.propagate(true)
	value, err = f.a.numeric.maximize(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, value.Finite, 0, 9, -2)
	if !f.a.notHandled.IsNull() {
		t.Fatal("linear source input refused")
	}
	if f.s.context.enodes.get(objective).NumArgs() != 0 || f.s.context.enodes.get(objective).CongruenceEnabled() {
		t.Fatal("reflection-off arithmetic node policy")
	}
	checkNativeLRANumeric(t)
}

func TestSMTLRAFrontendNativeNormalizationBoundaryAndNumeralWrapper(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-normal-x"), f.variable("lra-normal-y")
	raw := f.app(smtArithOpLE, x, y)
	astTestMust(t, f.s.Internalize(raw, false))
	if f.a.notHandled != raw || len(f.a.boundByBoolean) != 0 {
		t.Fatal("unnormalized comparison accepted")
	}
	astTestMust(t, f.s.PushScope())
	wrapped := f.app(smtArithOpMul, f.app(smtArithOpUMinus, f.number(2)), x)
	astTestMust(t, f.s.Internalize(wrapped, false))
	if f.a.notHandled != wrapped {
		t.Fatal("native one-pass numeral recognition changed")
	}
	astTestMust(t, f.s.PopScopes(1))
	if f.a.notHandled != raw {
		t.Fatal("parent refusal not restored")
	}
	astTestMust(t, f.a.Close())
	astTestMust(t, f.a.Close())
	if _, err := f.a.InternalizeTerm(x); err == nil {
		t.Fatal("closed frontend reused")
	}
}

func TestSMTLRAFrontendSubtractionAndRightNumeralMultiplication(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y, z := f.variable("lra-sub-x"), f.variable("lra-sub-y"), f.variable("lra-sub-z")
	term := f.app(smtArithOpSub, f.app(smtArithOpMul, x, f.number(3)), f.app(smtArithOpAdd, y, f.number(2)), z)
	astTestMust(t, f.s.Internalize(term, false))
	vars := []ast.ASTHandle{x, z, y}
	for i, h := range vars {
		if f.s.context.enodes.get(h).TheoryVar(f.a.FamilyID()) != SMTTheoryVar(i) {
			t.Fatal("swap-last subtraction order", i)
		}
	}
	index, ok := f.a.numeric.index(f.s.context.enodes.get(term).TheoryVar(f.a.FamilyID()))
	if !ok {
		t.Fatal("missing subtraction term")
	}
	tuple := f.a.numeric.solver.Terms[index-lp.LARTermStartIndex]
	constant, err := f.a.numeric.rational(&tuple.Value)
	astTestMust(t, err)
	if constant.Cmp(big.NewRat(-2, 1)) != 0 {
		t.Fatal("subtracted constant", constant)
	}
	for j, c := range tuple.Coefficients().All() {
		value, err := f.a.numeric.rational(c)
		astTestMust(t, err)
		want := int64(-1)
		if j == 0 {
			want = 3
		}
		if value.Cmp(big.NewRat(want, 1)) != 0 {
			t.Fatal("signed coefficient", j, value, want)
		}
	}
}

func TestSMTLRAFrontendUnderspecifiedReflectionAndIterativeUnitTerms(t *testing.T) {
	f := newLRAFrontendFixture(t, false)
	x, y := f.variable("lra-reflect-x"), f.variable("lra-reflect-y")
	zeroDiv := f.app(smtArithOpDiv, x, f.number(0))
	astTestMust(t, f.s.Internalize(zeroDiv, false))
	if node := f.s.context.enodes.get(zeroDiv); node.NumArgs() != 2 || !node.CongruenceEnabled() {
		t.Fatal("underspecified operator must reflect despite reflect=false")
	}
	if !f.a.notHandled.IsNull() || len(f.a.underspecified) != 0 {
		t.Fatal("native constant-denominator recognition changed")
	}
	variableDiv := f.app(smtArithOpDiv, x, y)
	astTestMust(t, f.s.Internalize(variableDiv, false))
	if f.a.notHandled != variableDiv || len(f.a.underspecified) != 1 || f.a.underspecified[0] != variableDiv {
		t.Fatal("variable-denominator refusal tracking")
	}
	term := x
	for i := 0; i < 600; i++ {
		term = f.app(smtArithOpUMinus, term)
	}
	v, err := f.a.internalizeDef(term)
	astTestMust(t, err)
	if v != f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID()) {
		t.Fatal("iterative unit chain lost the leaf variable")
	}
	if f.a.IsAttachedToVariable(f.s.context.enodes.get(term)) {
		t.Fatal("source unit-form optimization must not add a slack")
	}
}
