package rel

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestFiniteIntervalProductDisplayEstimatesAndEmptyCollection(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	intValue := func(value int64) ast.ASTHandle {
		numeral, e := f.ast.ArithmeticIntegerNumeralText(fmt.Sprint(value))
		astTestMust(t, e)
		astTestPin(t, f.ast, numeral)
		return numeral
	}
	p, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	if rows, e := p.SizeEstimateRows(); e != nil || rows != 0 {
		t.Fatalf("empty row estimate = %d (%v)", rows, e)
	}
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.falsehood, intValue(1)}))
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.truth, intValue(2)}))
	if rows, e := p.SizeEstimateRows(); e != nil || rows != 2 {
		t.Fatalf("outer row estimate = %d, want 2 (%v)", rows, e)
	}
	wantBytes := uint32(len(p.table.data.entries)) * p.table.layout.EntryBytes()
	if size, e := p.SizeEstimateBytes(); e != nil || size != wantBytes {
		t.Fatalf("outer byte estimate = %d, want %d (%v)", size, wantBytes, e)
	}

	// Make only the false-key inner empty. Display must collect it and its row.
	astTestMust(t, p.FilterEqual(1, intValue(2)))
	var output bytes.Buffer
	astTestMust(t, p.Display(&output))
	text := output.String()
	if !strings.Contains(text, "finite_product_relation:\n table:\n") || !strings.Contains(text, " inner relation 1:\n") || strings.Contains(text, " inner relation 0:\n") {
		t.Fatalf("display did not collect the empty inner in source order: %q", text)
	}
	if rows, e := p.SizeEstimateRows(); e != nil || rows != 1 {
		t.Fatalf("post-display outer row estimate = %d, want 1 (%v)", rows, e)
	}
	context, err := base.NewDatalogContext(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	predicateName, err := f.ast.Symbols.Intern("finite-interval-display-p")
	astTestMust(t, err)
	predicate, err := f.ast.MkFuncDecl(predicateName, []ast.ASTHandle{f.boolean, integer}, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.ast, predicate)
	var tuples bytes.Buffer
	astTestMust(t, p.DisplayTuples(context, predicate, &tuples))
	if tupleText := tuples.String(); !strings.HasPrefix(tupleText, "Tuples in finite-interval-display-p: \nfinite_product_relation:\n") || !strings.Contains(tupleText, " inner relation 1:\n") {
		t.Fatalf("non-table tuple display did not delegate to product display: %q", tupleText)
	}

	refusal := errors.New("interval product display refusal")
	if err = p.Display(productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatalf("display writer error = %v, want %v", err, refusal)
	}
	if err = p.Display(nil); !errors.Is(err, ErrTableRelation) {
		t.Fatalf("nil writer error = %v", err)
	}
	if err = p.DisplayTuples(context, predicate, productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatalf("tuple display writer error = %v, want %v", err, refusal)
	}
}
