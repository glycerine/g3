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
func (e *PropositionalEngine) QueryExpression(query ast.ASTHandle) (lbool.LBool, error) {
	return e.queryExpression(query, nil)
}

// QueryExpressionFromLevel runs a general Spacer expression query beginning
// at the requested proof level without changing the persistent rule set.
func (e *PropositionalEngine) QueryExpressionFromLevel(query ast.ASTHandle, level uint32) (lbool.LBool, error) {
	return e.queryExpression(query, &level)
}

func (e *PropositionalEngine) queryExpression(query ast.ASTHandle, level *uint32) (result lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if level != nil && e.backend != "spacer" {
		return lbool.Undef, ErrPropositionalEngine
	}
	if err := e.clearQueryResources(); err != nil {
		return lbool.Undef, err
	}
	e.queried = true
	clone, err := e.newQueryClone()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		if e.backend == "tab" {
			e.tabUnfold += clone.tabUnfold
			e.tabUnfoldFail += clone.tabUnfoldFail
			e.tabSubsumed += clone.tabSubsumed
		}
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
		if resultErr == nil {
			e.queryClone = clone
		} else {
			resultErr = errors.Join(resultErr, clone.Close())
		}
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
	if level != nil {
		result, err = clone.QueryPredicateFromLevel(declaration, *level)
	} else if clone.backend == "ddnf" {
		result, err = clone.queryDDNF()
	} else {
		result, err = clone.QueryPredicate(declaration)
	}
	if err == nil {
		e.queryClonePredicate = declaration
	}
	e.recordQueryStatus(result, err)
	return result, err
}

// clearQueryResources releases the backend state used to answer the previous
// expression query. It is also called when the engine itself is closed.
func (e *PropositionalEngine) clearQueryResources() error {
	e.queryRelations = nil
	e.queryRelationsSet = false
	if e.queryClone == nil {
		return nil
	}
	if err := e.queryClone.Close(); err != nil {
		return err
	}
	e.queryClone = nil
	e.queryClonePredicate = ast.ASTHandle{}
	return nil
}

// ExpressionAnswer returns the backend answer to the most recent expression
// query while its query snapshot is valid. The caller owns the returned ref.
func (e *PropositionalEngine) expressionQueryState() (*PropositionalEngine, EngineKind, error) {
	if err := e.open(); err != nil {
		return nil, EngineDatalog, err
	}
	kind, err := parseEngineKind(e.backend)
	if err != nil || !e.queryReady || e.queryRevision != e.revision || e.queryEngine != kind || e.queryClone == nil {
		return nil, kind, ErrPropositionalEngine
	}
	return e.queryClone, kind, nil
}

func (e *PropositionalEngine) ExpressionAnswer() (ast.ASTRef, error) {
	clone, kind, err := e.expressionQueryState()
	if err != nil {
		return ast.ASTRef{}, err
	}
	// BMC and tabulation transfer their proof/answer to the parent during query
	// cleanup; their answer methods do not inspect the predicate argument.
	if kind == EngineBMC || kind == EngineTab {
		return e.Answer(e.queryClonePredicate)
	}
	return clone.Answer(e.queryClonePredicate)
}

// ExpressionGroundSatAnswer returns the Spacer ground answer for the latest
// expression query. The caller owns the returned reference.
func (e *PropositionalEngine) ExpressionGroundSatAnswer() (ast.ASTRef, error) {
	clone, kind, err := e.expressionQueryState()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if kind != EngineSpacer {
		return ast.ASTRef{}, ErrPropositionalEngine
	}
	return clone.GroundSatAnswer(e.queryClonePredicate)
}
