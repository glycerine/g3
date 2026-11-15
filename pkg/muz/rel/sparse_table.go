package rel

import "github.com/glycerine/g3/pkg/muz/base"

type SparseTable struct {
	signature  Signature
	layout     SparseLayout
	data       *sparseStorage
	functional int
}

func NewSparseTable(signature Signature, functionalColumns int) (*SparseTable, error) {
	layout, e := NewSparseLayout(signature, functionalColumns)
	if e != nil {
		return nil, e
	}
	return &SparseTable{signature: signature, layout: layout, data: newSparseStorage(layout.entryBytes, layout.functionalBytes), functional: functionalColumns}, nil
}
func (t *SparseTable) open() error {
	if t == nil || t.data == nil {
		return ErrTable
	}
	return nil
}
func (t *SparseTable) Add(f Fact) (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	packed, e := t.layout.pack(t.signature, f)
	if e != nil {
		return false, e
	}
	added, _ := t.data.insert(packed)
	return added, nil
}
func (t *SparseTable) Contains(f Fact) (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	packed, e := t.layout.pack(t.signature, f)
	if e != nil {
		return false, e
	}
	index, ok := t.data.find(packed)
	if !ok {
		return false, nil
	}
	if t.functional == 0 {
		return true, nil
	}
	stored := t.layout.unpack(t.data.entries[index])
	for column := t.signature.Len() - t.functional; column < t.signature.Len(); column++ {
		if stored[column] != f[column] {
			return false, nil
		}
	}
	return true, nil
}
func (t *SparseTable) Remove(f Fact) (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	packed, e := t.layout.pack(t.signature, f)
	if e != nil {
		return false, e
	}
	return t.data.remove(packed), nil
}
func (t *SparseTable) Len() (int, error) {
	if e := t.open(); e != nil {
		return 0, e
	}
	return len(t.data.entries), nil
}

// Empty reports whether the concrete table has no stored rows, matching
// sparse_table::empty without exposing its packed entry storage.
func (t *SparseTable) Empty() (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	return len(t.data.entries) == 0, nil
}

// SizeEstimateRows reports the exact stored row count, matching sparse_table.
func (t *SparseTable) SizeEstimateRows() (uint32, error) {
	rows, err := t.Len()
	return uint32(rows), err
}

// SizeEstimateBytes reports the packed row payload held by the sparse table.
// The native estimate also includes allocator capacity for its index; Go's
// map allocation is runtime-managed, so this reports the owned encoded rows.
func (t *SparseTable) SizeEstimateBytes() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return uint32(len(t.data.entries)) * t.layout.entryBytes, nil
}

func (t *SparseTable) KnowsExactSize() (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	return true, nil
}

func (t *SparseTable) Facts() ([]Fact, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	r := make([]Fact, len(t.data.entries))
	for i, e := range t.data.entries {
		r[i] = t.layout.unpack(e)
	}
	return r, nil
}
func (t *SparseTable) Clone() (*SparseTable, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	r := &SparseTable{signature: t.signature, layout: t.layout, data: t.data.clone(), functional: t.functional}
	return r, nil
}
func (t *SparseTable) Reset() error {
	if e := t.open(); e != nil {
		return e
	}
	t.data.reset()
	return nil
}
func (t *SparseTable) Close() error {
	if t == nil {
		return ErrTable
	}
	// sparse_table::deallocate is sparse_table_plugin::recycle, whose
	// verbose_action _va("recycle", 2) wraps the reset.
	var err error
	if t.data != nil {
		var action base.VerboseAction
		action, err = base.StartVerboseAction("recycle", 2)
		if err == nil {
			err = action.End()
		}
	}
	t.data = nil
	return err
}

// sparseGetKeyIndexerVerbose is sparse_table::get_key_indexer's
// verbose_action _va("get_key_indexer") around the index lookup.
func sparseGetKeyIndexerVerbose() error {
	action, err := base.StartVerboseAction("get_key_indexer", base.VerboseActionDefaultLevel)
	if err != nil {
		return err
	}
	return action.End()
}
