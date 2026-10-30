package opt

// Source: src/opt/maxres.cpp process_sat.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) processCorrection(correction []ast.ASTHandle) error {
	if !e.validCorrection(correction) {
		return errMaxResState
	}
	expected, err := e.correctionWeight(correction)
	if err != nil {
		return err
	}
	weight, err := e.splitCore(correction)
	if err != nil {
		return err
	}
	if weight.Cmp(expected) != 0 {
		return errMaxResState
	}
	if err = e.correctionResolve(correction, weight); err != nil {
		return err
	}
	if err = e.assertCorrectionBlock(correction); err != nil {
		return err
	}
	e.correction = nil
	e.stats.CorrectionSets++
	return nil
}
