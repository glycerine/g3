package clp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (e *Engine) ground(h ast.ASTHandle) (out ast.ASTRef, resultErr error) {
	free, err := rewriter.NewExprFreeVars(e.manager)
	if err != nil {
		return out, err
	}
	if err = free.Collect(h); err != nil {
		return out, err
	}
	args := make([]ast.ASTHandle, free.Size())
	refs := make([]ast.ASTRef, free.Size())
	defer func() {
		for i := range refs {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&refs[i]))
		}
	}()
	for i := uint32(0); i < free.Size(); i++ {
		sort, x := free.Get(i)
		if x != nil {
			return out, x
		}
		if sort.IsNull() {
			continue
		}
		args[i], err = e.manager.MkFreshConst("c", sort)
		if err != nil {
			return out, err
		}
		refs[i], err = e.manager.NewRef(args[i])
		if err != nil {
			return out, err
		}
	}
	sub, err := rewriter.NewVarSubstitution(e.manager, false)
	if err != nil {
		return out, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	out, err = e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return out, err
	}
	if err = sub.Substitute(h, args, out); err != nil {
		resultErr = errors.Join(err, e.manager.DelRef(&out))
	}
	return out, resultErr
}
