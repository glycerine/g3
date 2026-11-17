package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp: the table of a
// finite_product_relation is a table of get_appropriate_plugin(table_sig)
// (relation_manager::try_get_appropriate_plugin: the favourite table plugin
// when it handles the signature, else the first registered plugin that
// does). The signature has a functional column, which the hashtable and
// bitvector plugins reject, so it is the sparse plugin's, or the lazy
// table plugin's when lazy_sparse is the favourite. The relation's other
// tables are tplugin.mk_empty (the table's own plugin) and table_base's
// default operations.

import (
	"errors"
	"io"
)

// SetDefaultTableChecked records datalog.default_table_checked: the
// favourite table plugin is then check_table_plugin, whose
// can_handle_signature needs both the checked and the checker plugin. g3
// makes check tables only for the evaluation (open limitation), so a
// finite product table under default_table_checked is the sparse plugin's,
// the first registered plugin that handles the signature.
func (m *RelationManager) SetDefaultTableChecked(checked bool) error {
	if err := m.open(); err != nil {
		return err
	}
	m.defaultTableChecked = checked
	return nil
}

// fprAppropriateTable is get_appropriate_plugin(sig).mk_empty(sig) (and
// relation_manager::mk_empty_table) for a finite product table signature.
func (m *RelationManager) fprAppropriateTable(signature Signature, functional int) (FiniteTable, error) {
	if m != nil && m.checkedFinite == nil && !m.defaultTableChecked && m.defaultFinite == "lazy_sparse" && m.lazyFinite != nil && m.lazyFinite.open() == nil && signature.Len() != 0 {
		return m.lazyFinite.engine.New(signature, functional)
	}
	return m.sparsePlugin().MkEmpty(signature, functional)
}

// fprTableLike is t.get_plugin().mk_empty(sig): a lazy table's plugin
// makes lazy tables.
func fprTableLike(t FiniteTable, signature Signature, functional int) (FiniteTable, error) {
	if lazy, ok := t.(*LazySparseTable); ok && signature.Len() != 0 {
		if err := lazy.open(); err != nil {
			return nil, err
		}
		return lazy.engine.New(signature, functional)
	}
	return sparseTablePluginOf(t).MkEmpty(signature, functional)
}

// fprTableHandles is t.get_plugin().can_handle_signature(sig) for a
// signature with functional columns.
func fprTableHandles(t FiniteTable, signature Signature) bool {
	switch t.(type) {
	case *SparseTable, *LazySparseTable:
		return signature.Len() != 0
	}
	return false
}

// fprSuggestFact is table_base::suggest_fact (sparse_table overrides it).
func fprSuggestFact(t FiniteTable, f Fact) (bool, error) {
	switch concrete := t.(type) {
	case *SparseTable:
		return concrete.SuggestFact(f)
	case *LazySparseTable:
		found, err := concrete.FetchFact(f)
		if err != nil || found {
			return false, err
		}
		// add_new_fact: lazy_table::add_fact.
		_, err = concrete.Add(f)
		return err == nil, err
	}
	return false, ErrTable
}

// fprEnsureFact is table_base::ensure_fact.
func fprEnsureFact(t FiniteTable, f Fact) error {
	switch concrete := t.(type) {
	case *SparseTable:
		return concrete.EnsureFact(f)
	case *LazySparseTable:
		return concrete.EnsureFact(f)
	}
	return ErrTable
}

// fprFetchFact is table_base::fetch_fact.
func fprFetchFact(t FiniteTable, f Fact) (bool, error) {
	switch concrete := t.(type) {
	case *SparseTable:
		return concrete.FetchFact(f)
	case *LazySparseTable:
		return concrete.FetchFact(f)
	}
	return false, ErrTable
}

// fprComplementFunctional is table_base::complement(p, func_columns).
func fprComplementFunctional(t FiniteTable, functional Fact) (FiniteTable, error) {
	switch concrete := t.(type) {
	case *SparseTable:
		return concrete.ComplementFunctional(functional)
	case *LazySparseTable:
		return concrete.ComplementFunctional(functional)
	}
	return nil, ErrTable
}

func fprSizeEstimateRows(t FiniteTable) (uint32, error) {
	switch concrete := t.(type) {
	case *SparseTable:
		return concrete.SizeEstimateRows()
	case *LazySparseTable:
		return concrete.SizeEstimateRows()
	}
	return 0, ErrTable
}

func fprSizeEstimateBytes(t FiniteTable) (uint32, error) {
	switch concrete := t.(type) {
	case *SparseTable:
		return concrete.SizeEstimateBytes()
	case *LazySparseTable:
		return concrete.SizeEstimateBytes()
	}
	return 0, ErrTable
}

func fprTableDisplay(t FiniteTable, out io.Writer) error {
	switch concrete := t.(type) {
	case *SparseTable:
		return concrete.Display(out)
	case *LazySparseTable:
		return concrete.Display(out)
	}
	return ErrTable
}

// fprFunctionalColumns is the table signature's functional_columns().
func fprFunctionalColumns(t FiniteTable) int {
	switch concrete := t.(type) {
	case *SparseTable:
		return concrete.functional
	case *LazySparseTable:
		if concrete.root != nil {
			return concrete.root.functional
		}
	}
	return 0
}

// fprProjectWithReduceDefault is default_table_project_with_reduce_fn on
// a table of another plugin than sparse: res = t.get_plugin().mk_empty;
// for each projected row, suggest_fact, else reduce and ensure_fact.
func fprProjectWithReduceDefault(t FiniteTable, removed []int, reducer SparseFunctionalRowReducer) (_ FiniteTable, resultErr error) {
	functionalCount := fprFunctionalColumns(t)
	if functionalCount == 0 || len(removed) == 0 || reducer == nil {
		return nil, ErrTable
	}
	sourceSignature, err := t.Signature()
	if err != nil {
		return nil, err
	}
	signature, err := projectedSignature(sourceSignature, removed)
	if err != nil {
		return nil, err
	}
	functional := functionalCount
	firstFunctional := sourceSignature.Len() - functionalCount
	for _, column := range removed {
		if column >= firstFunctional {
			functional--
		}
	}
	result, err := fprTableLike(t, signature, functional)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := t.Facts()
	if err != nil {
		return nil, err
	}
	resultFirstFunctional := signature.Len() - functional
	for _, fact := range facts {
		candidate := sparseProjectFact(fact, removed)
		incoming := append([]Element(nil), candidate[resultFirstFunctional:]...)
		added, addErr := fprSuggestFact(result, candidate)
		if addErr != nil {
			return nil, addErr
		}
		if added {
			continue
		}
		existing := append([]Element(nil), candidate[resultFirstFunctional:]...)
		reducer(existing, incoming)
		copy(candidate[resultFirstFunctional:], existing)
		if err = fprEnsureFact(result, candidate); err != nil {
			return nil, err
		}
	}
	keep = true
	return result, nil
}
