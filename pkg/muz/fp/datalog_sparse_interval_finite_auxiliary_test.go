package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSparseIntervalDerivedFiniteAuxiliaryShapeSurvivesClone(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	seed := declare("shape-finite-seed", []ast.ASTHandle{boolean})
	finite := declare("shape-finite-derived", []ast.ASTHandle{boolean})
	nullary := declare("shape-nullary-derived", nil)
	variable, err := m.MkVar(0, boolean)
	fragmentMust(t, err)
	app := func(declaration ast.ASTHandle, values ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, values)
		fragmentMust(t, cause)
		return result
	}
	rule, err := m.MkImplies(app(seed, variable), app(finite, variable))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "derive finite shape"))
	rule, err = m.MkImplies(app(seed, variable), app(nullary))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "derive nullary shape"))

	activation, selected, err := newClassifiedSparseIntervalDatalogActivation(m, engine.set, engine.guard, []sparseIntervalOriginal{{declaration: seed, class: sparseIntervalFiniteOriginal}})
	fragmentMust(t, err)
	if !selected {
		t.Fatal("finite activation was not selected")
	}
	fragmentMust(t, activation.syncAuxiliaryPredicates([]ast.ASTHandle{seed, finite, nullary}))
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	fragmentMust(t, activation.addFact(seed, []ast.ASTHandle{truth}))
	status, err := activation.query([]ast.ASTHandle{finite, nullary}, true)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("derived finite/nullary shape was not precise", status)
	}
	for _, declaration := range []ast.ASTHandle{finite, nullary} {
		if snapshot, exact, cause := activation.finiteSnapshot(declaration); cause != nil || exact || snapshot != nil {
			t.Fatal("generated auxiliary became a public original snapshot", declaration, exact, cause)
		}
	}

	clone, err := activation.clone(engine.set, engine.guard)
	fragmentMust(t, err)
	fragmentMust(t, activation.close())
	status, err = clone.query([]ast.ASTHandle{finite, nullary}, true)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("clone lost derived finite/nullary precision after source close", status)
	}
	fragmentMust(t, clone.close())
}
