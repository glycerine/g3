package rel

// Source: check_relation_plugin symbolic verification. The oracle depends on
// original column sorts and solver services, independently of relation storage.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

type relationFormulaChecker struct {
	manager *ast.ASTManager
	sorts   []ast.ASTHandle
	config  smtcore.SMTSolverConfig
}
