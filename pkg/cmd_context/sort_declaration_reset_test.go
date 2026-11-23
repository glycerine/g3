package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSortDeclarationResetAfterPartialPop(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetBoolOption("global-declarations", false))
	boolean, err := c.LookupSort("Bool")
	contextMust(t, err)
	contextMust(t, c.DefineSort("Alias", nil, boolean))
	contextMust(t, c.InsertSort("ParserAlias", boolean))
	_, err = c.DeclareSortConstructor("U", 0)
	contextMust(t, err)
	_, err = c.InstantiateSort("U", nil)
	contextMust(t, err)
	contextMust(t, c.Push(1))
	keep := len(c.sortTrail)
	contextMust(t, c.DefineSort("Temporary", nil, boolean))
	contextMust(t, c.DefineSort("Other", nil, boolean))
	// A pop releases the sort trail before erasing its dictionary entries.
	contextMust(t, c.restoreDeclarations(len(c.functionTrail), keep))
	contextMust(t, c.InsertSort("Replacement", boolean))
	contextMust(t, c.resetPSortDecls())
	for _, row := range c.sortTrail {
		if row.ref != (ast.ASTRef{}) {
			t.Fatal("sort reference survived reset", row.name)
		}
	}
	for _, ref := range c.sortConstructors["U"].refs {
		if ref != (ast.ASTRef{}) {
			t.Fatal("constructor instance survived reset")
		}
	}
	contextMust(t, c.resetPSortDecls())
	contextMust(t, c.Reset())
}
