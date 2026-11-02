package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, nonlinear::bind_vars.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// BindVariables universally binds an expression's free variables and uses the
// corresponding head application as its single matching pattern.
func (c *Compiler) BindVariables(expression, head ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	free, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return result, err
	}
	if err = free.Collect(expression); err != nil {
		return result, err
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	bindings := make([]ast.ASTHandle, free.Size())
	var sorts []ast.ASTHandle
	var names []symbol.Symbol
	for i := uint32(0); i < free.Size(); i++ {
		sort, err := free.Get(i)
		if err != nil {
			return result, err
		}
		if sort.IsNull() {
			continue
		}
		variable, err := m.MkVar(uint32(len(sorts)), sort)
		if err != nil {
			return result, err
		}
		ref, err := m.NewRef(variable)
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
		bindings[i] = variable
		sorts = append(sorts, sort)
		names = append(names, symbol.Numerical(i))
	}
	if len(sorts) == 0 {
		return m.NewRef(expression)
	}
	for i, j := 0, len(sorts)-1; i < j; i, j = i+1, j-1 {
		sorts[i], sorts[j] = sorts[j], sorts[i]
	}
	sub, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	bodyRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	refs = append(refs, bodyRef)
	if err = sub.Substitute(expression, bindings, bodyRef); err != nil {
		return result, err
	}
	body, err := m.RefGet(bodyRef)
	if err != nil {
		return result, err
	}
	headRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	refs = append(refs, headRef)
	if err = sub.Substitute(head, bindings, headRef); err != nil {
		return result, err
	}
	patternHead, err := m.RefGet(headRef)
	if err != nil {
		return result, err
	}
	pattern, err := m.MkPattern([]ast.ASTHandle{patternHead})
	if err != nil {
		return result, err
	}
	patternRef, err := m.NewRef(pattern)
	if err != nil {
		return result, err
	}
	refs = append(refs, patternRef)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body, Weight: 1, Patterns: []ast.ASTHandle{pattern}})
	if err != nil {
		return result, err
	}
	return m.NewRef(quantifier)
}
