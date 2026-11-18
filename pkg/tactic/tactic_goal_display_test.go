package tactic

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func goalDisplayBindParams(t *testing.T, m *ast.ASTManager) {
	t.Helper()
	b := m.BudgetField
	ctx, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: b})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: b})
	astTestMust(t, err)
	global, err := uparams.NewGlobalParamsService(m.Symbols, ctx, uparams.GlobalParamsConfig{Budget: b, Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, global.InitializeSource())
	previous, err := uparams.BindProcessGlobalParamsService(global)
	astTestMust(t, err)
	t.Cleanup(func() {
		_, e := uparams.BindProcessGlobalParamsService(previous)
		astTestMust(t, e)
		astTestMust(t, global.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, ctx.Close())
	})
}

func TestGoalSourceDefaultDisplayAndProcessPrinter(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	var out bytes.Buffer
	astTestMust(t, g.Display(&out))
	if out.String() != "(goal)\n" {
		t.Fatalf("empty source goal = %q", out.String())
	}
	p := goalTestAtom(t, m, "display-p")
	q := goalTestAtom(t, m, "display-q")
	notQ := goalTestApp(t, m, ast.ASTOpNot, q)
	astTestMust(t, g.Assert(p))
	astTestMust(t, g.Assert(notQ))
	out.Reset()
	if err := g.Display(&out); !errors.Is(err, uparams.ErrGlobalParamsOwner) || out.String() != "(goal\n  " {
		t.Fatalf("missing source process printer: output=%q err=%v", out.String(), err)
	}
	goalDisplayBindParams(t, m)
	out.Reset()
	astTestMust(t, g.Display(&out))
	if out.String() != "(goal\n  display-p\n  (not display-q))\n" {
		t.Fatalf("source goal display = %q", out.String())
	}
	if err := g.Display(shortGoalWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short borrowed writer: %v", err)
	}
}

func TestGoalSpecializedDisplayProfiles(t *testing.T) {
	m := astTestManager(t)
	goalDisplayBindParams(t, m)
	p := goalTestAtom(t, m, "display-special-p")
	q := goalTestAtom(t, m, "display-special-q")
	notQ := goalTestApp(t, m, ast.ASTOpNot, q)
	clause := goalTestApp(t, m, ast.ASTOpOr, p, notQ)
	dependencyExpr := goalTestApp(t, m, ast.ASTOpNot, p)
	leafP, err := m.DependencyLeaf(p)
	astTestMust(t, err)
	leafCompound, err := m.DependencyLeaf(dependencyExpr)
	astTestMust(t, err)
	dependency, err := m.DependencyJoin(leafP, leafCompound)
	astTestMust(t, err)
	g := goalTestNew(t, m, false, true)
	astTestMust(t, g.AssertWithProof(clause, ast.ASTHandle{}, dependency))
	astTestMust(t, g.Assert(p))
	astTestMust(t, g.SetDepth(3))
	astTestMust(t, g.SetPrecision(GoalUnderOver))
	var out bytes.Buffer
	astTestMust(t, g.DisplayWithAttributes(&out))
	want := "(goal\n  (or display-special-p (not display-special-q))\n  display-special-p\n  :precision under-over :depth 3)\n"
	if out.String() != want {
		t.Fatalf("attribute display = %q; want %q", out.String(), want)
	}
	out.Reset()
	astTestMust(t, g.DisplayWithDependencies(&out))
	id, err := m.GetASTID(dependencyExpr)
	astTestMust(t, err)
	want = "(goal\n  |- display-special-p #" + strconv.FormatUint(uint64(id), 10) +
		"\n  (or display-special-p (not display-special-q))\n  |-\n  display-special-p\n  :precision under-over :depth 3)\n"
	if out.String() != want {
		t.Fatalf("dependency display = %q; want %q", out.String(), want)
	}
	out.Reset()
	astTestMust(t, g.DisplayAsAnd(&out))
	if got, want := out.String(), "(and (or display-special-p (not display-special-q)) display-special-p)\n"; got != want {
		t.Fatalf("and display = %q; want %q", got, want)
	}
	out.Reset()
	astTestMust(t, g.DisplayLowLevel(&out))
	if out.Len() == 0 {
		t.Fatal("empty low-level display")
	}
}

type shortGoalWriter struct{}

func (shortGoalWriter) Write(p []byte) (int, error) { return 0, nil }
