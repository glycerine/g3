package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestTransferRelationFactsFormulaCheckedEligibilityAndExplicitBypass(t *testing.T) {
	for _, test := range []struct {
		name     string
		eligible bool
		sparse   bool
	}{
		{name: "formula-finite-sieve", eligible: true},
		{name: "explicit-sparse-bypass", sparse: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newTableRelationFixture(t)
			rules, err := base.NewRuleManager(f.ast)
			astTestMust(t, err)
			defer func() { astTestMust(t, rules.Close()) }()
			set, err := base.NewRuleSet(rules)
			astTestMust(t, err)
			defer func() { astTestMust(t, set.Close()) }()
			name, err := f.ast.Symbols.Intern("relation-formula-" + test.name)
			astTestMust(t, err)
			predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			astTestMust(t, rules.RegisterPredicate(predicate))
			astTestMust(t, set.SetOutputPredicate(predicate))
			head, err := f.ast.MkApp(predicate, []ast.ASTHandle{f.truth})
			astTestMust(t, err)
			rule, err := rules.NewRule(head, nil, nil, "formula-seed")
			astTestMust(t, err)
			astTestMust(t, set.Add(rule))
			astTestMust(t, rule.Close())
			if test.sparse {
				signature, e := NewSignature([]uint64{2})
				astTestMust(t, e)
				table, e := NewSparseTable(signature, 0)
				astTestMust(t, e)
				_, e = table.Add(Fact{1})
				astTestMust(t, e)
				astTestMust(t, f.relations.StoreFiniteRelation(predicate, table))
			} else {
				table, e := f.relations.GetRelation(predicate)
				astTestMust(t, e)
				_, e = table.Add(Fact{1})
				astTestMust(t, e)
			}
			transformer, err := NewExplanationTransformer(f.ast, rules)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformer.Close()) }()
			target, err := transformer.TransformRulesOnly(set)
			astTestMust(t, err)
			defer func() { astTestMust(t, target.Close()) }()
			eligible := make(map[ast.ASTHandle]bool)
			if test.eligible {
				eligible[predicate] = true
			}
			astTestMust(t, transformer.TransferRelationFactsFormulaChecked(set, target, f.relations, []ast.ASTHandle{predicate}, eligible, formulaCheckerConfig(t, f.ast)))
			product, found, err := transformer.RelationProduct(predicate)
			astTestMust(t, err)
			if !found {
				t.Fatal("missing relation product")
			}
			checked, err := product.FormulaChecked()
			astTestMust(t, err)
			if checked != test.eligible {
				t.Fatal("formula eligibility", checked, test.eligible)
			}
			primary, checker, pairChecked, err := product.CheckedConcreteKinds()
			astTestMust(t, err)
			if test.eligible {
				if primary != "hashtable" || checker != "formula" || !pairChecked {
					t.Fatal("formula backing", primary, checker, pairChecked)
				}
			} else if primary != "sparse" || checker != "" || pairChecked {
				t.Fatal("explicit sparse backing", primary, checker, pairChecked)
			}
			proof, found, err := product.Explanation([]ast.ASTHandle{f.truth})
			astTestMust(t, err)
			if !found || proof.IsNull() {
				t.Fatal("independent whole proof missing", found, proof)
			}
		})
	}
}
