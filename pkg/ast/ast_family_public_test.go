package ast

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestASTEnsureFamilyIDSourceSemantics(t *testing.T) {
	m := astTestManager(t)
	arith, err := m.EnsureFamilyID("arith")
	if err != nil || arith != 5 {
		t.Fatal("existing family", arith, err)
	}
	bv, err := m.EnsureFamilyID("bv")
	if err != nil || bv != 6 {
		t.Fatal("new family", bv, err)
	}
	again, err := m.EnsureFamilyID("bv")
	if err != nil || again != bv {
		t.Fatal("repeated family", again, err)
	}
	name, err := m.Symbols.Intern("bv")
	if err != nil {
		t.Fatal(err)
	}
	lookedUp, err := m.FamilyIDForSymbol(name)
	if err != nil || lookedUp != bv {
		t.Fatal("family lookup", lookedUp, err)
	}
	// mk_family_id only records the name. It does not silently install a
	// declaration plugin to make a dependent client compile.
	app, err := m.MkFamilyApp(bv, 0, nil, nil)
	if err != nil || !app.IsNull() {
		t.Fatal("family interning installed a plugin", app, err)
	}
}

func TestASTEnsureFamilyIDCStringAndLifetime(t *testing.T) {
	m := astTestManager(t)
	first, err := m.EnsureFamilyID("duality\x00ignored")
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.EnsureFamilyID("duality")
	if err != nil || second != first {
		t.Fatal("C string family name", first, second, err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.EnsureFamilyID("closed"); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed manager", err)
	}
}
