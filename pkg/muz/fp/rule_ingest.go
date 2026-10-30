package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

// addFormula applies the fixedpoint rule-ingestion option and the source's
// unconditional QE-lite reduction for unbound interpreted tail variables.
func (e *PropositionalEngine) addFormula(set *base.RuleSet, formula ast.ASTHandle, name string) error {
	if e.params == nil || e.globals == nil {
		return e.rules.AddFormula(set, formula, name)
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return err
	}
	fixUnbound, getErr := parameters.Bool("xform.fix_unbound_vars", false)
	err = errors.Join(getErr, parameters.Close())
	if err != nil {
		return err
	}
	if e.unboundLite == nil {
		cooperation, createErr := cooperate.NewCooperation(e.manager.Budget())
		if createErr != nil {
			return createErr
		}
		lite, createErr := qe.NewLite(e.manager, nil, false, qe.Config{Context: e.params, Globals: e.globals, Cooperation: cooperation})
		if createErr != nil {
			if lite != nil {
				createErr = errors.Join(createErr, lite.Close())
			}
			return errors.Join(createErr, cooperation.Close())
		}
		e.unboundCooperation = cooperation
		e.unboundLite = lite
	}
	return e.rules.AddFormulaWithUnboundFix(set, formula, name, e.unboundLite, fixUnbound)
}
