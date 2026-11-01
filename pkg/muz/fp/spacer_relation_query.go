package fp

// Source: src/muz/base/dl_engine_base.h engine_base::query and
// src/muz/base/dl_rule.cpp rule_manager::mk_query.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// makeSpacerRelationQuery creates a local synthetic rule without changing the
// persistent program. Its predicate remains registered for model filtering.
func (e *PropositionalEngine) makeSpacerRelationQuery(declaration ast.ASTHandle) (ast.ASTHandle, *base.Rule, error) {
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	arguments := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		sort, err := e.manager.GetDomain(declaration, i)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		arguments[i], err = e.manager.MkVar(i, sort)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
	}
	query, err := e.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	queryDeclaration, head, err := e.makeQueryHead(query)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if err = e.rules.RegisterPredicate(queryDeclaration); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	e.auxiliary[queryDeclaration] = true
	rule, err := e.rules.NewRule(head, []ast.ASTHandle{query}, nil, "")
	return queryDeclaration, rule, err
}
