package rel

func (a *SparseTable) Join(b *SparseTable, left, right []int) (*SparseTable, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	if e := b.open(); e != nil {
		return nil, e
	}
	if e := validateColumns(a.signature, b.signature, left, right); e != nil {
		return nil, e
	}
	domains := append(a.signature.Domains(), b.signature.Domains()...)
	sig, e := NewSignature(domains)
	if e != nil {
		return nil, e
	}
	out, e := NewSparseTable(sig, 0)
	if e != nil {
		return nil, e
	}
	af, _ := a.Facts()
	bf, _ := b.Facts()
	for _, x := range af {
		for _, y := range bf {
			match := true
			for i := range left {
				if x[left[i]] != y[right[i]] {
					match = false
					break
				}
			}
			if match {
				_, e = out.Add(append(append(Fact(nil), x...), y...))
				if e != nil {
					return nil, e
				}
			}
		}
	}
	return out, nil
}
func (a *SparseTable) JoinProject(b *SparseTable, left, right, removed []int) (*SparseTable, error) {
	joined, e := a.Join(b, left, right)
	if e != nil {
		return nil, e
	}
	defer joined.Close()
	return joined.Project(removed)
}
