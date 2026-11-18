package opt

// Source: src/opt/maxres.cpp process_sat correction-set state guards.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResCorrectionStateRequiresDistinctPositiveActiveWeights(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	p := maxResTestBool(t, manager, "maxres-cs-state-p")
	q := maxResTestBool(t, manager, "maxres-cs-state-q")
	engine := &maxResEngine{lower: testRat(0, 1), upper: testRat(0, 1), solver: &ArithmeticSolver{manager: newWeightedSoftFixture(t).manager}, weights: map[ast.ASTHandle]*numeral.Rational{
		p: testRatPtr(5, 2),
		q: testRatPtr(3, 2),
	}}
	if !engine.validCorrection([]ast.ASTHandle{p, q}) {
		t.Fatal("valid correction rejected")
	}
	weight, err := engine.correctionWeight([]ast.ASTHandle{p, q})
	if err != nil || !ratIs(&weight, 3, 2) {
		t.Fatal("minimum correction weight", weight, err)
	}
	if engine.validCorrection([]ast.ASTHandle{p, p}) {
		t.Fatal("duplicate correction accepted")
	}
	engine.weights[q] = testRatPtr(0, 1)
	if engine.validCorrection([]ast.ASTHandle{p, q}) {
		t.Fatal("zero correction weight accepted")
	}
	if _, err = engine.correctionWeight([]ast.ASTHandle{p, q}); !errors.Is(err, errMaxResState) {
		t.Fatal("zero correction weight", err)
	}
	if err = engine.processCorrection([]ast.ASTHandle{p, q}); !errors.Is(err, errMaxResState) {
		t.Fatal("invalid correction processing", err)
	}
}
