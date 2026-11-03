package rel_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestSourceRecursivePredicateOrderUsesConcreteDeclarations(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := command.Close(); err != nil {
			t.Fatal(err)
		}
	})
	manager, err := command.Manager()
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	var roots []ast.ASTRef
	t.Cleanup(func() {
		for i := len(roots) - 1; i >= 0; i-- {
			if err := manager.DelRef(&roots[i]); err != nil {
				t.Fatal(err)
			}
		}
	})
	declare := func(name string) ast.ASTHandle {
		declaration := compilerDeclaration(t, manager, name, nil, boolean)
		root, err := manager.NewRef(declaration)
		if err != nil {
			t.Fatal(err)
		}
		roots = append(roots, root)
		return declaration
	}
	h, g, external := declare("order-H"), declare("order-G"), declare("order-external")
	heads := []ast.ASTHandle{h, g}
	dependencies := map[ast.ASTHandle][]ast.ASTHandle{h: {h, g, g, external}, g: {h}}
	for run := 0; run < 2; run++ {
		order, err := rel.SourceRecursivePredicateOrder(heads, dependencies)
		if err != nil || len(order) != 2 || order[0] != g || order[1] != h {
			t.Fatal("self-cycle is removed before the mutual dependency order", order, err)
		}
		order[0] = external
	}
	if heads[0] != h || len(dependencies[h]) != 4 || dependencies[h][0] != h {
		t.Fatal("ordering changed caller predicate metadata")
	}
	order, err := rel.SourceRecursivePredicateOrder(heads, map[ast.ASTHandle][]ast.ASTHandle{h: {g, g}})
	if err != nil || len(order) != 2 || order[0] != g || order[1] != h {
		t.Fatal("duplicate dependencies changed acyclic ordering", order, err)
	}
	for _, invalid := range [][]ast.ASTHandle{{h, h}, {{}}} {
		if _, err := rel.SourceRecursivePredicateOrder(invalid, dependencies); !errors.Is(err, rel.ErrCompilerUnsupported) {
			t.Fatal("invalid declaration set accepted", err)
		}
	}
}
