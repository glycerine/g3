package rel

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type tableDisplayRefusingWriter struct{ err error }

func (w tableDisplayRefusingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestHashTableAndRelationDisplaySourceFormat(t *testing.T) {
	signature, err := NewSignature([]uint64{3, 2})
	if err != nil {
		t.Fatal("signature", err)
	}
	table := NewHashTable(signature)
	if _, err = table.Add(Fact{1, 0}); err != nil {
		t.Fatal("first row", err)
	}
	if _, err = table.Add(Fact{2, 1}); err != nil {
		t.Fatal("second row", err)
	}
	var output bytes.Buffer
	if err = table.Display(&output); err != nil {
		t.Fatal("display table", err)
	}
	const want = "table with signature (3,2):\n(1,0)\n(2,1)\n\n"
	if output.String() != want {
		t.Fatalf("table display\n got: %q\nwant: %q", output.String(), want)
	}

	f := newTableRelationFixture(t)
	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	if err != nil {
		t.Fatal("relation", err)
	}
	closeTableRelations(t, relation)
	if _, err = relation.AddFact([]ast.ASTHandle{f.truth, f.bv(2)}); err != nil {
		t.Fatal("relation row", err)
	}
	output.Reset()
	if err = relation.Display(&output); err != nil {
		t.Fatal("display relation", err)
	}
	const relationWant = "table with signature (2,4):\n(1,2)\n\n"
	if output.String() != relationWant {
		t.Fatalf("relation display\n got: %q\nwant: %q", output.String(), relationWant)
	}
}

func TestHashTableDisplayEmptyAndWriterFailure(t *testing.T) {
	signature, err := NewSignature(nil)
	if err != nil {
		t.Fatal("zero-arity signature", err)
	}
	table := NewHashTable(signature)
	var output bytes.Buffer
	if err = table.Display(&output); err != nil {
		t.Fatal("display empty table", err)
	}
	if got, want := output.String(), "table with signature ():\n\n"; got != want {
		t.Fatalf("empty display: got %q, want %q", got, want)
	}
	if _, err = table.Add(Fact{}); err != nil {
		t.Fatal("zero-arity row", err)
	}
	output.Reset()
	if err = table.Display(&output); err != nil {
		t.Fatal("display zero-arity fact", err)
	}
	if got, want := output.String(), "table with signature ():\n()\n\n"; got != want {
		t.Fatalf("zero-arity fact display: got %q, want %q", got, want)
	}
	refusal := errors.New("table display writer refused output")
	if err = table.Display(tableDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatal("display ignored writer refusal", err)
	}
}

func TestTableRelationDisplayTuplesUsesDatalogNamesAndConstants(t *testing.T) {
	f := newTableRelationFixture(t)
	context, err := base.NewDatalogContext(f.ast)
	if err != nil {
		t.Fatal("Datalog context", err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = context.RegisterFiniteSort(f.boolean, base.DatalogSortSymbol); err != nil {
		t.Fatal("register Boolean finite sort", err)
	}
	falseName, err := f.ast.Symbols.Intern("false")
	if err != nil {
		t.Fatal("false symbol", err)
	}
	trueName, err := f.ast.Symbols.Intern("true")
	if err != nil {
		t.Fatal("true symbol", err)
	}
	if _, err = context.SymbolNumber(f.boolean, falseName); err != nil {
		t.Fatal("register false constant", err)
	}
	if _, err = context.SymbolNumber(f.boolean, trueName); err != nil {
		t.Fatal("register true constant", err)
	}
	predicateName, err := f.ast.Symbols.Intern("display-p")
	if err != nil {
		t.Fatal("predicate symbol", err)
	}
	predicate, err := f.ast.MkFuncDecl(predicateName, []ast.ASTHandle{f.boolean, f.bits}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal("predicate declaration", err)
	}
	argumentNames := []symbol.Symbol{}
	for _, name := range []string{"truth", "value"} {
		symbol, symbolErr := f.ast.Symbols.Intern(name)
		if symbolErr != nil {
			t.Fatal("argument symbol", symbolErr)
		}
		argumentNames = append(argumentNames, symbol)
	}
	if err = context.SetArgumentNames(predicate, argumentNames); err != nil {
		t.Fatal("argument names", err)
	}
	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	if err != nil {
		t.Fatal("relation", err)
	}
	closeTableRelations(t, relation)
	if _, err = relation.AddFact([]ast.ASTHandle{f.truth, f.bv(2)}); err != nil {
		t.Fatal("relation fact", err)
	}
	var output bytes.Buffer
	if err = relation.DisplayTuples(context, predicate, &output); err != nil {
		t.Fatal("display tuples", err)
	}
	const want = "Tuples in display-p: \n\t(truth=true(1),value=2(2))\n"
	if output.String() != want {
		t.Fatalf("tuple display\n got: %q\nwant: %q", output.String(), want)
	}
}
