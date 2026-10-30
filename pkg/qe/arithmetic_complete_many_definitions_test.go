package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticCompleteManyToDefinitionVector(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	truth := f.own(f.r.manager.MkTrue())
	formula, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&formula)
	receipt, e := f.plugin.ProjectVariablesNNFComplete([]ast.ASTHandle{f.x}, f.model, formula, f.r.config)
	fmMust(t, e)
	defer receipt.Close()
	definitions, e := receipt.ToDefinitionVector(f.r.config)
	fmMust(t, e)
	defer definitions.Close()
	n, e := definitions.Size()
	fmMust(t, e)
	if n != 1 {
		t.Fatal(n)
	}
	declaration, e := definitions.Variable(0)
	fmMust(t, e)
	want, e := f.r.manager.GetDecl(f.x)
	fmMust(t, e)
	if declaration != want {
		t.Fatal(declaration, want)
	}
	definition, e := definitions.Definition(0)
	fmMust(t, e)
	if definition.IsNull() {
		t.Fatal("missing definition")
	}
}
