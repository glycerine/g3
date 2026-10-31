package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, operator() preparation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
)

// nlPurification owns the preparation state needed by the cooperating
// nonlinear and SMT solvers. The input goal is borrowed and updated in place.
type nlPurification struct {
	nonlinear  *tactic.Goal
	interfaces *nlInterfaceRewriter
	predicates *nlPredicateRewriter
	extraction *solver.ClauseExtraction
}

func prepareNLPurification(g *tactic.Goal) (result *nlPurification, resultErr error) {
	if g == nil {
		return nil, ast.ErrASTHandle
	}
	if err := g.CheckOwner(); err != nil {
		return nil, err
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	if proofs {
		return nil, tactic.NewTacticException("qfufnra-purify does not support proof generation")
	}
	result = &nlPurification{}
	state := result
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, state.Close())
			result = nil
		}
	}()
	m := g.Manager()
	result.nonlinear, err = tactic.NewGoal(m, false, true, false)
	if err != nil {
		return nil, err
	}
	result.interfaces, err = newNLInterfaceRewriter(m)
	if err != nil {
		return nil, err
	}
	if err = result.interfaces.rewriteGoal(g, result.nonlinear); err != nil {
		return nil, err
	}
	cores, err := g.UnsatCoresEnabled()
	if err != nil {
		return nil, err
	}
	if !cores {
		if err = removePureArithmetic(g, result.nonlinear); err != nil {
			return nil, err
		}
	}
	count, err := g.Size()
	if err != nil {
		return nil, err
	}
	roots := make([]ast.ASTHandle, count)
	for i := range roots {
		roots[i], err = g.Form(uint32(i))
		if err != nil {
			return nil, err
		}
	}
	polarities, err := purificationPolarities(m, roots)
	if err != nil {
		return nil, err
	}
	result.predicates, err = newNLPredicateRewriter(m, result.nonlinear, polarities)
	if err != nil {
		return nil, err
	}
	if err = result.predicates.rewriteGoal(g); err != nil {
		return nil, err
	}
	result.extraction, err = solver.ExtractClausesAndDependencies(g)
	if err != nil {
		return nil, err
	}
	if err = g.IncDepth(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *nlPurification) Close() (result error) {
	if p == nil {
		return nil
	}
	if p.extraction != nil {
		result = errors.Join(result, p.extraction.Close())
		p.extraction = nil
	}
	if p.predicates != nil {
		result = errors.Join(result, p.predicates.Close())
		p.predicates = nil
	}
	if p.interfaces != nil {
		result = errors.Join(result, p.interfaces.Close())
		p.interfaces = nil
	}
	if p.nonlinear != nil {
		result = errors.Join(result, p.nonlinear.Close())
		p.nonlinear = nil
	}
	return result
}
