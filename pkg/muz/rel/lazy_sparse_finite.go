package rel

import (
	"errors"
	"io"
)

func (e *LazySparseEngine) adoptSparse(table *SparseTable) (*LazySparseTable, error) {
	if err := e.open(); err != nil || table == nil || table.open() != nil {
		return nil, ErrLazySparse
	}
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	functional, err := table.FunctionalColumns()
	if err != nil {
		return nil, err
	}
	return e.adopt(&lazySparseNode{kind: LazyTableBase, signature: signature, functional: functional, evaluated: table}), nil
}

func (t *LazySparseTable) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: t.root.signature.Domains()}, nil
}

func (t *LazySparseTable) Contains(fact Fact) (bool, error) {
	table, err := t.finiteEval()
	if err != nil {
		return false, err
	}
	return table.Contains(fact)
}

func (t *LazySparseTable) Len() (int, error) {
	table, err := t.finiteEval()
	if err != nil {
		return 0, err
	}
	return table.Len()
}

// FetchFact is table_base::fetch_fact of a lazy table (lazy_table does
// not override it): without functional columns lazy_table::contains_fact
// (the evaluated table's contains_fact), else the functional suffix of the
// evaluated table's row with fact's key.
func (t *LazySparseTable) FetchFact(fact Fact) (bool, error) {
	table, err := t.eval()
	if err != nil {
		return false, err
	}
	if table.functional == 0 {
		return table.ContainsFact(fact)
	}
	return table.fetchFunctional(fact)
}

// EnsureFact is table_base::ensure_fact of a lazy table: without
// functional columns add_fact, else remove_fact then add_fact, each on the
// evaluated table (lazy_table::remove_fact, lazy_table::add_fact).
func (t *LazySparseTable) EnsureFact(fact Fact) error {
	table, err := t.eval()
	if err != nil {
		return err
	}
	if table.functional != 0 {
		if _, err = table.RemoveFacts([]Fact{fact}); err != nil {
			return err
		}
	}
	_, err = table.Add(fact)
	return err
}

// Remove forces this lazy root and removes the fact from its actual packed
// sparse owner or the source nullary Hash fallback.
func (t *LazySparseTable) Remove(fact Fact) (bool, error) {
	table, err := t.finiteEval()
	if err != nil {
		return false, err
	}
	switch concrete := table.(type) {
	case *SparseTable:
		return concrete.Remove(fact)
	case *HashTable:
		return concrete.Remove(fact)
	default:
		return false, ErrLazySparse
	}
}

func (t *LazySparseTable) SizeEstimateRows() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return 1, nil
}

func (t *LazySparseTable) SizeEstimateBytes() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return 1, nil
}

func (t *LazySparseTable) Display(out io.Writer) error {
	table, err := t.finiteEval()
	if err != nil {
		return err
	}
	return displayLazySparseFinite(table, out)
}

func (t *LazySparseTable) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.root.signature.Len() == 0 {
		signature, _ := NewSignature(nil)
		next := &lazySparseNode{kind: LazyTableBase, references: 1, signature: signature, nullary: NewHashTable(signature)}
		if err := releaseLazySparseNode(t.root); err != nil {
			return errors.Join(err, next.nullary.Close())
		}
		t.root = next
		return nil
	}
	table, err := t.engine.sparse.MkEmpty(t.root.signature, t.root.functional)
	if err != nil {
		return err
	}
	next := &lazySparseNode{kind: LazyTableBase, references: 1, signature: Signature{domains: t.root.signature.Domains()}, functional: t.root.functional, evaluated: table}
	if err = releaseLazySparseNode(t.root); err != nil {
		return errors.Join(err, table.Close())
	}
	t.root = next
	return nil
}

func (t *LazySparseTable) CloneFinite() (FiniteTable, error) {
	return t.Clone()
}

var _ FiniteTable = (*LazySparseTable)(nil)
