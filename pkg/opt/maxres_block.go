package opt

// Source: src/opt/maxres.cpp process_unsat hard core block.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// assertCoreBlock is fml = mk_not(m, mk_and(m, core.size(), core.c_ptr()));
// s().assert_expr(fml): fml is process_unsat's local, so only the solver
// keeps the block, and the conjunction only through it. The ast_util helpers
// return a singleton core's literal and strip a leading negation.
func (e *maxResEngine) assertCoreBlock(core []ast.ASTHandle) (resultErr error) {
	m := e.solver.manager
	fml := &maxResLocal{m: m}
	defer func() { resultErr = errors.Join(resultErr, fml.release()) }()
	conjunction, err := m.MkAndArray(core)
	if err != nil {
		return err
	}
	block, err := fml.set(m.MkNotSimplified(conjunction))
	if err != nil {
		return err
	}
	return e.solver.solver().Assert(block)
}
