package ast

import (
	"bytes"
	"errors"
	"fmt"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"math/big"

	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func astDeclarationHeadFixture(t *testing.T, m *ASTManager, wide bool) (ASTHandle, string) {
	t.Helper()
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	child, err := m.MkTrue()
	astTestMust(t, err)
	astTestPin(t, m, child)
	id, err := m.GetASTID(child)
	astTestMust(t, err)
	var ps [6]ASTParameter
	ps[0], err = m.NewParameterInt(-7)
	astTestMust(t, err)
	ps[1], err = m.NewParameterAST(child)
	astTestMust(t, err)
	ps[2], err = m.NewParameterSymbol(usymbol.Numerical(19))
	astTestMust(t, err)
	r, ok := new(big.Rat).SetString("123456789012345678901234567891/17")
	if !ok {
		t.Fatal("rational")
	}
	if wide {
		r.SetFrac(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 8192), big.NewInt(7)), big.NewInt(17))
	}
	pr := unumeral.NewRational()
	astTestMust(t, m.NumeralService().Manager.SetString(&pr, r.RatString()))
	ps[3], err = m.NewParameterFromRational(&pr)
	astTestMust(t, errors.Join(err, m.NumeralService().Manager.Del(&pr)))
	astTestMust(t, err)
	ps[4], err = m.NewParameterDouble(123456.789)
	astTestMust(t, err)
	ps[5], err = m.NewParameterExternal(29)
	astTestMust(t, err)
	info, err := m.NewFuncDeclInfo(NullFamilyID, 0, ps[:], ASTFuncFlags{}, true)
	astTestMust(t, err)
	decl, err := m.MkFuncDeclWithInfo(usymbol.Numerical(9), nil, b, info)
	astTestMust(t, err)
	astTestPin(t, m, decl)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	for i := range ps {
		astTestMust(t, m.DelParameter(&ps[i]))
	}
	return decl, fmt.Sprintf("k!9[-7:#%d:k!19:%s:123457:@29]", id, r.RatString())
}

type astHeadPrefixWriter struct {
	bytes.Buffer
	remaining int
}

func (w *astHeadPrefixWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n, _ := w.Buffer.Write(p[:w.remaining])
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return w.Buffer.Write(p)
}

type astHeadShortWriter struct{}

func (astHeadShortWriter) Write([]byte) (int, error) { return 0, nil }

func TestASTDeclarationHeadPrivateParametersAndPrefixes(t *testing.T) {
	m := astTestManager(t)
	decl, want := astDeclarationHeadFixture(t, m, false)
	var out bytes.Buffer
	astTestMust(t, m.DisplayDeclarationHead(&out, decl))
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	for cut := 0; cut < len(want); cut++ {
		w := &astHeadPrefixWriter{remaining: cut}
		// Override bytes.Buffer's promoted WriteString so the byte limit is used.
		if err := m.DisplayDeclarationHead(struct{ io.Writer }{w}, decl); !errors.Is(err, io.ErrClosedPipe) || w.String() != want[:cut] {
			t.Fatal(cut, err, w.String(), want[:cut])
		}
	}
	if err := m.DisplayDeclarationHead(astHeadShortWriter{}, decl); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}

func TestASTDeclarationHeadOrdinaryAndGuards(t *testing.T) {
	m := astTestManager(t)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "plain-head"), nil, b, ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, decl)
	var out bytes.Buffer
	astTestMust(t, m.DisplayDeclarationHead(&out, decl))
	if out.String() != "plain-head" {
		t.Fatal(out.String())
	}
	if err = m.DisplayDeclarationHead(nil, decl); err != ErrASTHandle {
		t.Fatal(err)
	}
	if err = m.DisplayDeclarationHead(&out, b); err == nil {
		t.Fatal("accepted sort as declaration")
	}
	var absent *ASTManager
	if err = absent.DisplayDeclarationHead(&out, decl); err == nil {
		t.Fatal("accepted nil manager")
	}
}

func TestASTDeclarationHeadLargeNumeralWithoutOwnerGrowth(t *testing.T) {
	m := astTestManager(t)
	decl, want := astDeclarationHeadFixture(t, m, true)
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	var out bytes.Buffer
	err := m.DisplayDeclarationHead(&out, decl)
	m.Budget().FailAfter(-1)
	astTestMust(t, err)
	if out.String() != want || m.Budget().Used() != before {
		t.Fatal("large borrowed parameter output", len(out.String()), len(want), m.Budget().Used(), before)
	}
	// Character and formatting buffers are Go slices in the retained numeral
	// service. Printing borrows the parameter and acquires no charged AST owner.
}
