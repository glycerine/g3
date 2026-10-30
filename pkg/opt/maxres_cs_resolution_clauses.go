package opt

// Source: src/opt/maxres.cpp cs_max_resolve dual clauses.

import "github.com/glycerine/g3/pkg/ast"

func maxResCorrectionClauses(owner *maxSMTASTOwner, correction []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if len(correction) == 0 {
		return nil, errMaxResState
	}
	falsehood, err := owner.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	disjunction := falsehood
	clauses := make([]ast.ASTHandle, 0, len(correction)-1)
	for i := 1; i < len(correction); i++ {
		disjunction, err = owner.hold(owner.manager.MkOr(correction[i-1], disjunction))
		if err != nil {
			return nil, err
		}
		clause, err := owner.hold(owner.manager.MkAnd(correction[i], disjunction))
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	return clauses, nil
}
