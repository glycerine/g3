package opt

// Source: src/opt/maxres.cpp max_resolve clause construction.

import "github.com/glycerine/g3/pkg/ast"

func maxResResolutionClauses(owner *maxSMTASTOwner, core []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if len(core) == 0 {
		return nil, errMaxResState
	}
	clauses := make([]ast.ASTHandle, 0, len(core)-1)
	conjunction := core[0]
	for i := 1; i < len(core); i++ {
		if i > 1 {
			var err error
			conjunction, err = owner.hold(owner.manager.MkAnd(core[i-1], conjunction))
			if err != nil {
				return nil, err
			}
		}
		clause, err := owner.hold(owner.manager.MkOr(core[i], conjunction))
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	return clauses, nil
}
