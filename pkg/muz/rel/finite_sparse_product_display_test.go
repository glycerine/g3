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

func TestFiniteSparseProductDisplayAndTuples(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("finite-sparse-product-display-domain")
	astTestMust(t, err)
	sort, err := f.ast.MkDatalogFiniteSort(name, 3)
	astTestMust(t, err)
	value := func(n uint64) ast.ASTHandle {
		h, e := f.ast.MkDatalogNumeral(n, sort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	product, err := NewFiniteSparseProduct(f.relations, []ast.ASTHandle{sort, sort}, []bool{true, false})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact([]ast.ASTHandle{value(0), value(1)}))
	var display bytes.Buffer
	if err = product.Display(&display); err != nil {
		t.Fatal(err)
	}
	if text := display.String(); !strings.Contains(text, "finite_product_relation:\n table:\n") || !strings.Contains(text, " inner relation 0:\n") {
		t.Fatalf("product display missing source structure: %q", text)
	}
	refusal := errors.New("display writer refusal")
	if err = product.Display(productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatalf("display writer error = %v, want %v", err, refusal)
	}

	context, err := base.NewDatalogContext(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	predicateName, err := f.ast.Symbols.Intern("finite-sparse-display-p")
	astTestMust(t, err)
	predicate, err := f.ast.MkFuncDecl(predicateName, []ast.ASTHandle{sort, sort}, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.ast, predicate)
	arguments := make([]symbol.Symbol, 2)
	for index, text := range []string{"left", "right"} {
		arguments[index], err = f.ast.Symbols.Intern(text)
		astTestMust(t, err)
	}
	astTestMust(t, context.SetArgumentNames(predicate, arguments))
	var tuples bytes.Buffer
	if err = product.DisplayTuples(context, predicate, &tuples); err != nil {
		t.Fatal(err)
	}
	if got, want := tuples.String(), "Tuples in finite-sparse-display-p: \n\t(left=0(0),right=1(1))\n"; got != want {
		t.Fatalf("tuple display = %q, want %q", got, want)
	}
}

type productDisplayRefusingWriter struct{ err error }

func (w productDisplayRefusingWriter) Write([]byte) (int, error) { return 0, w.err }
