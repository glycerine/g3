package fp

// Source: rel_context::set_predicate_representation and rule-free query
// dispatch for an external_relation plus interval_relation product.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) hasExternalIntervalRepresentation() bool {
	for _, state := range e.predicates {
		if state != nil && isExternalIntervalRepresentation(state.requestedRepresentations) {
			return true
		}
	}
	return false
}

func (e *PropositionalEngine) externalIntervalPredicates() ([]externalIntervalDatalogPredicate, bool, error) {
	selected := e.hasExternalIntervalRepresentation()
	if !selected {
		return nil, false, nil
	}
	predicates := make([]externalIntervalDatalogPredicate, 0, len(e.predicateOrder))
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil {
			return nil, true, ErrExternalIntervalDatalogFragment
		}
		if e.auxiliary[declaration] || !isExternalIntervalRepresentation(state.requestedRepresentations) {
			continue
		}
		if state.external != nil || state.explicitFiniteRepresentation || !rel.CanHandleIntervalSorts(e.manager, state.sorts) {
			return nil, true, ErrExternalIntervalDatalogFragment
		}
		if state.table != nil {
			count, err := state.table.Len()
			if err != nil {
				return nil, true, err
			}
			if count != 0 {
				return nil, true, ErrExternalIntervalDatalogFragment
			}
		}
		predicates = append(predicates, externalIntervalDatalogPredicate{
			declaration: declaration,
			sorts:       append([]ast.ASTHandle(nil), state.sorts...),
			kinds:       append([]string(nil), state.requestedRepresentations...),
		})
	}
	return predicates, true, nil
}

func (e *PropositionalEngine) ensureExternalIntervalDatalog() (bool, error) {
	predicates, selected, err := e.externalIntervalPredicates()
	if err != nil || !selected {
		return selected, err
	}
	if e.external == nil || len(predicates) == 0 {
		return true, ErrExternalIntervalDatalogFragment
	}
	count, err := e.set.Len()
	if err != nil {
		return true, err
	}
	if count != 0 {
		return true, ErrExternalIntervalDatalogFragment
	}
	if e.udocDatalog != nil || e.sparseIntervalDatalog != nil || e.intervalDatalog != nil || e.boundDatalog != nil {
		return true, ErrExternalIntervalDatalogFragment
	}
	if enabled, cause := e.datalogExplanationsEnabled(); cause != nil || enabled {
		return true, errors.Join(cause, ErrExternalIntervalDatalogFragment)
	}
	if _, enabled, cause := e.datalogCheckRelationName(); cause != nil || enabled {
		return true, errors.Join(cause, ErrExternalIntervalDatalogFragment)
	}
	if _, enabled, cause := e.datalogDefaultTableCheck(); cause != nil || enabled {
		return true, errors.Join(cause, ErrExternalIntervalDatalogFragment)
	}
	if e.params != nil && e.globals != nil {
		view, cause := e.fixedpointParamView()
		if cause != nil {
			return true, cause
		}
		magic, cause := view.Bool("datalog.magic_sets_for_queries", false)
		cause = errors.Join(cause, view.Close())
		if cause != nil || magic {
			return true, errors.Join(cause, ErrExternalIntervalDatalogFragment)
		}
	}
	if e.externalIntervalDatalog != nil {
		return true, e.externalIntervalDatalog.syncPredicates(predicates)
	}
	owner, err := newExternalIntervalDatalogActivation(e.manager, e.external, predicates)
	if err != nil {
		return true, err
	}
	e.externalIntervalDatalog = owner
	return true, nil
}

func (e *PropositionalEngine) queryExternalIntervalDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	e.compiled = false
	e.datalogStats.Reset()
	return e.externalIntervalDatalog.query(declarations)
}

func (e *PropositionalEngine) closeExternalIntervalDatalog() error {
	if e.externalIntervalDatalog == nil {
		return nil
	}
	if err := e.externalIntervalDatalog.close(); err != nil {
		return err
	}
	e.externalIntervalDatalog = nil
	return nil
}
