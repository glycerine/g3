package opt

// Source: src/opt/maxres.cpp upper-bound block gating.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResUpperBoundUsesOnlySourceParameterGate(t *testing.T) {
	engine := maxResEngine{config: maxSMTConfig{addUpperBoundBlock: true}, objective: &arithmeticObjective{softTerms: make([]ast.ASTRef, 1)}}
	if !engine.upperBoundEligible() {
		t.Fatal("source upper block was refused before search progress")
	}
	engine.config.addUpperBoundBlock = false
	if engine.upperBoundEligible() {
		t.Fatal("disabled source upper block")
	}
	engine.config.addUpperBoundBlock = true
	engine.objective.softTerms = nil
	if engine.upperBoundEligible() {
		t.Fatal("upper block without soft terms")
	}
}
