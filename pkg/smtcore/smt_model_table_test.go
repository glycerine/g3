package smtcore

import (
	"math/rand"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type tableTest struct {
	t  testing.TB
	m  *ast.ASTManager
	md *protomodel.ProtoModel
}

func (f tableTest) term(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	astTestMust(f.t, err)
	astTestPin(f.t, f.m, h)
	return h
}

func (f tableTest) forall(body ast.ASTHandle, sorts ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	names := make([]symbol.Symbol, len(sorts))
	for i := range names {
		names[i] = symbol.Numerical(uint32(i))
	}
	return f.term(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body}))
}

func (f tableTest) check(q ast.ASTHandle, status lbool.LBool, want ...ast.ASTHandle) {
	f.t.Helper()
	body, err := f.m.GetQuantifierBody(q)
	astTestMust(f.t, err)
	n, err := f.m.GetNumDeclarations(q)
	astTestMust(f.t, err)
	var scratch tableModel
	got, witness, err := scratch.check(f.m, f.md, q, body, n)
	astTestMust(f.t, err)
	if got != status || !slices.Equal(witness, want) {
		f.t.Fatalf("got status %v witness %v, want %v %v", got, witness, status, want)
	}
	if got == lbool.True {
		inst, err := f.m.NewRef(ast.ASTHandle{})
		astTestMust(f.t, err)
		out, err := f.m.NewRef(ast.ASTHandle{})
		astTestMust(f.t, err)
		astTestMust(f.t, rewriter.InstantiateQuantifier(f.m, q, witness, inst))
		h, err := f.m.RefGet(inst)
		astTestMust(f.t, err)
		ok, err := f.md.Eval(h, out, true)
		astTestMust(f.t, err)
		v, err := f.m.RefGet(out)
		astTestMust(f.t, err)
		if !ok || !f.m.IsFalse(v) {
			f.t.Fatal("ordinary model evaluation did not confirm witness", v)
		}
		astTestMust(f.t, f.m.DelRef(&out))
		astTestMust(f.t, f.m.DelRef(&inst))
	}
}

func TestModelTablesUnseenEqualityAndFrozenUniverse(t *testing.T) {
	m, _, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := tableTest{t, m, md}
	s := f.term(m.MkUninterpretedSort(astTestSymbol(t, m, "table-elements"), nil))
	a := f.term(m.MkModelValue(0, s))
	astTestMust(t, md.RegisterValue(a))
	x, y := f.term(m.MkVar(1, s)), f.term(m.MkVar(0, s))
	eq := f.term(m.MkEq(x, y))
	q := f.forall(eq, s, s)
	// With just one registered element, a real two-element assignment can
	// still falsify x=y. An abstract witness must go to the ordinary solver.
	f.check(q, lbool.Undef)
	// This formula specifically needs two *different* unseen elements.
	xa, ya := f.term(m.MkEq(x, a)), f.term(m.MkEq(y, a))
	pair := f.forall(f.term(m.MkOrArgs([]ast.ASTHandle{xa, ya, eq})), s, s)
	f.check(pair, lbool.Undef)
	astTestMust(t, md.FreezeUniverse(s))
	f.check(q, lbool.False)
	f.check(pair, lbool.False)
}

func TestModelTablesFunctionRowsAndArgumentOrder(t *testing.T) {
	m, _, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := tableTest{t, m, md}
	s := f.term(m.MkUninterpretedSort(astTestSymbol(t, m, "table-values"), nil))
	a, b := f.term(m.MkModelValue(0, s)), f.term(m.MkModelValue(1, s))
	astTestMust(t, md.RegisterValue(a))
	astTestMust(t, md.RegisterValue(b))
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	fn := f.term(m.MkFuncDeclWithInfo(astTestSymbol(t, m, "choose"), []ast.ASTHandle{s, s}, s, info))
	astTestMust(t, m.DelFuncDeclInfo(&info))
	x, y := f.term(m.MkVar(1, s)), f.term(m.MkVar(0, s))
	app := f.term(m.MkApp(fn, []ast.ASTHandle{x, y}))
	eq := f.term(m.MkEq(app, x))
	q := f.forall(eq, s, s)
	f.check(q, lbool.Undef)
	tab, err := model.NewFunctionInterpretation(m, 2)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterFunction(fn, tab))
	f.check(q, lbool.Undef)
	// A function else uses #0 for argument 0. The outer quantifier uses
	// #1 for its first variable. On (a,b) the explicit result overrides x.
	astTestMust(t, tab.SetElse(y))
	f.check(q, lbool.False)
	astTestMust(t, tab.InsertNewEntry([]ast.ASTHandle{a, b}, b))
	f.check(q, lbool.True, a, b)
	isA, isB := f.term(m.MkEq(x, a)), f.term(m.MkEq(y, b))
	expected := f.term(m.MkIte(f.term(m.MkAndArgs([]ast.ASTHandle{isA, isB})), b, x))
	f.check(f.forall(f.term(m.MkEq(app, expected)), s, s), lbool.False)

	// Recursive defaults cannot be evaluated by a finite table walk.
	astTestMust(t, tab.SetElse(app))
	f.check(q, lbool.Undef)
}

func TestModelTablesRespectAuxiliaryPremisesAndLimits(t *testing.T) {
	m, c, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := tableTest{t, m, md}
	b := f.term(m.MkBoolSort())
	no := f.term(m.MkFalse())
	x := f.term(m.MkVar(0, b))
	q := f.forall(x, b)
	f.check(q, lbool.True, no)
	s := f.term(m.MkUninterpretedSort(astTestSymbol(t, m, "large-universe"), nil))
	vals := make([]ast.ASTHandle, 9)
	for i := range vals {
		vals[i] = f.term(m.MkModelValue(uint32(i), s))
		astTestMust(t, md.RegisterValue(vals[i]))
	}
	astTestMust(t, md.FreezeUniverse(s))
	id := f.term(m.MkVar(0, s))
	wide := f.forall(f.term(m.MkEq(id, id)), s, s, s)
	f.check(wide, lbool.False)
	m.ResourceLimit().IncCancel()
	f.check(q, lbool.Undef)
	f.check(wide, lbool.Undef)
	m.ResourceLimit().DecCancel()
	astTestMust(t, c.initAuxiliaryContext())
	a := c.auxiliaryContext
	a.config.CheckUninterpretedModelTables = true
	if !c.canCheckModelTables() {
		t.Fatal("empty auxiliary solver rejected")
	}
	a.kernel.smtState.MaxConflicts = 0
	if c.canCheckModelTables() {
		t.Fatal("ignored conflict limit")
	}
	a.kernel.smtState.MaxConflicts = ^uint32(0)
	m.ResourceLimit().Limit = 13
	if c.canCheckModelTables() {
		t.Fatal("ignored resource limit")
	}
	m.ResourceLimit().Limit = 0
	astTestMust(t, a.Assert(no))
	remaining := uint32(1)
	found, err := c.auxiliaryCounterexamples(md, q, &remaining)
	astTestMust(t, err)
	if found || remaining != 1 || c.HasNewInstances() {
		t.Fatal("ignored false auxiliary premise")
	}
}

func TestModelTablesRecoverOriginalBindingsFromPulledQuantifier(t *testing.T) {
	m, c, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := tableTest{t, m, md}
	b := f.term(m.MkBoolSort())
	no, yes := f.term(m.MkFalse()), f.term(m.MkTrue())
	x, y := f.term(m.MkVar(1, b)), f.term(m.MkVar(0, b))
	body := f.term(m.MkOrArgs([]ast.ASTHandle{x, f.term(m.MkNot(y))}))
	q := f.forall(f.forall(body, b), b)
	astTestMust(t, c.state.Internalize(q, true))
	flat := c.modelFinder.flatQuantifier(q)
	if flat == q {
		t.Fatal("nested universal did not flatten")
	}
	f.check(flat, lbool.True, no, yes)
	astTestMust(t, c.state.Internalize(no, false))
	_, err := c.modelFinder.InsertInstantiationTerm(q, 0, no, 13)
	astTestMust(t, err)
	set, err := c.modelFinder.instantiationSet(q, 0, false)
	astTestMust(t, err)
	astTestMust(t, set.MakeInverse(smtTestProtoEval(t, m, md)))
	astTestMust(t, c.initAuxiliaryContext())
	c.auxiliaryContext.config.CheckUninterpretedModelTables = true
	n := uint32(1)
	found, err := c.auxiliaryCounterexamples(md, q, &n)
	astTestMust(t, err)
	if !found || n != 0 || len(c.newInstances) != 1 || !slices.Equal(c.newInstances[0].bindings, []ast.ASTHandle{no}) ||
		c.newInstances[0].generation != 13 || c.auxiliaryContext.kernel.Statistics().NumChecks != 0 {
		t.Fatal("failed to recover the original binding from a flat witness", c.newInstances)
	}
}

func TestModelTablesAgreeWithGroundModelEvaluation(t *testing.T) {
	m, _, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := tableTest{t, m, md}
	sort := f.term(m.MkUninterpretedSort(astTestSymbol(t, m, "table-domain"), nil))
	var values []ast.ASTHandle
	for i := 0; i < 3; i++ {
		v := f.term(m.MkModelValue(uint32(i), sort))
		astTestMust(t, md.RegisterValue(v))
		values = append(values, v)
	}
	astTestMust(t, md.FreezeUniverse(sort))
	universe, err := md.KnownUniverse(sort)
	astTestMust(t, err)
	x, y := f.term(m.MkVar(1, sort)), f.term(m.MkVar(0, sort))
	function := smtQuickPredicate(t, m, "next-element", sort, sort)
	tab, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, tab.SetElse(y))
	astTestMust(t, tab.InsertNewEntry(values[:1], values[1]))
	astTestMust(t, md.Core().RegisterFunction(function, tab))
	fx := f.term(m.MkApp(function, []ast.ASTHandle{x}))
	fy := f.term(m.MkApp(function, []ast.ASTHandle{y}))
	pool := []ast.ASTHandle{f.term(m.MkEq(x, y)), f.term(m.MkEq(fx, y)), f.term(m.MkEq(x, fy)),
		f.term(m.MkEq(fx, fy)), f.term(m.MkEq(y, values[0])), f.term(m.MkEq(x, values[2]))}
	rng := rand.New(rand.NewSource(2573))
	for i := 0; i < 90; i++ {
		a, b := pool[rng.Intn(len(pool))], pool[rng.Intn(len(pool))]
		var body ast.ASTHandle
		switch i % 6 {
		case 0:
			body = f.term(m.MkIff(a, b))
		case 1:
			body = f.term(m.MkNot(a))
		case 2:
			body = f.term(m.MkIte(pool[rng.Intn(len(pool))], a, b))
		case 3:
			body = f.term(m.MkImplies(a, b))
		case 4:
			body = f.term(m.MkAndArgs([]ast.ASTHandle{a, b}))
		case 5:
			body = f.term(m.MkXor(a, b))
		}
		q := f.forall(body, sort, sort)
		var want []ast.ASTHandle
		for _, first := range universe {
			for _, second := range universe {
				binding := []ast.ASTHandle{first, second}
				instance, err := m.NewRef(ast.ASTHandle{})
				astTestMust(t, err)
				out, err := m.NewRef(ast.ASTHandle{})
				astTestMust(t, err)
				astTestMust(t, rewriter.InstantiateQuantifier(m, q, binding, instance))
				h, err := m.RefGet(instance)
				astTestMust(t, err)
				ok, err := md.Eval(h, out, true)
				astTestMust(t, err)
				v, err := m.RefGet(out)
				astTestMust(t, err)
				if !ok || !m.IsTrue(v) && !m.IsFalse(v) {
					t.Fatalf("unresolved ground model evaluation for trial %d", i)
				}
				if want == nil && m.IsFalse(v) {
					want = binding
				}
				astTestMust(t, m.DelRef(&out))
				astTestMust(t, m.DelRef(&instance))
			}
		}
		if want == nil {
			f.check(q, lbool.False)
		} else {
			f.check(q, lbool.True, want...)
		}
		// Four unused outer declarations lift the same body from 9 tuples
		// to 729. The diagram must produce the same first witness as the
		// independent ground-model evaluation above, including those binders.
		wide := f.forall(body, sort, sort, sort, sort, sort, sort)
		if want == nil {
			f.check(wide, lbool.False)
		} else {
			f.check(wide, lbool.True, universe[0], universe[0], universe[0], universe[0], want[0], want[1])
		}
		pool = append(pool, body)
	}
}
