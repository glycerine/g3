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
	if a.functional != 0 || b.functional != 0 {
		if !sparseJoinInvolvesFunctional(a, b, left, right) {
			return a.JoinFunctional(b, left, right)
		}
		// The sparse plugin declines functional-column indexes. The source
		// relation manager then uses its generic nested-loop table join.
		sig, functional, err := sparseJoinFunctionalSignature(a, b)
		if err != nil {
			return nil, err
		}
		out, err := NewSparseTable(sig, functional)
		if err != nil {
			return nil, err
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
					_, err = out.Add(sparseFunctionalJoinFact(x, y, a.signature.Len()-a.functional, b.signature.Len()-b.functional))
					if err != nil {
						_ = out.Close()
						return nil, err
					}
				}
			}
		}
		return out, nil
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
