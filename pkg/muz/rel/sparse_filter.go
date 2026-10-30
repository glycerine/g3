package rel

func (t *SparseTable) SelectEqualAndProject(value Element, column int) (*SparseTable, error) {
	if column < 0 || column >= t.signature.Len() {
		return nil, ErrTable
	}
	out, e := t.Project([]int{column})
	if e != nil {
		return nil, e
	}
	out.Reset()
	facts, _ := t.Facts()
	for _, f := range facts {
		if f[column] == value {
			g := append(Fact(nil), f[:column]...)
			g = append(g, f[column+1:]...)
			out.Add(g)
		}
	}
	return out, nil
}
func (t *SparseTable) FilterNegation(negative *SparseTable, tCols, nCols []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := negative.open(); err != nil {
		return err
	}
	if err := validateColumns(t.signature, negative.signature, tCols, nCols); err != nil {
		return err
	}
	if sparseJoinInvolvesFunctional(t, negative, tCols, nCols) {
		// The sparse plugin refuses indexes on functional columns, so the
		// relation manager falls back to its generic row filter, which removes
		// the collected rows in table iteration order.
		facts, err := t.Facts()
		if err != nil {
			return err
		}
		negated, err := negative.Facts()
		if err != nil {
			return err
		}
		remove := make([]Fact, 0)
		for _, fact := range facts {
			for _, excluded := range negated {
				match := true
				for i := range tCols {
					if fact[tCols[i]] != excluded[nCols[i]] {
						match = false
						break
					}
				}
				if match {
					remove = append(remove, fact)
					break
				}
			}
		}
		_, err = t.RemoveFacts(remove)
		return err
	}
	_, err := t.FilterNegationFunctional(negative, tCols, nCols)
	return err
}
