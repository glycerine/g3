package fp

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_query and
// src/muz/spacer/spacer_dl_interface.cpp dl_interface::query.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/muz/spacer"
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
	// Every failure after a query begins invalidates the previous snapshot,
	// including normalization, clone construction, and deferred cleanup.
	defer func() {
		if resultErr != nil {
			e.recordQueryStatus(result, resultErr)
		}
	}()
	if err := e.clearQueryResources(); err != nil {
		return lbool.Undef, err
	}
	if e.backend == "duality" {
		e.queried = true
		result, err := e.queryDuality(query)
		e.recordQueryStatus(result, err)
		return result, err
	}
	hoisted, err := e.hoistQueryExistentials(query)
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: hoist query existentials: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&hoisted)) }()
	query, err = e.manager.RefGet(hoisted)
	if err != nil {
		return lbool.Undef, err
	}
	compacted, err := e.compactQueryVariables(query)
	if err != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: compact query variables: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&compacted)) }()
	query, err = e.manager.RefGet(compacted)
	if err != nil {
		return lbool.Undef, err
	}
	e.queried = true
	clone, err := e.newQueryClone()
	if err != nil {
		return lbool.Undef, err
	}
	var prequeryPDR *pdr.Context
	var prequerySpacer *spacer.Context
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
		if (e.backend == "bmc" || e.backend == "qbmc") && resultErr == nil {
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
			e.queryCloneRevision = e.revision
		} else {
			if prequeryPDR != nil && clone.pdr == prequeryPDR {
				initialized, stateErr := prequeryPDR.Initialized()
				resultErr = errors.Join(resultErr, stateErr)
				if stateErr == nil && !initialized {
					e.pdr = clone.pdr
					e.pdrRevision = e.revision
					clone.pdr = nil
					clone.pdrRevision = 0
				}
			}
			if prequerySpacer != nil && clone.spacer == prequerySpacer {
				initialized, stateErr := prequerySpacer.Initialized()
				resultErr = errors.Join(resultErr, stateErr)
				if stateErr == nil && !initialized {
					e.spacer = clone.spacer
					e.spacerRevision = e.revision
					clone.spacer = nil
					clone.spacerRevision = 0
				}
			}
			if clone.backend == "spacer" && clone.spacer != nil && clone.spacerRules != nil {
				e.queryClone = clone
				e.queryCloneRevision = e.revision
				return
			}
			if (clone.backend == "pdr" || clone.backend == "qpdr") && clone.pdr != nil && clone.pdrRules != nil {
				// Installed frames remain inspectable after a failed solve;
				// result access still requires the separate successful snapshot.
				e.queryClone = clone
				e.queryCloneRevision = e.revision
				return
			}
			resultErr = errors.Join(resultErr, clone.Close())
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
	// The source PDR context already exists when covers are supplied before
	// the first query. Move that owner after query preparation succeeds.
	if (e.backend == "pdr" || e.backend == "qpdr") && e.pdr != nil && e.pdrRules == nil {
		prequeryPDR = e.pdr
		clone.pdr = e.pdr
		clone.pdrRevision = clone.revision
		e.pdr = nil
		e.pdrRevision = 0
	} else if (e.backend == "pdr" || e.backend == "qpdr") && e.pdr != nil {
		if err := e.closePDRForReplacement(); err != nil {
			return lbool.Undef, err
		}
	}
	// Spacer properties likewise belong to the context that installs the
	// expression query's transformed rules.
	if e.backend == "spacer" && e.spacer != nil && e.spacerRules == nil {
		prequerySpacer = e.spacer
		clone.spacer = e.spacer
		clone.spacerRevision = clone.revision
		e.spacer = nil
		e.spacerRevision = 0
	} else if e.backend == "spacer" && e.spacer != nil {
		if err := e.closeSpacerForReplacement(); err != nil {
			return lbool.Undef, err
		}
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
	var archived pdr.StatisticsSnapshot
	var result error
	archivePDR := e.queryClone.backend == "pdr" || e.queryClone.backend == "qpdr"
	if archivePDR {
		archived, result = e.queryClone.snapshotPDRStatistics()
	}
	if err := e.queryClone.Close(); err != nil {
		return errors.Join(result, fmt.Errorf("muz/fp: close expression-query clone: %w", err))
	}
	if archivePDR && result == nil {
		e.pdrStatistics.Merge(archived)
	}
	e.queryClone = nil
	e.queryCloneRevision = 0
	e.queryClonePredicate = ast.ASTHandle{}
	return result
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
	if kind == EngineBMC || kind == EngineQBMC || kind == EngineTab {
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
