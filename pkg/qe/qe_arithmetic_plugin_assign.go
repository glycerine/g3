package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, value *numeral.OwnedRational) (resultErr error) {
	if value == nil {
		return ast.ErrASTHandle
	}
	if err := p.open(); err != nil {
		return err
	}
	e, target, err := p.entry(variable, formula)
	if err != nil {
		return err
	}
	branchNumber, err := p.manager.NumeralService().Unsigned(value)
	if err != nil {
		return err
	}
	key := ArithmeticBranchKey{Formula: formula, Variable: target, Branch: branchNumber}
	if cached := p.branches[key]; cached != nil {
		return p.replayBranch(cached)
	}
	freshStart := len(p.pendingFresh)
	v := new(big.Rat).SetInt64(int64(branchNumber))
	selection, err := p.SelectBranch(variable, formula, v)
	if err != nil {
		return err
	}
	result, err := p.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&result))
		}
	}()
	substitution, _, err := p.applyCooper(target, e.processor, result)
	if err != nil {
		return err
	}
	if selection.Kind == ArithmeticInfinityBranch {
		err = p.rewriteInfinity(e, selection.Lower, result)
	} else {
		err = p.rewriteFinite(e, target, selection, result)
	}
	if err == nil {
		// m_util.simplify(result) before add_cache.
		err = p.simplify(result)
	}
	if err != nil {
		return err
	}
	// add_cache keeps x_t's coefficient and term; the definition is built
	// only when subst asks for it (get_def).
	return p.storeBranch(key, result, substitution, selection, p.pendingFresh[freshStart:])
}
