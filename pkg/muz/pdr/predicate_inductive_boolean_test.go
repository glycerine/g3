package pdr

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestInductiveAssumptionsUseRewrittenCubeAndRawNegation(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	makeAtom := func(name string) (ast.ASTHandle, ast.ASTRef) {
		symbol, x := m.Symbols.Intern(name)
		closureMust(t, x)
		decl, x := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		closureMust(t, x)
		atom, x := m.MkApp(decl, nil)
		closureMust(t, x)
		root, x := m.NewRef(atom)
		closureMust(t, x)
		return atom, root
	}
	p, pRoot := makeAtom("inductive_rewrite_p")
	tag, tagRoot := makeAtom("inductive_rewrite_tag")
	transformer.rules = []*normalizedRule{{tag: tag, predecessors: []*PredicateTransformer{transformer}}}
	// Duplicate input must be folded before source constructs a raw not.
	result, err := transformer.inductiveAssumptionsRef([]ast.ASTHandle{p, p})
	closureMust(t, err)
	formula, err := m.RefGet(result)
	closureMust(t, err)
	op, err := m.GetDeclKind(formula)
	closureMust(t, err)
	if op != ast.ASTOpImplies {
		t.Fatal("expected guarded assumption", op)
	}
	arg, err := m.GetArg(formula, 1)
	closureMust(t, err)
	isNot, inner, err := m.IsNot(arg)
	closureMust(t, err)
	if !isNot || inner != p {
		t.Fatal("duplicate cube was not rewritten before negation", arg, inner)
	}
	closureMust(t, m.DelRef(&result))
	// A source AST-manager not remains raw even when the conjunction is true.
	result, err = transformer.inductiveAssumptionsRef(nil)
	closureMust(t, err)
	formula, err = m.RefGet(result)
	closureMust(t, err)
	arg, err = m.GetArg(formula, 1)
	closureMust(t, err)
	isNot, inner, err = m.IsNot(arg)
	closureMust(t, err)
	if !isNot || !m.IsTrue(inner) {
		t.Fatal("source raw not(true) was folded", arg, inner)
	}
	closureMust(t, m.DelRef(&result))
	transformer.rules = nil
	closureMust(t, m.DelRef(&tagRoot))
	closureMust(t, m.DelRef(&pRoot))
	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}

func TestInductiveAssumptionsInvalidInputReleasesReferences(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	before := m.Stats().LiveReferences
	result, err := transformer.inductiveAssumptionsRef([]ast.ASTHandle{{}})
	if err == nil || result != (ast.ASTRef{}) {
		t.Fatal("invalid input returned an owner", result, err)
	}
	if got := m.Stats().LiveReferences; got != before {
		t.Fatal("failed construction leaked references", got, before)
	}
	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}
