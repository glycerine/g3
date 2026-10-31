package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIASTPrinterUsesSourceSMT2DeclarationAndQuoting(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	declaration := f.pin(f.c.Function("a b", []ast.ASTHandle{boolean}, boolean))
	x := f.pin(f.c.Constant("x y", boolean))
	application := f.pin(f.c.Apply(declaration, x))
	for _, mode := range []APIASTPrintMode{APIASTPrintSMTFull, APIASTPrintSMT2} {
		if got, err := f.c.FunctionString(declaration, mode); err != nil || got != "(declare-fun |a b| (Bool) Bool)" {
			t.Fatal("source declaration document", mode, got, err)
		}
		if got, err := f.c.ASTString(application, mode); err != nil || got != "(|a b| |x y|)" {
			t.Fatal("source quoted application", mode, got, err)
		}
	}
}

func TestAPIASTPrinterFullOverridesOnlyAliasParameters(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	globals, err := f.c.command.GlobalParameters()
	astTestMust(t, err)
	astTestMust(t, globals.SourceSetName("pp.max_depth", "1"))
	astTestMust(t, globals.SourceSetName("pp.min_alias_size", "1"))
	astTestMust(t, globals.SourceSetName("pp.bv_literals", "false"))
	boolean := f.pin(f.c.BoolSort())
	function := f.pin(f.c.Function("f", []ast.ASTHandle{boolean}, boolean))
	term := f.pin(f.c.Constant("x", boolean))
	for i := 0; i < 8; i++ {
		term = f.pin(f.c.Apply(function, term))
	}
	full, err := f.c.ASTString(term, APIASTPrintSMTFull)
	astTestMust(t, err)
	if full != "(f (f (f (f (f (f (f (f x))))))))" {
		t.Fatal("full mode changed or abbreviated term", full)
	}
	compliant, err := f.c.ASTString(term, APIASTPrintSMT2)
	astTestMust(t, err)
	if !strings.Contains(compliant, "(let") {
		t.Fatal("SMT2 mode ignored global alias parameters", compliant)
	}
	bits := f.pin(f.c.BitVectorSort(8))
	value := f.pin(f.c.UintNumeral(3, bits))
	for _, mode := range []APIASTPrintMode{APIASTPrintSMTFull, APIASTPrintSMT2} {
		if got, err := f.c.ASTString(value, mode); err != nil || got != "(_ bv3 8)" {
			t.Fatal("full mode must preserve other global pp settings", mode, got, err)
		}
	}
}

func TestAPIASTPrinterLowLevelUsesSourceDefaultFlags(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	declaration := f.pin(f.c.Function("f", []ast.ASTHandle{boolean}, boolean))
	x := f.pin(f.c.Constant("x", boolean))
	application := f.pin(f.c.Apply(declaration, x))
	for _, tc := range []struct {
		value ast.ASTHandle
		want  string
	}{
		{x, "x\n"}, {application, "(f x)\n"}, {declaration, ""}, {boolean, "Bool"},
	} {
		if got, err := f.c.ASTString(tc.value, APIASTPrintLowLevel); err != nil || got != tc.want {
			t.Fatal("source compact expression-only document", got, tc.want, err)
		}
	}
}
