package fp

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_query and
// src/muz/spacer/spacer_dl_interface.cpp dl_interface::query.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// QueryExpression evaluates an arbitrary Boolean Horn query without adding
// its synthetic query predicate or rule to the caller's persistent program.
func (e *PropositionalEngine) QueryExpression(query ast.ASTHandle) (result lbool.LBool, resultErr error) {
	clone, err := e.newQueryClone()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, clone.Close()) }()
	declaration, head, err := clone.makeQueryHead(query)
	if err != nil {
		return lbool.Undef, err
	}
	if err = clone.RegisterPredicate(declaration); err != nil {
		return lbool.Undef, err
	}
	clone.auxiliary[declaration] = true
	rule, err := clone.manager.MkImplies(query, head)
	if err != nil {
		return lbool.Undef, err
	}
	if err = clone.AddRule(rule, ""); err != nil {
		return lbool.Undef, err
	}
	result, err = clone.QueryPredicate(declaration)
	e.recordQueryStatus(result, err)
	return result, err
}
