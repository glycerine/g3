package rel

// Source: src/muz/rel/dl_mk_explanations.cpp, transform_facts.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// TransferFiniteFacts adds the source fact explanation to each existing
// finite tuple and inherits predicates with facts or rules into target. The
// transformer owns the resulting products until Close. Product registration
// remains a separate port step.
func (t *ExplanationTransformer) TransferFiniteFacts(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle) error {
	return t.transferFacts(source, target, relations, predicates, false)
}

// TransferRelationFacts attaches the source fact explanation to the whole
// finite relation rather than to individual tuples.
func (t *ExplanationTransformer) TransferRelationFacts(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle) error {
	return t.transferFacts(source, target, relations, predicates, true)
}

func (t *ExplanationTransformer) transferFacts(source, target *base.RuleSet, relations *RelationManager, predicates []ast.ASTHandle, relationLevel bool) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || target == nil || relations == nil || relations.open() != nil || relations.manager != t.manager {
		return ast.ErrASTHandle
	}
	for _, set := range []*base.RuleSet{source, target} {
		manager, err := set.RuleManager()
		if err != nil {
			return err
		}
		if manager != t.rules {
			return ast.ErrASTHandle
		}
	}
	symbol, err := t.manager.Symbols.Intern("fact")
	if err != nil {
		return err
	}
	fact, err := t.manager.MkDatalogRule(symbol, nil)
	if err != nil {
		return err
	}
	factRef, err := t.manager.NewRef(fact)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, t.manager.DelRef(&factRef)) }()
	for _, original := range predicates {
		explained, err := t.ExplanationDeclaration(original)
		if err != nil {
			return err
		}
		table, exists, err := relations.TryGetRelation(original)
		if err != nil {
			return err
		}
		group, err := source.PredicateRules(original)
		if err != nil {
			return err
		}
		hasRules := len(group) != 0
		for i := len(group) - 1; i >= 0; i-- {
			err = errors.Join(err, group[i].Close())
		}
		if err != nil {
			return err
		}
		if !exists && !hasRules {
			continue
		}
		if _, exists := t.factProducts[original]; exists {
			return ast.ErrASTDomain
		}
		if _, exists := t.relationProducts[original]; exists {
			return ast.ErrASTDomain
		}
		derived, err := target.DerivedPredicate(original)
		if err != nil {
			return err
		}
		if derived != explained {
			if err = target.InheritPredicate(source, original, explained); err != nil {
				return err
			}
		}
		sorts, _, err := relations.PredicateSignature(original)
		if err != nil {
			return err
		}
		if relationLevel {
			product, err := newRelationExplanationProduct(relations, sorts, table, fact)
			if err != nil {
				return err
			}
			t.relationProducts[original] = product
			continue
		}
		product, err := NewFiniteExplanationProduct(relations, sorts)
		if err != nil {
			return err
		}
		if exists {
			facts, err := table.Facts()
			if err != nil {
				return errors.Join(err, product.Close())
			}
			for _, row := range facts {
				key := append(append(Fact(nil), row...), 0)
				if err = product.addEncoded(key, fact); err != nil {
					return errors.Join(err, product.Close())
				}
			}
		}
		t.factProducts[original] = product
	}
	return nil
}

// RelationProduct returns a transformer-owned whole-relation explanation
// product created by TransferRelationFacts.
func (t *ExplanationTransformer) RelationProduct(original ast.ASTHandle) (*RelationExplanationProduct, bool, error) {
	if err := t.open(); err != nil {
		return nil, false, err
	}
	product, found := t.relationProducts[original]
	return product, found, nil
}

// FactProduct returns the transformer-owned finite/explanation product for an
// original predicate, if TransferFiniteFacts created one.
func (t *ExplanationTransformer) FactProduct(original ast.ASTHandle) (*FiniteExplanationProduct, bool, error) {
	if err := t.open(); err != nil {
		return nil, false, err
	}
	product, found := t.factProducts[original]
	return product, found, nil
}
