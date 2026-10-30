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
	// Match sparse_table_plugin::join_project_fn: for joins, scan the smaller
	// table outside and index the larger one; for a product, keep the larger
	// table outside. Preserve the original left/right result column order when
	// the scan roles are swapped.
	swapped := (len(af) > len(bf)) == (len(left) != 0)
	if swapped {
		af, bf = bf, af
		left, right = right, left
	}
	for _, outer := range af {
		for _, inner := range bf {
			match := true
			for i := range left {
				if outer[left[i]] != inner[right[i]] {
					match = false
					break
				}
			}
			if match {
				var row Fact
				if swapped {
					row = append(append(Fact(nil), inner...), outer...)
				} else {
					row = append(append(Fact(nil), outer...), inner...)
				}
				_, e = out.Add(row)
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
