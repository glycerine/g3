package fp

// Source: dl_bound_relation.cpp bound_relation::add_fact and rel_context's
// imprecise relation query status. Positive Horn rules are lowered by the
// companion bound_datalog_positive implementation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrBoundDatalogFragment = errors.New("muz/fp: unsupported arithmetic bound Datalog fragment")

type boundDatalogEntry struct {
	sorts []ast.ASTHandle
	seed  *rel.BoundRelation
	root  ast.ASTRef
}

// boundDatalogRuntime owns declaration pins and seed facts, and publishes a
// separate saturated snapshot only after the complete supported program succeeds.
type boundDatalogRuntime struct {
	manager   *ast.ASTManager
	rules     *base.RuleSet
	entries   map[ast.ASTHandle]*boundDatalogEntry
	order     []ast.ASTHandle
	evaluated map[ast.ASTHandle]*rel.BoundRelation
	closed    bool
}

func newBoundDatalogRuntime(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle) (_ *boundDatalogRuntime, resultErr error) {
	if manager == nil || rules == nil || len(declarations) == 0 {
		return nil, ErrBoundDatalogFragment
	}
	r := &boundDatalogRuntime{manager: manager, rules: rules, entries: make(map[ast.ASTHandle]*boundDatalogEntry, len(declarations))}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, r.close())
		}
	}()
	plugin, err := rel.NewBoundRelationPlugin(manager)
	if err != nil {
		return nil, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	for _, declaration := range declarations {
		if r.entries[declaration] != nil {
			return nil, fmt.Errorf("%w: duplicate predicate", ErrBoundDatalogFragment)
		}
		rangeSort, err := manager.GetRange(declaration)
		if err != nil {
			return nil, err
		}
		if rangeSort != boolean {
			return nil, ErrBoundDatalogFragment
		}
		arity, err := manager.GetArity(declaration)
		if err != nil {
			return nil, err
		}
		sorts := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[i], err = manager.GetDomain(declaration, i)
			if err != nil {
				return nil, err
			}
		}
		capable, err := plugin.CanHandleSignature(sorts)
		if err != nil {
			return nil, err
		}
		if !capable {
			return nil, ErrBoundDatalogFragment
		}
		root, err := manager.NewRef(declaration)
		if err != nil {
			return nil, err
		}
		r.entries[declaration] = &boundDatalogEntry{sorts: sorts, seed: rel.NewBoundRelation(arity, true), root: root}
		r.order = append(r.order, declaration)
	}
	return r, nil
}

func (r *boundDatalogRuntime) open() error {
	if r == nil || r.closed || r.manager == nil || r.rules == nil || r.entries == nil {
		return ErrBoundDatalogFragment
	}
	return nil
}

func (r *boundDatalogRuntime) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	entry := r.entries[declaration]
	if entry == nil || len(values) != len(entry.sorts) {
		return ErrBoundDatalogFragment
	}
	for i, value := range values {
		sort, err := r.manager.GetSort(value)
		if err != nil {
			return err
		}
		if sort != entry.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	if err := entry.seed.AddFact(values); err != nil {
		return err
	}
	r.evaluated = nil
	return nil
}

func (r *boundDatalogRuntime) query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := r.saturatePositive(true); err != nil {
		return lbool.Undef, err
	}
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	for _, declaration := range declarations {
		if r.evaluated[declaration] == nil {
			return lbool.Undef, fmt.Errorf("%w: undeclared query predicate", ErrBoundDatalogFragment)
		}
	}
	for _, declaration := range declarations {
		if !r.evaluated[declaration].Empty() {
			return lbool.Undef, nil
		}
	}
	return lbool.False, nil
}

func (r *boundDatalogRuntime) relation(declaration ast.ASTHandle) (*rel.BoundRelation, error) {
	if err := r.open(); err != nil || r.evaluated == nil || r.evaluated[declaration] == nil {
		if err == nil {
			err = ErrBoundDatalogFragment
		}
		return nil, err
	}
	return r.evaluated[declaration].Clone(), nil
}

func (r *boundDatalogRuntime) relationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := r.open(); err != nil || r.evaluated == nil || r.evaluated[declaration] == nil {
		if err == nil {
			err = ErrBoundDatalogFragment
		}
		return ast.ASTRef{}, err
	}
	return r.evaluated[declaration].ToFormula(r.manager, r.entries[declaration].sorts)
}

func (r *boundDatalogRuntime) reset() error {
	if err := r.open(); err != nil {
		return err
	}
	for _, entry := range r.entries {
		entry.seed = rel.NewBoundRelation(uint32(len(entry.sorts)), true)
	}
	r.evaluated = nil
	return nil
}

func (r *boundDatalogRuntime) close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	for i := len(r.order) - 1; i >= 0; i-- {
		entry := r.entries[r.order[i]]
		if entry != nil && entry.root != (ast.ASTRef{}) {
			result = errors.Join(result, r.manager.DelRef(&entry.root))
		}
	}
	if result != nil {
		return result
	}
	r.closed = true
	r.manager, r.rules, r.entries, r.order, r.evaluated = nil, nil, nil, nil, nil
	return nil
}
