package rel

// Source: src/muz/rel/dl_sparse_table.{h,cpp} sparse_table_plugin's table
// pool: mk_empty, mk_clone, recycle and reset.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// SparseTablePlugin is a relation manager's sparse_table_plugin. pool is
// m_pool: the deallocated tables of each signature (functional columns
// included), reused last first by mk_empty.
type SparseTablePlugin struct {
	pool map[string][]*SparseTable
}

func newSparseTablePlugin() *SparseTablePlugin {
	return &SparseTablePlugin{pool: make(map[string][]*SparseTable)}
}

// sparsePlugin is the manager's sparse_table_plugin (nil for no manager).
func (m *RelationManager) sparsePlugin() *SparseTablePlugin {
	if m == nil {
		return nil
	}
	return m.sparse
}

// sparseTablePluginOf is the sparse_table_plugin behind a table: its own
// for a sparse table, the lazy plugin's m_plugin for a lazy one.
func sparseTablePluginOf(t FiniteTable) *SparseTablePlugin {
	switch v := t.(type) {
	case *SparseTable:
		return v.plugin
	case *LazySparseTable:
		return v.engine.sparse
	case *BitvectorTable:
		return v.sparse
	}
	return nil
}

// sparsePoolKey is table_signature::hash/eq's key: the column domains and
// the functional column count.
func sparsePoolKey(signature Signature, functional int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d", functional)
	for _, d := range signature.domains {
		fmt.Fprintf(&b, ",%d", d)
	}
	return b.String()
}

// MkEmpty is sparse_table_plugin::mk_empty: the last pooled table of the
// signature, else a new one. A nil plugin makes a table outside any pool.
func (p *SparseTablePlugin) MkEmpty(signature Signature, functional int) (*SparseTable, error) {
	if p == nil {
		return NewSparseTable(signature, functional)
	}
	key := sparsePoolKey(signature, functional)
	vect := p.pool[key]
	if len(vect) == 0 {
		return newSparseTable(p, signature, functional)
	}
	res := vect[len(vect)-1]
	vect[len(vect)-1] = nil
	p.pool[key] = vect[:len(vect)-1]
	return res, nil
}

// mkClone is sparse_table_plugin::mk_clone: an empty table of the
// signature whose m_data is assigned the source's.
func (p *SparseTablePlugin) mkClone(t *SparseTable) (*SparseTable, error) {
	res, err := p.MkEmpty(t.signature, t.functional)
	if err != nil {
		return nil, err
	}
	res.data.assign(t.data)
	return res, nil
}

// recycle is sparse_table_plugin::recycle with its verbose_action
// _va("recycle", 2): the table is reset and pooled. The handle t is
// closed; the pool keeps the table's state in a new handle.
func (p *SparseTablePlugin) recycle(t *SparseTable) (result error) {
	action, err := base.StartVerboseAction("recycle", 2)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, action.End()) }()
	t.reset()
	if diagnostic.VerboseProgress {
		if err := diagnostic.ProgressVerbose(12, fmt.Sprintf("Recycle: %d\n", t.sizeEstimateBytes())); err != nil {
			return err
		}
	}
	pooled := *t
	t.data = nil
	if p != nil {
		key := sparsePoolKey(pooled.signature, pooled.functional)
		p.pool[key] = append(p.pool[key], &pooled)
	}
	return nil
}

// reset is sparse_table_plugin::reset: the pooled tables are destroyed.
func (p *SparseTablePlugin) reset() {
	if p == nil {
		return
	}
	for key, vect := range p.pool {
		for _, t := range vect {
			t.resetIndexes()
			t.data = nil
		}
		delete(p.pool, key)
	}
}

// MkEmptySparseTable is the sparse_table_plugin's mk_empty.
func (m *RelationManager) MkEmptySparseTable(signature Signature, functional int) (*SparseTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.sparse.MkEmpty(signature, functional)
}

// MkEmptyBitvectorTable is bitvector_table_plugin::mk_empty in this
// manager.
func (m *RelationManager) MkEmptyBitvectorTable(signature Signature) (*BitvectorTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return newBitvectorTableIn(m.sparse, signature)
}

// NewLazySparseFiniteFactory is a lazy_table_plugin over this manager's
// sparse_table_plugin.
func (m *RelationManager) NewLazySparseFiniteFactory() *LazySparseFiniteFactory {
	f := NewLazySparseFiniteFactory()
	f.engine.sparse = m.sparsePlugin()
	return f
}
