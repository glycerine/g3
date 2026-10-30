package smt2

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMT2LocalSymbolTableNestedAndRepeatedBindings(t *testing.T) {
	var env smt2LocalEnv
	x, y, z := symbol.Numerical(1), symbol.Numerical(2), symbol.Numerical(3)
	check := func(name symbol.Symbol, level uint32, present bool) {
		t.Helper()
		value, ok := env.find(name)
		if ok != present || env.contains(name) != present || present && value.level != level {
			t.Fatalf("binding %v: got (%v, %v), want level %d present %v", name, value, ok, level, present)
		}
	}
	env.insert(x, smt2Local{level: 1})
	env.beginScope()
	env.insert(x, smt2Local{level: 2})
	env.insert(y, smt2Local{level: 3})
	env.insert(x, smt2Local{level: 4})
	env.insert(y, smt2Local{level: 5})
	check(x, 4, true)
	check(y, 5, true)
	env.beginScope()
	env.insert(x, smt2Local{level: 6})
	env.insert(z, smt2Local{level: 7})
	if err := env.endScope(); err != nil {
		t.Fatal(err)
	}
	check(x, 4, true)
	check(y, 5, true)
	check(z, 0, false)
	if err := env.endScope(); err != nil {
		t.Fatal(err)
	}
	check(x, 1, true)
	check(y, 0, false)
	if len(env.trail) != 0 || len(env.limits) != 0 {
		t.Fatal("scope trail was not drained")
	}
	if err := env.endScope(); !errors.Is(err, errSMT2LocalScope) {
		t.Fatal("empty scope guard", err)
	}
	env.insert(x, smt2Local{level: 8})
	check(x, 8, true)
}

// Source: src/test/symbol_table.cpp::tst1. The value is the parser's retained
// concrete symbol_table<local> specialization rather than an int-only adapter.
func TestSMT2LocalSymbolTableSourceAssertions(t *testing.T) {
	m := localLookupManager(t)
	foo, err := m.Symbols.Intern("foo")
	if err != nil {
		t.Fatal(err)
	}
	boo, err := m.Symbols.Intern("boo")
	if err != nil {
		t.Fatal(err)
	}
	var env smt2LocalEnv
	env.insert(foo, smt2Local{level: 35})
	if !env.contains(foo) || env.contains(boo) {
		t.Fatal("source initial membership")
	}
	env.beginScope()
	env.insert(boo, smt2Local{level: 20})
	if !env.contains(boo) {
		t.Fatal("source scoped membership")
	}
	if value, found := env.find(boo); !found || value.level != 20 {
		t.Fatal("source scoped lookup", value, found)
	}
	if value, found := env.find(foo); !found || value.level != 35 {
		t.Fatal("source outer lookup", value, found)
	}
	env.insert(foo, smt2Local{level: 100})
	if value, found := env.find(foo); !found || value.level != 100 {
		t.Fatal("source scoped overwrite", value, found)
	}
	if err := env.endScope(); err != nil {
		t.Fatal(err)
	}
	if value, found := env.find(foo); !found || value.level != 35 {
		t.Fatal("source shadow restoration", value, found)
	}
	if env.contains(boo) {
		t.Fatal("source scoped key removal")
	}
	env.reset()
	if env.contains(boo) || env.contains(foo) {
		t.Fatal("source reset")
	}
}

func TestSMT2LocalSymbolTableResetDiscardsOpenScopes(t *testing.T) {
	var env smt2LocalEnv
	x, y := symbol.Numerical(1), symbol.Numerical(2)
	env.insert(x, smt2Local{level: 1})
	env.beginScope()
	env.insert(x, smt2Local{level: 2})
	env.insert(y, smt2Local{level: 3})
	env.reset()
	if env.contains(x) || env.contains(y) || len(env.trail) != 0 || len(env.limits) != 0 {
		t.Fatal("reset retained a binding or an open scope")
	}
	if err := env.endScope(); !errors.Is(err, errSMT2LocalScope) {
		t.Fatal("reset retained an endable scope", err)
	}
	env.beginScope()
	env.insert(y, smt2Local{level: 4})
	if err := env.endScope(); err != nil || env.contains(y) {
		t.Fatal("fresh scope after reset", err)
	}
}
