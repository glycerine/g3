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
	original    rel.FiniteTable
	shadow      *rel.HashTable
	checked     *rel.FormulaCheckedTable
	sparse      *rel.CheckedRelation
	pair        *rel.FiniteCheckedPair
	publish     rel.FiniteTable
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

// finiteHashSnapshot materializes the concrete finite input required by the
// formula checker without assuming manager storage is already a hashtable.
func finiteHashSnapshot(source rel.FiniteTable) (_ *rel.HashTable, resultErr error) {
	if source == nil {
		return nil, rel.ErrTable
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	result := rel.NewHashTable(signature)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := source.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// publishCheckedFactsLike builds query output using the original selected
// finite representation rather than leaking the checker's hashtable shadow.
func publishCheckedFactsLike(original rel.FiniteTable, facts []rel.Fact) (_ rel.FiniteTable, resultErr error) {
	if original == nil {
		return nil, rel.ErrTable
	}
	result, err := original.CloneFinite()
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.Reset(); err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

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

// datalogDefaultTableCheck returns the exact configured checker plugin. The
// selected primary is the predicate's current concrete storage, so an explicit
// SetPredicateRepresentation remains authoritative.
func (e *PropositionalEngine) datalogDefaultTableCheck() (checker string, enabled bool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return "", false, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return "", false, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	enabled, err = view.Bool("datalog.default_table_checked", false)
	if err != nil || !enabled {
		return "", false, err
	}
	readName := func(key string) (string, error) {
		value, err := view.Symbol(key, symbol.Symbol{})
		if err != nil || value.Value == 0 {
			return "", err
		}
		name, _, err := e.manager.Symbols.BareString(value)
		return name, err
	}
	base, err := readName("datalog.default_table")
	if err != nil {
		return "", false, err
	}
	checker, err = readName("datalog.default_table_checker")
	if err != nil {
		return "", false, err
	}
	if checker == "" || checker == "null" {
		// Source registration leaves checking disabled when the named checker
		// plugin does not exist; null is its catalog default.
		return "", false, nil
	}
	normalize := func(name string) string {
		switch name {
		case "tr_hashtable":
			return "hashtable"
		case "tr_sparse":
			return "sparse"
		default:
			return name
		}
	}
	base, checker = normalize(base), normalize(checker)
	if (base != "hashtable" && base != "sparse") || (checker != "hashtable" && checker != "sparse") {
		return "", false, fmt.Errorf("muz/fp: checked default table pair %s/%s is not available", base, checker)
	}
	return checker, true, nil
}

func (e *PropositionalEngine) beginDatalogCheckRelation() (_ *datalogCheckRelationScope, resultErr error) {
	name, enabled, err := e.datalogCheckRelationName()
	if err != nil {
		return nil, err
	}
	tableCheck := false
	if !enabled {
		name, enabled, err = e.datalogDefaultTableCheck()
		if err != nil || !enabled {
			return nil, err
		}
		tableCheck = true
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
		if tableCheck {
			signature, err := state.table.Signature()
			if err != nil {
				return nil, err
			}
			// sparse_table_plugin cannot handle the empty signature; source
			// relation selection falls back instead of constructing check_table.
			if name == "sparse" && signature.Len() == 0 {
				continue
			}
			pair, err := rel.NewFiniteCheckedPair(state.table, name)
			if err != nil {
				return nil, err
			}
			entry := datalogCheckedPredicate{declaration: declaration, state: state, original: state.table, pair: pair}
			scope.entries = append(scope.entries, entry)
			state.table = pair
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
			snapshot, err := finiteHashSnapshot(state.table)
			if err != nil {
				return nil, err
			}
			checked, err := rel.NewCheckedRelationFromTable(snapshot)
			closeErr := snapshot.Close()
			if err != nil {
				return nil, errors.Join(err, closeErr)
			}
			if closeErr != nil {
				return nil, errors.Join(closeErr, checked.Close())
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
		shadow, err := finiteHashSnapshot(state.table)
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
		if entry.state != nil && (entry.state.checked == entry.checked && entry.checked != nil || entry.state.sparse == entry.sparse && entry.sparse != nil || entry.pair != nil && entry.state.table == entry.pair) {
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
			var facts []rel.Fact
			var err error
			if entry.pair != nil {
				facts, err = entry.pair.Facts()
			} else {
				facts, err = entry.shadow.Facts()
			}
			if err != nil {
				result = err
				publish = false
				break
			}
			entry.publish, result = publishCheckedFactsLike(entry.original, facts)
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
			entry.shadow = nil
		}
		if entry.sparse != nil {
			if err := entry.sparse.Close(); err != nil {
				result = errors.Join(result, err)
				continue
			}
			entry.sparse = nil
			entry.shadow = nil
		}
		if entry.pair != nil {
			if err := entry.pair.Close(); err != nil {
				result = errors.Join(result, err)
				continue
			}
			entry.pair = nil
		}
		if entry.shadow != nil {
			result = errors.Join(result, entry.shadow.Close())
			entry.shadow = nil
		}
	}
	return result
}
