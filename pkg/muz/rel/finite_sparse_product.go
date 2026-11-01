package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp/.h, finite sparse-table
// outer relation with sparse-table inner relations.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

// FiniteSparseProduct maps each finite table key to a sparse relation over the
// remaining finite columns. It is the concrete table/table product used by the
// retained relation tests; it borrows its manager and sort handles.
type FiniteSparseProduct struct {
	manager                 *RelationManager
	sorts                   []ast.ASTHandle
	tableColumns, innerCols []int
	innerSignature          Signature
	table                   *SparseTable
	inner                   []*SparseTable
	available               []int
}

func NewFiniteSparseProduct(manager *RelationManager, sorts []ast.ASTHandle, tableColumns []bool) (*FiniteSparseProduct, error) {
	if manager == nil || manager.open() != nil || tableColumns != nil && len(tableColumns) != len(sorts) {
		return nil, ast.ErrASTDomain
	}
	p := &FiniteSparseProduct{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...)}
	var outerDomains, innerDomains []uint64
	for i, sort := range sorts {
		domain, err := manager.sortDomain(sort)
		if err != nil {
			return nil, err
		}
		isTable := tableColumns == nil || tableColumns[i]
		if isTable {
			p.tableColumns = append(p.tableColumns, i)
			outerDomains = append(outerDomains, domain)
		} else {
			p.innerCols = append(p.innerCols, i)
			innerDomains = append(innerDomains, domain)
		}
	}
	outerDomains = append(outerDomains, math.MaxInt32)
	outerSig, err := NewSignature(outerDomains)
	if err != nil {
		return nil, err
	}
	p.table, err = NewSparseTable(outerSig, 1)
	if err != nil {
		return nil, err
	}
	innerSig, err := NewSignature(innerDomains)
	if err != nil {
		return nil, errors.Join(err, p.table.Close())
	}
	// Keep the signature with a prototype so zero-column inner relations still
	// have a concrete, reusable table layout.
	p.innerSignature = innerSig
	return p, nil
}

func (p *FiniteSparseProduct) open() error {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.table == nil || p.table.open() != nil {
		return ErrTableRelation
	}
	return nil
}

func (p *FiniteSparseProduct) Close() error {
	if p == nil || p.table == nil {
		return nil
	}
	err := p.table.Close()
	for i, inner := range p.inner {
		if inner != nil {
			err = errors.Join(err, inner.Close())
			p.inner[i] = nil
		}
	}
	p.table, p.manager, p.sorts, p.tableColumns, p.innerCols, p.inner, p.available = nil, nil, nil, nil, nil, nil, nil
	return err
}

func (p *FiniteSparseProduct) split(values []ast.ASTHandle) (Fact, Fact, error) {
	if err := p.open(); err != nil {
		return nil, nil, err
	}
	if len(values) != len(p.sorts) {
		return nil, nil, ast.ErrASTDomain
	}
	key := make(Fact, len(p.tableColumns)+1)
	inner := make(Fact, len(p.innerCols))
	for i, column := range p.tableColumns {
		value, err := p.manager.EncodeValue(p.sorts[column], values[column])
		if err != nil {
			return nil, nil, err
		}
		key[i] = value
	}
	for i, column := range p.innerCols {
		value, err := p.manager.EncodeValue(p.sorts[column], values[column])
		if err != nil {
			return nil, nil, err
		}
		inner[i] = value
	}
	return key, inner, nil
}

func (p *FiniteSparseProduct) AddFact(values []ast.ASTHandle) (resultErr error) {
	key, innerFact, err := p.split(values)
	if err != nil {
		return err
	}
	found, err := p.table.FetchFact(key)
	if err != nil {
		return err
	}
	var inner *SparseTable
	index := -1
	if found {
		index = int(key[len(key)-1])
		if index < 0 || index >= len(p.inner) || p.inner[index] == nil {
			return ErrTableRelation
		}
		inner, err = p.inner[index].Clone()
	} else {
		inner, err = NewSparseTable(p.innerSignature, 0)
	}
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, inner.Close())
		}
	}()
	if _, err = inner.Add(innerFact); err != nil {
		return err
	}
	if found {
		old := p.inner[index]
		p.inner[index] = inner
		keep = true
		return old.Close()
	}
	if len(p.available) != 0 {
		index = p.available[len(p.available)-1]
		p.available = p.available[:len(p.available)-1]
	} else {
		if uint64(len(p.inner)) >= math.MaxInt32 {
			return ErrTableRelation
		}
		index = len(p.inner)
		p.inner = append(p.inner, nil)
	}
	key[len(key)-1] = Element(index)
	if err = p.table.EnsureFact(key); err != nil {
		if index == len(p.inner)-1 && p.inner[index] == nil {
			p.inner = p.inner[:index]
		} else {
			p.available = append(p.available, index)
		}
		return err
	}
	p.inner[index] = inner
	keep = true
	return nil
}

func (p *FiniteSparseProduct) ContainsFact(values []ast.ASTHandle) (bool, error) {
	key, innerFact, err := p.split(values)
	if err != nil {
		return false, err
	}
	found, err := p.table.FetchFact(key)
	if err != nil || !found {
		return false, err
	}
	index := uint64(key[len(key)-1])
	if index >= uint64(len(p.inner)) || p.inner[index] == nil {
		return false, ErrTableRelation
	}
	return p.inner[index].Contains(innerFact)
}

// Empty reports whether the product has any live inner facts. Empty inner
// relations are removed along with their outer keys, matching the native
// finite-product garbage collection performed by empty().
func (p *FiniteSparseProduct) Empty() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	keys, err := p.table.Facts()
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		index := int(key[len(key)-1])
		if index < 0 || index >= len(p.inner) || p.inner[index] == nil {
			return false, ErrTableRelation
		}
		n, err := p.inner[index].Len()
		if err != nil {
			return false, err
		}
		if n != 0 {
			continue
		}
		if _, err = p.table.Remove(key); err != nil {
			return false, err
		}
		if err = p.inner[index].Close(); err != nil {
			return false, err
		}
		p.inner[index] = nil
		p.available = append(p.available, index)
	}
	n, err := p.table.Len()
	return n == 0, err
}

func (p *FiniteSparseProduct) Clone() (*FiniteSparseProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	table, err := p.table.Clone()
	if err != nil {
		return nil, err
	}
	copy := &FiniteSparseProduct{
		manager: p.manager, sorts: append([]ast.ASTHandle(nil), p.sorts...),
		tableColumns: append([]int(nil), p.tableColumns...), innerCols: append([]int(nil), p.innerCols...),
		innerSignature: p.innerSignature, table: table, inner: make([]*SparseTable, len(p.inner)),
		available: append([]int(nil), p.available...),
	}
	for i, inner := range p.inner {
		if inner != nil {
			copy.inner[i], err = inner.Clone()
			if err != nil {
				return nil, errors.Join(err, copy.Close())
			}
		}
	}
	return copy, nil
}
