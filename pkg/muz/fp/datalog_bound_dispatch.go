package fp

// Source: dl_relation_manager.cpp mk_empty_relation first tries a favourite,
// then finite table conversion, then registered relation plugins. The bound
// plugin precedes interval in rel_context.cpp registration order.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) ensureBoundDatalog() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	if len(e.predicateOrder) == 0 {
		return false, nil
	}
	explicit := e.datalogRelationFavourite == "bound_relation"
	if !explicit && e.datalogRelationFavourite != "pentagon" && e.datalogRelationFavourite != "doc" {
		return false, nil
	}
	if !explicit {
		arithmetic, finite := false, false
		for _, d := range e.predicateOrder {
			if e.auxiliary[d] {
				continue
			}
			state := e.predicates[d]
			if len(state.sorts) > 0 && rel.CanHandleIntervalSorts(e.manager, state.sorts) {
				arithmetic = true
			} else {
				finite = true
			}
		}
		// Original finite predicates keep their table semantics. A synthetic
		// query predicate instead inherits its arithmetic producer's relation kind.
		// relation_manager::mk_empty_relation chooses a plugin per
		// signature: finite signatures get tables, so any finite predicate
		// sends the query to the finite-table path, where rule-free
		// arithmetic predicates read as empty relations.
		if !arithmetic || finite {
			return false, nil
		}
	}
	for _, state := range e.predicates {
		if state.explicitFiniteRepresentation || state.external != nil {
			return true, ErrBoundDatalogFragment
		}
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return true, errors.Join(err, ErrBoundDatalogFragment)
	}
	// check_relation over bound_relation runs on the relation plugins.
	_, checkEnabled, err := e.datalogCheckRelationName()
	if err != nil || checkEnabled {
		return true, errors.Join(err, ErrBoundDatalogFragment)
	}
	var formulaCheck *smtcore.SMTSolverConfig
	if checkEnabled {
		formulaCheck = &smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrBoundDatalogFragment)
	}
	// rel_context::query(expr*) runs mk_magic_sets; rel_query does not.
	if e.expressionClone && e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		magic, err := view.Bool("datalog.magic_sets_for_queries", false)
		err = errors.Join(err, view.Close())
		if err != nil || magic {
			return true, errors.Join(err, ErrBoundDatalogFragment)
		}
	}
	if e.boundDatalog != nil {
		e.boundDatalog.formulaCheck = formulaCheck
		return true, e.boundDatalog.syncPredicates(e.predicateOrder)
	}
	_, selected, err := boundDatalogDeclarations(e.manager, e.predicateOrder)
	if err != nil {
		return true, err
	}
	if !selected {
		// The bound_relation favourite is used only for signatures it can
		// handle (relation_manager::try_get_appropriate_plugin); the others
		// get tables. Without an explicit request, continue on that path.
		if !boundRepresentationRequested(e) && !checkEnabled {
			return false, nil
		}
		return true, ErrBoundDatalogFragment
	}
	// Programs of Int/Real predicates run on the relation plugins
	// (arithRelationsRoute, rel_context's compiled rules); the bound
	// evaluator is no longer installed. What reaches here is a
	// configuration whose plugins are not ported yet: an external relation
	// next to arithmetic predicates (fixedpoint_context's
	// external_relation_plugin) or relation-level explanations.
	return true, ErrBoundDatalogFragment
}

func (e *PropositionalEngine) queryBoundDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	e.compiled = false
	e.datalogStats.Reset()
	enabled, threshold := true, uint32(11)
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return lbool.Undef, err
		}
		enabled, err = view.Bool("datalog.similarity_compressor", true)
		if err == nil && enabled {
			threshold, err = view.UInt("datalog.similarity_compressor_threshold", 11)
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return lbool.Undef, err
		}
	}
	if enabled {
		return e.boundDatalog.queryWithSimilarity(declarations, threshold)
	}
	return e.boundDatalog.query(declarations)
}
func (e *PropositionalEngine) closeBoundDatalog() error {
	if e.boundDatalog == nil {
		return nil
	}
	if err := e.boundDatalog.close(); err != nil {
		return err
	}
	e.boundDatalog = nil
	return nil
}

// boundRepresentationRequested reports a set-predicate-representation request
// naming bound_relation.
func boundRepresentationRequested(e *PropositionalEngine) bool {
	for _, state := range e.predicates {
		for _, name := range state.requestedRepresentations {
			if name == "bound_relation" {
				return true
			}
		}
	}
	return false
}
