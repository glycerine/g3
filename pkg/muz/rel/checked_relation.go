package rel

import "fmt"

type CheckedRelation struct {
	signature Signature
	primary   *HashTable
	checker   *SparseTable
	closed    bool
}

func NewCheckedRelation(signature Signature) (*CheckedRelation, error) {
	s, e := NewSparseTable(signature, 0)
	if e != nil {
		return nil, e
	}
	r := &CheckedRelation{signature: signature, primary: NewHashTable(signature), checker: s}
	return r, nil
}
func (r *CheckedRelation) open() error {
	if r == nil || r.closed || r.primary == nil || r.checker == nil {
		return ErrTable
	}
	return nil
}
func (r *CheckedRelation) Close() error {
	if r == nil {
		return ErrTable
	}
	if r.closed {
		return nil
	}
	e := r.checker.Close()
	r.primary = nil
	r.checker = nil
	r.closed = true
	return e
}
func (r *CheckedRelation) fail(objective string) error {
	return fmt.Errorf("%w: check_relation %s mismatch", ErrTable, objective)
}
func (r *CheckedRelation) Len() (int, error) {
	if e := r.open(); e != nil {
		return 0, e
	}
	n, e := r.primary.Len()
	if e != nil {
		return 0, e
	}
	m, e := r.checker.Len()
	if e != nil {
		return 0, e
	}
	if n != m {
		return 0, r.fail("size")
	}
	return n, nil
}
func (r *CheckedRelation) Facts() ([]Fact, error) {
	if e := r.verify("facts"); e != nil {
		return nil, e
	}
	return r.primary.Facts()
}
