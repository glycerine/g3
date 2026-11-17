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
