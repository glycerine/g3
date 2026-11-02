package rel

// Source: src/muz/rel/dl_relation_manager.cpp get_relation and
// src/muz/rel/dl_external_relation.cpp external_relation_plugin::mk_empty.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// PredicateRelation returns the manager-owned external relation for predicate,
// constructing it from the predicate signature on first use. The returned
// relation is borrowed and is retired with the plugin.
func (p *ExternalRelationPlugin) PredicateRelation(predicate ast.ASTHandle) (_ *ExternalRelation, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.ast == nil || p.predicates == nil {
		return nil, ErrTableRelation
	}
	if relation := p.predicates[predicate]; relation != nil {
		if err := relation.open(); err != nil {
			return nil, err
		}
		return relation, nil
	}
	sorts, _, err := p.manager.PredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	rangeSort, err := p.ast.GetRange(predicate)
	if err != nil {
		return nil, err
	}
	boolean, err := p.ast.MkBoolSort()
	if err != nil {
		return nil, err
	}
	if rangeSort != boolean {
		return nil, ErrTableRelation
	}
	relation, err := p.NewEmpty(sorts)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, relation.Close())
		}
	}()
	if err = p.ast.IncRef(predicate); err != nil {
		return nil, err
	}
	p.predicates[predicate] = relation
	published = true
	return relation, nil
}

// DropPredicateRelation retires one manager-owned external relation. Refused
// native cleanup leaves the concrete relation attached for an explicit retry.
func (p *ExternalRelationPlugin) DropPredicateRelation(predicate ast.ASTHandle) error {
	if p == nil || p.closed || p.ast == nil || p.predicates == nil {
		return ErrTableRelation
	}
	relation := p.predicates[predicate]
	if relation == nil {
		return nil
	}
	if err := relation.Close(); err != nil {
		return err
	}
	consumed, err := p.ast.DecRefConsumed(predicate)
	if consumed {
		delete(p.predicates, predicate)
	}
	if err != nil {
		return err
	}
	return nil
}
