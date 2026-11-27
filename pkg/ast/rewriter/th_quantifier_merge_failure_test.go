package rewriter

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestTheoryRewriterRefusedQuantifierMerge(t *testing.T) {
	m, r := theoryRewriterFixture(t)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	vars := make([]ast.ASTHandle, 129)
	for i := range vars {
		vars[i], err = m.MkVar(uint32(i), b)
		astTestMust(t, err)
	}
	body, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpOr, nil, vars)
	astTestMust(t, err)
	inner, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{symbol.Numerical(7)}, Body: body})
	astTestMust(t, err)
	sorts := make([]ast.ASTHandle, 128)
	names := make([]symbol.Symbol, len(sorts))
	for i := range sorts {
		sorts[i] = b
		names[i] = symbol.Numerical(uint32(i))
	}
	outer, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: inner})
	astTestMust(t, err)
	out := astTestPin(t, m, outer)
	proof := astTestPin(t, m, ast.ASTHandle{})
	warm, err := elimUnusedVars(m, outer, true)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&warm))
	m.Budget().FailAfter(0)
	done, err := r.configuration.ReduceQuantifier(outer, inner, ast.AppArguments{}, ast.AppArguments{}, out, proof)
	m.Budget().FailAfter(-1)
	if done || !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatalf("merge reported done=%v, err=%v", done, err)
	}
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if got != outer {
		t.Fatal("refused merge changed output")
	}
	done, err = r.configuration.ReduceQuantifier(outer, inner, ast.AppArguments{}, ast.AppArguments{}, out, proof)
	astTestMust(t, err)
	got, err = m.RefGet(out)
	astTestMust(t, err)
	n, err := m.GetNumDeclarations(got)
	astTestMust(t, err)
	if !done || n != 129 {
		t.Fatalf("retry returned done=%v with %d declarations", done, n)
	}
}
