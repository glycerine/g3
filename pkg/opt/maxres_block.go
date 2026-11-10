package opt

// Source: src/opt/maxres.cpp process_unsat hard core block.

import "github.com/glycerine/g3/pkg/ast"

// assertCoreBlock is fml = mk_not(m, mk_and(m, core.size(), core.c_ptr())):
// the ast_util helpers return a singleton core's literal and strip a
// leading negation.
func (e *maxResEngine) assertCoreBlock(core []ast.ASTHandle) error {
	conjunction, err := e.owner.hold(e.solver.manager.MkAndArray(core))
	if err != nil {
		return err
	}
	block, err := e.owner.hold(e.solver.manager.MkNotSimplified(conjunction))
	if err != nil {
		return err
	}
	return e.solver.solver().Assert(block)
}
