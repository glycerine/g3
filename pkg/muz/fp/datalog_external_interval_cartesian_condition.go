package fp

// Source: dl_compiler.cpp interpreted-tail conditions stay rooted in compiled
// instruction app_ref fields throughout execution_context and program reuse.

import "github.com/glycerine/g3/pkg/ast"

func (p *externalIntervalDatalogCartesianRule) pin(manager *ast.ASTManager) error {
	if p == nil {
		return nil
	}
	return p.condition.prepare(manager)
}

func (p *externalIntervalDatalogCartesianRule) close(manager *ast.ASTManager) error {
	if p == nil {
		return nil
	}
	return p.condition.close(manager)
}
