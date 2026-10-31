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
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	e.queried = true
	clone, err := e.newQueryClone()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		if e.backend == "tab" && resultErr == nil {
			if err := e.clearTabAnswer(); err != nil {
				resultErr = errors.Join(resultErr, err)
			} else {
				e.tabAnswer = clone.tabAnswer
				clone.tabAnswer = ast.ASTRef{}
			}
		}
		if e.backend == "bmc" && resultErr == nil {
			if err := e.clearBMCTrace(); err != nil {
				resultErr = errors.Join(resultErr, err)
			} else {
				e.bmcTrace = clone.bmcTrace
				clone.bmcTrace = nil
				e.bmcProof = clone.bmcProof
				clone.bmcProof = ast.ASTRef{}
				e.bmcStatistics.Reset()
				e.bmcStatistics.Copy(clone.bmcStatistics)
			}
		}
		if e.backend == "ddnf" && e.ddnf.registry == nil && clone.ddnf.registry != nil {
			e.ddnf.registry = clone.ddnf.registry
			clone.ddnf.registry = nil
			clone.ddnf.registryBorrowed = false
		}
		if e.backend == "ddnf" {
			clone.ddnf.transferTrail(&e.ddnf)
		}
		resultErr = errors.Join(resultErr, clone.Close())
		if resultErr != nil {
			e.recordQueryStatus(result, resultErr)
		}
	}()
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
	if err = clone.applyQueryMagicSets(declaration); err != nil {
		return lbool.Undef, err
	}
	if clone.backend == "ddnf" {
		result, err = clone.queryDDNF()
	} else {
		result, err = clone.QueryPredicate(declaration)
	}
	e.recordQueryStatus(result, err)
	return result, err
}
