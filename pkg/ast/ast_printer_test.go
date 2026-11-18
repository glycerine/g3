package ast

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestASTPrinterDefaultIDsAndSimpleSMT2Context(t *testing.T) {
	m := astTestManager(t)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "printer-x"), b)
	astTestMust(t, err)
	astTestPin(t, m, x)
	d, err := m.GetDecl(x)
	astTestMust(t, err)

	base, err := NewDefaultASTPrinter(m)
	astTestMust(t, err)
	var out bytes.Buffer
	astTestMust(t, base.DisplaySort(&out, b, 99))
	id, err := m.GetASTID(b)
	astTestMust(t, err)
	if got, want := out.String(), "#"+strconv.FormatUint(uint64(id), 10)+"\n"; got != want {
		t.Fatal(got, want)
	}

	var regular, diagnostic bytes.Buffer
	p, err := NewSimpleASTPrinterContext(m, &regular, &diagnostic)
	astTestMust(t, err)
	if p.Manager() != m || p.RegularStream() != &regular || p.DiagnosticStream() != &diagnostic {
		t.Fatal("printer context owners")
	}
	out.Reset()
	astTestMust(t, p.DisplaySort(&out, b, 2))
	// mk_ismt2_pp(s, m, 2): mk_indent indents continuation lines only.
	if out.String() != "Bool" {
		t.Fatal("sort display", out.String())
	}
	out.Reset()
	astTestMust(t, p.DisplayExpr(&out, x, 0))
	if !strings.Contains(out.String(), "printer-x") {
		t.Fatal("expression display", out.String())
	}
	out.Reset()
	astTestMust(t, p.DisplayFuncDecl(&out, d, 3))
	// simple_ast_printer_context::display(out, f, indent) prints f's name.
	if out.String() != "printer-x" {
		t.Fatal("declaration display", out.String())
	}
	if err = p.DisplayExprVariables(&out, x, 0, 0, "x", nil); err == nil {
		t.Fatal("source NOT_IMPLEMENTED_YET branch")
	} else if z, ok := err.(z3exception.Z3Error); !ok || z.ErrorCode() != z3exception.ErrorNotImplementedYet {
		t.Fatal(err)
	}
}
