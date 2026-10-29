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
	if e := validateColumns(t.signature, negative.signature, tCols, nCols); e != nil {
		return e
	}
	facts, _ := t.Facts()
	neg, _ := negative.Facts()
	for _, f := range facts {
		matched := false
		for _, n := range neg {
			same := true
			for i := range tCols {
				if f[tCols[i]] != n[nCols[i]] {
					same = false
					break
				}
			}
			if same {
				matched = true
				break
			}
		}
		if matched {
			t.Remove(f)
		}
	}
	return nil
}
