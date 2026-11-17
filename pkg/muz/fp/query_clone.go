package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp query/query_from_lvl local
// rule-set copies and src/muz/pdr/pdr_dl_interface.cpp query local rules.

import (
	"errors"

	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *PropositionalEngine) newQueryClone() (*PropositionalEngine, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.backend == "datalog" && e.hasExternalIntervalRepresentation() {
		return nil, ErrExternalIntervalDatalogFragment
	}
	if err := e.clearQueryCleanup(); err != nil {
		return nil, err
	}
	var clone *PropositionalEngine
	var err error
	if e.params != nil && e.globals != nil {
		clone, err = newPropositionalEngineWithServices(e.manager, e.params, e.globals, e.rulesShare)
	} else {
		clone, err = newPropositionalEngine(e.manager, e.rulesShare)
	}
	if err != nil {
		return nil, err
	}
	// PDR queries on a temporary rule-set clone still borrow the original
	// fixedpoint context's already configured source rewriter.
	clone.pdrTheory = e.pdrTheory
	if clone.pdrTheory == nil {
		clone.pdrTheory = e.guard
	}
	clone.backend = e.backend
	clone.engineSelectionSet = e.engineSelectionSet
	// The clone stands for the same datalog::context, so it shares m_mc.
	clone.contextModelConverter = e.contextModelConverter
	if e.backend == EngineTab.String() && e.tabSession != nil {
		clone.tabSession = e.tabSession
		clone.tabSessionBorrowed = true
	}
	if e.backend == "datalog" {
		if e.datalogExecTotals == nil {
			e.datalogExecTotals = &rel.ExecutionStats{}
		}
		clone.datalogExecTotals = e.datalogExecTotals
	}
	if e.bmcSlot != nil {
		clone.bmcSlot = e.bmcSlot
		clone.bmcSlotBorrowed = true
	}
	clone.datalogRelationFavourite = e.datalogRelationFavourite
	clone.datalogDefaultTable = e.datalogDefaultTable
	clone.softTimeout = e.softTimeout
	if clone.datalogDefaultTable != "" {
		if err = clone.configureDatalogFiniteDefault(clone.datalogDefaultTable); err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	if e.queryParamsReady {
		if err = clone.SetQueryParameters(&e.queryParams); err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
	}
	clone.ddnf.output = e.ddnf.output
	clone.dualityReport = e.dualityReport
	if e.ddnf.registry != nil {
		clone.ddnf.registry = e.ddnf.registry
		clone.ddnf.registryBorrowed = true
	}
	clone.forceProofTrace = e.forceProofTrace
	if e.backend == "datalog" && e.external != nil {
		clone.external, err = e.external.CloneForManager(clone.relations)
		if err != nil {
			return nil, errors.Join(err, e.discardQueryClone(clone))
		}
		if e.externalPluginRegistered() {
			// The clone's relation manager has the context's plugins.
			if err = clone.registerExternalRelationPlugin(); err != nil {
				return nil, errors.Join(err, e.discardQueryClone(clone))
			}
		}
	}
	if err = e.copyQueryPredicates(clone); err != nil {
		return nil, errors.Join(err, e.discardQueryClone(clone))
	}
	if err = e.copyQueryRules(clone); err != nil {
		return nil, errors.Join(err, e.discardQueryClone(clone))
	}
	if err = e.copyQueryBackground(clone); err != nil {
		return nil, errors.Join(err, e.discardQueryClone(clone))
	}
	return clone, nil
}

// contextRewriter is datalog::context's m_rewriter (ctx.get_rewriter()): a
// query clone borrows the original context's.
func (e *PropositionalEngine) contextRewriter() *astrewriter.TheoryRewriter {
	if e.pdrTheory != nil {
		return e.pdrTheory
	}
	return e.guard
}

// expressionQueryContext is the engine an expression query runs on: for
// datalog, the datalog context of the earlier expression queries when
// there is one (resynced with the owner), otherwise a new query clone.
func (e *PropositionalEngine) expressionQueryContext() (*PropositionalEngine, error) {
	if e.datalogContext != nil {
		if e.backend == "datalog" && e.datalogContext.backend == "datalog" {
			if err := e.clearQueryCleanup(); err != nil {
				return nil, err
			}
			clone := e.datalogContext
			if err := e.resyncDatalogContext(clone); err != nil {
				return clone, errors.Join(err, e.discardQueryClone(clone))
			}
			return clone, nil
		}
		previous := e.datalogContext
		e.datalogContext = nil
		if err := previous.Close(); err != nil {
			return nil, err
		}
	}
	return e.newQueryClone()
}

// retireDatalogQuery is ~scoped_query at the end of the previous
// rel_context::query(expr*) on the datalog context: restrict_predicates
// (m_preds) drops the query's auxiliary predicates and their relations,
// and replace_rules(m_rules) gives the context back the rules it had
// before mk_query (they are copied from the owner again by the next query).
// The compiled code and the execution registers stay until the next
// saturate resets them.
func (c *PropositionalEngine) retireDatalogQuery() error {
	if c.datalogQueryPreds != nil {
		set, err := base.NewRuleSet(c.rules)
		if err != nil {
			return err
		}
		scope := &datalogUnboundScope{engine: c, set: set, predicates: c.datalogQueryPreds}
		if err = scope.Close(); err != nil {
			return err
		}
		c.datalogQueryPreds = nil
	}
	if err := c.set.Reset(); err != nil {
		return err
	}
	c.queryReady = false
	return nil
}

// resyncDatalogContext brings the reused datalog context up to its owner:
// the facts, predicates and rules added since the previous query, and the
// current query parameters.
func (e *PropositionalEngine) resyncDatalogContext(clone *PropositionalEngine) error {
	clone.datalogRelationFavourite = e.datalogRelationFavourite
	clone.datalogDefaultTable = e.datalogDefaultTable
	clone.softTimeout = e.softTimeout
	clone.forceProofTrace = e.forceProofTrace
	clone.contextModelConverter = e.contextModelConverter
	if e.queryParamsReady {
		if err := clone.SetQueryParameters(&e.queryParams); err != nil {
			return err
		}
	}
	if err := e.copyQueryPredicates(clone); err != nil {
		return err
	}
	return e.copyQueryRules(clone)
}
