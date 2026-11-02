package rel

// Source: src/muz/rel/dl_check_table.cpp query-reached check_table operations.

import (
	"errors"
	"fmt"
)

// FiniteCheckedPair owns independently instantiated concrete primary and
// checker tables and compares their observable results after every operation.
type FiniteCheckedPair struct {
	primary FiniteTable
	checker FiniteTable
	closed  bool
}

func adoptFiniteCheckedPair(primary, checker FiniteTable) (_ *FiniteCheckedPair, resultErr error) {
	if primary == nil || checker == nil {
		return nil, ErrCheckTable
	}
	result := &FiniteCheckedPair{primary: primary, checker: checker}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	primarySignature, err := primary.Signature()
	if err != nil {
		return nil, err
	}
	checkerSignature, err := checker.Signature()
	if err != nil || !primarySignature.equal(checkerSignature) {
		return nil, errors.Join(err, ErrCheckTable)
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
	default:
		return nil, fmt.Errorf("%w: finite checker plugin %s", ErrTable, kind)
	}
}

// NewFiniteCheckedPair clones source as the selected implementation and
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

func (p *FiniteCheckedPair) open() error {
	if p == nil || p.closed || p.primary == nil || p.checker == nil {
		return ErrCheckTable
	}
	return nil
}

func (p *FiniteCheckedPair) verify(objective string) error {
	if err := p.open(); err != nil {
		return err
	}
	a, err := p.primary.Facts()
	if err != nil {
		return err
	}
	b, err := p.checker.Facts()
	if err != nil {
		return err
	}
	if len(a) != len(b) {
		return fmt.Errorf("%w: %s row counts differ", ErrCheckTable, objective)
	}
	for _, fact := range a {
		found, err := p.checker.Contains(fact)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: %s checker misses %v", ErrCheckTable, objective, fact)
		}
	}
	for _, fact := range b {
		found, err := p.primary.Contains(fact)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: %s primary misses %v", ErrCheckTable, objective, fact)
		}
	}
	return nil
}

func (p *FiniteCheckedPair) Signature() (Signature, error) {
	if err := p.open(); err != nil {
		return Signature{}, err
	}
	return p.primary.Signature()
}
func (p *FiniteCheckedPair) Facts() ([]Fact, error) {
	if err := p.verify("facts"); err != nil {
		return nil, err
	}
	return p.primary.Facts()
}
func (p *FiniteCheckedPair) Add(fact Fact) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	a, err := p.primary.Add(fact)
	if err != nil {
		return false, err
	}
	b, err := p.checker.Add(fact)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, fmt.Errorf("%w: insertion results differ", ErrCheckTable)
	}
	return a, p.verify("add")
}
func (p *FiniteCheckedPair) Contains(fact Fact) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	a, err := p.primary.Contains(fact)
	if err != nil {
		return false, err
	}
	b, err := p.checker.Contains(fact)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, fmt.Errorf("%w: membership results differ", ErrCheckTable)
	}
	return a, nil
}
func (p *FiniteCheckedPair) Len() (int, error) {
	if err := p.verify("size"); err != nil {
		return 0, err
	}
	return p.primary.Len()
}
func (p *FiniteCheckedPair) SizeEstimateRows() (uint32, error) {
	rows, err := p.Len()
	return uint32(rows), err
}
func (p *FiniteCheckedPair) SizeEstimateBytes() (uint32, error) {
	if err := p.verify("size estimate"); err != nil {
		return 0, err
	}
	switch primary := p.primary.(type) {
	case *HashTable:
		return primary.SizeEstimateBytes()
	case *SparseTable:
		return primary.SizeEstimateBytes()
	default:
		return 0, ErrCheckTable
	}
}
func (p *FiniteCheckedPair) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	return errors.Join(p.primary.Reset(), p.checker.Reset())
}

// EmptyLike creates empty primary and checker instances of the same concrete
// kinds. It is the checked equivalent of table_plugin::mk_empty.
func (p *FiniteCheckedPair) EmptyLike() (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	primary, err := emptyFiniteLike(p.primary)
	if err != nil {
		return nil, err
	}
	checker, err := emptyFiniteLike(p.checker)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

// Complement implements the live finite complement boundary used by checked
// full construction. Wider complements remain compiler-decomposed.
func (p *FiniteCheckedPair) Complement() (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	signature, err := p.Signature()
	if err != nil {
		return nil, err
	}
	if signature.Len() > 1 {
		return nil, fmt.Errorf("%w: complement requires compiler decomposition", ErrCheckTable)
	}
	result, err := p.EmptyLike()
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = result.Close()
		}
	}()
	if signature.Len() == 0 {
		found, err := p.Contains(Fact{})
		if err != nil {
			return nil, err
		}
		if !found {
			if _, err = result.Add(Fact{}); err != nil {
				return nil, err
			}
		}
	} else {
		domain, err := signature.Domain(0)
		if err != nil {
			return nil, err
		}
		for value := uint64(0); value < domain; value++ {
			fact := Fact{Element(value)}
			found, err := p.Contains(fact)
			if err != nil {
				return nil, err
			}
			if !found {
				if _, err = result.Add(fact); err != nil {
					return nil, err
				}
			}
		}
	}
	failed = false
	return result, nil
}

// FullLike creates a checked full relation of the same concrete pair kinds.
func (p *FiniteCheckedPair) FullLike() (*FiniteCheckedPair, error) {
	empty, err := p.EmptyLike()
	if err != nil {
		return nil, err
	}
	full, fullErr := empty.Complement()
	closeErr := empty.Close()
	if fullErr != nil || closeErr != nil {
		if full != nil {
			closeErr = errors.Join(closeErr, full.Close())
		}
		return nil, errors.Join(fullErr, closeErr)
	}
	return full, nil
}

func (p *FiniteCheckedPair) Join(other *FiniteCheckedPair, left, right []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil || other == nil || other.open() != nil {
		return nil, ErrCheckTable
	}
	primary, err := finiteTableJoin(p.primary, other.primary, left, right)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableJoin(p.checker, other.checker, left, right)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) JoinProject(other *FiniteCheckedPair, left, right, removed []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil || other == nil || other.open() != nil {
		return nil, ErrCheckTable
	}
	primary, err := finiteTableJoinProject(p.primary, other.primary, left, right, removed)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableJoinProject(p.checker, other.checker, left, right, removed)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) Project(removed []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	primary, err := finiteTableProject(p.primary, removed)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableProject(p.checker, removed)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) RenameCycle(cycle []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	primary, err := finiteTableRename(p.primary, cycle)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableRename(p.checker, cycle)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) SelectEqualAndProject(value Element, column int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	primary, err := finiteTableSelectProject(p.primary, value, column)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableSelectProject(p.checker, value, column)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}

func (p *FiniteCheckedPair) FilterEqual(value Element, column int) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := finiteTableFilterEqual(p.primary, value, column); err != nil {
		return err
	}
	if err := finiteTableFilterEqual(p.checker, value, column); err != nil {
		return err
	}
	return p.verify("filter equal")
}

func (p *FiniteCheckedPair) FilterIdentical(columns []int) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := finiteTableFilterIdentical(p.primary, columns); err != nil {
		return err
	}
	if err := finiteTableFilterIdentical(p.checker, columns); err != nil {
		return err
	}
	return p.verify("filter identical")
}

func (p *FiniteCheckedPair) FilterByNegation(negative *FiniteCheckedPair, targetColumns, negativeColumns []int) error {
	if err := p.open(); err != nil || negative == nil || negative.open() != nil {
		return ErrCheckTable
	}
	if err := finiteTableFilterNegation(p.primary, negative.primary, targetColumns, negativeColumns); err != nil {
		return err
	}
	if err := finiteTableFilterNegation(p.checker, negative.checker, targetColumns, negativeColumns); err != nil {
		return err
	}
	return p.verify("filter negation")
}

// UnionFrom executes the reached check_table union semantics and accumulates
// only newly inserted rows in delta. Existing delta rows are preserved.
func (p *FiniteCheckedPair) UnionFrom(source, delta *FiniteCheckedPair) error {
	if err := p.open(); err != nil {
		return err
	}
	if source == nil {
		return ErrCheckTable
	}
	rows, err := source.Facts()
	if err != nil {
		return err
	}
	for _, fact := range rows {
		added, err := p.Add(fact)
		if err != nil {
			return err
		}
		if added && delta != nil {
			if _, err = delta.Add(fact); err != nil {
				return err
			}
		}
	}
	if err = p.verify("union target"); err != nil {
		return err
	}
	if delta != nil {
		return delta.verify("union delta")
	}
	return nil
}

func (p *FiniteCheckedPair) CloneFinite() (FiniteTable, error) {
	if err := p.verify("clone"); err != nil {
		return nil, err
	}
	primary, err := p.primary.CloneFinite()
	if err != nil {
		return nil, err
	}
	checker, err := p.checker.CloneFinite()
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return &FiniteCheckedPair{primary: primary, checker: checker}, nil
}
func (p *FiniteCheckedPair) Close() error {
	if p == nil {
		return ErrCheckTable
	}
	if p.closed {
		return nil
	}
	err := errors.Join(p.primary.Close(), p.checker.Close())
	p.primary, p.checker, p.closed = nil, nil, true
	return err
}

func (p *FiniteCheckedPair) ConcreteKinds() (primary, checker string, err error) {
	if err = p.open(); err != nil {
		return "", "", err
	}
	kind := func(table FiniteTable) string {
		switch table.(type) {
		case *HashTable:
			return "hashtable"
		case *SparseTable:
			return "sparse"
		default:
			return fmt.Sprintf("%T", table)
		}
	}
	return kind(p.primary), kind(p.checker), nil
}

var _ FiniteTable = (*FiniteCheckedPair)(nil)
