package rel

// Source: src/muz/rel/dl_table_relation.cpp
// table_relation_plugin::mk_filter_interpreted_fn and
// mk_select_equal_and_project_fn, and src/muz/rel/dl_relation_manager.cpp
// relation_manager::mk_filter_interpreted_fn(const table_base&, app*): the
// table plugin's own filter (lazy_table_plugin, sparse_table_plugin's
// default) or default_table_filter_interpreted_fn, whose should_remove
// grounds the condition with the context var_subst (standard order) and
// simplifies it with the context rewriter.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// SetContextRewriter records context::get_rewriter(), which the table
// filters of table_relation_plugin use (borrowed).
func (m *RelationManager) SetContextRewriter(rewriter *astrewriter.TheoryRewriter) {
	m.contextRewriter = rewriter
}

// MkFilterInterpretedFn is table_relation_plugin::mk_filter_interpreted_fn:
// the table filter of mk_filter_interpreted_fn(tr.get_table(), condition).
func (p *TableRelationPlugin) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
	if _, ok := p.view(t); !ok {
		return nil, nil
	}
	if p.manager.contextRewriter == nil {
		return nil, ErrInstruction
	}
	// default_table_filter_interpreted_fn keeps app_ref m_condition.
	ref, err := p.manager.manager.NewRef(condition)
	if err != nil {
		return nil, err
	}
	p.manager.fnRoots = append(p.manager.fnRoots, ref)
	return func(a RelationBase) error {
		x, ok := p.view(a)
		if !ok {
			return ErrInstruction
		}
		return p.manager.tableFilterInterpreted(x.table, x.sorts, condition)
	}, nil
}

// MkSelectEqualAndProjectFn is
// table_relation_plugin::mk_select_equal_and_project_fn.
func (p *TableRelationPlugin) MkSelectEqualAndProjectFn(t RelationBase, value ast.ASTHandle, col int) (RelationTransformerFn, error) {
	if _, ok := p.view(t); !ok {
		return nil, nil
	}
	element, err := p.manager.EncodeValue(t.RelationSignature()[col], value)
	if err != nil {
		return nil, err
	}
	sorts := projectSorts(t.RelationSignature(), []int{col})
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.view(a)
		if !ok {
			return nil, ErrInstruction
		}
		r, err := finiteTableSelectProject(x.table, element, col)
		if err != nil {
			return nil, err
		}
		return p.manager.tableView(r, sorts, false), nil
	}, nil
}

// tableFilterInterpreted applies mk_filter_interpreted_fn(table, condition)
// to table, whose columns have the relation sorts sorts.
func (m *RelationManager) tableFilterInterpreted(table FiniteTable, sorts []ast.ASTHandle, condition ast.ASTHandle) error {
	switch concrete := table.(type) {
	case *LazySparseTable:
		return concrete.FilterInterpreted(m, sorts, m.contextRewriter, condition)
	case *SparseTable:
		return concrete.FilterInterpreted(m, sorts, m.contextRewriter, condition)
	}
	removed, err := m.defaultTableShouldRemove(table, sorts, condition)
	if err != nil {
		return err
	}
	// auxiliary_table_filter_fn: r.remove_facts of the collected rows.
	handled, err := RemoveFiniteTableFacts(table, removed)
	if err != nil || handled {
		return err
	}
	return fmt.Errorf("%w: interpreted filter on %T", ErrTable, table)
}

// defaultTableShouldRemove lists the rows of table for which
// default_table_filter_interpreted_fn::should_remove holds: the condition
// grounded with the row's values for its free variables rewrites to false.
func (m *RelationManager) defaultTableShouldRemove(table FiniteTable, sorts []ast.ASTHandle, condition ast.ASTHandle) (removed []Fact, resultErr error) {
	manager := m.manager
	freeVars, err := astrewriter.NewExprFreeVars(manager)
	if err != nil {
		return nil, err
	}
	if err = freeVars.Collect(condition); err != nil {
		return nil, err
	}
	if int(freeVars.Size()) > len(sorts) {
		return nil, fmt.Errorf("%w: condition variable exceeds the table signature", ErrTable)
	}
	facts, err := table.Facts()
	if err != nil {
		return nil, err
	}
	notEqual, notEqualMatched, err := m.notEqualFilter(condition)
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if notEqualMatched {
			if fact[notEqual.variable] == notEqual.value {
				removed = append(removed, fact)
			}
			continue
		}
		remove, err := m.groundConditionFalse(freeVars, sorts, condition, fact)
		if err != nil {
			return nil, err
		}
		if remove {
			removed = append(removed, fact)
		}
	}
	return removed, nil
}

func (m *RelationManager) groundConditionFalse(freeVars *astrewriter.ExprFreeVars, sorts []ast.ASTHandle, condition ast.ASTHandle, fact Fact) (_ bool, resultErr error) {
	manager := m.manager
	bindings := make([]ast.ASTHandle, len(sorts))
	refs := make([]ast.ASTRef, 0, freeVars.Size()+1)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&refs[i]))
		}
	}()
	for variable := uint32(0); variable < freeVars.Size(); variable++ {
		if !freeVars.Contains(variable) {
			continue
		}
		value, err := m.DecodeValue(sorts[variable], fact[variable])
		if err != nil {
			return false, err
		}
		refs = append(refs, value)
		handle, err := manager.RefGet(value)
		if err != nil {
			return false, err
		}
		bindings[tableRelationBindingIndex(len(bindings), variable)] = handle
	}
	output, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	refs = append(refs, output)
	if err = m.GroundCondition(m.contextRewriter, condition, bindings, output); err != nil {
		return false, err
	}
	result, err := manager.RefGet(output)
	if err != nil {
		return false, err
	}
	return manager.IsFalse(result), nil
}

// newFullTable is the table of mk_full_relation(s, pred) when no requested
// kind applies: get_appropriate_plugin(s) is the favourite relation plugin
// (a table_relation plugin of the default table unless
// datalog.default_relation names a relation plugin) or, when the favourite
// relation plugin cannot handle s, the first table_relation plugin in
// registration order that can (sparse, hashtable, bitvector, lazy_sparse).
func (m *RelationManager) newFullTable(predicate ast.ASTHandle, signature Signature) (FiniteTable, error) {
	if m.favouriteRelation == nil || m.checkedFinite != nil || (!predicate.IsNull() && m.requestedTable[predicate] != "") {
		if !predicate.IsNull() {
			return m.newRequestedFiniteTable(predicate, signature)
		}
		return m.newFavouriteFiniteTable(signature)
	}
	m.initRelationKinds()
	for _, p := range m.tableRelationPlugins {
		if p.canHandle(signature) {
			return p.newTable(signature)
		}
	}
	return m.NewDefaultFiniteTable(signature)
}
