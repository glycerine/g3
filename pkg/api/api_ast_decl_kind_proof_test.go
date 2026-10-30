package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIPublicProofDeclarationKindTable(t *testing.T) {
	operations := []int32{
		ast.ASTPrUndef, ast.ASTPrTrue, ast.ASTPrAsserted, ast.ASTPrGoal,
		ast.ASTPrModusPonens, ast.ASTPrReflexivity, ast.ASTPrSymmetry,
		ast.ASTPrTransitivity, ast.ASTPrTransitivityStar, ast.ASTPrMonotonicity,
		ast.ASTPrQuantIntro, ast.ASTPrDistributivity, ast.ASTPrAndElim,
		ast.ASTPrNotOrElim, ast.ASTPrRewrite, ast.ASTPrRewriteStar,
		ast.ASTPrPullQuant, ast.ASTPrPushQuant, ast.ASTPrElimUnusedVars,
		ast.ASTPrDER, ast.ASTPrQuantInst, ast.ASTPrHypothesis, ast.ASTPrLemma,
		ast.ASTPrUnitResolution, ast.ASTPrIffTrue, ast.ASTPrIffFalse,
		ast.ASTPrCommutativity, ast.ASTPrDefAxiom, ast.ASTPrDefIntro,
		ast.ASTPrApplyDef, ast.ASTPrIffOEq, ast.ASTPrNNFPos, ast.ASTPrNNFNeg,
		ast.ASTPrSkolemize, ast.ASTPrModusPonensOEq, ast.ASTPrTheoryLemma,
		ast.ASTPrHyperResolve,
	}
	for index, operation := range operations {
		want := APIDeclarationProofUndefined + APIDeclarationKind(index)
		if got := apiProofDeclarationKind(operation); got != want {
			t.Fatalf("proof operation %d: got %d want %d", operation, got, want)
		}
	}
}

func TestAPIPublicUnknownBasicOperationIsInternal(t *testing.T) {
	if got := apiBasicDeclarationKind(ast.ASTLastBasicPR + 1); got != APIDeclarationInternal {
		t.Fatal(got)
	}
}
