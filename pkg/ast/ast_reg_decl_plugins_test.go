package ast

// Source: src/ast/reg_decl_plugins.cpp:30-55.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTRegisterDeclarationPluginsSourceOrder(t *testing.T) {
	m := astTestManager(t)
	names := [...]string{"arith", "bv", "array", "datatype", "datalog_relation", "seq", "fpa", "pb"}
	kinds := [...]astPluginKind{astArithmeticPlugin, astBitVectorPlugin, astArrayPlugin, astDatatypePlugin, astDatalogPlugin, astSequencePlugin, astFloatingPointPlugin, astPseudoBooleanPlugin}
	before := m.families.next
	arith, e := m.families.GetID(astTestSymbol(t, m, "arith"))
	astTestMust(t, e)
	astTestMust(t, m.RegisterDeclarationPlugins())
	for i, name := range names {
		id, e := m.families.GetID(astTestSymbol(t, m, name))
		astTestMust(t, e)
		want := before + FamilyID(i) - 1
		if i == 0 {
			want = arith // ast_manager eagerly creates this family, but not its plugin.
		}
		if id != want || m.pluginAt(id) != (astPluginBinding{kinds[i], true}) {
			t.Fatalf("%s: family %d, binding %+v; want %d, kind %d", name, id, m.pluginAt(id), want, kinds[i])
		}
	}
	if m.bitVectorPlugin.fields[4] != m.arithmeticPlugin.fields[1] || m.floatingPointPlugin.bitvector != m.bitVectorPlugin || m.sequencePlugin.char.slot == 0 {
		t.Fatal("original dependent plugin constructors were not connected")
	}
	// A second call must not construct any new owner or reserve any storage.
	a, b, d, s, f := m.arithmeticPlugin, m.bitVectorPlugin, m.datatypePlugin, m.sequencePlugin, m.floatingPointPlugin
	used, next := m.BudgetField.Used(), m.families.next
	m.BudgetField.FailAfter(0)
	e = m.RegisterDeclarationPlugins()
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	if used != m.BudgetField.Used() || next != m.families.next || a != m.arithmeticPlugin || b != m.bitVectorPlugin || d != m.datatypePlugin || s != m.sequencePlugin || f != m.floatingPointPlugin {
		t.Fatal("repeated registration changed owners, family order or storage")
	}
}

func TestASTRegisterDeclarationPluginsPreservesExisting(t *testing.T) {
	m := astTestManager(t)
	id, e := m.RegisterArrayPlugin()
	astTestMust(t, e)
	p := m.arrayPlugin
	astTestMust(t, m.RegisterDeclarationPlugins())
	got, e := m.families.GetID(astTestSymbol(t, m, "array"))
	astTestMust(t, e)
	if got != id || m.arrayPlugin != p {
		t.Fatal("existing plugin or its family ID was replaced")
	}
	if !m.pluginAt(m.floatingPointPlugin.family).present || m.auxiliaryPlugin(astPseudoBooleanPlugin) == nil {
		t.Fatal("registration did not continue past the existing plugin")
	}
}

func TestASTRegisterDeclarationPluginsStopsAtError(t *testing.T) {
	m := astTestManager(t)
	// The eagerly interned arith family reaches its real constructor's first
	// owner reservation. This is one bounded failure, not an allocation sweep.
	m.BudgetField.FailAfter(0)
	e := m.RegisterDeclarationPlugins()
	m.BudgetField.FailAfter(-1)
	if !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatalf("constructor refusal: %v", e)
	}
	id, e := m.families.GetID(astTestSymbol(t, m, "arith"))
	astTestMust(t, e)
	if m.pluginAt(id).present || m.bitVectorPlugin != nil || m.arrayPlugin != nil || m.floatingPointPlugin != nil {
		t.Fatal("registration continued after the refused constructor")
	}
	astTestMust(t, m.RegisterDeclarationPlugins())
}

func TestRepeatedDeclarationRegistrationChecksOwners(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.RegisterDeclarationPlugins())
	other := astTestManager(t)
	original := m.Symbols
	m.Symbols = other.Symbols
	err := m.RegisterDeclarationPlugins()
	m.Symbols = original
	if !errors.Is(err, usymbol.ErrOwner) {
		t.Fatalf("foreign symbol service returned %v", err)
	}
	astTestMust(t, m.RegisterDeclarationPlugins())
	astTestMust(t, m.Close())
	if err := m.RegisterDeclarationPlugins(); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed manager returned %v", err)
	}
}

func BenchmarkRepeatedDeclarationRegistration(b *testing.B) {
	m := astTestManager(b)
	astTestMust(b, m.RegisterDeclarationPlugins())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := m.RegisterDeclarationPlugins(); err != nil {
			b.Fatal(err)
		}
	}
}
