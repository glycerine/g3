package rel

func sparseJoinTraversal(leftFacts, rightFacts []Fact, leftColumns, rightColumns []int, emit func(Fact, Fact) error) error {
	match := func(left, right Fact, leftCols, rightCols []int) bool {
		for index := range leftCols {
			if left[leftCols[index]] != right[rightCols[index]] {
				return false
			}
		}
		return true
	}
	swapped := (len(leftFacts) > len(rightFacts)) == (len(leftColumns) != 0)
	if swapped {
		for _, right := range rightFacts {
			for _, left := range leftFacts {
				if match(left, right, leftColumns, rightColumns) {
					if err := emit(left, right); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	for _, left := range leftFacts {
		for _, right := range rightFacts {
			if match(left, right, leftColumns, rightColumns) {
				if err := emit(left, right); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

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
	if e = sparseJoinTraversal(af, bf, left, right, func(x, y Fact) error {
		_, addErr := out.Add(append(append(Fact(nil), x...), y...))
		return addErr
	}); e != nil {
		return nil, e
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
