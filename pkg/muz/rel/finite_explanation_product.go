package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp and
// src/muz/rel/dl_mk_explanations.cpp, the concrete finite-table/explanation
// product used when explanations are stored per fact.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

// FiniteExplanationProduct maps a finite tuple to its single explanation row.
// It borrows the relation manager and finite sort handles and owns the sparse
// table and explanation relations.
type FiniteExplanationProduct struct {
	manager     *RelationManager
	finiteSorts []ast.ASTHandle
	table       *SparseTable
	inner       []*ExplanationRelation
}

func NewFiniteExplanationProduct(manager *RelationManager, finiteSorts []ast.ASTHandle) (*FiniteExplanationProduct, error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	domains := make([]uint64, 0, len(finiteSorts)+1)
	for _, sort := range finiteSorts {
		domain, err := manager.sortDomain(sort)
		if err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	domains = append(domains, math.MaxInt32)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	table, err := NewSparseTable(signature, 1)
	if err != nil {
		return nil, err
	}
	return &FiniteExplanationProduct{manager: manager, finiteSorts: append([]ast.ASTHandle(nil), finiteSorts...), table: table}, nil
}

func (p *FiniteExplanationProduct) open() error {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.table == nil || p.table.open() != nil {
		return ErrTableRelation
	}
	return nil
}

func (p *FiniteExplanationProduct) encode(values []ast.ASTHandle) (Fact, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(values) != len(p.finiteSorts) {
		return nil, ast.ErrASTDomain
	}
	key := make(Fact, len(values)+1)
	for i, value := range values {
		encoded, err := p.manager.EncodeValue(p.finiteSorts[i], value)
		if err != nil {
			return nil, err
		}
		key[i] = encoded
	}
	return key, nil
}

// AddFact inserts a finite tuple and its rule-sort explanation. The source
// explanation relation accepts one fact per finite key.
func (p *FiniteExplanationProduct) AddFact(values []ast.ASTHandle, explanation ast.ASTHandle) error {
	key, err := p.encode(values)
	if err != nil {
		return err
	}
	return p.addEncoded(key, explanation)
}

func (p *FiniteExplanationProduct) addEncoded(key Fact, explanation ast.ASTHandle) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if len(key) != len(p.finiteSorts)+1 {
		return ast.ErrASTDomain
	}
	found, err := p.table.FetchFact(key)
	if err != nil {
		return err
	}
	if found || uint64(len(p.inner)) >= math.MaxInt32 {
		return ast.ErrASTDomain
	}
	sort, err := p.manager.manager.MkDatalogRuleSort()
	if err != nil {
		return err
	}
	inner, err := NewExplanationRelation(p.manager.manager, []ast.ASTHandle{sort})
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, inner.Close())
		}
	}()
	if err = inner.AddFact([]ast.ASTHandle{explanation}); err != nil {
		return err
	}
	key[len(key)-1] = Element(len(p.inner))
	if err = p.table.EnsureFact(key); err != nil {
		return err
	}
	p.inner = append(p.inner, inner)
	keep = true
	return nil
}

// Explanation returns a borrowed explanation for a finite tuple.
func (p *FiniteExplanationProduct) Explanation(values []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	key, err := p.encode(values)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	found, err := p.table.FetchFact(key)
	if err != nil || !found {
		return ast.ASTHandle{}, false, err
	}
	index := uint64(key[len(key)-1])
	if index >= uint64(len(p.inner)) || p.inner[index] == nil {
		return ast.ASTHandle{}, false, ErrTableRelation
	}
	data, err := p.inner[index].Data()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(data) != 1 {
		return ast.ASTHandle{}, false, ErrTableRelation
	}
	return data[0], true, nil
}

func (p *FiniteExplanationProduct) Len() (int, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.table.Len()
}

func (p *FiniteExplanationProduct) Close() (result error) {
	if p == nil || p.table == nil {
		return nil
	}
	for i := len(p.inner) - 1; i >= 0; i-- {
		if p.inner[i] != nil {
			if err := p.inner[i].Close(); err != nil {
				result = errors.Join(result, err)
			} else {
				p.inner[i] = nil
			}
		}
	}
	if result != nil {
		return result
	}
	result = p.table.Close()
	if result == nil {
		p.table, p.manager, p.finiteSorts, p.inner = nil, nil, nil, nil
	}
	return result
}
