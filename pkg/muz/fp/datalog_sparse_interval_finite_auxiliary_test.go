package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSparseIntervalFiniteAuxiliaryPrecisionSurvivesClone(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	original := declare("finite-auxiliary-mixed-original", []ast.ASTHandle{boolean, integer})
	finite := declare("finite-auxiliary-finite-query", []ast.ASTHandle{boolean})
	nullary := declare("finite-auxiliary-nullary-query", nil)
	arithmetic := declare("finite-auxiliary-arithmetic-query", []ast.ASTHandle{integer})
	activation, selected, err := newSparseIntervalDatalogActivation(m, engine.set, engine.guard, []ast.ASTHandle{original})
	fragmentMust(t, err)
	if !selected {
		t.Fatal("mixed activation was not selected")
	}
	fragmentMust(t, activation.syncAuxiliaryPredicates([]ast.ASTHandle{original, finite, nullary, arithmetic}))
	fragmentMust(t, activation.markFiniteAuxiliary(finite))
	fragmentMust(t, activation.markFiniteAuxiliary(nullary))
	fragmentMust(t, activation.markFiniteAuxiliary(arithmetic))
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, activation.addFact(finite, []ast.ASTHandle{truth}))
	fragmentMust(t, activation.addFact(nullary, nil))
	fragmentMust(t, activation.addFact(arithmetic, []ast.ASTHandle{one}))
	status, err := activation.query([]ast.ASTHandle{finite}, true)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("finite auxiliary query was not precise", status)
	}
	if snapshot, ok, cause := activation.finiteSnapshot(finite); cause != nil || ok || snapshot != nil {
		t.Fatal("generated finite auxiliary became a public original snapshot", ok, cause)
	}
	status, err = activation.query([]ast.ASTHandle{arithmetic}, true)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("arithmetic auxiliary was incorrectly marked precise", status)
	}

	clone, err := activation.clone(engine.set, engine.guard)
	fragmentMust(t, err)
	fragmentMust(t, activation.close())
	for _, declaration := range []ast.ASTHandle{finite, nullary} {
		status, cause := clone.query([]ast.ASTHandle{declaration}, true)
		fragmentMust(t, cause)
		if status != lbool.True {
			t.Fatal("clone lost precise finite auxiliary class", declaration, status)
		}
	}
	fragmentMust(t, clone.close())
}
