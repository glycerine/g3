package tactic

// Source: src/tactic/sine_filter.cpp; src/tactic/sine_filter.h.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSineTacticSelectsLastConnectedComponent(t *testing.T) {
	m := astTestManager(t)
	b, err := m.MkBoolSort()
	satOK(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "sine-f"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	satOK(t, err)
	g, err := m.MkFuncDecl(astTestSymbol(t, m, "sine-g"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	satOK(t, err)
	a := goalTestAtom(t, m, "sine-a")
	bv := goalTestAtom(t, m, "sine-b")
	c := goalTestAtom(t, m, "sine-c")
	first, err := m.MkApp(f, []ast.ASTHandle{a})
	satOK(t, err)
	astTestPin(t, m, first)
	separate, err := m.MkApp(g, []ast.ASTHandle{bv})
	satOK(t, err)
	astTestPin(t, m, separate)
	last, err := m.MkApp(f, []ast.ASTHandle{c})
	satOK(t, err)
	astTestPin(t, m, last)
	goal := goalTestNew(t, m, false, true)
	satOK(t, goal.Assert(first))
	satOK(t, goal.Assert(separate))
	satOK(t, goal.Assert(last))
	satOK(t, goal.SetDepth(4))
	tacticBase, err := NewSineTactic(m, nil)
	satOK(t, err)
	tactic := tacticBase.(*sineTactic)
	result := &TacticResult{}
	t.Cleanup(func() { satOK(t, result.Close()) })
	satOK(t, tactic.Apply(goal, result))
	goalTestForms(t, goal, first, last)
	depth, err := goal.Depth()
	satOK(t, err)
	precision, err := goal.Precision()
	satOK(t, err)
	if depth != 5 || precision != GoalOver || len(result.Goals) != 1 || result.Goals[0] != goal {
		t.Fatal("SInE result metadata", depth, precision, len(result.Goals))
	}
	if _, ok := result.ModelConverter.(*tacticFilterModelConverter); !ok {
		t.Fatal("SInE model converter", result.ModelConverter)
	}
}

func TestSineTacticQuantifierPatternGating(t *testing.T) {
	m := astTestManager(t)
	b, err := m.MkBoolSort()
	satOK(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "sine-trigger"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	satOK(t, err)
	k, err := m.MkFuncDecl(astTestSymbol(t, m, "sine-body"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	satOK(t, err)
	v, err := m.MkVar(0, b)
	satOK(t, err)
	triggerTerm, err := m.MkApp(f, []ast.ASTHandle{v})
	satOK(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{triggerTerm})
	satOK(t, err)
	body, err := m.MkApp(k, []ast.ASTHandle{v})
	satOK(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, m, "x")}, Body: body, Patterns: []ast.ASTHandle{pattern}})
	satOK(t, err)
	astTestPin(t, m, q)
	a := goalTestAtom(t, m, "sine-trigger-value")
	seed, err := m.MkApp(f, []ast.ASTHandle{a})
	satOK(t, err)
	astTestPin(t, m, seed)
	goal := goalTestNew(t, m, false, false)
	satOK(t, goal.Assert(q))
	satOK(t, goal.Assert(seed))
	tacticBase, err := NewSineTactic(m, nil)
	satOK(t, err)
	selected, err := tacticBase.(*sineTactic).filterExpressions(goal)
	satOK(t, err)
	if len(selected) != 1 || selected[0] != seed {
		t.Fatal("pattern-gated component", selected)
	}
}

// AUDIT.md tactic_top_01-F5: sine_filter re-asserts the kept formulas with a
// null proof; goal::assert_expr only SASSERTs proof presence, so release
// builds keep the formulas (with null proofs) under proof mode.
func TestSineTacticProofModeKeepsNullProofs(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	p := goalTestAtom(t, m, "sine-proof-p")
	q := goalTestAtom(t, m, "sine-proof-q")
	goal := goalTestNew(t, m, true, false)
	for _, a := range []ast.ASTHandle{p, q} {
		pr, err := m.MkProofAsserted(a)
		satOK(t, err)
		satOK(t, goal.AssertWithProof(a, pr, ast.ASTDependency{}))
	}
	tacticBase, err := NewSineTactic(m, nil)
	satOK(t, err)
	result := &TacticResult{}
	t.Cleanup(func() { satOK(t, result.Close()) })
	err = tacticBase.Apply(goal, result)
	if ast.ASTSourceDebug {
		if !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("source debug goal::assert_expr proof SASSERT", err)
		}
		return
	}
	satOK(t, err)
	goalTestForms(t, goal, q)
	proof, err := goal.Proof(0)
	satOK(t, err)
	if !proof.IsNull() {
		t.Fatal("sine kept a proof", proof)
	}
}
