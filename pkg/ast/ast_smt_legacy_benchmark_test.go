package ast

import (
	"strings"
	"testing"
)

func TestLegacySMTPrinterBenchmarkMetadataDeclarationsAndAssertion(t *testing.T) {
	m := astTestManager(t)
	printer, err := NewLegacySMTPrinter(m)
	astTestMust(t, err)
	aSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "legacy-A"), nil)
	astTestMust(t, err)
	astTestPin(t, m, aSort)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "legacy-a"), aSort)
	astTestMust(t, err)
	astTestPin(t, m, a)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "legacy-f"), []ASTHandle{aSort}, aSort, ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, f)
	fa, err := m.MkApp(f, []ASTHandle{a})
	astTestMust(t, err)
	astTestPin(t, m, fa)
	predicate, err := m.MkFuncDecl(astTestSymbol(t, m, "legacy-p"), []ASTHandle{aSort}, boolSort, ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, predicate)
	root, err := m.MkApp(predicate, []ASTHandle{fa})
	astTestMust(t, err)
	astTestPin(t, m, root)
	config := LegacySMTBenchmarkConfig{
		BenchmarkName: astTestSymbol(t, m, "bench"),
		SourceInfo:    astTestSymbol(t, m, "source"),
		Status:        astTestSymbol(t, m, "sat"),
		Category:      astTestSymbol(t, m, "crafted"),
		Logic:         astTestSymbol(t, m, "QF_UF"),
		Attributes:    ":notes exact\n",
	}
	var out strings.Builder
	astTestMust(t, printer.DisplayBenchmark(&out, root, config))
	want := "; bench\n; :source { source }\n(set-info :status sat)\n; :category { crafted }\n(set-logic QF_UF)\n; :notes exact\n" +
		"(declare-sort legacy-A)\n" +
		" (declare-fun legacy-f (legacy-A) legacy-A)\n" +
		"(declare-fun legacy-a () legacy-A)\n" +
		"(declare-fun legacy-p (legacy-A) Bool)\n" +
		"(assert\n (legacy-p (legacy-f legacy-a)))\n(check-sat)\n"
	if out.String() != want {
		t.Fatalf("legacy SMT benchmark\n got: %q\nwant: %q", out.String(), want)
	}
}

func TestLegacySMTPrinterBenchmarkTrueDefaultStatus(t *testing.T) {
	m := astTestManager(t)
	printer, err := NewLegacySMTPrinter(m)
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	astTestPin(t, m, truth)
	var out strings.Builder
	astTestMust(t, printer.DisplayBenchmark(&out, truth, LegacySMTBenchmarkConfig{}))
	if out.String() != "(set-info :status unknown)\n(check-sat)\n" {
		t.Fatalf("legacy SMT true benchmark %q", out.String())
	}
}
