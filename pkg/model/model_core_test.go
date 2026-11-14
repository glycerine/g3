package model_test

// Source: src/model/model_core.cpp:21-66,85-109; src/model/model_core.h:37-87;
// src/model/model.cpp:44-50; src/sat/tactic/sat_tactic.cpp:89-112.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/sat"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"

	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func modelTestCore(t *testing.T, m *ast.ASTManager) *model.ModelCore {
	t.Helper()
	md, e := model.NewModelCore(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, md.Close()) })
	return md
}
func modelTestDecl(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	p := goalTestAtom(t, m, name)
	d, e := m.GetDecl(p)
	astTestMust(t, e)
	return d
}

func TestModelCoreConstantRegistrationReplacementAndReferences(t *testing.T) {
	m := astTestManager(t)
	p := modelTestDecl(t, m, "p")
	q := modelTestDecl(t, m, "q")
	md := modelTestCore(t, m)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	falsity, e := m.MkFalse()
	astTestMust(t, e)
	before, e := m.GetRefCount(p)
	astTestMust(t, e)
	trueBefore, e := m.GetRefCount(truth)
	astTestMust(t, e)
	astTestMust(t, md.RegisterConstant(p, truth))
	astTestMust(t, md.RegisterConstant(q, falsity))
	astTestMust(t, md.RegisterConstant(p, falsity))
	astTestMust(t, md.RegisterConstant(p, falsity))
	n, e := md.NumDeclarations()
	astTestMust(t, e)
	if n != 2 {
		t.Fatal("replacement changed source declaration domain", n)
	}
	n, e = md.NumConstants()
	astTestMust(t, e)
	if n != 2 {
		t.Fatal(n)
	}
	for i, want := range []ast.ASTHandle{p, q} {
		got, e := md.Constant(uint32(i))
		astTestMust(t, e)
		if got != want {
			t.Fatal("source insertion order", got, want)
		}
	}
	got, e := md.ConstantInterpretation(p)
	astTestMust(t, e)
	if got != falsity {
		t.Fatal("replacement did not publish value", got)
	}
	after, e := m.GetRefCount(p)
	astTestMust(t, e)
	if after != before+1 {
		t.Fatal("model owns declaration once", before, after)
	}
	after, e = m.GetRefCount(truth)
	astTestMust(t, e)
	if after != trueBefore {
		t.Fatal("replaced value remains retained", trueBefore, after)
	}
	astTestMust(t, md.Close())
	after, e = m.GetRefCount(p)
	astTestMust(t, e)
	if after != before {
		t.Fatal("model declaration leaked", before, after)
	}
}

func TestModelCoreEvalDeclarationAndMissingResult(t *testing.T) {
	m := astTestManager(t)
	p := modelTestDecl(t, m, "p")
	q := modelTestDecl(t, m, "q")
	md := modelTestCore(t, m)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	astTestMust(t, md.RegisterConstant(p, truth))
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	found, e := md.EvalConstant(p, out)
	astTestMust(t, e)
	if !found {
		t.Fatal("source registered declaration missing")
	}
	h, e := m.RefGet(out)
	astTestMust(t, e)
	if h != truth {
		t.Fatal(h)
	}
	found, e = md.EvalConstant(q, out)
	astTestMust(t, e)
	if found {
		t.Fatal("missing interpretation invented")
	}
	h, e = m.RefGet(out)
	astTestMust(t, e)
	if !h.IsNull() {
		t.Fatal("missing source constant must clear expr_ref", h)
	}
	present, e := md.HasConstantInterpretation(q)
	astTestMust(t, e)
	if present {
		t.Fatal("missing declaration reported present")
	}
}

func TestModelCoreUnregisterPreservesLiteralSourceDeclarationVector(t *testing.T) {
	m := astTestManager(t)
	p := modelTestDecl(t, m, "p")
	q := modelTestDecl(t, m, "q")
	md := modelTestCore(t, m)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	for _, d := range []ast.ASTHandle{p, q} {
		astTestMust(t, md.RegisterConstant(d, truth))
	}
	astTestMust(t, md.UnregisterConstant(p))
	astTestMust(t, md.UnregisterConstant(p))
	n, e := md.NumConstants()
	astTestMust(t, e)
	if n != 1 {
		t.Fatal(n)
	}
	h, e := md.Constant(0)
	astTestMust(t, e)
	if h != q {
		t.Fatal("source erase shifts surviving constants", h)
	}
	n, e = md.NumDeclarations()
	astTestMust(t, e)
	if n != 2 {
		t.Fatal("source unregister leaves m_decls unchanged", n)
	}
	h, e = md.ConstantInterpretation(p)
	astTestMust(t, e)
	if !h.IsNull() {
		t.Fatal(h)
	}
	astTestMust(t, md.RegisterConstant(p, truth))
	n, e = md.NumDeclarations()
	astTestMust(t, e)
	if n != 3 {
		t.Fatal("source re-registration appends again", n)
	}
}

func TestModelCoreHashGrowthCopyAndIndependentOwnership(t *testing.T) {
	m := astTestManager(t)
	md := modelTestCore(t, m)
	copy := modelTestCore(t, m)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	falsity, e := m.MkFalse()
	astTestMust(t, e)
	decls := make([]ast.ASTHandle, 80)
	for i := range decls {
		decls[i] = modelTestDecl(t, m, fmt.Sprintf("p%d", i))
		v := truth
		if i%2 != 0 {
			v = falsity
		}
		astTestMust(t, md.RegisterConstant(decls[i], v))
	}
	astTestMust(t, copy.CopyConstantInterpretations(md))
	astTestMust(t, md.Close())
	for i, d := range decls {
		got, e := copy.ConstantInterpretation(d)
		astTestMust(t, e)
		want := truth
		if i%2 != 0 {
			want = falsity
		}
		if got != want {
			t.Fatal("copied binding lost", i, got, want)
		}
	}
	for i, d := range decls {
		if i%3 == 0 {
			astTestMust(t, copy.UnregisterConstant(d))
		}
	}
	for i, d := range decls {
		present, e := copy.HasConstantInterpretation(d)
		astTestMust(t, e)
		if present != (i%3 != 0) {
			t.Fatal("source map deletion lost binding", i, present)
		}
	}
}

func TestModelCoreRejectsForeignAndNonconstantDeclarations(t *testing.T) {
	m := astTestManager(t)
	other := astTestManager(t)
	md := modelTestCore(t, m)
	foreign := modelTestDecl(t, other, "p")
	truth, e := m.MkTrue()
	astTestMust(t, e)
	if ast.ASTValidateHandles { // foreign, stale or closed-owner handles are rejected only in validating builds
		if e = md.RegisterConstant(foreign, truth); !errors.Is(e, ast.ErrASTHandle) {
			t.Fatal(e)
		}
	}
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	if e = md.RegisterConstant(d, truth); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("source arity assertion", e)
	}
	n, e := md.NumDeclarations()
	astTestMust(t, e)
	if n != 0 {
		t.Fatal("invalid declaration mutated model", n)
	}
}

func TestSATBooleanModelRetainsBindingsAfterEngineAndAtomsClose(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	p, e := m.MkConst(astTestSymbol(t, m, "p"), sort)
	astTestMust(t, e)
	q, e := m.MkConst(astTestSymbol(t, m, "q"), sort)
	astTestMust(t, e)
	pd, e := m.GetDecl(p)
	astTestMust(t, e)
	qd, e := m.GetDecl(q)
	astTestMust(t, e)
	// Source goal and atom map own the apps; no test pin retains them.
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(p))
	nq, e := m.MkNot(q)
	astTestMust(t, e)
	astTestMust(t, g.Assert(nq))
	formulas, e := g.AppendFormulas(nil)
	astTestMust(t, e)
	s, _, _ := satSolverCheckFixture(t)
	tr, e := sattactic.NewGoalTranslator(m, s)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	satOK(t, tr.Compile(formulas))
	result, e := s.Check(nil)
	satOK(t, e)
	if result != lbool.True {
		t.Fatal(result)
	}
	values, e := s.AppendModel(nil)
	satOK(t, e)
	md, e := sattactic.BuildBooleanModel(tr, values)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, md.Close()) })
	astTestMust(t, g.Close())
	satOK(t, tr.Close())
	satOK(t, s.Close())
	if m.IsLiveHandle(p) {
		t.Fatal("source app unexpectedly kept alive")
	}
	truth, e := m.MkTrue()
	astTestMust(t, e)
	falsity, e := m.MkFalse()
	astTestMust(t, e)
	for _, pair := range [][2]ast.ASTHandle{{pd, truth}, {qd, falsity}} {
		got, e := md.ConstantInterpretation(pair[0])
		astTestMust(t, e)
		if got != pair[1] {
			t.Fatal("model binding lost after engine cleanup", got, pair[1])
		}
	}
}

func TestSATBooleanModelSkipsUndefinedSourceRows(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	s, _, _ := satSolverCheckFixture(t)
	tr, e := sattactic.NewGoalTranslator(m, s)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	satOK(t, tr.Compile([]ast.ASTHandle{goalTestApp(t, m, ast.ASTOpOr, p, q)}))
	pv, e := tr.Variable(p)
	satOK(t, e)
	qv, e := tr.Variable(q)
	satOK(t, e)
	// This is a direct source model-row reader fixture, not a solver result.
	rows := make(sat.Model, s.Variables.NumVars())
	for i := range rows {
		rows[i] = lbool.Undef
	}
	rows[pv] = lbool.True
	rows[qv] = lbool.Undef
	md, e := sattactic.BuildBooleanModel(tr, rows)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, md.Close()) })
	pd, e := m.GetDecl(p)
	astTestMust(t, e)
	qd, e := m.GetDecl(q)
	astTestMust(t, e)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	got, e := md.ConstantInterpretation(pd)
	astTestMust(t, e)
	if got != truth {
		t.Fatal(got)
	}
	got, e = md.ConstantInterpretation(qd)
	astTestMust(t, e)
	if !got.IsNull() {
		t.Fatal("undefined row must remain uninterpreted", got)
	}
}

func TestSATBooleanModelMatchesOriginalGoalTruthTable(t *testing.T) {
	for bits := 0; bits < 8; bits++ {
		t.Run(fmt.Sprint(bits), func(t *testing.T) {
			m := astTestManager(t)
			p := goalTestAtom(t, m, "p")
			q := goalTestAtom(t, m, "q")
			r := goalTestAtom(t, m, "r")
			formula := goalTestApp(t, m, ast.ASTOpAnd, goalTestApp(t, m, ast.ASTOpOr, p, q), goalTestApp(t, m, ast.ASTOpNot, goalTestApp(t, m, ast.ASTOpAnd, p, q)), goalTestApp(t, m, ast.ASTOpIte, r, p, q))
			g := goalTestNew(t, m, false, false)
			astTestMust(t, g.Assert(formula))
			for i, h := range []ast.ASTHandle{p, q, r} {
				if bits&(1<<i) == 0 {
					h = goalTestApp(t, m, ast.ASTOpNot, h)
				}
				astTestMust(t, g.Assert(h))
			}
			astTestMust(t, g.ElimRedundancies())
			formulas, e := g.AppendFormulas(nil)
			astTestMust(t, e)
			s, _, _ := satSolverCheckFixture(t)
			tr, e := sattactic.NewGoalTranslator(m, s)
			satOK(t, e)
			t.Cleanup(func() { satOK(t, tr.Close()) })
			satOK(t, tr.Compile(formulas))
			result, e := s.Check(nil)
			satOK(t, e)
			pv, qv, rv := bits&1 != 0, bits&2 != 0, bits&4 != 0
			selected := qv
			if rv {
				selected = pv
			}
			want := lbool.False
			if (pv || qv) && !(pv && qv) && selected {
				want = lbool.True
			}
			if result != want {
				t.Fatal(result, want)
			}
			if result != lbool.True {
				return
			}
			rows, e := s.AppendModel(nil)
			satOK(t, e)
			md, e := sattactic.BuildBooleanModel(tr, rows)
			astTestMust(t, e)
			t.Cleanup(func() { astTestMust(t, md.Close()) })
			expectedOrder := make([]ast.ASTHandle, 0, 3)
			for h := range tr.Atoms.All() {
				d, e := m.GetDecl(h)
				astTestMust(t, e)
				expectedOrder = append(expectedOrder, d)
			}
			for i, want := range expectedOrder {
				d, e := md.Constant(uint32(i))
				astTestMust(t, e)
				if d != want {
					t.Fatal("atom discovery order must determine model registration order", d, want)
				}
			}
			satOK(t, tr.Close())
			satOK(t, s.Close())
			astTestMust(t, g.Close())
			truth, e := m.MkTrue()
			astTestMust(t, e)
			falsity, e := m.MkFalse()
			astTestMust(t, e)
			for i, h := range []ast.ASTHandle{p, q, r} {
				d, e := m.GetDecl(h)
				astTestMust(t, e)
				value, e := md.ConstantInterpretation(d)
				astTestMust(t, e)
				expected := falsity
				if bits&(1<<i) != 0 {
					expected = truth
				}
				if value != expected {
					t.Fatal("retained model violates original goal", i, value, expected)
				}
			}
		})
	}
}

func TestSATBooleanModelRejectsInterpretedRecoveryBranch(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "predicate"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	atom, e := m.MkApp(d, []ast.ASTHandle{p})
	astTestMust(t, e)
	astTestPin(t, m, atom)
	s, _, _ := satSolverCheckFixture(t)
	tr, e := sattactic.NewGoalTranslator(m, s)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	satOK(t, tr.Compile([]ast.ASTHandle{atom}))
	if md, e := sattactic.BuildBooleanModel(tr, nil); md != nil || !errors.Is(e, sattactic.ErrSATBooleanModelInterpreted) {
		t.Fatal("source interpreted case must use sat2goal rather than register Boolean bindings", md, e)
	}
}

func TestModelCoreFinalReferenceAndCopiedOwnerGuard(t *testing.T) {
	m := astTestManager(t)
	md := modelTestCore(t, m)
	copied := *md
	if _, e := copied.NumConstants(); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal(e)
	}
	astTestMust(t, md.IncRef())
	astTestMust(t, md.DecRef())
	if _, e := md.NumConstants(); e == nil {
		t.Fatal("final source model reference must retire owner")
	}
}

func TestModelCoreConstantLookupAcceptsFunctionDeclaration(t *testing.T) {
	m := astTestManager(t)
	md := modelTestCore(t, m)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	// get_const_interp is a table lookup; only register_decl has the arity-zero
	// assertion. The original getter permits this query and reports no binding.
	value, e := md.ConstantInterpretation(d)
	astTestMust(t, e)
	if !value.IsNull() {
		t.Fatal(value)
	}
	present, e := md.HasConstantInterpretation(d)
	astTestMust(t, e)
	if present {
		t.Fatal("function was registered as a constant")
	}
	astTestMust(t, md.UnregisterConstant(d))
}
