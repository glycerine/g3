package clp

// Source: src/muz/clp/clp_context.cpp, reset_ground and ground.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// groundEnvironment owns the rule-local fresh constants shared by the head
// and every tail. Each attempted rule receives a new environment.
type groundEnvironment struct {
	manager *ast.ASTManager
	values  []ast.ASTHandle
	refs    []ast.ASTRef
}

func (g *groundEnvironment) close() (result error) {
	for i := len(g.refs) - 1; i >= 0; i-- {
		if g.refs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, g.manager.DelRef(&g.refs[i]))
		}
	}
	g.values, g.refs = nil, nil
	return result
}

func (g *groundEnvironment) ground(h ast.ASTHandle) (out ast.ASTRef, resultErr error) {
	free, err := rewriter.NewExprFreeVars(g.manager)
	if err != nil {
		return out, err
	}
	if err = free.Collect(h); err != nil {
		return out, err
	}
	if int(free.Size()) > len(g.values) {
		count := int(free.Size()) - len(g.values)
		g.values = append(g.values, make([]ast.ASTHandle, count)...)
		g.refs = append(g.refs, make([]ast.ASTRef, count)...)
	}
	for i := uint32(0); i < free.Size(); i++ {
		sort, err := free.Get(i)
		if err != nil {
			return out, err
		}
		if sort.IsNull() {
			continue
		}
		if !g.values[i].IsNull() {
			oldSort, err := g.manager.GetSort(g.values[i])
			if err != nil {
				return out, err
			}
			if oldSort != sort {
				return out, ast.ErrASTDomain
			}
			continue
		}
		value, err := g.manager.MkFreshConst("c", sort)
		if err != nil {
			return out, err
		}
		ref, err := g.manager.NewRef(value)
		if err != nil {
			return out, err
		}
		g.values[i], g.refs[i] = value, ref
	}
	sub, err := rewriter.NewVarSubstitution(g.manager, false)
	if err != nil {
		return out, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	out, err = g.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return out, err
	}
	if err = sub.Substitute(h, g.values, out); err != nil {
		resultErr = errors.Join(err, g.manager.DelRef(&out))
	}
	return out, resultErr
}

func (e *Engine) ground(h ast.ASTHandle) (out ast.ASTRef, resultErr error) {
	environment := &groundEnvironment{manager: e.manager}
	defer func() { resultErr = errors.Join(resultErr, environment.close()) }()
	return environment.ground(h)
}
