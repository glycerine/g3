package util

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func parserTestManager(t *testing.T) *ast.ASTManager {
	t.Helper()
	budget := arena.NewBudget(1 << 26)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
		if err := numerals.Close(); err != nil {
			t.Error(err)
		}
		if err := symbols.Close(); err != nil {
			t.Error(err)
		}
		if budget.Used() != 0 {
			t.Errorf("owners retained %d bytes", budget.Used())
		}
	})
	return manager
}

func TestSimpleParserBuiltinVariablesAndFailures(t *testing.T) {
	manager := parserTestManager(t)
	parser, err := NewSimpleParser(manager)
	if err != nil {
		t.Fatal(err)
	}
	parser.SetWarnings(nil)
	if err = parser.AddBuiltin("not", manager.BasicFamilyID(), ast.ASTOpNot); err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkFamilyApp(manager.BasicFamilyID(), ast.ASTOpTrue, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	truthRef, err := manager.NewRef(truth)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.DelRef(&truthRef)
	if err = parser.AddVariable("x", truth); err != nil {
		t.Fatal(err)
	}
	result, ok, err := parser.ParseString("(not x)")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	defer manager.DelRef(&result)
	handle, err := manager.RefGet(result)
	if err != nil {
		t.Fatal(err)
	}
	if kind, err := manager.GetDeclKind(handle); err != nil || kind != ast.ASTOpNot {
		t.Fatal(kind, err)
	}
	if _, ok, err = parser.ParseString("(missing x)"); err != nil || ok {
		t.Fatal("unknown builtin accepted")
	}
}

func TestCostParserSourceExpressionAndVariableReset(t *testing.T) {
	manager := parserTestManager(t)
	parser, err := NewCostParser(manager)
	if err != nil {
		t.Fatal(err)
	}
	defer parser.Close()
	parser.SetWarnings(nil)
	if index, err := parser.AddCostVariable("x"); err != nil || index != 0 {
		t.Fatal(index, err)
	}
	result, ok, err := parser.ParseString("(ite (<= x 1.5) (+ x 2) 0)")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	handle, err := manager.RefGet(result)
	if err != nil {
		t.Fatal(err)
	}
	if kind, err := manager.GetDeclKind(handle); err != nil || kind != ast.ASTOpIte {
		t.Fatal(kind, err)
	}
	if err = manager.DelRef(&result); err != nil {
		t.Fatal(err)
	}
	if err = parser.ResetCostVariables(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err = parser.ParseString("x"); err != nil || ok {
		t.Fatal("reset variable accepted")
	}
}

// parsers_shell_01-F17: simple_parser::parse reports a parse failure with
// warning_msg("parser error"), including a null top-level result, and the
// warning service's enable switch applies.
func TestSimpleParserErrorUsesWarningService(t *testing.T) {
	manager := parserTestManager(t)
	parser, err := NewSimpleParser(manager)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	warnings := diagnostic.NewWarningService(&buf)
	parser.SetWarningService(warnings)
	for _, text := range []string{"(foo)", ")", ""} {
		buf.Reset()
		if _, ok, err := parser.ParseString(text); ok || err != nil {
			t.Fatalf("%q: ok=%v err=%v", text, ok, err)
		}
		if got := buf.String(); got != "WARNING: parser error\n" {
			t.Fatalf("%q: warning %q", text, got)
		}
	}
	buf.Reset()
	if err = warnings.Enable(false); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := parser.ParseString("(foo)"); ok || err != nil {
		t.Fatalf("disabled: ok=%v err=%v", ok, err)
	}
	if buf.Len() != 0 {
		t.Fatalf("disabled warning service wrote %q", buf.String())
	}
}
