package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp check_reset/updt_params.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
)

func (e *PropositionalEngine) closeSpacerForReplacement() (result error) {
	if e.spacer != nil {
		if err := e.spacer.Close(); err != nil {
			return err
		}
		e.spacer = nil
		e.spacerRevision = 0
	}
	e.spacerOriginalQuery = ast.ASTHandle{}
	e.spacerQuery = ast.ASTHandle{}
	if e.spacerRules != nil {
		result = errors.Join(result, e.spacerRules.Close())
		e.spacerRules = nil
	}
	return result
}

func (e *PropositionalEngine) shouldResetSpacerRules() (bool, error) {
	// The source comparison loop is skipped when no nonempty previous
	// rule set exists, preserving covers supplied before the first query.
	if e.spacer == nil || e.spacerRules == nil {
		return false, nil
	}
	count, err := e.spacerRules.Len()
	if err != nil || count == 0 {
		return false, err
	}
	// check_reset compares source rules before query preprocessing. Only
	// properties implied by rules that subsume every new rule may survive.
	covered, err := hornRuleSetSubsumes(e.spacerRules, e.set)
	return !covered, err
}

// spacerContext exposes the source dl_interface context to cover operations
// before the first query as well as to rule installation during queries.
func (e *PropositionalEngine) spacerContext() (_ *spacer.Context, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.backend != "spacer" {
		return nil, ErrPropositionalEngine
	}
	if e.spacer != nil {
		return e.spacer, nil
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	maximumContexts, err := parameters.UInt("pdr.max_num_contexts", 500)
	if err != nil {
		return nil, err
	}
	context, err := spacer.NewContext(e.manager, parameters.local, maximumContexts, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return nil, err
	}
	background, err := e.manager.RefGet(e.background)
	if err == nil {
		err = context.SetBackground(background)
	}
	if err != nil {
		return nil, errors.Join(err, context.Close())
	}
	e.spacer = context
	e.spacerRevision = e.revision
	return context, nil
}
