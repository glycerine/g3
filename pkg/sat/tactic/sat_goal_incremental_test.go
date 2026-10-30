package tactic

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATGoalBorrowedAtomMapIncrementalScopesAndOwnership(t *testing.T) {
	m := astTestManager(t)
	s, c, params := satSolverCheckFixture(t)
	key := "elim_vars"
	satOK(t, c.SetBoolName(&params, &key, false))
	satOK(t, s.UpdateParams(&params))
	mapping, err := NewAtom2BoolVar(m)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, mapping.Close()) })
	p := goalTestAtom(t, m, "incremental-p")
	q := goalTestAtom(t, m, "incremental-q")
	r := goalTestAtom(t, m, "incremental-r")
	baseRefs, err := m.GetRefCount(p)
	satOK(t, err)
	compile := func(forms ...ast.ASTHandle) {
		t.Helper()
		tr, err := NewGoalTranslatorWithAtomMap(m, s, mapping, true)
		satOK(t, err)
		satOK(t, tr.Compile(forms))
		satOK(t, tr.Close())
	}
	compile(goalTestApp(t, m, ast.ASTOpOr, p, q))
	pv, err := mapping.ToBoolVar(p)
	satOK(t, err)
	if pv == sat.NullBoolVar {
		t.Fatal("missing persistent atom")
	}
	refs, err := m.GetRefCount(p)
	satOK(t, err)
	// The OR application owns another reference, plus exactly one map root.
	if refs != baseRefs+2 {
		t.Fatal("translator destruction lost or duplicated the mapping root", refs, baseRefs)
	}
	external, err := s.SourceIsExternal(pv)
	satOK(t, err)
	if !external {
		t.Fatal("incremental default-external atom was eliminable")
	}
	satOK(t, s.UserPush())
	satOK(t, mapping.Push())
	compile(goalTestApp(t, m, ast.ASTOpNot, p), r)
	oldR, err := mapping.ToBoolVar(r)
	satOK(t, err)
	status, err := s.Check(nil)
	satOK(t, err)
	if status != lbool.True {
		t.Fatal("scoped Boolean extension", status)
	}
	satOK(t, mapping.Pop(1))
	satOK(t, s.UserPop(1))
	got, err := mapping.ToBoolVar(r)
	satOK(t, err)
	if got != sat.NullBoolVar {
		t.Fatal("popped atom survived its source scope")
	}
	got, err = mapping.ToBoolVar(p)
	satOK(t, err)
	if got != pv {
		t.Fatal("base atom identity changed across pop")
	}
	compile(p, goalTestApp(t, m, ast.ASTOpNot, q), goalTestApp(t, m, ast.ASTOpNot, r))
	newR, err := mapping.ToBoolVar(r)
	satOK(t, err)
	if newR == oldR {
		t.Fatal("new compilation reused a popped atom identity")
	}
	status, err = s.Check(nil)
	satOK(t, err)
	if status != lbool.True {
		t.Fatal("popped clauses remained active", status)
	}
}

func TestSATGoalBorrowedAtomMapDependenciesAndPerCallInterpretedAtoms(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	mapping, err := NewAtom2BoolVar(m)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, mapping.Close()) })
	p := goalTestAtom(t, m, "borrowed-dependency")
	negative := goalTestApp(t, m, ast.ASTOpNot, p)
	g := goalTestNew(t, m, false, true)
	leaf, err := m.DependencyLeaf(negative)
	satOK(t, err)
	falsehood, err := m.MkFalse()
	satOK(t, err)
	satOK(t, g.AssertWithProof(falsehood, ast.ASTHandle{}, leaf))
	tr, err := NewGoalTranslatorWithAtomMap(m, s, mapping, true)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	satOK(t, tr.CompileGoal(g))
	bindings, err := tr.DependencyLiterals()
	satOK(t, err)
	if len(bindings) != 1 || bindings[0].Expression != negative || !bindings[0].Literal.Sign() {
		t.Fatal("original signed dependency was not exposed", bindings)
	}
	status, err := s.Check([]sat.Literal{bindings[0].Literal})
	satOK(t, err)
	if status != lbool.False {
		t.Fatal("signed dependency did not enable guarded contradiction", status)
	}
	satOK(t, tr.Close())
	boolSort, err := m.MkBoolSort()
	satOK(t, err)
	d, err := m.MkFuncDecl(astTestSymbol(t, m, "borrowed-f"), []ast.ASTHandle{boolSort}, boolSort, ast.ASTFuncDeclInfo{})
	satOK(t, err)
	application, err := m.MkApp(d, []ast.ASTHandle{p})
	satOK(t, err)
	astTestPin(t, m, application)
	for call := 0; call < 2; call++ {
		satOK(t, s.PopToBaseLevel())
		current, err := NewGoalTranslatorWithAtomMap(m, s, mapping, true)
		satOK(t, err)
		satOK(t, current.Compile([]ast.ASTHandle{application}))
		atoms, err := current.InterpretedAtoms()
		satOK(t, err)
		if call == 0 && (len(atoms) != 1 || atoms[0] != application) || call == 1 && len(atoms) != 0 {
			t.Fatal("interpreted discovery accumulated across source calls", call, atoms)
		}
		satOK(t, current.Close())
	}
	interpreted, err := mapping.InterpretedAtoms()
	satOK(t, err)
	if !interpreted {
		t.Fatal("persistent source map lost its interpreted flag")
	}
}

func TestSATGoalBorrowedAtomMapBooleanModelIncludesEarlierAtoms(t *testing.T) {
	m := astTestManager(t)
	s, c, params := satSolverCheckFixture(t)
	key := "elim_vars"
	satOK(t, c.SetBoolName(&params, &key, false))
	satOK(t, s.UpdateParams(&params))
	mapping, err := NewAtom2BoolVar(m)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, mapping.Close()) })
	p := goalTestAtom(t, m, "earlier-p")
	q := goalTestAtom(t, m, "later-q")
	first, err := NewGoalTranslatorWithAtomMap(m, s, mapping, true)
	satOK(t, err)
	satOK(t, first.Compile([]ast.ASTHandle{p}))
	satOK(t, first.Close())
	last, err := NewGoalTranslatorWithAtomMap(m, s, mapping, true)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, last.Close()) })
	satOK(t, last.Compile([]ast.ASTHandle{q}))
	status, err := s.Check(nil)
	satOK(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	values, err := s.AppendModel(nil)
	satOK(t, err)
	md, err := BuildBooleanModel(last, values)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, md.Close()) })
	for _, atom := range []ast.ASTHandle{p, q} {
		d, err := m.GetDecl(atom)
		satOK(t, err)
		value, err := md.ConstantInterpretation(d)
		satOK(t, err)
		if !m.IsTrue(value) {
			t.Fatal("model omitted an earlier compilation's atom")
		}
	}
}
