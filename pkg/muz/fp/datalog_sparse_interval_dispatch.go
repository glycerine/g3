package fp

// Source: rel_context::set_predicate_representation creates a product kind
// from requested relation plugins; generated query heads inherit that kind.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) ensureSparseIntervalDatalog() (bool, error) {
	selected := false
	for _, state := range e.predicates {
		selected = selected || isSparseIntervalRepresentation(state.requestedRepresentations)
	}
	if !selected {
		return false, nil
	}
	originals := make([]ast.ASTHandle, 0, len(e.predicateOrder))
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state.external != nil || state.explicitFiniteRepresentation {
			return true, ErrSparseIntervalDatalogFragment
		}
		if e.auxiliary[declaration] {
			continue
		}
		if !isSparseIntervalRepresentation(state.requestedRepresentations) {
			return true, ErrSparseIntervalDatalogFragment
		}
		originals = append(originals, declaration)
	}
	if len(originals) == 0 {
		return true, ErrSparseIntervalDatalogFragment
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return true, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	if _, enabled, err := e.datalogCheckRelationName(); err != nil || enabled {
		return true, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		magic, err := view.Bool("datalog.magic_sets_for_queries", false)
		err = errors.Join(err, view.Close())
		if err != nil || magic {
			return true, errors.Join(err, ErrSparseIntervalDatalogFragment)
		}
	}
	if e.sparseIntervalDatalog == nil {
		owner, supported, err := newSparseIntervalDatalogActivation(e.manager, e.set, e.guard, originals)
		if err != nil {
			return true, err
		}
		if !supported {
			return true, ErrSparseIntervalDatalogFragment
		}
		e.sparseIntervalDatalog = owner
	}
	// Preserve previously installed auxiliaries while registering new original
	// predicates through the strict mixed-signature path.
	strict := append([]ast.ASTHandle(nil), originals...)
	seen := make(map[ast.ASTHandle]bool, len(strict))
	for _, d := range strict {
		seen[d] = true
	}
	for _, d := range e.sparseIntervalDatalog.predicates {
		if !seen[d] {
			strict = append(strict, d)
			seen[d] = true
		}
	}
	if err := e.sparseIntervalDatalog.syncPredicates(strict); err != nil {
		return true, err
	}
	// Original predicates were checked above; only the engine's generated
	// auxiliaries may use a projected pure-component or empty signature.
	return true, e.sparseIntervalDatalog.syncAuxiliaryPredicates(e.predicateOrder)
}

func (e *PropositionalEngine) querySparseIntervalDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
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
	return e.sparseIntervalDatalog.query(declarations, widen)
}

func (e *PropositionalEngine) closeSparseIntervalDatalog() error {
	if e.sparseIntervalDatalog == nil {
		return nil
	}
	if err := e.sparseIntervalDatalog.close(); err != nil {
		return err
	}
	e.sparseIntervalDatalog = nil
	return nil
}
