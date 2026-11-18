package model

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// AUDIT.md model_01-F8: set_v marks only m1, so the -v:35 "assigned" line
// reads is_false for numbers and values: "false", not "unknown".
func TestModelImplicantEvaluationTruthConcreteIsFalse(t *testing.T) {
	h := ast.ASTHandle{}
	i := &ModelImplicant{states: map[ast.ASTHandle]implicantValue{h: implicantConcrete}}
	if got := i.evaluationTruth(h); got != "false" {
		t.Fatalf("concrete term reads %q", got)
	}
	i.states[h] = implicantX
	if got := i.evaluationTruth(h); got != "unknown" {
		t.Fatalf("x term reads %q", got)
	}
}
