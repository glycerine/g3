package opt

// Source: src/opt/maxres.cpp process_sat.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (e *maxResEngine) processCorrection(correction []ast.ASTHandle) (err error) {
	if len(correction) == 0 {
		e.stats.CorrectionSets++
		e.currentCorrection = nil
		return e.clearCorrectionModel()
	}
	if !e.validCorrection(correction) {
		return errMaxResState
	}
	expected, err := e.correctionWeight(correction)
	defer func() { err = errors.Join(err, e.q().Del(&expected)) }()
	if err != nil {
		return err
	}
	weight, err := e.splitCore(correction)
	defer func() { err = errors.Join(err, e.q().Del(&weight)) }()
	if err != nil {
		return err
	}
	if e.cmp(&weight, &expected) != 0 {
		return errors.Join(errMaxResState, e.ratErr)
	}
	if err = e.correctionResolve(correction, &weight); err != nil {
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
