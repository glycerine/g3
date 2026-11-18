package normal_forms

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func definedNamesMust(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}

func definedNamesManager(t *testing.T, proofs bool) *ast.ASTManager {
	t.Helper()
	b := arena.NewBudget(1 << 30)
	sy, e := usymbol.NewManager(usymbol.Config{Budget: b})
	definedNamesMust(t, e)
	q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{
		Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}},
	}})
	definedNamesMust(t, e)
	mode := ast.ASTProofDisabled
	if proofs {
		mode = ast.ASTProofEnabled
	}
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b, ProofMode: mode})
	definedNamesMust(t, e)
	t.Cleanup(func() {
		definedNamesMust(t, m.Close())
		definedNamesMust(t, q.Close())
		definedNamesMust(t, sy.Close())
		if b.Used() != 0 {
			t.Errorf("owner graph retained %d bytes", b.Used())
		}
	})
	return m
}

func definedNamesPin(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) {
	t.Helper()
	definedNamesMust(t, m.IncRef(h))
	t.Cleanup(func() { definedNamesMust(t, m.DecRef(h)) })
}

func definedNamesOutputs(t *testing.T, m *ast.ASTManager) [4]ast.ASTRef {
	t.Helper()
	var out [4]ast.ASTRef
	for i := range out {
		var e error
		out[i], e = m.NewRef(ast.ASTHandle{})
		definedNamesMust(t, e)
	}
	t.Cleanup(func() {
		for i := range out {
			definedNamesMust(t, m.DelRef(&out[i]))
		}
	})
	return out
}

func TestDefinedNamesFreeVariablesProofsAndCache(t *testing.T) {
	m := definedNamesManager(t, true)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	v2, e := m.MkVar(2, b)
	definedNamesMust(t, e)
	definedNamesPin(t, m, v2)
	d, e := NewDefinedNames(m)
	definedNamesMust(t, e)
	t.Cleanup(func() { definedNamesMust(t, d.Close()) })
	out := definedNamesOutputs(t, m)
	created, e := d.MkName(v2, out[0], out[1], out[2], out[3])
	definedNamesMust(t, e)
	if !created {
		t.Fatal("first name was reported cached")
	}
	definition, e := m.RefGet(out[0])
	definedNamesMust(t, e)
	if op, x := m.GetDeclKind(definition); x != nil || op != ast.ASTOpAnd {
		t.Fatal("boolean definition is not a conjunction", op, x)
	}
	for i := uint32(0); i < 2; i++ {
		conjunct, x := m.GetArg(definition, i)
		definedNamesMust(t, x)
		if kind, x := m.GetKind(conjunct); x != nil || kind != ast.ASTQuantifier {
			t.Fatal("free-variable definition conjunct is not closed", i, kind, x)
		}
		if count, x := m.GetNumDeclarations(conjunct); x != nil || count != 1 {
			t.Fatal("unused placeholder binders were not removed", i, count, x)
		}
		if binder, x := m.GetDeclarationName(conjunct, 0); x != nil || binder != usymbol.Numerical(2) {
			t.Fatal("source reverse binder name", i, binder, x)
		}
		if patterns, x := m.GetNumPatterns(conjunct); x != nil || patterns != 1 {
			t.Fatal("name trigger", i, patterns, x)
		}
	}
	if op, x := m.GetDeclKind(func() ast.ASTHandle { h, y := m.RefGet(out[1]); definedNamesMust(t, y); return h }()); x != nil || op != ast.ASTPrDefIntro {
		t.Fatal("definition-introduction proof", op, x)
	}
	if op, x := m.GetDeclKind(func() ast.ASTHandle { h, y := m.RefGet(out[3]); definedNamesMust(t, y); return h }()); x != nil || op != ast.ASTPrApplyDef {
		t.Fatal("apply-definition proof", op, x)
	}
	firstName, e := m.RefGet(out[2])
	definedNamesMust(t, e)
	second := definedNamesOutputs(t, m)
	created, e = d.MkName(v2, second[0], second[1], second[2], second[3])
	definedNamesMust(t, e)
	secondName, e := m.RefGet(second[2])
	definedNamesMust(t, e)
	if created || firstName != secondName {
		t.Fatal("cached ordinary name", created, firstName, secondName)
	}
}

func TestDefinedNamesPositiveScopesOrderingAndTermITE(t *testing.T) {
	m := definedNamesManager(t, false)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	condition, e := m.MkVar(0, b)
	definedNamesMust(t, e)
	definedNamesPin(t, m, condition)
	sortName, e := m.Symbols.Intern("defined-name-sort")
	definedNamesMust(t, e)
	s, e := m.MkSort(sortName, ast.ASTSortInfo{})
	definedNamesMust(t, e)
	definedNamesPin(t, m, s)
	left, e := m.MkFreshConst("left", s)
	definedNamesMust(t, e)
	definedNamesPin(t, m, left)
	right, e := m.MkFreshConst("right", s)
	definedNamesMust(t, e)
	definedNamesPin(t, m, right)
	ite, e := m.MkIte(condition, left, right)
	definedNamesMust(t, e)
	definedNamesPin(t, m, ite)
	d, e := NewDefinedNamesWithPrefix(m, nil)
	definedNamesMust(t, e)
	t.Cleanup(func() { definedNamesMust(t, d.Close()) })
	definedNamesMust(t, d.Push())
	ordinary := definedNamesOutputs(t, m)
	created, e := d.MkName(ite, ordinary[0], ordinary[1], ordinary[2], ordinary[3])
	definedNamesMust(t, e)
	if !created {
		t.Fatal("term ITE name was cached")
	}
	definition, e := m.RefGet(ordinary[0])
	definedNamesMust(t, e)
	if op, x := m.GetDeclKind(definition); x != nil || op != ast.ASTOpAnd {
		t.Fatal("term ITE definition", op, x)
	}
	positive := definedNamesOutputs(t, m)
	created, e = d.MkPositiveName(condition, positive[0], positive[1], positive[2], positive[3])
	definedNamesMust(t, e)
	if !created {
		t.Fatal("positive name collided with ordinary cache")
	}
	if count, x := d.NumNames(); x != nil || count != 2 {
		t.Fatal("combined name count", count, x)
	}
	ordinaryName, e := m.RefGet(ordinary[2])
	definedNamesMust(t, e)
	positiveName, e := m.RefGet(positive[2])
	definedNamesMust(t, e)
	ordinaryDecl, e := m.GetDecl(ordinaryName)
	definedNamesMust(t, e)
	positiveDecl, e := m.GetDecl(positiveName)
	definedNamesMust(t, e)
	if got, x := d.NameDecl(0); x != nil || got != ordinaryDecl {
		t.Fatal("ordinary declaration order", got, x)
	}
	if got, x := d.NameDecl(1); x != nil || got != positiveDecl {
		t.Fatal("positive declaration order", got, x)
	}
	definedNamesMust(t, d.Pop(1))
	if count, x := d.NumNames(); x != nil || count != 0 {
		t.Fatal("paired scope pop", count, x)
	}
	if _, x := d.NameDecl(0); x == nil {
		t.Fatal("out-of-range declaration accepted")
	}
	definedNamesMust(t, d.Pop(0))
	copyOwner := *d
	if _, x := copyOwner.NumNames(); x == nil {
		t.Fatal("copied owner accepted")
	}
	definedNamesMust(t, d.Close())
	definedNamesMust(t, d.Close())
	if _, x := d.NumNames(); x == nil {
		t.Fatal("closed owner accepted")
	}
}

// mk_definition quantifies the first conjunct (bound_vars) before the second
// disjunction is built, so the first quantifier's id is below the second
// conjunct's body.
func TestDefinedNamesQuantifiesFirstConjunctFirst(t *testing.T) {
	m := definedNamesManager(t, false)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	v0, e := m.MkVar(0, b)
	definedNamesMust(t, e)
	definedNamesPin(t, m, v0)
	d, e := NewDefinedNames(m)
	definedNamesMust(t, e)
	t.Cleanup(func() { definedNamesMust(t, d.Close()) })
	out := definedNamesOutputs(t, m)
	_, e = d.MkName(v0, out[0], out[1], out[2], out[3])
	definedNamesMust(t, e)
	definition, e := m.RefGet(out[0])
	definedNamesMust(t, e)
	first, e := m.GetArg(definition, 0)
	definedNamesMust(t, e)
	second, e := m.GetArg(definition, 1)
	definedNamesMust(t, e)
	body, e := m.GetQuantifierBody(second)
	definedNamesMust(t, e)
	firstID, e := m.GetASTID(first)
	definedNamesMust(t, e)
	bodyID, e := m.GetASTID(body)
	definedNamesMust(t, e)
	if firstID >= bodyID {
		t.Fatal("first conjunct must be quantified before the second is built", firstID, bodyID)
	}
}
