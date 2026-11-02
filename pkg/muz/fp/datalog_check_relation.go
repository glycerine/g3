package fp

// Source: src/muz/rel/rel_context.cpp rel_context::updt_params and
// src/muz/rel/check_relation.cpp checked fact insertion.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type datalogCheckedPredicate struct {
	declaration ast.ASTHandle
	state       *predicateState
	original    *rel.HashTable
	shadow      *rel.HashTable
	checked     *rel.FormulaCheckedTable
	sparse      *rel.CheckedRelation
	publish     *rel.HashTable
}

type datalogCheckRelationScope struct {
	engine  *PropositionalEngine
	entries []datalogCheckedPredicate
}

type datalogCheckRelationCleanupFailure struct {
	cause error
	scope *datalogCheckRelationScope
}

func (f *datalogCheckRelationCleanupFailure) Error() string  { return f.cause.Error() }
func (f *datalogCheckRelationCleanupFailure) Unwrap() error  { return f.cause }
func (f *datalogCheckRelationCleanupFailure) Cleanup() error { return f.scope.dispose() }

func (e *PropositionalEngine) datalogCheckRelationName() (name string, enabled bool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return "", false, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return "", false, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	selection, err := view.Symbol("datalog.check_relation", symbol.Symbol{})
	if err != nil {
		return "", false, err
	}
	if selection.Value == 0 {
		return "", false, nil
	}
	name, _, err = e.manager.Symbols.BareString(selection)
	if err != nil {
		return "", false, err
	}
	if name == "" || name == "null" {
		return name, false, nil
	}
	// Source table_relation_plugin::create_plugin_name registers these
	// relation names. Keep the existing bare Go convenience names too.
	switch name {
	case "tr_hashtable":
		name = "hashtable"
	case "tr_sparse":
		name = "sparse"
	}
	// doc uses finite execution here while preserving source's compressor
	// interaction. Sparse is checked by the concrete paired sparse table.
	if name != "hashtable" && name != "sparse" && name != "doc" {
		return "", false, fmt.Errorf("muz/fp: relation plugin %s is not available for datalog.check_relation", name)
	}
	return name, true, nil
}

func (e *PropositionalEngine) beginDatalogCheckRelation() (_ *datalogCheckRelationScope, resultErr error) {
	name, enabled, err := e.datalogCheckRelationName()
	if err != nil || !enabled {
		return nil, err
	}
	scope := &datalogCheckRelationScope{engine: e}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, scope.dispose())
		}
	}()
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil || state.table == nil {
			continue
		}
		if name == "sparse" {
			signature, err := state.table.Signature()
			if err != nil {
				return nil, err
			}
			// sparse_table_plugin cannot handle the empty signature; source
			// relation selection falls back instead of wrapping that predicate.
			if signature.Len() == 0 {
				continue
			}
			checked, err := rel.NewCheckedRelationFromTable(state.table)
			if err != nil {
				return nil, err
			}
			shadow, err := checked.Primary()
			if err != nil {
				return nil, errors.Join(err, checked.Close())
			}
			entry := datalogCheckedPredicate{declaration: declaration, state: state, original: state.table, shadow: shadow, sparse: checked}
			scope.entries = append(scope.entries, entry)
			state.table, state.sparse = shadow, checked
			continue
		}
		shadow, err := state.table.Clone()
		if err != nil {
			return nil, err
		}
		table, err := rel.AdoptTableRelation(e.relations, state.sorts, shadow)
		if err != nil {
			return nil, errors.Join(err, shadow.Close())
		}
		checked, err := rel.NewFormulaCheckedTable(table, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
		if err != nil {
			return nil, errors.Join(err, table.Close(), shadow.Close())
		}
		entry := datalogCheckedPredicate{declaration: declaration, state: state, original: state.table, shadow: shadow, checked: checked}
		scope.entries = append(scope.entries, entry)
		state.table, state.checked = checked.Table(), checked
	}
	published = true
	return scope, nil
}

func (s *datalogCheckRelationScope) restoreStates() {
	for i := range s.entries {
		entry := &s.entries[i]
		if entry.state != nil && (entry.state.checked == entry.checked && entry.checked != nil || entry.state.sparse == entry.sparse && entry.sparse != nil) {
			entry.state.table, entry.state.checked, entry.state.sparse = entry.original, nil, nil
		}
	}
}

func (s *datalogCheckRelationScope) complete(publish bool) (result error) {
	if s == nil {
		return nil
	}
	if publish {
		for i := range s.entries {
			entry := &s.entries[i]
			entry.publish, result = entry.shadow.Clone()
			if result != nil {
				publish = false
				break
			}
		}
	}
	s.restoreStates()
	if publish {
		for i := range s.entries {
			entry := &s.entries[i]
			if err := s.engine.installDatalogTable(entry.declaration, entry.publish); err != nil {
				result = errors.Join(result, err)
				entry.publish = nil
				publish = false
				break
			}
			entry.publish = nil
		}
	}
	for i := range s.entries {
		if s.entries[i].publish != nil {
			result = errors.Join(result, s.entries[i].publish.Close())
			s.entries[i].publish = nil
		}
	}
	if err := s.dispose(); err != nil {
		result = errors.Join(result, &datalogCheckRelationCleanupFailure{cause: err, scope: s})
	}
	return result
}

func (s *datalogCheckRelationScope) dispose() (result error) {
	if s == nil {
		return nil
	}
	s.restoreStates()
	for i := range s.entries {
		entry := &s.entries[i]
		if entry.checked != nil {
			if err := entry.checked.Close(); err != nil {
				result = errors.Join(result, err)
				continue
			}
			entry.checked = nil
		}
		if entry.sparse != nil {
			if err := entry.sparse.Close(); err != nil {
				result = errors.Join(result, err)
				continue
			}
			entry.sparse = nil
			entry.shadow = nil
		}
		if entry.shadow != nil {
			result = errors.Join(result, entry.shadow.Close())
			entry.shadow = nil
		}
	}
	return result
}
