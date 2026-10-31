package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
)

func TestPredicateSelectionStrategiesAndCachedScores(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	globals, err := ctx.GlobalParameters()
	must(err)
	rw, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	integer, err := m.GetSort(one)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	var decls []ast.ASTHandle
	for _, name := range []string{"sel-r", "sel-p", "sel-q"} {
		sym, e := m.Symbols.Intern(name)
		must(e)
		decl, e := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		decls = append(decls, decl)
	}
	apply := func(decl, arg ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(decl, []ast.ASTHandle{arg})
		must(e)
		return h
	}
	x, err := m.MkVar(0, integer)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	var clauses []*Clause
	defer func() {
		for _, c := range clauses {
			must(c.Close())
		}
	}()
	makeClause := func(head ast.ASTHandle, preds []ast.ASTHandle) *Clause {
		c, e := NewClause(m, rw, head, preds, truth)
		must(e)
		clauses = append(clauses, c)
		return c
	}
	r1, p1, q1, qx := apply(decls[0], one), apply(decls[1], one), apply(decls[2], one), apply(decls[2], x)
	rules := &RuleIndex{manager: m, clauses: []*Clause{makeClause(r1, nil), makeClause(p1, nil), makeClause(q1, nil), makeClause(qx, nil)}}
	target := makeClause(r1, []ast.ASTHandle{r1, apply(decls[1], x), qx})
	for _, tc := range []struct {
		strategy string
		want     uint32
	}{{"first", 0}, {"basic-weight", 0}, {"weight", 0}, {"var-use", 1}, {"unknown", 0}} {
		sel, e := newPredicateSelection(m, tc.strategy)
		must(e)
		must(sel.Init(rules))
		got, e := sel.Select(target)
		must(e)
		if got != tc.want {
			t.Fatalf("%s selected %d want %d", tc.strategy, got, tc.want)
		}
		if tc.strategy == "weight" {
			for i := 0; i < 19; i++ {
				_, e = sel.Select(target)
				must(e)
			}
			if sel.multiplier != 1.1 || sel.frequency != 22 || sel.next != 22 {
				t.Fatalf("weight schedule %v %d %d", sel.multiplier, sel.frequency, sel.next)
			}
			if len(sel.cache) != 3 {
				t.Fatalf("cache size %d", len(sel.cache))
			}
		}
		must(sel.Close())
	}
}
