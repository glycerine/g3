package clp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *Engine) Query(query ast.ASTHandle) (status lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	g, err := e.ground(query)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&g)) }()
	h, err := e.manager.RefGet(g)
	if err != nil {
		return lbool.Undef, err
	}
	status, err = e.search([]ast.ASTHandle{h}, e.depth, 0)
	if err == nil {
		truth, x := e.manager.MkTrue()
		if x != nil {
			return status, x
		}
		_ = e.manager.DelRef(&e.answer)
		e.answer, err = e.manager.NewRef(truth)
	}
	return status, err
}
