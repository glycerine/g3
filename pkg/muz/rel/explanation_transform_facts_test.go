package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestExplanationTransformerTransfersFiniteFacts(t *testing.T) {
	f := newTableRelationFixture(t)
	rm, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		d, e := f.ast.MkFuncDecl(sym, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, rm.RegisterPredicate(d))
		return d
	}
	ext := declare("explanation-extensional", []ast.ASTHandle{f.boolean})
	zero := declare("explanation-nullary", nil)
	dummy := declare("explanation-rule-head", nil)
	unused := declare("explanation-unused", nil)
	table, err := f.relations.GetRelation(ext)
	astTestMust(t, err)
	_, err = table.Add(Fact{1})
	astTestMust(t, err)
	zeroTable, err := f.relations.GetRelation(zero)
	astTestMust(t, err)
	_, err = zeroTable.Add(Fact{})
	astTestMust(t, err)
	head, err := f.ast.MkApp(dummy, nil)
	astTestMust(t, err)
	rule, err := rm.NewRule(head, nil, nil, "dummy")
	astTestMust(t, err)
	astTestMust(t, set.Add(rule))
	astTestMust(t, rule.Close())
	astTestMust(t, set.SetOutputPredicate(ext))
	transformer, err := NewExplanationTransformer(f.ast, rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, transformer.Close()) }()
	target, err := transformer.TransformRulesOnly(set)
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	astTestMust(t, transformer.TransferFiniteFacts(set, target, f.relations, []ast.ASTHandle{ext, zero, dummy, unused}))
	product, found, err := transformer.FactProduct(ext)
	astTestMust(t, err)
	if !found {
		t.Fatal("missing fact product")
	}
	explanation, found, err := product.Explanation([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found {
		t.Fatal("existing fact missing from product")
	}
	decl, err := f.ast.GetDecl(explanation)
	astTestMust(t, err)
	sym, err := f.ast.GetASTName(decl)
	astTestMust(t, err)
	name, err := f.ast.Symbols.Display(sym)
	astTestMust(t, err)
	if name != "fact" {
		t.Fatal("fact explanation symbol", name)
	}
	zeroProduct, found, err := transformer.FactProduct(zero)
	astTestMust(t, err)
	if !found {
		t.Fatal("missing zero-column fact product")
	}
	zeroExplanation, found, err := zeroProduct.Explanation(nil)
	astTestMust(t, err)
	if !found || zeroExplanation != explanation {
		t.Fatal("nullary fact conversion lost its explanation", zeroExplanation, found)
	}
	joined, err := product.Join(product, []int{0}, []int{0})
	astTestMust(t, err)
	row, found, err := joined.ExplanationRow([]ast.ASTHandle{f.truth, f.truth})
	astTestMust(t, err)
	if !found || len(row) != 2 || row[0] != explanation || row[1] != explanation {
		t.Fatal("transferred fact join", row, found)
	}
	astTestMust(t, joined.Close())
	_, found, err = product.Explanation([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	if found {
		t.Fatal("invented finite fact")
	}
	if _, found, err = transformer.FactProduct(unused); err != nil || found {
		t.Fatal("unused predicate product", found, err)
	}
	derived, err := target.DerivedPredicate(ext)
	astTestMust(t, err)
	output, err := target.OutputPredicate()
	astTestMust(t, err)
	if output != derived || output == ext {
		t.Fatal("fact predicate inheritance", output, derived)
	}
	count, err := table.Len()
	astTestMust(t, err)
	if count != 1 {
		t.Fatal("original table mutated", count)
	}
	astTestMust(t, transformer.Close())
	if _, _, err = product.Explanation([]ast.ASTHandle{f.truth}); err == nil {
		t.Fatal("transformer left fact product live")
	}
}

func TestExplanationTransformerTransfersRelationFacts(t *testing.T) {
	f := newTableRelationFixture(t)
	rm, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()
	makeDecl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		d, e := f.ast.MkFuncDecl(sym, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, rm.RegisterPredicate(d))
		return d
	}
	ext := makeDecl("relation-explanation-extensional", []ast.ASTHandle{f.boolean})
	dummy := makeDecl("relation-explanation-head", nil)
	table, err := f.relations.GetRelation(ext)
	astTestMust(t, err)
	_, err = table.Add(Fact{1})
	astTestMust(t, err)
	_, err = table.Add(Fact{0})
	astTestMust(t, err)
	head, err := f.ast.MkApp(dummy, nil)
	astTestMust(t, err)
	rule, err := rm.NewRule(head, nil, nil, "dummy")
	astTestMust(t, err)
	astTestMust(t, set.Add(rule))
	astTestMust(t, rule.Close())
	astTestMust(t, set.SetOutputPredicate(ext))
	transformer, err := NewExplanationTransformer(f.ast, rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, transformer.Close()) }()
	target, err := transformer.TransformRulesOnly(set)
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	astTestMust(t, transformer.TransferRelationFacts(set, target, f.relations, []ast.ASTHandle{ext, dummy}))
	product, found, err := transformer.RelationProduct(ext)
	astTestMust(t, err)
	if !found {
		t.Fatal("missing relation-level product")
	}
	a, found, err := product.Explanation([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found {
		t.Fatal("first tuple missing")
	}
	b, found, err := product.Explanation([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	if !found || a != b {
		t.Fatal("relation-level explanation differs", a, b, found)
	}
	astTestMust(t, table.Reset())
	_, found, err = product.Explanation([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found {
		t.Fatal("product did not copy original facts")
	}
	astTestMust(t, transformer.Close())
	if _, _, err = product.Explanation([]ast.ASTHandle{f.truth}); err == nil {
		t.Fatal("closed relation product remains live")
	}
}
