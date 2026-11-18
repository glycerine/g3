package model_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source: src/model/model_v2_pp.cpp:22-83. Display keeps source registration
// order, function-entry order and the distinct empty-entry else prefix.
func TestModelV2DisplayConstantsFunctionsAndPartialElse(t *testing.T) {
	m, md, _, context, _, config := evaluatorFixture(t)
	no, err := m.MkFalse()
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	constant := goalTestAtom(t, m, "display-const")
	d, err := m.GetDecl(constant)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(d, no))
	f := modelTestFunctionDecl(t, m, "display-f")
	fi := modelTestFunction(t, m, 1)
	astTestMust(t, fi.InsertEntry([]ast.ASTHandle{yes}, no))
	astTestMust(t, fi.SetElse(yes))
	astTestMust(t, md.Core().RegisterFunction(f, fi))
	g := modelTestFunctionDecl(t, m, "display-g")
	gi := modelTestFunction(t, m, 1)
	astTestMust(t, gi.SetElse(no))
	astTestMust(t, md.Core().RegisterFunction(g, gi))
	var out bytes.Buffer
	astTestMust(t, md.Core().WriteV2(&out, context, config.Globals, false))
	want := "display-const -> false\n" +
		"display-f -> {\n  true -> false\n  else -> true\n}\n" +
		"display-g -> {\n  false\n}\n"
	if out.String() != want {
		t.Fatalf("source full model display: got %q, want %q", out.String(), want)
	}
	out.Reset()
	astTestMust(t, md.Core().WriteV2(&out, context, config.Globals, true))
	want = "display-const -> false\n" +
		"display-f -> {\n  true -> false\n  else -> #unspecified\n}\n" +
		"display-g -> {\n  #unspecified\n}\n"
	if out.String() != want {
		t.Fatalf("source partial model display: got %q, want %q", out.String(), want)
	}
	refused := &refusingModelEvaluatorTraceWriter{}
	if err := md.Core().WriteV2(refused, context, config.Globals, false); !errors.Is(err, io.ErrClosedPipe) || refused.calls != 1 {
		t.Fatal("ordinary model display must report its writer refusal", err, refused.calls)
	}
}

// AUDIT.md model_01-F9: model_v2_pp prints a constant's value with
// mk_pp(value, m, len(name)+4), so the continuation lines of a value that
// does not fit on one line are indented past "name -> ".
func TestModelV2DisplayIndentsWrappedValues(t *testing.T) {
	m, md, _, context, _, config := evaluatorFixture(t)
	var atoms []ast.ASTHandle
	for i := 0; i < 40; i++ {
		atoms = append(atoms, goalTestAtom(t, m, "display-wide-atom-"+string(rune('a'+i%26))+string(rune('a'+i/26))))
	}
	wide, err := m.MkAndArgs(atoms)
	astTestMust(t, err)
	constant := goalTestAtom(t, m, "display-wide")
	d, err := m.GetDecl(constant)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(d, wide))
	var out bytes.Buffer
	astTestMust(t, md.Core().WriteV2(&out, context, config.Globals, false))
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("value did not wrap: %q", out.String())
	}
	indent := strings.Repeat(" ", len("display-wide -> "))
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, indent) {
			t.Fatalf("continuation line not indented past the name: %q", out.String())
		}
	}
}
