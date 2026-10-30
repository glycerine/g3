package opt

// Source: src/opt/maxres.cpp upper-bound block gating.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResUpperBoundRequiresProgressAndSmallCorrection(t *testing.T) {
	engine := maxResEngine{config: maxSMTConfig{addUpperBoundBlock: true, maxCorrectionSetSize: 3}, objective: &arithmeticObjective{softTerms: make([]ast.ASTRef, 1)}}
	if engine.upperBoundEligible(0) {
		t.Fatal("upper block before core progress")
	}
	engine.stats.Cores = 1
	if !engine.upperBoundEligible(maxResUpperBlockCorrectionLimit) {
		t.Fatal("eligible upper block refused")
	}
	if engine.upperBoundEligible(maxResUpperBlockCorrectionLimit + 1) {
		t.Fatal("oversized correction upper block")
	}
	if engine.upperBoundEligible(-1) {
		t.Fatal("negative correction size")
	}
	engine.config.addUpperBoundBlock = false
	if engine.upperBoundEligible(0) {
		t.Fatal("disabled upper block")
	}
	engine.config.addUpperBoundBlock = true
	engine.objective.softTerms = nil
	if engine.upperBoundEligible(0) {
		t.Fatal("upper block without soft terms")
	}
}
