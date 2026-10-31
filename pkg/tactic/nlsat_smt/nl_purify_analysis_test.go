package nlsatsmt

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func analysisKeep(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, err error) ast.ASTHandle {
	t.Helper()
	astTestMust(t, err)
	astTestPin(t, m, h)
	return h
}

func analysisConst(t *testing.T, m *ast.ASTManager, sort ast.ASTHandle, name string) ast.ASTHandle {
	t.Helper()
	h, err := m.MkConst(astTestSymbol(t, m, name), sort)
	return analysisKeep(t, m, h, err)
}

func TestNLPurifyPolaritiesUpgradeSharedSubexpressions(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, q := analysisConst(t, m, boolean, "p"), analysisConst(t, m, boolean, "q")
	h, err := m.MkOr(p, q)
	shared := analysisKeep(t, m, h, err)
	h, err = m.MkNot(shared)
	negated := analysisKeep(t, m, h, err)
	// LIFO source traversal first sees shared positively, then upgrades it and
	// its children after reaching it through negated.
	got, err := purificationPolarities(m, []ast.ASTHandle{negated, shared})
	astTestMust(t, err)
	for _, term := range []ast.ASTHandle{shared, p, q} {
		if got[term] != dual {
			t.Fatalf("shared term %v has polarity %v", term, got[term])
		}
	}
	if got[negated] != positive {
		t.Fatal("root polarity", got)
	}
	got, err = purificationPolarities(m, []ast.ASTHandle{negated})
	astTestMust(t, err)
	if got[shared] != negative || got[p] != negative || got[q] != negative {
		t.Fatal(got)
	}
	h, err = m.MkImplies(p, q)
	implies := analysisKeep(t, m, h, err)
	got, err = purificationPolarities(m, []ast.ASTHandle{implies})
	astTestMust(t, err)
	if got[implies] != positive || got[p] != dual || got[q] != dual {
		t.Fatal("non-monotone basic operator", got)
	}
	for _, p := range []polarity{positive, negative, dual} {
		if negatePolarity(negatePolarity(p)) != p {
			t.Fatal("polarity negation", p)
		}
	}
}

func TestNLPurifyRejectsQuantifiersAndFreeBoundVariables(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	h, err := m.MkVar(0, boolean)
	v := analysisKeep(t, m, h, err)
	h, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: v})
	q := analysisKeep(t, m, h, err)
	for _, root := range []ast.ASTHandle{v, q} {
		_, err := purificationPolarities(m, []ast.ASTHandle{root})
		if err == nil || !strings.Contains(err.Error(), "nl-purify tactic does not support quantifiers") {
			t.Fatal(err)
		}
		pure, err := pureArithmeticExpressions(m, []ast.ASTHandle{root})
		astTestMust(t, err)
		if pure[root] {
			t.Fatal("non-app treated as arithmetic")
		}
	}
}

func TestNLPurifyArithmeticPuritySeparatesInterfaces(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x := analysisConst(t, m, real, "x")
	i := analysisConst(t, m, integer, "i")
	p := analysisConst(t, m, boolean, "p")
	h, err := m.ArithmeticRealNumeralText("0")
	zero := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticMul(x, x)
	square := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticLE(square, zero)
	bound := analysisKeep(t, m, h, err)
	h, err = m.MkEq(x, zero)
	equality := analysisKeep(t, m, h, err)
	h, err = m.MkAnd(bound, equality)
	pureFormula := analysisKeep(t, m, h, err)
	h, err = m.MkAnd(bound, p)
	mixedFormula := analysisKeep(t, m, h, err)
	h, err = m.MkFamilyApp(family, 14, nil, []ast.ASTHandle{i})
	cast := analysisKeep(t, m, h, err)
	h, err = m.MkFuncDecl(astTestSymbol(t, m, "f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
	f := analysisKeep(t, m, h, err)
	h, err = m.MkApp(f, []ast.ASTHandle{x})
	fx := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticLE(fx, zero)
	interfaceBound := analysisKeep(t, m, h, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	roots := []ast.ASTHandle{pureFormula, mixedFormula, interfaceBound, cast, truth}
	got, err := pureArithmeticExpressions(m, roots)
	astTestMust(t, err)
	for _, term := range []ast.ASTHandle{x, zero, square, bound, equality, pureFormula, truth} {
		if !got[term] {
			t.Fatalf("pure arithmetic term rejected: %v", term)
		}
	}
	for _, term := range []ast.ASTHandle{i, p, fx, cast, mixedFormula, interfaceBound} {
		if got[term] {
			t.Fatalf("interface/non-real term accepted: %v", term)
		}
	}
	pols, err := purificationPolarities(m, []ast.ASTHandle{bound})
	astTestMust(t, err)
	if pols[bound] != positive || pols[square] != dual || pols[x] != dual {
		t.Fatal("arithmetic arguments require dual polarity", pols)
	}
}

func TestNLPurifyMovesArithmeticGoalRoots(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		m := astTestManager(t)
		if proofs {
			astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
		}
		family, err := m.RegisterArithmeticPlugin()
		astTestMust(t, err)
		real, err := m.MkFamilySort(family, 0, nil)
		astTestMust(t, err)
		boolean, err := m.MkBoolSort()
		astTestMust(t, err)
		x := analysisConst(t, m, real, "x")
		p := analysisConst(t, m, boolean, "p")
		h, err := m.ArithmeticRealNumeralText("0")
		zero := analysisKeep(t, m, h, err)
		h, err = m.MkArithmeticLE(x, zero)
		upper := analysisKeep(t, m, h, err)
		h, err = m.MkArithmeticGE(x, zero)
		lower := analysisKeep(t, m, h, err)
		g, err := tactic.NewGoal(m, proofs, false, false)
		astTestMust(t, err)
		nl, err := tactic.NewGoal(m, proofs, false, false)
		astTestMust(t, err)
		var originalProofs []ast.ASTHandle
		for _, root := range []ast.ASTHandle{upper, p, lower} {
			var proof ast.ASTHandle
			if proofs {
				proof, err = m.MkProofAsserted(root)
				astTestMust(t, err)
			}
			astTestMust(t, g.AssertWithProof(root, proof, ast.ASTDependency{}))
			originalProofs = append(originalProofs, proof)
		}
		astTestMust(t, removePureArithmetic(g, nl))
		size, err := g.Size()
		astTestMust(t, err)
		if size != 3 {
			t.Fatal("goal slots changed", size)
		}
		size, err = nl.Size()
		astTestMust(t, err)
		if size != 2 {
			t.Fatal("nonlinear goal size", size)
		}
		for i := uint32(0); i < 3; i++ {
			root, err := g.Form(i)
			astTestMust(t, err)
			if i == 1 {
				if root != p {
					t.Fatal("interface formula moved")
				}
			} else if !m.IsTrue(root) {
				t.Fatal("arithmetic root not cleared")
			}
		}
		for i, root := range []ast.ASTHandle{upper, lower} {
			got, err := nl.Form(uint32(i))
			astTestMust(t, err)
			if got != root {
				t.Fatal("transfer order")
			}
			pr, err := nl.Proof(uint32(i))
			astTestMust(t, err)
			if pr != originalProofs[2*i] {
				t.Fatal("proof changed during transfer")
			}
		}
		astTestMust(t, g.Close())
		// The destination's roots and proofs survive closing their source goal.
		_, err = nl.Form(0)
		astTestMust(t, err)
		astTestMust(t, nl.Close())
	}
}
