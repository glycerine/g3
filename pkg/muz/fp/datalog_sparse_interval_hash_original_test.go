package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSparseIntervalHashOriginalPreservesHashAndNullary(t *testing.T) {
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
	finite := declare("hash-original-finite", []ast.ASTHandle{boolean})
	nullary := declare("hash-original-nullary", nil)
	originals := []sparseIntervalOriginal{
		{declaration: finite, class: sparseIntervalHashOriginal},
		{declaration: nullary, class: sparseIntervalHashOriginal},
	}
	activation, selected, err := newClassifiedSparseIntervalDatalogActivation(m, engine.set, engine.guard, originals)
	fragmentMust(t, err)
	if !selected {
		t.Fatal("hash originals were not selected")
	}
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	fragmentMust(t, activation.addFact(finite, []ast.ASTHandle{truth}))
	fragmentMust(t, activation.addFact(nullary, nil))
	for _, declaration := range []ast.ASTHandle{finite, nullary} {
		status, cause := activation.query([]ast.ASTHandle{declaration}, true)
		fragmentMust(t, cause)
		if status != lbool.True {
			t.Fatal("exact hash original was not precise", declaration, status)
		}
		snapshot, exact, cause := activation.finiteSnapshot(declaration)
		fragmentMust(t, cause)
		if !exact {
			t.Fatal("hash original was not enumerable", declaration)
		}
		if _, ok := snapshot.(*rel.HashTable); !ok {
			t.Fatalf("hash original normalized to %T", snapshot)
		}
		facts, cause := snapshot.Facts()
		fragmentMust(t, cause)
		if len(facts) != 1 {
			t.Fatal("hash snapshot lost row", declaration, facts)
		}
		fragmentMust(t, snapshot.Close())
	}
	clone, err := activation.clone(engine.set, engine.guard)
	fragmentMust(t, err)
	fragmentMust(t, activation.close())
	for _, declaration := range []ast.ASTHandle{finite, nullary} {
		status, cause := clone.query([]ast.ASTHandle{declaration}, true)
		fragmentMust(t, cause)
		if status != lbool.True {
			t.Fatal("clone lost exact hash original", declaration, status)
		}
	}
	fragmentMust(t, clone.close())
}

func TestSparseIntervalArithmeticNullaryResetPreservesComponentRoles(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("interval-original-nullary")
	fragmentMust(t, err)
	declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(declaration))
	activation, selected, err := newClassifiedSparseIntervalDatalogActivation(m, engine.set, engine.guard, []sparseIntervalOriginal{{declaration: declaration, class: sparseIntervalArithmeticOriginal}})
	fragmentMust(t, err)
	if !selected {
		t.Fatal("interval nullary activation was not selected")
	}
	fragmentMust(t, activation.addFact(declaration, nil))
	status, err := activation.query([]ast.ASTHandle{declaration}, true)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("nonempty interval nullary was reported exact", status)
	}
	fragmentMust(t, activation.reset())
	status, err = activation.query([]ast.ASTHandle{declaration}, true)
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("reset interval nullary was not false", status)
	}
	fragmentMust(t, activation.close())
}
