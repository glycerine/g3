package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestConeOfInfluenceBailsOutForMaterializedExplanationRelations(t *testing.T) {
	for _, relationLevel := range []bool{false, true} {
		name := "per-fact"
		if relationLevel {
			name = "whole-relation"
		}
		t.Run(name, func(t *testing.T) {
			f := newConeFixture(t, "P")
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			ruleSort, err := f.manager.MkDatalogRuleSort()
			coneMust(t, err)
			symbol, err := f.manager.Symbols.Intern("cone-explained-tail-" + name)
			coneMust(t, err)
			explained, err := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{ruleSort}, boolean, ast.ASTFuncDeclInfo{})
			coneMust(t, err)
			coneMust(t, f.rules.RegisterPredicate(explained))
			proofName, err := f.manager.Symbols.Intern("cone-materialized-proof-" + name)
			coneMust(t, err)
			proof, err := f.manager.MkDatalogRule(proofName, nil)
			coneMust(t, err)
			tail, err := f.manager.MkApp(explained, []ast.ASTHandle{proof})
			coneMust(t, err)
			addTransformRule(t, f, f.applications["P"], []ast.ASTHandle{tail}, []bool{false}, "explained-tail")
			coneMust(t, f.set.SetOutputPredicate(f.declarations["P"]))
			valid, err := f.set.Finalize()
			coneMust(t, err)
			if !valid {
				t.Fatal("explanation relation cone fixture rejected")
			}

			withoutFacts, err := transforms.ConeOfInfluence(f.set)
			coneMust(t, err)
			if withoutFacts == nil {
				t.Fatal("unreachable explained tail was not pruned")
			}
			coneMust(t, withoutFacts.Close())

			relations, err := rel.NewRelationManager(f.manager)
			coneMust(t, err)
			defer func() { coneMust(t, relations.Close()) }()
			if relationLevel {
				product, err := rel.NewRelationExplanationTotal(relations, 1)
				coneMust(t, err)
				coneMust(t, relations.StoreRelationExplanation(explained, product))
			} else {
				product, err := rel.NewFiniteExplanationProduct(relations, nil)
				coneMust(t, err)
				coneMust(t, product.AddFact(nil, proof))
				coneMust(t, relations.StoreExplanationRelation(explained, product))
			}
			materialized, err := relations.HasFacts(explained)
			coneMust(t, err)
			if !materialized {
				t.Fatal("explanation relation fact probe reported empty")
			}
			result, err := transforms.ConeOfInfluenceWithRelations(f.set, relations)
			coneMust(t, err)
			if result != nil {
				coneMust(t, result.Close())
				t.Fatal("cone filter ignored materialized explanation relation")
			}
		})
	}
}
