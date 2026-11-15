package qe

// Source: src/muz/spacer/spacer_qe_project.cpp array_project.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type legacyArrayProjectionResult struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

func (r *legacyArrayProjectionResult) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	// Refuse cleanup before discarding any roots when manager entry is pending.
	if _, err := r.manager.ProofMode(); err != nil {
		return err
	}
	for len(r.variables) != 0 {
		index := len(r.variables) - 1
		if err := r.manager.DelRef(&r.variables[index]); err != nil {
			return err
		}
		r.variables = r.variables[:index]
	}
	if r.formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.formula); err != nil {
			return err
		}
	}
	r.manager = nil
	return result
}

func (p *legacyArrayProjection) projectFormula(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAllSelects bool) (result *legacyArrayProjectionResult, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, ast.ErrASTHandle
	}
	result = &legacyArrayProjectionResult{manager: p.syntax.manager}
	ownedResult := result
	current, err := p.syntax.manager.NewRef(formula)
	if err != nil {
		_ = ownedResult.Close()
		return nil, err
	}
	defer func() {
		resultErr = finishLegacyArrayFormula(p.syntax.manager, current, ownedResult, resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	// 1. project array equalities
	remaining, err := p.projectEqualities(md, variables, current, result)
	if err != nil {
		return nil, fmt.Errorf("array equalities: %w", err)
	}
	currentHandle, err := p.syntax.manager.RefGet(current)
	if err != nil {
		return nil, err
	}
	reduced, err := p.reduceSelects(md, remaining, currentHandle, reduceAllSelects)
	if err != nil {
		return nil, fmt.Errorf("select reduction: %w", err)
	}
	next, err := p.syntax.manager.RefGet(reduced.formula)
	if err == nil {
		err = p.syntax.manager.SetRef(current, next)
	}
	err = closeFailedLegacyArrayRewrite(reduced.manager, reduced.formula, err)
	if err != nil {
		return nil, err
	}
	currentHandle, err = p.syntax.manager.RefGet(current)
	if err != nil {
		return nil, err
	}
	selects, err := p.projectSelects(md, remaining, currentHandle)
	if err != nil {
		return nil, fmt.Errorf("select projection: %w", err)
	}
	next, err = p.syntax.manager.RefGet(selects.formula)
	if err == nil {
		result.formula, err = p.syntax.manager.NewRef(next)
	}
	for _, auxiliary := range selects.variables {
		if err != nil {
			break
		}
		handle, e := p.syntax.manager.RefGet(auxiliary)
		if e == nil {
			var copy ast.ASTRef
			copy, e = p.syntax.manager.NewRef(handle)
			if e == nil {
				result.variables = append(result.variables, copy)
			}
		}
		err = e
	}
	err = closeTransferredLegacyArraySelect(selects, err)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// projectEqualities is array_project_eqs: it updates current, appends the
// auxiliary variables to result and returns the remaining array variables.
func (p *legacyArrayProjection) projectEqualities(md *model.ModelCore, variables []ast.ASTHandle, current ast.ASTRef, result *legacyArrayProjectionResult) (remaining []ast.ASTHandle, resultErr error) {
	ap, err := newLegacyArrayProjectEqs(p)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = closeFailedLegacyArrayProjectEqs(ap, resultErr) }()
	remaining, auxiliary, err := ap.run(md, variables, current)
	if err != nil {
		return nil, err
	}
	for _, variable := range auxiliary {
		copy, err := p.syntax.manager.NewRef(variable)
		if err != nil {
			return nil, err
		}
		result.variables = append(result.variables, copy)
	}
	return remaining, nil
}
