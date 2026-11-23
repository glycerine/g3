package cmd_context

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFunctionDeclarationResetAfterTrailReuse(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetBoolOption("global-declarations", false))
	boolean, err := c.LookupSort("Bool")
	contextMust(t, err)
	integer, err := c.LookupSort("Int")
	contextMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		h, err := c.DeclareFunction(name, domain, boolean)
		contextMust(t, err)
		return h
	}
	original := declare("f", nil)
	contextMust(t, c.Push(1))
	declare("f", []ast.ASTHandle{integer})
	declare("reused", []ast.ASTHandle{integer})
	contextMust(t, c.Pop(1))
	replacement := declare("reused", nil)
	overload := declare("f", []ast.ASTHandle{boolean})
	for i := 0; i < 128; i++ {
		declare("other"+strconv.Itoa(i), nil)
	}
	entry := c.funcDictionary().find("f")
	got := entry.funcs.entries(c.functionTrail)
	if len(got) != 2 || !(got[0] == original && got[1] == overload || got[1] == original && got[0] == overload) {
		t.Fatalf("live overloads %v, want %v and %v", got, original, overload)
	}
	got = c.funcDictionary().find("reused").funcs.entries(c.functionTrail)
	if len(got) != 1 || got[0] != replacement {
		t.Fatal("reused declaration", got, replacement)
	}

	// An interrupted release can leave an empty reference in a live row.
	// A retry must still release every remaining single and overloaded row.
	for i := range c.functionTrail {
		row := &c.functionTrail[i]
		if row.handle == replacement {
			contextMust(t, c.manager.DelRef(&row.ref))
			break
		}
	}
	contextMust(t, c.resetFuncDecls())
	for _, row := range c.functionTrail {
		if row.ref != (ast.ASTRef{}) {
			t.Fatalf("reference for %s survived reset", row.name)
		}
	}
	contextMust(t, c.resetFuncDecls())
	contextMust(t, c.Reset())
}
