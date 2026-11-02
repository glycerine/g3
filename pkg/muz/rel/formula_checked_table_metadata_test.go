package rel

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFormulaCheckedTableMetadataDisplayAndConsistencyDiagnostic(t *testing.T) {
	f := newTableRelationFixture(t)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := NewFormulaCheckedTable(table, formulaCheckerConfig(t, f.ast))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { astTestMust(t, checked.Close()) }()
	if _, err = checked.AddFact([]ast.ASTHandle{f.truth}); err != nil {
		t.Fatal(err)
	}
	if precise, err := checked.IsPrecise(); err != nil || !precise {
		t.Fatal("precision", precise, err)
	}
	if rows, err := checked.SizeEstimateRows(); err != nil || rows != 1 {
		t.Fatal("row estimate", rows, err)
	}
	consistent, err := checked.ConsistentFormula(nil)
	if err != nil || !consistent {
		t.Fatal("consistent formula", consistent, err)
	}
	var output bytes.Buffer
	if err = checked.Display(&output); err != nil {
		t.Fatal(err)
	}
	if text := output.String(); !strings.HasPrefix(text, "table with signature (2):\n(1)\n\n") || !strings.HasSuffix(text, "\n") {
		t.Fatal("checked display", text)
	}

	falsehood, err := f.ast.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	if err = f.ast.SetRef(checked.formula, falsehood); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	consistent, err = checked.ConsistentFormula(&output)
	if err != nil || consistent || !strings.Contains(output.String(), "relation does not have a consistent formula") || !strings.Contains(output.String(), "table with signature") {
		t.Fatal("inconsistent formula diagnostic", consistent, err, output.String())
	}
	if _, err = checked.ConsistentFormula(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("missing diagnostic writer", err)
	}
	if err = checked.Display(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil display writer", err)
	}
	if err = checked.Display(tableDisplayRefusingWriter{err: ioRefusal}); !errors.Is(err, ioRefusal) {
		t.Fatal("display writer refusal", err)
	}
}

var ioRefusal = errors.New("checked display refused")
