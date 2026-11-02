package fp

// Source: dl_relation_manager.cpp register_relation_plugin_impl chooses the
// favourite during engine construction; interval_relation is not the default
// arithmetic fallback (bound_relation precedes it in registration order).

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (e *PropositionalEngine) cacheDatalogRelationFavourite() error {
	if e.datalogRelationFavourite != "" {
		return nil
	}
	name := "pentagon"
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return err
		}
		selected, err := view.Symbol("datalog.default_relation", symbol.Symbol{})
		if err == nil && selected.Value != 0 {
			name, _, err = e.manager.Symbols.BareString(selected)
		}
		if err = errors.Join(err, view.Close()); err != nil {
			return err
		}
	}
	e.datalogRelationFavourite = name
	return nil
}

func (e *PropositionalEngine) ensureIntervalDatalog() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	if e.datalogRelationFavourite != "interval_relation" || len(e.predicateOrder) == 0 {
		return false, nil
	}
	for _, state := range e.predicates {
		if state.explicitFiniteRepresentation || state.external != nil {
			return true, ErrIntervalDatalogFragment
		}
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return true, errors.Join(err, ErrIntervalDatalogFragment)
	}
	if _, enabled, err := e.datalogCheckRelationName(); err != nil || enabled {
		return true, errors.Join(err, ErrIntervalDatalogFragment)
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrIntervalDatalogFragment)
	}
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		magic, err := view.Bool("datalog.magic_sets_for_queries", false)
		err = errors.Join(err, view.Close())
		if err != nil || magic {
			return true, errors.Join(err, ErrIntervalDatalogFragment)
		}
	}
	if e.intervalDatalog != nil {
		return true, e.intervalDatalog.syncPredicates(e.predicateOrder)
	}
	owner, selected, err := newIntervalDatalogActivation(e.manager, e.set, e.predicateOrder)
	if err != nil {
		return true, err
	}
	if !selected {
		return true, ErrIntervalDatalogFragment
	}
	e.intervalDatalog = owner
	// Any eagerly stored nullary tuples predate activation. Copy them into the
	// selected imprecise relation without changing their source fact meaning.
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if state.table != nil && len(state.sorts) == 0 {
			count, err := state.table.Len()
			if err != nil {
				return true, err
			}
			if count != 0 {
				if err = owner.addFact(d, nil); err != nil {
					return true, err
				}
			}
		}
	}
	return true, nil
}

func (e *PropositionalEngine) queryIntervalDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	widen := false
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return lbool.Undef, err
		}
		widen, err = view.Bool("datalog.compile_with_widening", false)
		err = errors.Join(err, view.Close())
		if err != nil {
			return lbool.Undef, err
		}
	}
	e.compiled = false
	e.datalogStats.Reset()
	return e.intervalDatalog.query(declarations, widen)
}

func (e *PropositionalEngine) closeIntervalDatalog() error {
	if e.intervalDatalog == nil {
		return nil
	}
	if err := e.intervalDatalog.close(); err != nil {
		return err
	}
	e.intervalDatalog = nil
	return nil
}
