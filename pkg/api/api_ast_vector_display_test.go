package api

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"

	"strings"
	"testing"
)

func TestASTVectorStringUsesSourceEnvelopeAndKinds(t *testing.T) {
	f, a, _ := astVectorFixture(t)
	boolean := f.pin(f.m.MkBoolSort())
	declaration, err := f.m.GetDecl(a)
	astTestMust(t, err)
	v, err := f.c.NewASTVectorValues(a, boolean, declaration)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, v.Close()) })
	text, err := v.String()
	astTestMust(t, err)
	if !strings.HasPrefix(text, "(ast-vector\n  ") || !strings.HasSuffix(text, ")") || strings.Count(text, "\n  ") != 3 {
		t.Fatalf("vector display: %q", text)
	}
}

// AUDIT.md api_01-F3: Z3_ast_vector_to_string / Z3_ast_map_to_string print
// every element with mk_ismt2_pp(e, m, 2 / 3): a declaration as
// declare-fun, and continuation lines shifted by the indent (text from
// build/libz3.so).
func TestASTVectorAndMapStringUseIsmt2PP(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	intSort := f.pin(f.c.IntegerSort())
	fd := f.pin(f.c.Function("f", []ast.ASTHandle{intSort}, boolean))
	var cs []ast.ASTHandle
	for i := 0; i < 8; i++ {
		cs = append(cs, f.pin(f.c.Constant(fmt.Sprintf("a_rather_long_constant_name_%d", i), boolean)))
	}
	big := f.pin(f.c.And(cs...))
	v, err := f.c.NewASTVectorValues(fd, big)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, v.Close()) })
	text, err := v.String()
	astTestMust(t, err)
	want := "(ast-vector\n  (declare-fun f (Int) Bool)\n  (and a_rather_long_constant_name_0\n"
	for i := 1; i < 8; i++ {
		want += fmt.Sprintf("       a_rather_long_constant_name_%d", i)
		if i < 7 {
			want += "\n"
		}
	}
	want += "))"
	if text != want {
		t.Fatalf("vector display:\n%s\nwant:\n%s", text, want)
	}
	mp, err := f.c.NewASTMap()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, mp.Close()) })
	astTestMust(t, mp.Insert(fd, big))
	text, err = mp.String()
	astTestMust(t, err)
	want = "(ast-map\n  ((declare-fun f (Int) Bool)\n   (and a_rather_long_constant_name_0\n"
	for i := 1; i < 8; i++ {
		want += fmt.Sprintf("        a_rather_long_constant_name_%d", i)
		if i < 7 {
			want += "\n"
		}
	}
	want += ")))"
	if text != want {
		t.Fatalf("map display:\n%s\nwant:\n%s", text, want)
	}
}
