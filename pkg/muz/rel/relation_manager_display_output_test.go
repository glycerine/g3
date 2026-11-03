package rel

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRelationManagerDisplaysFiniteExplanationAndMissingOutputs(t *testing.T) {
	f := newTableRelationFixture(t)
	context, err := base.NewDatalogContext(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	rules, err := context.Rules()
	astTestMust(t, err)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		predicate, e := f.ast.MkFuncDecl(sym, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, context.RegisterPredicate(predicate, true))
		astTestMust(t, context.SetOutputPredicate(predicate))
		return predicate
	}
	finite := declare("display-output-finite", []ast.ASTHandle{f.boolean})
	missing := declare("display-output-missing", []ast.ASTHandle{f.boolean})
	explained := declare("display-output-explained", []ast.ASTHandle{f.boolean, ruleSort})
	argument, err := f.ast.Symbols.Intern("flag")
	astTestMust(t, err)
	astTestMust(t, context.SetArgumentNames(finite, []symbol.Symbol{argument}))
	table, err := f.relations.GetRelation(finite)
	astTestMust(t, err)
	_, err = table.Add(Fact{1})
	astTestMust(t, err)
	explanationName, err := f.ast.Symbols.Intern("display-output-proof")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(explanationName, ruleSort)
	astTestMust(t, err)
	product, err := f.relations.GetExplanationRelation(explained)
	astTestMust(t, err)
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth}, explanation))

	var output bytes.Buffer
	astTestMust(t, f.relations.DisplayOutputTables(context, rules, &output))
	text := output.String()
	for _, fragment := range []string{
		"Tuples in display-output-finite: \n\t(flag=1(1))\n",
		"Tuples in display-output-missing: \n",
		"Tuples in display-output-explained: \nfinite_product_relation:\n",
		"display-output-proof",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("output tables missing %q: %q", fragment, text)
		}
	}
	refusal := errors.New("output table writer refusal")
	if err = f.relations.DisplayOutputTables(context, rules, productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatalf("writer error = %v, want %v", err, refusal)
	}
	if err = f.relations.DisplayOutputTables(context, rules, nil); !errors.Is(err, ErrTableRelation) {
		t.Fatalf("nil writer error = %v", err)
	}
	if _, found, err := f.relations.TryGetRelation(missing); err != nil || found {
		t.Fatalf("display created missing relation: %v %v", found, err)
	}
}

func TestRelationManagerDisplaysSparseAndLazyFiniteOutputs(t *testing.T) {
	lazyFactory := NewLazySparseFiniteFactory()
	t.Cleanup(func() { astTestMust(t, lazyFactory.Close()) })
	f := newTableRelationFixture(t)
	context, err := base.NewDatalogContext(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	rules, err := context.Rules()
	astTestMust(t, err)
	declare := func(name, argument string) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		predicate, e := f.ast.MkFuncDecl(sym, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, context.RegisterPredicate(predicate, true))
		astTestMust(t, context.SetOutputPredicate(predicate))
		argumentName, e := f.ast.Symbols.Intern(argument)
		astTestMust(t, e)
		astTestMust(t, context.SetArgumentNames(predicate, []symbol.Symbol{argumentName}))
		return predicate
	}
	sparsePredicate := declare("display-output-sparse", "sparse-flag")
	lazyPredicate := declare("display-output-lazy", "lazy-flag")
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	sparse, err := NewSparseTable(signature, 0)
	astTestMust(t, err)
	_, err = sparse.Add(Fact{1})
	astTestMust(t, err)
	_, err = sparse.Add(Fact{0})
	astTestMust(t, err)
	astTestMust(t, f.relations.StoreFiniteRelation(sparsePredicate, sparse))

	lazyBase, err := lazyFactory.New(signature)
	astTestMust(t, err)
	_, err = lazyBase.Add(Fact{0})
	astTestMust(t, err)
	lazy, err := lazyBase.RenameCycle(nil)
	astTestMust(t, err)
	astTestMust(t, lazyBase.Close())
	if lazy.Forced() {
		t.Fatal("lazy output was forced before display")
	}
	astTestMust(t, f.relations.StoreFiniteRelation(lazyPredicate, lazy))

	var output bytes.Buffer
	astTestMust(t, f.relations.DisplayOutputTables(context, rules, &output))
	text := output.String()
	sparseOne := strings.Index(text, "\t(sparse-flag=1(1))\n")
	sparseZero := strings.Index(text, "\t(sparse-flag=0(0))\n")
	if !strings.Contains(text, "Tuples in display-output-sparse: \n") || sparseOne < 0 || sparseZero <= sparseOne {
		t.Fatalf("sparse output lost rows or insertion order: %q", text)
	}
	if !strings.Contains(text, "Tuples in display-output-lazy: \n\t(lazy-flag=0(0))\n") {
		t.Fatalf("lazy output missing: %q", text)
	}
	if !lazy.Forced() {
		t.Fatal("display did not force lazy source iteration")
	}
	if found, e := sparse.Contains(Fact{1}); e != nil || !found {
		t.Fatal("display consumed sparse owner", found, e)
	}
	if found, e := lazy.Contains(Fact{0}); e != nil || !found {
		t.Fatal("display consumed lazy owner", found, e)
	}
	if stored, found, e := f.relations.TryGetFiniteRelation(sparsePredicate); e != nil || !found || stored != sparse {
		t.Fatal("display replaced sparse owner", stored, found, e)
	}
	if stored, found, e := f.relations.TryGetFiniteRelation(lazyPredicate); e != nil || !found || stored != lazy {
		t.Fatal("display replaced lazy owner", stored, found, e)
	}
	refusal := errors.New("finite output writer refusal")
	if err = f.relations.DisplayOutputTables(context, rules, productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatal("finite tuple display ignored writer refusal", err)
	}
}

func TestRelationManagerDisplaysRelationLevelExplanationOutput(t *testing.T) {
	f := newTableRelationFixture(t)
	context, err := base.NewDatalogContext(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	rules, err := context.Rules()
	astTestMust(t, err)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	name, err := f.ast.Symbols.Intern("display-output-relation-level")
	astTestMust(t, err)
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean, ruleSort}, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestMust(t, context.RegisterPredicate(predicate, true))
	astTestMust(t, context.SetOutputPredicate(predicate))

	product, err := f.relations.GetRelationExplanation(predicate)
	astTestMust(t, err)
	_, err = product.table.Add(Fact{1})
	astTestMust(t, err)
	proofName, err := f.ast.Symbols.Intern("display-output-relation-proof")
	astTestMust(t, err)
	proof, err := f.ast.MkConst(proofName, ruleSort)
	astTestMust(t, err)
	astTestMust(t, product.explanation.AddFact([]ast.ASTHandle{proof}))

	var output bytes.Buffer
	astTestMust(t, f.relations.DisplayOutputTables(context, rules, &output))
	want := "Tuples in display-output-relation-level: \n" +
		"Product of the following relations:\n" +
		"Sieve relation (0)\n" +
		"table with signature (2):\n" +
		"(1)\n\n" +
		"Sieve relation (1)\n" +
		"display-output-relation-proof\n"
	if output.String() != want {
		t.Fatalf("relation-level output\n got: %q\nwant: %q", output.String(), want)
	}
	stored, found, err := f.relations.TryGetRelationExplanation(predicate)
	if err != nil || !found || stored != product {
		t.Fatal("display replaced relation-level product", stored, found, err)
	}
	storedProof, found, err := product.Explanation([]ast.ASTHandle{f.truth})
	if err != nil || !found || storedProof != proof {
		t.Fatal("display changed relation-level contents", storedProof, found, err)
	}
	refusal := errors.New("relation-level output writer refusal")
	if err = f.relations.DisplayOutputTables(context, rules, productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatalf("writer error = %v, want %v", err, refusal)
	}
	if err = product.Display(productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatalf("product writer error = %v, want %v", err, refusal)
	}
	if err = product.Display(nil); !errors.Is(err, ErrTableRelation) {
		t.Fatalf("nil product writer error = %v", err)
	}
}
