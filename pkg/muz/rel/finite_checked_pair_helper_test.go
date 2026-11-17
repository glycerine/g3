package rel

import (
	"errors"
	"fmt"
)

// NewFiniteCheckedPair (a test helper) clones source as the selected implementation and
// creates an independent configured checker seeded with the same rows.
func NewFiniteCheckedPair(source FiniteTable, checkerKind string) (_ *FiniteCheckedPair, resultErr error) {
	if source == nil {
		return nil, ErrTable
	}
	primary, err := source.CloneFinite()
	if err != nil {
		return nil, err
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	checker, err := newFiniteTableKind(checkerKind, signature)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	result := &FiniteCheckedPair{primary: primary, checker: checker}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := source.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = checker.Add(fact); err != nil {
			return nil, err
		}
	}
	if err = result.verify("construction"); err != nil {
		return nil, err
	}
	return result, nil
}

func newFiniteTableKind(kind string, signature Signature) (FiniteTable, error) {
	switch kind {
	case "hashtable", "tr_hashtable":
		return NewHashTable(signature), nil
	case "sparse", "tr_sparse":
		return NewSparseTable(signature, 0)
	case "bitvector", "tr_bitvector":
		return NewBitvectorTable(signature)
	default:
		return nil, fmt.Errorf("%w: finite checker plugin %s", ErrTable, kind)
	}
}
