package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp/.h, concrete sparse-table
// and interval-relation instantiation.

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// FiniteIntervalProduct maps each tuple of finite columns to an interval
// abstraction over the remaining arithmetic columns. It borrows the manager
// and sort handles, and owns the table and interval relations.
type FiniteIntervalProduct struct {
	manager                       *RelationManager
	sorts                         []ast.ASTHandle
	tableColumns, intervalColumns []int
	table                         *SparseTable
	intervals                     []*IntervalRelation
	available                     []int
}

// NewFiniteIntervalProduct selects every table-compatible sort when columns is
// nil; an explicit mask may instead leave arithmetic columns in the interval.
func NewFiniteIntervalProduct(manager *RelationManager, sorts []ast.ASTHandle, columns []bool) (*FiniteIntervalProduct, error) {
	if manager == nil || manager.open() != nil || columns != nil && len(columns) != len(sorts) {
		return nil, ast.ErrASTDomain
	}
	m := manager.manager
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	realSort, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		return nil, err
	}
	intSort, err := m.MkFamilySort(family, 1, nil)
	if err != nil {
		return nil, err
	}
	p := &FiniteIntervalProduct{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...)}
	var domains []uint64
	for i, sort := range sorts {
		kind, err := m.GetKind(sort)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTSort {
			return nil, ast.ErrASTDomain
		}
		domain, domainErr := manager.sortDomain(sort)
		asTable := domainErr == nil
		if columns != nil {
			asTable = columns[i]
		}
		if asTable {
			if domainErr != nil {
				return nil, domainErr
			}
			p.tableColumns = append(p.tableColumns, i)
			domains = append(domains, domain)
		} else {
			if sort != intSort && sort != realSort {
				return nil, fmt.Errorf("%w: interval columns must be arithmetic", ErrTableRelation)
			}
			p.intervalColumns = append(p.intervalColumns, i)
		}
	}
	// The source stores inner relation indexes in an INT_MAX-sized functional column.
	domains = append(domains, math.MaxInt32)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	p.table, err = NewSparseTable(signature, 1)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *FiniteIntervalProduct) open() error {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.table == nil || p.table.open() != nil {
		return ErrTableRelation
	}
	return nil
}

func (p *FiniteIntervalProduct) splitFact(values []ast.ASTHandle) (Fact, []*big.Rat, error) {
	if err := p.open(); err != nil {
		return nil, nil, err
	}
	if len(values) != len(p.sorts) {
		return nil, nil, ast.ErrASTDomain
	}
	key := make(Fact, len(p.tableColumns)+1)
	for i, col := range p.tableColumns {
		value, err := p.manager.EncodeValue(p.sorts[col], values[col])
		if err != nil {
			return nil, nil, err
		}
		key[i] = value
	}
	numbers := make([]*big.Rat, len(p.intervalColumns))
	for i, col := range p.intervalColumns {
		sort, err := p.manager.manager.GetSort(values[col])
		if err != nil {
			return nil, nil, err
		}
		if sort != p.sorts[col] {
			return nil, nil, ast.ErrASTDomain
		}
		text, _, matched, err := p.manager.manager.ArithmeticNumeralText(values[col])
		if err != nil {
			return nil, nil, err
		}
		if !matched {
			return nil, nil, ast.ErrASTDomain
		}
		numbers[i], matched = new(big.Rat).SetString(text)
		if !matched {
			return nil, nil, ast.ErrASTDomain
		}
	}
	return key, numbers, nil
}

func cloneIntervalRelation(source *IntervalRelation) *IntervalRelation {
	values := source.values
	copy := newIntervalColumns(len(values.columns), values.empty)
	copy.parent = append(copy.parent[:0], values.parent...)
	copy.size = append(copy.size[:0], values.size...)
	for i, interval := range values.columns {
		lo, loOpen, hi, hiOpen := interval.Bounds()
		copy.columns[i] = smtcore.NewRelationInterval(lo, loOpen, hi, hiOpen)
	}
	return &IntervalRelation{values: copy}
}

// AddFact creates a fresh inner relation for a new table key, and replaces an
// existing key's inner relation with a clone extended by the new fact.
func (p *FiniteIntervalProduct) AddFact(values []ast.ASTHandle) error {
	key, numbers, err := p.splitFact(values)
	if err != nil {
		return err
	}
	found, err := p.table.FetchFact(key)
	if err != nil {
		return err
	}
	inner := NewIntervalEmpty(uint32(len(numbers)))
	if found {
		index := uint64(key[len(key)-1])
		if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
			return ErrTableRelation
		}
		inner = cloneIntervalRelation(p.intervals[index])
	}
	if err = inner.AddFact(numbers); err != nil {
		return err
	}
	return p.storeInner(key, inner)
}

func (p *FiniteIntervalProduct) storeInner(key Fact, inner *IntervalRelation) error {
	if len(p.available) == 0 && uint64(len(p.intervals)) >= math.MaxInt32 {
		return ErrTableRelation
	}
	index := len(p.intervals)
	if len(p.available) != 0 {
		index = p.available[len(p.available)-1]
	}
	key[len(key)-1] = Element(index)
	if err := p.table.EnsureFact(key); err != nil {
		return err
	}
	if index == len(p.intervals) {
		p.intervals = append(p.intervals, inner)
	} else {
		p.available = p.available[:len(p.available)-1]
		p.intervals[index] = inner
	}
	return nil
}

func (p *FiniteIntervalProduct) ContainsFact(values []ast.ASTHandle) (bool, error) {
	key, numbers, err := p.splitFact(values)
	if err != nil {
		return false, err
	}
	found, err := p.table.FetchFact(key)
	if err != nil || !found {
		return false, err
	}
	index := uint64(key[len(key)-1])
	if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
		return false, ErrTableRelation
	}
	return p.intervals[index].ContainsFact(numbers), nil
}

// FilterEqual restricts a finite column in the table or an arithmetic column
// in each live interval, following the two source filter paths.
func (p *FiniteIntervalProduct) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if uint64(column) >= uint64(len(p.sorts)) {
		return ast.ErrASTDomain
	}
	for local, global := range p.tableColumns {
		if global != int(column) {
			continue
		}
		encoded, err := p.manager.EncodeValue(p.sorts[column], value)
		if err != nil {
			return err
		}
		rows, err := p.table.Facts()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row[local] != encoded {
				if _, err = p.table.Remove(row); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for local, global := range p.intervalColumns {
		if global != int(column) {
			continue
		}
		sort, err := p.manager.manager.GetSort(value)
		if err != nil {
			return err
		}
		if sort != p.sorts[column] {
			return ast.ErrASTDomain
		}
		text, _, matched, err := p.manager.manager.ArithmeticNumeralText(value)
		if err != nil {
			return err
		}
		if !matched {
			return ast.ErrASTDomain
		}
		number, ok := new(big.Rat).SetString(text)
		if !ok {
			return ast.ErrASTDomain
		}
		if err = p.garbageCollect(false); err != nil {
			return err
		}
		for _, inner := range p.intervals {
			if inner != nil {
				if err = inner.FilterEqual(uint32(local), number); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return ErrTableRelation
}

func (p *FiniteIntervalProduct) sameSpecification(other *FiniteIntervalProduct) bool {
	if other == nil || p.manager != other.manager || len(p.sorts) != len(other.sorts) || len(p.tableColumns) != len(other.tableColumns) {
		return false
	}
	for i := range p.sorts {
		if p.sorts[i] != other.sorts[i] {
			return false
		}
	}
	for i := range p.tableColumns {
		if p.tableColumns[i] != other.tableColumns[i] {
			return false
		}
	}
	return true
}

// Union merges aligned table keys and unions their interval relations. Delta,
// when supplied, accumulates the fresh interval deltas and source-only rows.
func (p *FiniteIntervalProduct) Union(source, delta *FiniteIntervalProduct) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil || !p.sameSpecification(source) {
		return ErrTableRelation
	}
	if delta != nil {
		if delta == p || delta == source || delta.open() != nil || !p.sameSpecification(delta) {
			return ErrTableRelation
		}
		if _, err := delta.Empty(); err != nil {
			return err
		}
	}
	var fresh *FiniteIntervalProduct
	if delta != nil {
		mask := make([]bool, len(p.sorts))
		for _, column := range p.tableColumns {
			mask[column] = true
		}
		var err error
		fresh, err = NewFiniteIntervalProduct(p.manager, p.sorts, mask)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, fresh.Close()) }()
	}
	rows, err := source.table.Facts()
	if err != nil {
		return err
	}
	for _, row := range rows {
		sourceIndex := uint64(row[len(row)-1])
		if sourceIndex >= uint64(len(source.intervals)) || source.intervals[sourceIndex] == nil {
			return ErrTableRelation
		}
		src := source.intervals[sourceIndex]
		key := cloneFact(row)
		key[len(key)-1] = 0
		found, err := p.table.FetchFact(key)
		if err != nil {
			return err
		}
		var merged, change *IntervalRelation
		if found {
			index := uint64(key[len(key)-1])
			if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
				return ErrTableRelation
			}
			merged = cloneIntervalRelation(p.intervals[index])
			if fresh != nil {
				change = NewIntervalEmpty(uint32(len(p.intervalColumns)))
				_, err = merged.UnionWithDelta(src, change, false)
			} else {
				_, err = merged.Union(src, false)
			}
			if err != nil {
				return err
			}
		} else {
			merged = cloneIntervalRelation(src)
			if fresh != nil {
				change = cloneIntervalRelation(src)
			}
		}
		if err = p.storeInner(key, merged); err != nil {
			return err
		}
		if fresh != nil {
			if err = fresh.storeInner(key, change); err != nil {
				return err
			}
		}
	}
	if fresh != nil {
		return delta.Union(fresh, nil)
	}
	return nil
}

// garbageCollect drops inner relations no longer referenced by the table and,
// when requested, removes rows referring to empty inner relations.
func (p *FiniteIntervalProduct) garbageCollect(removeEmpty bool) error {
	rows, err := p.table.Facts()
	if err != nil {
		return err
	}
	live := make([]bool, len(p.intervals))
	for _, row := range rows {
		index := uint64(row[len(row)-1])
		if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
			return ErrTableRelation
		}
		if removeEmpty && p.intervals[index].Empty() {
			if _, err = p.table.Remove(row); err != nil {
				return err
			}
		} else {
			live[index] = true
		}
	}
	for i, interval := range p.intervals {
		if interval != nil && !live[i] {
			p.intervals[i] = nil
			p.available = append(p.available, i)
		}
	}
	if len(p.available) == len(p.intervals) {
		p.available = nil
		p.intervals = nil
	}
	return nil
}

func (p *FiniteIntervalProduct) Empty() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if err := p.garbageCollect(true); err != nil {
		return false, err
	}
	n, err := p.table.Len()
	return n == 0, err
}

func (p *FiniteIntervalProduct) Clone() (*FiniteIntervalProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	table, err := p.table.Clone()
	if err != nil {
		return nil, err
	}
	copy := &FiniteIntervalProduct{manager: p.manager, sorts: append([]ast.ASTHandle(nil), p.sorts...), tableColumns: append([]int(nil), p.tableColumns...), intervalColumns: append([]int(nil), p.intervalColumns...), table: table, intervals: make([]*IntervalRelation, len(p.intervals)), available: append([]int(nil), p.available...)}
	for i, inner := range p.intervals {
		if inner != nil {
			copy.intervals[i] = cloneIntervalRelation(inner)
		}
	}
	return copy, nil
}

func (p *FiniteIntervalProduct) Close() error {
	if p == nil || p.table == nil {
		return nil
	}
	err := p.table.Close()
	p.table, p.manager, p.sorts, p.intervals, p.available = nil, nil, nil, nil, nil
	return err
}
