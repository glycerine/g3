package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// addFormula applies mk_horn_rule's option gate before unbound-variable
// reduction. Source dl_rule.cpp:218 skips the entire rewrite when disabled.
func (e *PropositionalEngine) addFormula(set *base.RuleSet, formula ast.ASTHandle, name symbol.Symbol) error {
	if e.params == nil || e.globals == nil {
		return e.rules.AddFormulaWithSymbol(set, formula, name)
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
	if !fixUnbound {
		return e.rules.AddFormulaWithSymbol(set, formula, name)
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
	return e.rules.AddFormulaWithSymbolAndUnboundFix(set, formula, name, e.unboundLite, true)
}
