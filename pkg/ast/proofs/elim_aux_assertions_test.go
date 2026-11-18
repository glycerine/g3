package proofs

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestEliminateAuxAssertionsCollapsesUnitResolution(t *testing.T) {
	m := proofManager(t)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	b, err := m.MkBoolSort()
	proofMust(t, err)
	aux, err := m.MkConst(usymbol.Numerical(2001), b)
	proofMust(t, err)
	other, err := m.MkConst(usymbol.Numerical(2002), b)
	proofMust(t, err)
	notAux, err := m.MkNot(aux)
	proofMust(t, err)
	clause, err := m.MkOr(notAux, other)
	proofMust(t, err)
	clauseProof, err := m.MkProofAsserted(clause)
	proofMust(t, err)
	clauseProof = proofPin(t, m, clauseProof)
	auxProof, err := m.MkProofAsserted(aux)
	proofMust(t, err)
	auxProof = proofPin(t, m, auxProof)
	root, err := m.MkProofUnitResolution([]ast.ASTHandle{clauseProof, auxProof})
	proofMust(t, err)
	root = proofPin(t, m, root)
	out, err := m.NewRef(ast.ASTHandle{})
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&out)) }()
	proofMust(t, EliminateAuxAssertions(m, aux, root, out))
	result, err := m.RefGet(out)
	proofMust(t, err)
	yes, err := m.MatchProofAsserted(result, &clause)
	proofMust(t, err)
	if !yes || clause != other {
		t.Fatal("collapsed unit resolution", yes, clause, other)
	}
}

func TestEliminateAuxAssertionsPreservesIffShape(t *testing.T) {
	m := proofManager(t)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	b, err := m.MkBoolSort()
	proofMust(t, err)
	aux, err := m.MkConst(usymbol.Numerical(2011), b)
	proofMust(t, err)
	other, err := m.MkConst(usymbol.Numerical(2012), b)
	proofMust(t, err)
	notAux, err := m.MkNot(aux)
	proofMust(t, err)
	notOther, err := m.MkNot(other)
	proofMust(t, err)
	fact, err := m.MkIff(notAux, notOther)
	proofMust(t, err)
	root, err := m.MkProofAsserted(fact)
	proofMust(t, err)
	root = proofPin(t, m, root)
	out, err := m.NewRef(ast.ASTHandle{})
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&out)) }()
	proofMust(t, EliminateAuxAssertions(m, aux, root, out))
	result, err := m.RefGet(out)
	proofMust(t, err)
	var got ast.ASTHandle
	yes, err := m.MatchProofAsserted(result, &got)
	proofMust(t, err)
	falsehood, err := m.MkFalse()
	proofMust(t, err)
	want, err := m.MkIff(falsehood, notOther)
	proofMust(t, err)
	if !yes || got != want {
		t.Fatal("raw IFF reconstruction", yes, got, want)
	}
}

func TestEliminateAuxAssertionsUsesDefaultBooleanReducer(t *testing.T) {
	m := proofManager(t)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	b, err := m.MkBoolSort()
	proofMust(t, err)
	aux, err := m.MkConst(usymbol.Numerical(2021), b)
	proofMust(t, err)
	other, err := m.MkConst(usymbol.Numerical(2022), b)
	proofMust(t, err)
	fact, err := m.MkAnd(aux, other)
	proofMust(t, err)
	root, err := m.MkProofAsserted(fact)
	proofMust(t, err)
	root = proofPin(t, m, root)
	out, err := m.NewRef(ast.ASTHandle{})
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&out)) }()
	proofMust(t, EliminateAuxAssertions(m, aux, root, out))
	result, err := m.RefGet(out)
	proofMust(t, err)
	var got ast.ASTHandle
	yes, err := m.MatchProofAsserted(result, &got)
	proofMust(t, err)
	if !yes || got != other {
		t.Fatal("default Boolean reduction", yes, got, other)
	}
}

// mk_or_core's SASSERT(j >= 1) is debug-only: in the release build an or
// whose disjuncts all became false rebuilds to args.get(0), i.e. false
// (AUDIT ast_sub_03-F5).
func TestEliminateAuxAllFalseDisjunction(t *testing.T) {
	if ast.ASTSourceDebug {
		t.Skip("the source-debug build asserts j >= 1")
	}
	m := proofManager(t)
	proofMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	b, err := m.MkBoolSort()
	proofMust(t, err)
	aux, err := m.MkConst(usymbol.Numerical(2021), b)
	proofMust(t, err)
	notAux, err := m.MkNot(aux)
	proofMust(t, err)
	fact, err := m.MkOrArgs([]ast.ASTHandle{notAux, notAux})
	proofMust(t, err)
	root, err := m.MkProofAsserted(fact)
	proofMust(t, err)
	root = proofPin(t, m, root)
	out, err := m.NewRef(ast.ASTHandle{})
	proofMust(t, err)
	defer func() { proofMust(t, m.DelRef(&out)) }()
	proofMust(t, EliminateAuxAssertions(m, aux, root, out))
	result, err := m.RefGet(out)
	proofMust(t, err)
	var got ast.ASTHandle
	yes, err := m.MatchProofAsserted(result, &got)
	proofMust(t, err)
	if !yes || !m.IsFalse(got) {
		t.Fatal("all-false disjunction must rebuild to false", yes, got)
	}
}
