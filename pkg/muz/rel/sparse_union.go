package rel

func (t *SparseTable) Union(source, delta *SparseTable) error {
	if !t.signature.equal(source.signature) {
		return ErrTable
	}
	facts, e := source.Facts()
	if e != nil {
		return e
	}
	for _, f := range facts {
		added, e := t.Add(f)
		if e != nil {
			return e
		}
		if added && delta != nil {
			if !delta.signature.equal(t.signature) {
				return ErrTable
			}
			if _, e = delta.Add(f); e != nil {
				return e
			}
		}
	}
	return nil
}
