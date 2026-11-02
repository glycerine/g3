package rel

// Source: src/muz/rel/dl_mk_explanations.cpp,
// translate_rel_level_relation's concrete finite-table and explanation sieves.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// RelationExplanationProduct keeps one explanation for an entire finite
// relation, alongside a private copy of its table. It borrows the manager and
// finite sort handles and owns the copied table and explanation relation.
type RelationExplanationProduct struct {
	manager     *RelationManager
	finiteSorts []ast.ASTHandle
	table       *HashTable
	explanation *ExplanationRelation
}

// NewRelationExplanationProduct constructs an empty finite/whole-explanation
// product. Its external signature is the finite predicate signature; the
// single Datalog-rule explanation column is internal to the product.
func NewRelationExplanationProduct(manager *RelationManager, sorts []ast.ASTHandle) (_ *RelationExplanationProduct, resultErr error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	domains := make([]uint64, len(sorts))
	for i, sort := range sorts {
		var err error
		domains[i], err = manager.sortDomain(sort)
		if err != nil {
			return nil, err
		}
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	p := &RelationExplanationProduct{manager: manager, finiteSorts: append([]ast.ASTHandle(nil), sorts...), table: NewHashTable(signature)}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	ruleSort, err := manager.manager.MkDatalogRuleSort()
	if err != nil {
		return nil, err
	}
	p.explanation, err = NewExplanationRelation(manager.manager, []ast.ASTHandle{ruleSort})
	if err != nil {
		return nil, err
	}
	return p, nil
}

func newRelationExplanationProduct(manager *RelationManager, sorts []ast.ASTHandle, source *HashTable, fact ast.ASTHandle) (_ *RelationExplanationProduct, resultErr error) {
	p, err := NewRelationExplanationProduct(manager, sorts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	if source != nil {
		if err = p.table.Union(source, nil); err != nil {
			return nil, err
		}
	}
	if err = p.explanation.AddFact([]ast.ASTHandle{fact}); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *RelationExplanationProduct) open() error {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.table == nil || p.explanation == nil {
		return ErrTableRelation
	}
	return nil
}

func (p *RelationExplanationProduct) FiniteSignature() (Signature, error) {
	if err := p.open(); err != nil {
		return Signature{}, err
	}
	return p.table.Signature()
}

// Explanation returns the relation-level explanation for a present tuple.
// The returned AST handle is borrowed from the product.
func (p *RelationExplanationProduct) Explanation(values []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(values) != len(p.finiteSorts) {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	key := make(Fact, len(values))
	for i, value := range values {
		encoded, err := p.manager.EncodeValue(p.finiteSorts[i], value)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		key[i] = encoded
	}
	found, err := p.table.Contains(key)
	if err != nil || !found {
		return ast.ASTHandle{}, false, err
	}
	data, err := p.explanation.Data()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(data) != 1 {
		return ast.ASTHandle{}, false, ErrTableRelation
	}
	return data[0], true, nil
}

// Clone copies both the finite sieve and the single whole-relation proof.
func (p *RelationExplanationProduct) Clone() (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	table, err := p.table.Clone()
	if err != nil {
		return nil, err
	}
	explanation, err := p.explanation.Clone()
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	return &RelationExplanationProduct{manager: p.manager, finiteSorts: append([]ast.ASTHandle(nil), p.finiteSorts...), table: table, explanation: explanation}, nil
}

func (p *RelationExplanationProduct) compatible(other *RelationExplanationProduct) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if p.manager != other.manager || len(p.finiteSorts) != len(other.finiteSorts) {
		return ast.ErrASTDomain
	}
	for i := range p.finiteSorts {
		if p.finiteSorts[i] != other.finiteSorts[i] {
			return ast.ErrASTDomain
		}
	}
	return p.explanation.compatible(other.explanation)
}

func (p *RelationExplanationProduct) requireCoherent() error {
	if err := p.open(); err != nil {
		return err
	}
	rows, err := p.table.Len()
	if err != nil || rows == 0 {
		return err
	}
	empty, err := p.explanation.Empty()
	if err != nil {
		return err
	}
	if empty {
		return ErrTableRelation
	}
	return nil
}

// Union combines finite rows while aggregating the single whole-relation
// proof with e_union. Delta accumulates newly added rows and relation-level
// proof contributions independently of individual tuple keys.
func (p *RelationExplanationProduct) Union(source, delta *RelationExplanationProduct) error {
	if err := p.compatible(source); err != nil {
		return err
	}
	if delta != nil {
		if delta == p || delta == source {
			return ast.ErrASTDomain
		}
		if err := p.compatible(delta); err != nil {
			return err
		}
	}
	for _, product := range []*RelationExplanationProduct{p, source, delta} {
		if product == nil {
			continue
		}
		defined, err := product.explanation.NoUndefined()
		if err != nil {
			return err
		}
		if !defined {
			return ast.ErrASTDomain
		}
		if err = product.requireCoherent(); err != nil {
			return err
		}
	}
	var deltaTable *HashTable
	if delta != nil {
		deltaTable = delta.table
	}
	if err := p.table.Union(source.table, deltaTable); err != nil {
		return err
	}
	var deltaExplanation *ExplanationRelation
	if delta != nil {
		deltaExplanation = delta.explanation
	}
	return p.explanation.Union(source.explanation, deltaExplanation, true)
}

func (p *RelationExplanationProduct) Close() (result error) {
	if p == nil || p.table == nil {
		return nil
	}
	if p.explanation != nil {
		if err := p.explanation.Close(); err != nil {
			return err
		}
		p.explanation = nil
	}
	if err := p.table.Close(); err != nil {
		return err
	}
	p.table, p.manager, p.finiteSorts = nil, nil, nil
	return nil
}
