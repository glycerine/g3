package rel

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
	t.data = nil
	return nil
}
