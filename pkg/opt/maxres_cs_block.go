package opt

// Source: src/opt/maxres.cpp cs_max_resolve hard correction disjunction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) assertCorrectionBlock(correction []ast.ASTHandle) error {
	// fml = m.mk_or(cs); s().assert_expr(fml): fml is a local.
	fml := &maxResLocal{m: e.solver.manager}
	block, err := fml.set(e.solver.manager.MkOrArgs(correction))
	if err == nil {
		err = e.solver.solver().Assert(block)
	}
	return errors.Join(err, fml.release())
}
