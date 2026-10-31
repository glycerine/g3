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

func newRelationExplanationProduct(manager *RelationManager, sorts []ast.ASTHandle, source *HashTable, fact ast.ASTHandle) (_ *RelationExplanationProduct, resultErr error) {
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
	if source != nil {
		if err = p.table.Union(source, nil); err != nil {
			return nil, err
		}
	}
	ruleSort, err := manager.manager.MkDatalogRuleSort()
	if err != nil {
		return nil, err
	}
	p.explanation, err = NewExplanationRelation(manager.manager, []ast.ASTHandle{ruleSort})
	if err != nil {
		return nil, err
	}
	if err = p.explanation.AddFact([]ast.ASTHandle{fact}); err != nil {
		return nil, err
	}
	return p, nil
}

// Explanation returns the relation-level explanation for a present tuple.
// The returned AST handle is borrowed from the product.
func (p *RelationExplanationProduct) Explanation(values []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.table == nil || p.explanation == nil {
		return ast.ASTHandle{}, false, ErrTableRelation
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
