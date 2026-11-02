package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSparseIntervalDatalogStoreMixedSeedsResetAndOwnership(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	integer, err := m.MkFamilySort(family, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	name, err := m.Symbols.Intern("sparse-interval-store")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	predicateRoot, err := m.NewRef(predicate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.DelRef(&predicateRoot) })
	before, err := m.GetRefCount(predicate)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewSparseIntervalDatalogStore(m)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Register(predicate); err != nil {
		t.Fatal(err)
	}
	pinned, err := m.GetRefCount(predicate)
	if err != nil || pinned != before+1 {
		t.Fatal("predicate pin", before, pinned, err)
	}
	emptySnapshot, err := store.Evaluator(f.engine.set, f.engine.guard)
	if err != nil {
		t.Fatal(err)
	}
	truth, err := m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AddFact(predicate, []ast.ASTHandle{truth, one}); err != nil {
		t.Fatal(err)
	}
	if status, cause := emptySnapshot.Query([]ast.ASTHandle{predicate}); cause != nil || status != lbool.False {
		t.Fatal("seed snapshot changed after store insertion", status, cause)
	}
	if err = emptySnapshot.Close(); err != nil {
		t.Fatal(err)
	}
	seededSnapshot, err := store.Evaluator(f.engine.set, f.engine.guard)
	if err != nil {
		t.Fatal(err)
	}
	if status, cause := seededSnapshot.Query([]ast.ASTHandle{predicate}); cause != nil || status != lbool.Undef {
		t.Fatal("nonempty overapproximate query status", status, cause)
	}
	if err = seededSnapshot.Close(); err != nil {
		t.Fatal(err)
	}
	empty, err := store.entries[predicate].relation.Empty()
	if err != nil || empty {
		t.Fatal("seed product remained empty", empty, err)
	}
	if err = store.Reset(); err != nil {
		t.Fatal(err)
	}
	empty, err = store.entries[predicate].relation.Empty()
	if err != nil || !empty {
		t.Fatal("reset product not empty", empty, err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := m.GetRefCount(predicate)
	if err != nil || after != before {
		t.Fatal("predicate pin release", before, after, err)
	}
	if err = store.AddFact(predicate, []ast.ASTHandle{truth, one}); err == nil {
		t.Fatal("closed store accepted fact")
	}
}

func TestSparseIntervalDatalogStoreRejectsSingleComponentSignatures(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	name, err := m.Symbols.Intern("sparse-interval-finite-only")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewSparseIntervalDatalogStore(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err = store.Register(predicate); err == nil {
		t.Fatal("finite-only signature accepted")
	}
}
