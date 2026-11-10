package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTTheoryDisplayFlatSourceStackAndForeignApplications(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y, z, w, q := f.variable("x"), f.variable("y"), f.variable("z"), f.variable("w"), f.variable("q")
	left, right := f.app(smtArithOpAdd, x, y), f.app(smtArithOpAdd, z, w)
	term := f.app(smtArithOpAdd, left, q, right)
	for _, tc := range []struct {
		flat bool
		want string
	}{{false, "(+ (+ x y) q (+ z w))"}, {true, "(+ q z w x y)"}} {
		var out bytes.Buffer
		var err error
		if tc.flat {
			err = f.a.DisplayFlatApplication(&out, term)
		} else {
			err = f.a.DisplayApplication(&out, term)
		}
		astTestMust(t, err)
		if out.String() != tc.want {
			t.Fatal(tc.flat, out.String(), tc.want)
		}
	}
	// Child applications with another declaration recurse without flattening.
	product := f.app(smtArithOpMul, left, right)
	wrapped := f.app(smtArithOpAdd, q, product)
	var out bytes.Buffer
	astTestMust(t, f.a.DisplayFlatApplication(&out, wrapped))
	if out.String() != "(+ q (* (+ x y) (+ z w)))" {
		t.Fatal(out.String())
	}
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "foreign"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
	foreign := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
	id, err := f.m.GetASTID(foreign)
	astTestMust(t, err)
	out.Reset()
	astTestMust(t, f.a.DisplayFlatApplication(&out, foreign))
	if out.String() != fmt.Sprintf("#%d", id) {
		t.Fatal(out.String())
	}
	out.Reset()
	astTestMust(t, f.a.DisplayApplication(&out, f.number(7)))
	if out.String() != "Real[7:0]" {
		t.Fatal(out.String())
	}
}

type smtLRAOutputPrefix struct {
	text      bytes.Buffer
	remaining int
}

func (w *smtLRAOutputPrefix) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n, _ := w.text.Write(p[:w.remaining])
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return w.text.Write(p)
}

func TestSMTLRADisplayContextCacheAndIndependentRelevancy(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("display-x")
	_, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "display-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
	foreign := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
	astTestMust(t, f.s.Internalize(foreign, false))
	// No relevancy mark and no LP model: sharing is true independently of both.
	var out bytes.Buffer
	astTestMust(t, f.s.DisplayTheories(&out))
	want := "v0, shared: 1, rel: 0, def: display-x\n"
	if out.String() != want || len(f.a.modelValues) != 0 {
		t.Fatal(out.String(), want, f.a.modelValues)
	}
	astTestMust(t, f.s.relevancy.MarkRelevant(x))
	astTestMust(t, f.s.relevancy.Propagate())
	out.Reset()
	astTestMust(t, f.a.Display(&out))
	if out.String() != "v0, shared: 1, rel: 1, def: display-x\n" {
		t.Fatal(out.String())
	}
	lraFix(t, f, x, 3)
	f.propagate(true)
	astTestMust(t, f.a.initializeVariableValues())
	out.Reset()
	astTestMust(t, f.s.DisplayTheories(&out))
	if !strings.HasPrefix(out.String(), "v0 >= 3\nv0 <= 3\n") || !strings.Contains(out.String(), "v0, value: 3, shared: 1, rel: 1, def: display-x\n") {
		t.Fatal(out.String())
	}
}

func TestSMTLRADisplayEveryWriterPrefixAndScope(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("display-a"), f.variable("display-b")
	term := f.app(smtArithOpAdd, x, y)
	_, err := f.a.internalizeDef(term)
	astTestMust(t, err)
	var output bytes.Buffer
	astTestMust(t, f.a.Display(&output))
	// lar_term's unordered_map iterates column 1 before column 0.
	want := "v1 + v0\n" +
		"v0, shared: 0, rel: 0, def: display-b\n" +
		"v1, shared: 0, rel: 0, def: display-a\n" +
		"v2, value: 0, shared: 0, rel: 0, def: (+ display-a display-b)\n"
	if output.String() != want {
		t.Fatal(output.String(), want)
	}
	// Terms are cache eligible even before model initialization; their getter
	// inserts zero-valued missing columns, as the C++ source does.
	if len(f.a.modelValues) != 3 {
		t.Fatal("source cache insertion", f.a.modelValues)
	}
	astTestMust(t, f.s.PushScope())
	_, err = f.a.internalizeDef(f.app(smtArithOpAdd, x, f.number(9)))
	astTestMust(t, err)
	astTestMust(t, f.s.PopScopes(1))
	output.Reset()
	astTestMust(t, f.a.Display(&output))
	cachedWant := strings.ReplaceAll(strings.ReplaceAll(want, "v0, shared:", "v0, value: 0, shared:"), "v1, shared:", "v1, value: 0, shared:")
	if output.String() != cachedWant {
		t.Fatal("popped display with preserved cache", output.String(), cachedWant)
	}
	for cut := 0; cut < len(want); cut++ {
		// A refusal in delegated numerical printing stops that solver. Each
		// independent instance must still close all interrupted lexical owners.
		t.Run(fmt.Sprint(cut), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			x, y := f.variable("display-a"), f.variable("display-b")
			_, err := f.a.internalizeDef(f.app(smtArithOpAdd, x, y))
			astTestMust(t, err)
			w := &smtLRAOutputPrefix{remaining: cut}
			if err = f.s.DisplayTheories(w); !errors.Is(err, io.ErrClosedPipe) || w.text.String() != want[:cut] {
				t.Fatal(err, w.text.String(), want[:cut])
			}
		})
	}
	if err = f.a.Display(nil); err != ast.ErrASTHandle {
		t.Fatal(err)
	}
	if err = f.a.DisplayApplication(nil, x); err != ast.ErrASTHandle {
		t.Fatal(err)
	}
}

func TestSMTLRADisplayEmptyClosedAndShortWriter(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	var out bytes.Buffer
	astTestMust(t, f.a.Display(&out))
	if out.Len() != 0 {
		t.Fatal(out.String())
	}
	var absent *SMTLRATheory
	if err := absent.Display(&out); err != ast.ErrASTHandle {
		t.Fatal(err)
	}
	x := f.variable("display-guard-x")
	_, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	if err = f.a.Display(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	// The short write occurred after the empty LP lists; it must not stop an
	// unrelated numerical child or prevent a later ordinary display.
	astTestMust(t, f.a.Display(&out))
	astTestMust(t, f.a.Close())
	if err = f.a.Display(&out); err == nil {
		t.Fatal("closed theory displayed")
	}
}

type smtLRAShortOutput struct{}

func (smtLRAShortOutput) Write([]byte) (int, error) { return 0, nil }
