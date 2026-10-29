package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type arithmeticModelFixture struct {
	*fmFixture
	plugin   *ArithmeticPlugin
	context  *SolverContext
	contains *ast.ContainsApp
	model    *model.Model
	x        ast.ASTHandle
}

func newArithmeticModelFixture(t *testing.T, real bool, value string) *arithmeticModelFixture {
	f := &arithmeticModelFixture{fmFixture: newFMFixture(t)}
	fmMust(t, f.r.manager.RegisterDeclarationPlugins())
	sort := f.integer
	if real {
		sort = f.real
	}
	f.x = f.constant("arith-model-x", sort)
	var interpreted ast.ASTHandle
	if real {
		interpreted = f.own(f.r.manager.ArithmeticRealNumeralText(value))
	} else {
		interpreted = f.number(value)
	}
	var err error
	f.contains, err = ast.NewContainsApp(f.r.manager, f.x)
	fmMust(t, err)
	f.context, err = NewSolverContext(f.r.manager, []ast.ASTHandle{f.x})
	fmMust(t, err)
	f.plugin, err = NewArithmeticPlugin(f.context)
	fmMust(t, err)
	f.model, err = model.NewModel(f.r.manager)
	fmMust(t, err)
	decl, err := f.r.manager.GetDecl(f.x)
	fmMust(t, err)
	fmMust(t, f.model.Core().RegisterConstant(decl, interpreted))
	t.Cleanup(func() {
		fmMust(t, f.model.Close())
		fmMust(t, f.plugin.Close())
		fmMust(t, f.context.Close())
		fmMust(t, f.contains.Close())
	})
	return f
}

func (f *arithmeticModelFixture) project(t *testing.T, atoms ...ast.ASTHandle) (ast.ASTHandle, ast.ASTRef) {
	t.Helper()
	probe, err := NewArithmeticBoundsProcessor(f.plugin.util)
	fmMust(t, err)
	for i, atom := range atoms {
		matched, cause := probe.Add(f.contains, atom)
		fmMust(t, cause)
		if !matched {
			t.Fatalf("model projection bound %d rejected", i)
		}
	}
	fmMust(t, probe.Close())
	formula := f.own(f.r.manager.MkAndArray(atoms))
	ok, err := f.plugin.Prepare(f.contains, formula, atoms, nil)
	fmMust(t, err)
	if !ok {
		t.Fatal("model projection bounds rejected")
	}
	result, err := f.r.manager.NewRef(formula)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, f.r.manager.DelRef(&result)) })
	projected, err := f.plugin.ProjectModel(f.contains, f.model, result, f.r.config)
	fmMust(t, err)
	if !projected {
		t.Fatal("model projection declined prepared bounds")
	}
	h, err := f.r.manager.RefGet(result)
	fmMust(t, err)
	if found, cause := f.contains.Check(h); cause != nil || found {
		t.Fatal("model projection retained variable", found, cause)
	}
	return formula, result
}
