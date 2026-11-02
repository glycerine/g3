package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestFiniteExplanationProductToFormulaRowsAndOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-explanation-formula-proof")
	astTestMust(t, err)
	proof, err := f.ast.MkDatalogRule(name, nil)
	astTestMust(t, err)
	product, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth}, proof))
	formula, err := product.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	// The returned formula owns the proof expression and finite value after the
	// product and its inner explanation relation have released their roots.
	astTestMust(t, product.Close())
	handle, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	theory := tableRelationTheoryRewriter(t, f.ast)
	evaluate := func(value, explanation ast.ASTHandle) bool {
		out, e := f.ast.NewRef(ast.ASTHandle{})
		astTestMust(t, e)
		defer func() { astTestMust(t, f.ast.DelRef(&out)) }()
		// Rewrite bindings are supplied in descending de-Bruijn index order.
		astTestMust(t, theory.RewriteWithBindings(handle, []ast.ASTHandle{explanation, value}, out))
		answer, e := f.ast.RefGet(out)
		astTestMust(t, e)
		return f.ast.IsTrue(answer)
	}
	if !evaluate(f.truth, proof) {
		t.Fatal("stored finite/explanation row rejected")
	}
	if evaluate(f.falsehood, proof) {
		t.Fatal("formula accepted an absent finite key")
	}
}

func TestFiniteExplanationProductToFormulaEmptyNullaryAndUndefined(t *testing.T) {
	f := newTableRelationFixture(t)
	empty, err := NewFiniteExplanationProduct(f.relations, nil)
	astTestMust(t, err)
	emptyFormula, err := empty.ToFormula()
	astTestMust(t, err)
	emptyHandle, err := f.ast.RefGet(emptyFormula)
	astTestMust(t, err)
	if !f.ast.IsFalse(emptyHandle) {
		t.Fatal("empty product formula is not false")
	}
	astTestMust(t, f.ast.DelRef(&emptyFormula))
	astTestMust(t, empty.Close())

	unit, err := newFiniteExplanationProduct(f.relations, nil, 0)
	astTestMust(t, err)
	astTestMust(t, unit.addEncodedRow(Fact{0}, nil))
	unitFormula, err := unit.ToFormula()
	astTestMust(t, err)
	unitHandle, err := f.ast.RefGet(unitFormula)
	astTestMust(t, err)
	if !f.ast.IsTrue(unitHandle) {
		t.Fatal("nullary zero-explanation tuple formula is not true")
	}
	astTestMust(t, f.ast.DelRef(&unitFormula))
	astTestMust(t, unit.Close())

	name, err := f.ast.Symbols.Intern("finite-explanation-undefined-proof")
	astTestMust(t, err)
	proof, err := f.ast.MkDatalogRule(name, nil)
	astTestMust(t, err)
	undefined, err := NewFiniteExplanationProduct(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, undefined.Close()) }()
	astTestMust(t, undefined.AddFact(nil, proof))
	astTestMust(t, undefined.inner[0].assignData([]ast.ASTHandle{{}}))
	if formula, e := undefined.ToFormula(); !errors.Is(e, ErrTableRelation) || formula != (ast.ASTRef{}) {
		t.Fatal("undefined explanation formula", formula, e)
	}
}

func TestExplanationTransformerTransfersSparseFiniteFacts(t *testing.T) {
	f := newTableRelationFixture(t)
	rules, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rules.Close()) }()
	source, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	name, err := f.ast.Symbols.Intern("sparse-fact-explanation")
	astTestMust(t, err)
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestMust(t, rules.RegisterPredicate(predicate))
	astTestMust(t, source.SetOutputPredicate(predicate))
	head, err := f.ast.MkApp(predicate, []ast.ASTHandle{f.truth})
	astTestMust(t, err)
	rule, err := rules.NewRule(head, nil, nil, "sparse-fact-seed")
	astTestMust(t, err)
	astTestMust(t, source.Add(rule))
	astTestMust(t, rule.Close())
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	sparse, err := NewSparseTable(signature, 0)
	astTestMust(t, err)
	_, err = sparse.Add(Fact{1})
	astTestMust(t, err)
	astTestMust(t, f.relations.StoreFiniteRelation(predicate, sparse))
	transformer, err := NewExplanationTransformer(f.ast, rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, transformer.Close()) }()
	target, err := transformer.TransformRulesOnly(source)
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	astTestMust(t, transformer.TransferFiniteFacts(source, target, f.relations, []ast.ASTHandle{predicate}))
	product, found, err := transformer.FactProduct(predicate)
	astTestMust(t, err)
	if !found {
		t.Fatal("sparse finite fact product missing")
	}
	if _, found, err = product.Explanation([]ast.ASTHandle{f.truth}); err != nil || !found {
		t.Fatal("sparse finite fact explanation missing", found, err)
	}
	if _, found, err = product.Explanation([]ast.ASTHandle{f.falsehood}); err != nil || found {
		t.Fatal("sparse conversion invented a tuple", found, err)
	}
	stored, found, err := f.relations.TryGetFiniteRelation(predicate)
	astTestMust(t, err)
	if !found || stored != sparse {
		t.Fatalf("transfer replaced concrete sparse owner: %T %v", stored, found)
	}
}
