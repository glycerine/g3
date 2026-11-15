package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (t *SparseTable) Union(source, delta *SparseTable) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	if delta != nil {
		if err := delta.open(); err != nil {
			return err
		}
	}
	if !sparseSameFunctionalSignature(t, source) || (delta != nil && !sparseSameFunctionalSignature(t, delta)) {
		return ErrTable
	}
	// union_fn::operator()'s verbose_action _va("union").
	action, e := base.StartVerboseAction("union", base.VerboseActionDefaultLevel)
	if e != nil {
		return e
	}
	defer func() { resultErr = errors.Join(resultErr, action.End()) }()
	facts, e := source.Facts()
	if e != nil {
		return e
	}
	for _, f := range facts {
		added, e := sparseVerboseAdd(t, f)
		if e != nil {
			return e
		}
		if added && delta != nil {
			if _, e = sparseVerboseAdd(delta, f); e != nil {
				return e
			}
		}
	}
	return nil
}

// sparseVerboseAdd is sparse_table::add_fact(const char*), whose
// verbose_action _va("add_fact", 10) wraps each row union_fn copies.
func sparseVerboseAdd(t *SparseTable, f Fact) (added bool, resultErr error) {
	action, err := base.StartVerboseAction("add_fact", 10)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, action.End()) }()
	return t.Add(f)
}
