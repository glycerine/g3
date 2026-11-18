package proofs

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func legacyBoolAtom(t *testing.T, m *ast.ASTManager, id uint32) ast.ASTHandle {
	t.Helper()
	b, err := m.MkBoolSort()
	proofMust(t, err)
	h, err := m.MkConst(usymbol.Numerical(id), b)
	proofMust(t, err)
	return h
}

func TestReduceHypothesesLegacyDiscoversClosedUnitAndRebuilds(t *testing.T) {
	m := proofManager(t)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a, b := legacyBoolAtom(t, m, 2101), legacyBoolAtom(t, m, 2102)
	fact, err := m.MkIff(a, b)
	proofMust(t, err)
	hypothesis, err := m.MkProofHypothesis(fact)
	proofMust(t, err)
	hypothesis = proofPin(t, m, hypothesis)
	closed, err := m.MkProofAsserted(fact)
	proofMust(t, err)
	closed = proofPin(t, m, closed)
	// Reverse-parent DFS visits the hypothesis before the closed proof, which
	// lets the source unit-discovery pass register the latter.
	root, err := m.MkProofHyperResolve([]ast.ASTHandle{closed, hypothesis}, fact, nil, [][]ast.ASTHandle{nil})
	proofMust(t, err)
	root = proofPin(t, m, root)
	ref, err := m.NewRef(root)
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&ref)) }()
	proofMust(t, ReduceHypothesesLegacy(m, ref))
	result, err := m.RefGet(ref)
	proofMust(t, err)
	parents, conclusion, _ := proofHyperRead(t, m, result)
	if conclusion != fact || len(parents) != 2 || parents[0] != closed || parents[1] != closed {
		t.Fatal("legacy unit substitution", parents, conclusion, fact, closed)
	}
}

func TestReduceHypothesesLegacyRebuildsUnitResolution(t *testing.T) {
	m := proofManager(t)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a, b := legacyBoolAtom(t, m, 2111), legacyBoolAtom(t, m, 2112)
	notA, err := m.MkNot(a)
	proofMust(t, err)
	clause, err := m.MkOr(notA, b)
	proofMust(t, err)
	clauseProof, err := m.MkProofAsserted(clause)
	proofMust(t, err)
	clauseProof = proofPin(t, m, clauseProof)
	hypothesis, err := m.MkProofHypothesis(a)
	proofMust(t, err)
	hypothesis = proofPin(t, m, hypothesis)
	resolution, err := m.MkProofUnitResolution([]ast.ASTHandle{clauseProof, hypothesis})
	proofMust(t, err)
	resolution = proofPin(t, m, resolution)
	closed, err := m.MkProofAsserted(a)
	proofMust(t, err)
	closed = proofPin(t, m, closed)
	root, err := m.MkProofHyperResolve([]ast.ASTHandle{closed, resolution}, b, nil, [][]ast.ASTHandle{nil})
	proofMust(t, err)
	root = proofPin(t, m, root)
	ref, err := m.NewRef(root)
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&ref)) }()
	proofMust(t, ReduceHypothesesLegacy(m, ref))
	result, err := m.RefGet(ref)
	proofMust(t, err)
	parents, _, _ := proofHyperRead(t, m, result)
	if len(parents) != 2 {
		t.Fatal("legacy root parents", parents)
	}
	yes, err := m.IsProofKind(parents[1], ast.ASTPrUnitResolution)
	proofMust(t, err)
	unit, err := m.GetProofParent(parents[1], 1)
	proofMust(t, err)
	if !yes || unit != closed {
		t.Fatal("legacy rebuilt resolution", yes, unit, closed)
	}
}

func TestReduceHypothesesLegacyShortCircuitsReducedFalse(t *testing.T) {
	m := proofManager(t)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a := legacyBoolAtom(t, m, 2121)
	hypothesis, err := m.MkProofHypothesis(a)
	proofMust(t, err)
	hypothesis = proofPin(t, m, hypothesis)
	notA, err := m.MkNot(a)
	proofMust(t, err)
	hypothesisNotA, err := m.MkProofHypothesis(notA)
	proofMust(t, err)
	hypothesisNotA = proofPin(t, m, hypothesisNotA)
	contradiction, err := m.MkProofUnitResolution([]ast.ASTHandle{hypothesis, hypothesisNotA})
	proofMust(t, err)
	contradiction = proofPin(t, m, contradiction)
	tautology, err := m.MkOr(notA, a)
	proofMust(t, err)
	lemma, err := m.MkProofLemma(contradiction, tautology)
	proofMust(t, err)
	lemma = proofPin(t, m, lemma)
	closed, err := m.MkProofAsserted(a)
	proofMust(t, err)
	closed = proofPin(t, m, closed)
	root, err := m.MkProofHyperResolve([]ast.ASTHandle{closed, lemma}, a, nil, [][]ast.ASTHandle{nil})
	proofMust(t, err)
	root = proofPin(t, m, root)
	ref, err := m.NewRef(root)
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&ref)) }()
	proofMust(t, ReduceHypothesesLegacy(m, ref))
	result, err := m.RefGet(ref)
	proofMust(t, err)
	fact, err := m.GetProofFact(result)
	proofMust(t, err)
	if !m.IsFalse(fact) {
		t.Fatal("legacy false short circuit", result, fact)
	}
}

// m_units is an obj_map: inserting a second hypothesis-free proof of the
// same fact replaces the first (AUDIT ast_sub_03-F1).
func TestReduceHypothesesLegacyLastUnitWins(t *testing.T) {
	m := proofManager(t)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a, b := legacyBoolAtom(t, m, 2111), legacyBoolAtom(t, m, 2112)
	fact, err := m.MkIff(a, b)
	proofMust(t, err)
	hypothesis, err := m.MkProofHypothesis(fact)
	proofMust(t, err)
	hypothesis = proofPin(t, m, hypothesis)
	first, err := m.MkProofAsserted(fact)
	proofMust(t, err)
	first = proofPin(t, m, first)
	c := legacyBoolAtom(t, m, 2113)
	assertedC, err := m.MkProofAsserted(c)
	proofMust(t, err)
	rewrite, err := m.MkProofRewrite(c, fact)
	proofMust(t, err)
	second, err := m.MkProofModusPonens(assertedC, rewrite)
	proofMust(t, err)
	second = proofPin(t, m, second)
	// Reverse-parent DFS visits hypothesis, second, then first: first is the
	// last unit inserted for the fact.
	root, err := m.MkProofHyperResolve([]ast.ASTHandle{first, second, hypothesis}, fact, nil, [][]ast.ASTHandle{nil})
	proofMust(t, err)
	root = proofPin(t, m, root)
	ref, err := m.NewRef(root)
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&ref)) }()
	proofMust(t, ReduceHypothesesLegacy(m, ref))
	result, err := m.RefGet(ref)
	proofMust(t, err)
	parents, _, _ := proofHyperRead(t, m, result)
	if len(parents) != 3 || parents[2] != first {
		t.Fatal("hypothesis must be replaced by the last unit inserted", parents, first, second)
	}
}
