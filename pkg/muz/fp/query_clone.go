package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp query/query_from_lvl local
// rule-set copies and src/muz/pdr/pdr_dl_interface.cpp query local rules.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *PropositionalEngine) newQueryClone() (*PropositionalEngine, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	var clone *PropositionalEngine
	var err error
	if e.params != nil && e.globals != nil {
		clone, err = NewPropositionalEngineWithServices(e.manager, e.params, e.globals)
	} else {
		clone, err = NewPropositionalEngine(e.manager)
	}
	if err != nil {
		return nil, err
	}
	clone.backend = e.backend
	clone.forceProofTrace = e.forceProofTrace
	if err = e.copyQueryPredicates(clone); err != nil {
		return nil, errors.Join(err, clone.Close())
	}
	if err = e.copyQueryRules(clone); err != nil {
		return nil, errors.Join(err, clone.Close())
	}
	if err = e.copyQueryBackground(clone); err != nil {
		return nil, errors.Join(err, clone.Close())
	}
	return clone, nil
}

func closeASTRefs(manager *ast.ASTManager, references []ast.ASTRef) (result error) {
	for i := len(references) - 1; i >= 0; i-- {
		result = errors.Join(result, manager.DelRef(&references[i]))
	}
	return result
}
