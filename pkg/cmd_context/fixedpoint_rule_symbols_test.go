package cmd_context

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFixedpointCommandRuleSymbolsUpdateAndArchive(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	var atoms []ast.ASTHandle
	for _, text := range []string{"symbol-P", "symbol-Q", "symbol-R"} {
		declaration, err := c.DeclareRelation(text, nil)
		contextMust(t, err)
		atom, err := m.MkApp(declaration, nil)
		contextMust(t, err)
		ref, err := m.NewRef(atom)
		contextMust(t, err)
		t.Cleanup(func() { contextMust(t, m.DelRef(&ref)) })
		atoms = append(atoms, atom)
	}
	empty, err := m.Symbols.Intern("")
	contextMust(t, err)
	spelling, err := m.Symbols.Intern("k!7")
	contextMust(t, err)
	names := []symbol.Symbol{{}, empty, symbol.Numerical(7), spelling}
	for _, name := range names {
		contextMust(t, c.UpdateFixedpointRuleWithSymbol(atoms[0], name))
	}
	stronger, err := m.MkImplies(atoms[1], atoms[0])
	contextMust(t, err)
	strongerRef, err := m.NewRef(stronger)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, m.DelRef(&strongerRef)) })
	contextMust(t, c.UpdateFixedpointRuleWithSymbol(stronger, symbol.Numerical(7)))
	emptyReplacement, err := m.MkImplies(atoms[2], atoms[0])
	contextMust(t, err)
	emptyRef, err := m.NewRef(emptyReplacement)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, m.DelRef(&emptyRef)) })
	contextMust(t, c.UpdateFixedpointRuleWithSymbol(emptyReplacement, empty))
	formulas, actualNames, err := c.FixedpointNamedRulesWithSymbols()
	contextMust(t, err)
	defer func() {
		for i := range formulas {
			contextMust(t, m.DelRef(&formulas[i]))
		}
	}()
	if !reflect.DeepEqual(actualNames, names) || len(formulas) != 4 {
		t.Fatal("typed update matched colliding label spellings", actualNames, len(formulas))
	}
	for i, want := range []ast.ASTHandle{atoms[0], emptyReplacement, stronger, atoms[0]} {
		got, err := m.RefGet(formulas[i])
		contextMust(t, err)
		if got != want {
			t.Fatal("typed update replaced the wrong source slot", i, got, want)
		}
	}
	legacy, texts, err := c.FixedpointNamedRules()
	contextMust(t, err)
	for i := range legacy {
		contextMust(t, m.DelRef(&legacy[i]))
	}
	if !reflect.DeepEqual(texts, []string{"", "", "k!7", "k!7"}) {
		t.Fatal("legacy name text changed", texts)
	}
	for i, name := range names {
		contextMust(t, c.AddFixedpointRuleWithSymbolAndBound(atoms[1], name, uint32(i)))
	}
	contextMust(t, c.AddFixedpointRuleWithSymbol(atoms[2], empty))
	contextMust(t, c.AddFixedpointRule(atoms[0], ""))
	engine, err := c.fixedpointContext()
	contextMust(t, err)
	checkArchive := func() {
		t.Helper()
		refs, actual, bounds, err := engine.RawRulesWithSymbols()
		contextMust(t, err)
		defer func() {
			for i := range refs {
				contextMust(t, m.DelRef(&refs[i]))
			}
		}()
		want := append(append([]symbol.Symbol(nil), names...), empty, symbol.Symbol{})
		if !reflect.DeepEqual(actual, want) || !reflect.DeepEqual(bounds, []uint32{0, 1, 2, 3, ^uint32(0), ^uint32(0)}) {
			t.Fatal("typed command ingestion changed labels or recursion bounds", actual, bounds)
		}
	}
	checkArchive()
	contextMust(t, c.PushFixedpoint())
	contextMust(t, c.AddFixedpointRuleWithSymbol(atoms[2], symbol.Numerical(7)))
	contextMust(t, c.PopFixedpoint())
	checkArchive()
}
