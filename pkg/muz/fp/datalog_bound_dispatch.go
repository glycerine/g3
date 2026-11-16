package fp

// Source: dl_relation_manager.cpp mk_empty_relation first tries a favourite,
// then finite table conversion, then registered relation plugins. The bound
// plugin precedes interval in rel_context.cpp registration order.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// ErrBoundDatalogFragment reports a configuration with arithmetic
// predicates whose relation plugins are not ported.
var ErrBoundDatalogFragment = errors.New("muz/fp: unsupported arithmetic bound Datalog fragment")

type boundDatalogDeclaration struct {
	declaration ast.ASTHandle
	sorts       []ast.ASTHandle
}

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
	_, selected, err := boundDatalogDeclarations(e.manager, e.predicateOrder)
	if err != nil {
		return true, err
	}
	if !selected {
		// The bound_relation favourite is used only for signatures it can
		// handle (relation_manager::try_get_appropriate_plugin); the others
		// get tables. Without an explicit request, continue on that path.
		if !boundRepresentationRequested(e) {
			return false, nil
		}
		return true, ErrBoundDatalogFragment
	}
	// Programs of Int/Real predicates run on the relation plugins
	// (arithRelationsRoute, rel_context's compiled rules). What reaches here
	// is a configuration whose plugins are not ported yet: an external
	// relation next to arithmetic predicates (fixedpoint_context's
	// external_relation_plugin) or relation-level explanations.
	return true, ErrBoundDatalogFragment
}

// boundDatalogDeclarations reports whether bound_relation_plugin can handle
// every declaration's signature (vacuously the nullary one).
func boundDatalogDeclarations(manager *ast.ASTManager, declarations []ast.ASTHandle) ([]boundDatalogDeclaration, bool, error) {
	if manager == nil || len(declarations) == 0 {
		return nil, false, nil
	}
	plugin, err := rel.NewBoundRelationPlugin(manager)
	if err != nil {
		return nil, false, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, false, err
	}
	seen := make(map[ast.ASTHandle]bool, len(declarations))
	validated := make([]boundDatalogDeclaration, 0, len(declarations))
	for _, declaration := range declarations {
		kind, err := manager.GetKind(declaration)
		if err != nil {
			return nil, false, err
		}
		if kind != ast.ASTFuncDecl || seen[declaration] {
			return nil, false, ErrBoundDatalogFragment
		}
		seen[declaration] = true
		rangeSort, err := manager.GetRange(declaration)
		if err != nil {
			return nil, false, err
		}
		if rangeSort != boolean {
			return nil, false, nil
		}
		arity, err := manager.GetArity(declaration)
		if err != nil {
			return nil, false, err
		}
		sorts := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[i], err = manager.GetDomain(declaration, i)
			if err != nil {
				return nil, false, err
			}
		}
		capable, err := plugin.CanHandleSignature(sorts)
		if err != nil {
			return nil, false, err
		}
		if !capable {
			return nil, false, nil
		}
		validated = append(validated, boundDatalogDeclaration{declaration: declaration, sorts: sorts})
	}
	return validated, true, nil
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
