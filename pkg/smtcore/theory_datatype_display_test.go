package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTDatatypeDisplayPrivateRootsAndScopedMetadata(t *testing.T) {
	m, dt, x, _, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	var out bytes.Buffer
	xID, err := m.GetASTID(x)
	astTestMust(t, err)
	xRow := fmt.Sprintf("v0 #%d -> v0 (null)\n", xID)
	astTestMust(t, s.DisplayTheories(&out))
	if out.String() != "Theory datatype:\n"+xRow {
		t.Fatal("context omitted datatype variables", out.String())
	}
	c, err := m.MkApp(constructors[0], nil)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(c, false))
	cID, err := m.GetASTID(c)
	astTestMust(t, err)
	cRow := fmt.Sprintf("v1 #%d -> v1 incremental-a\n", cID)
	astTestMust(t, s.PushScope())
	// A private merge leaves the enode roots distinct. The diagnostic must
	// report the private root and each variable's own constructor pointer.
	astTestMust(t, dt.NewEquality(0, 1))
	if s.context.enodes.get(x).root == s.context.enodes.get(c).root {
		t.Fatal("fixture lost distinct egraph roots")
	}
	out.Reset()
	astTestMust(t, s.DisplayTheories(&out))
	want := "Theory datatype:\n" + fmt.Sprintf("v0 #%d -> v1 (null)\n", xID) + cRow
	if out.String() != want {
		t.Fatal("display substituted root metadata for non-root metadata", out.String(), want)
	}
	astTestMust(t, s.PopScopes(1))
	out.Reset()
	astTestMust(t, s.DisplayTheories(&out))
	if out.String() != "Theory datatype:\n"+xRow+cRow {
		t.Fatal("private root survived pop", out.String())
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, dt.NewEquality(1, 0))
	out.Reset()
	astTestMust(t, dt.DisplayVariable(&out, 0))
	if out.String() != fmt.Sprintf("v0 #%d -> v0 incremental-a\n", xID) {
		t.Fatal("adopted constructor missing", out.String())
	}
	astTestMust(t, s.PopScopes(1))
	out.Reset()
	astTestMust(t, dt.DisplayVariable(&out, 0))
	if out.String() != xRow {
		t.Fatal("adopted constructor survived pop", out.String())
	}
	if err = dt.DisplayVariable(&out, SMTNullTheoryVar); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid variable accepted", err)
	}
	if err = dt.Display(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil writer accepted", err)
	}
	if err = dt.Display(smtArrayDisplayErrorWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("writer failure lost", err)
	}
}

func TestSMTDatatypeEmptyDisplayAndClosedOwner(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	s, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() {
		if !s.closed {
			astTestMust(t, s.Close())
		}
	})
	dt, err := NewSMTDatatypeTheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(dt))
	var out bytes.Buffer
	astTestMust(t, s.DisplayTheories(&out))
	if out.Len() != 0 {
		t.Fatal("empty theory printed a header")
	}
	astTestMust(t, s.Close())
	if err = dt.Display(&out); err == nil {
		t.Fatal("closed theory accepted display")
	}
}

func TestSMTDatatypeDisplayBoundsConstructorArguments(t *testing.T) {
	f := newSMTDatatypeConflictFixture(t, false)
	m, s := f.m, f.k.internalizer
	element, err := m.GetArg(f.left, 0)
	astTestMust(t, err)
	sort, err := m.GetSort(element)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "display-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	var cut ast.ASTHandle
	for i := 0; i < 4; i++ {
		element, err = m.MkApp(function, []ast.ASTHandle{element})
		astTestMust(t, err)
		if i == 1 {
			cut = element
		}
	}
	constructor, err := m.GetDecl(f.left)
	astTestMust(t, err)
	value, err := m.MkApp(constructor, []ast.ASTHandle{element})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(value, false))
	node := s.context.enodes.get(value)
	variable := node.TheoryVar(f.dt.FamilyID())
	id, err := m.GetASTID(value)
	astTestMust(t, err)
	cutID, err := m.GetASTID(cut)
	astTestMust(t, err)
	var out bytes.Buffer
	astTestMust(t, f.dt.DisplayVariable(&out, variable))
	want := fmt.Sprintf("v%d #%d -> v%d (conflict-left (display-f (display-f #%d)))\n", variable, id, variable, cutID)
	if out.String() != want {
		t.Fatal("constructor display ignored source depth three", out.String(), want)
	}
}
