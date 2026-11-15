package opt

// Source: src/opt/maxres.cpp process_sat.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (e *maxResEngine) processCorrection(correction []ast.ASTHandle) error {
	if len(correction) == 0 {
		e.stats.CorrectionSets++
		e.currentCorrection = nil
		return e.clearCorrectionModel()
	}
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
	if diagnostic.VerboseProgress {
		if err = optProgress(2, "(opt.maxres.correction-set "+strconv.Itoa(len(correction))+")\n"); err != nil {
			return err
		}
	}
	e.currentCorrection = nil
	e.stats.CorrectionSets++
	return e.clearCorrectionModel()
}
