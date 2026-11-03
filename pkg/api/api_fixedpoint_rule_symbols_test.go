package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func apiFixedpointRuleNameSymbols(t *testing.T, manager *ast.ASTManager) []symbol.Symbol {
	t.Helper()
	empty, err := manager.Symbols.Intern("")
	astTestMust(t, err)
	spelling, err := manager.Symbols.Intern("k!7")
	astTestMust(t, err)
	names := []symbol.Symbol{{}, empty, symbol.Numerical(7), spelling}
	for i, name := range names {
		for _, previous := range names[:i] {
			if name == previous {
				t.Fatal("fixture collapsed source symbol kinds")
			}
		}
	}
	return names
}

func checkAPIFixedpointRuleNameFormulas(t *testing.T, manager *ast.ASTManager, command *cmd_context.Context, expected map[symbol.Symbol]ast.ASTHandle) {
	t.Helper()
	formulas, names, err := command.FixedpointNamedRulesWithSymbols()
	astTestMust(t, err)
	defer releaseAPIRefs(t, manager, formulas)
	if len(formulas) != len(expected) || len(names) != len(expected) {
		t.Fatalf("typed rule enumeration has %d formulas and %d names, want %d", len(formulas), len(names), len(expected))
	}
	seen := make(map[symbol.Symbol]bool)
	for i, name := range names {
		want, present := expected[name]
		if !present || seen[name] {
			t.Fatal("unexpected or repeated rule name", name)
		}
		seen[name] = true
		formula, err := manager.RefGet(formulas[i])
		astTestMust(t, err)
		if formula != want {
			t.Fatal("rule update matched the wrong symbol", name, formula, want)
		}
	}
}

func TestAPIFixedpointTypedRuleNamesAddFlushAndUpdateIndependently(t *testing.T) {
	for _, object := range []bool{true, false} {
		label := "context"
		if object {
			label = "object"
		}
		t.Run(label, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			command := f.fp.command
			p, q, u := f.pAtom, f.qAtom, f.uAtom
			declaration := f.p
			if !object {
				command = f.c.command
				declaration = f.pin(f.c.DeclareRelation("typed-context-p", nil))
				qDecl := f.pin(f.c.DeclareRelation("typed-context-q", nil))
				uDecl := f.pin(f.c.DeclareRelation("typed-context-u", nil))
				p = f.pin(f.m.MkApp(declaration, nil))
				q = f.pin(f.m.MkApp(qDecl, nil))
				u = f.pin(f.m.MkApp(uDecl, nil))
			}
			names := apiFixedpointRuleNameSymbols(t, f.m)
			expected := make(map[symbol.Symbol]ast.ASTHandle)
			for i, name := range names {
				if object {
					if i%2 == 0 {
						astTestMust(t, f.fp.AddRuleWithSymbol(p, name))
					} else {
						astTestMust(t, f.fp.AddRuleWithSymbolAndBound(p, name, uint32(i+1)))
					}
				} else if i%2 == 0 {
					astTestMust(t, f.c.AddFixedpointRuleWithSymbol(p, name))
				} else {
					astTestMust(t, f.c.AddFixedpointRuleWithSymbolAndBound(p, name, uint32(i+1)))
				}
				expected[name] = p
			}
			checkAPIFixedpointRuleNameFormulas(t, f.m, command, expected)
			var status lbool.LBool
			var err error
			if object {
				status, err = f.fp.Query(declaration)
			} else {
				status, err = f.c.QueryFixedpoint(declaration)
			}
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("named source facts were not queried", status)
			}
			var third, fourth ast.ASTHandle
			if object {
				third = f.pin(f.fp.DeclareRelation("typed-object-third", nil))
				fourth = f.pin(f.fp.DeclareRelation("typed-object-fourth", nil))
			} else {
				third = f.pin(f.c.DeclareRelation("typed-context-third", nil))
				fourth = f.pin(f.c.DeclareRelation("typed-context-fourth", nil))
			}
			thirdBody := f.pin(f.m.MkApp(third, nil))
			fourthBody := f.pin(f.m.MkApp(fourth, nil))
			bodies := []ast.ASTHandle{q, u, thirdBody, fourthBody}
			for i, name := range names {
				replacement := f.pin(f.m.MkImplies(bodies[i], p))
				if object {
					astTestMust(t, f.fp.UpdateRuleWithSymbol(replacement, name))
				} else {
					astTestMust(t, f.c.UpdateFixedpointRuleWithSymbol(replacement, name))
				}
				expected[name] = replacement
				checkAPIFixedpointRuleNameFormulas(t, f.m, command, expected)
			}
		})
	}
}

func TestAPIFixedpointTypedRuleNamesRejectInvalidOwners(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	other := newSMTAPIFixture(t, "HORN", nil)
	foreign, err := other.m.Symbols.Intern("foreign-rule")
	astTestMust(t, err)
	local, err := f.m.Symbols.Intern("local-rule")
	astTestMust(t, err)
	marked, err := local.Mark()
	astTestMust(t, err)
	astTestMust(t, f.fp.AddRuleWithSymbol(f.pAtom, local))
	expected := map[symbol.Symbol]ast.ASTHandle{local: f.pAtom}
	for _, name := range []symbol.Symbol{foreign, marked} {
		for _, operation := range []func() error{
			func() error { return f.fp.AddRuleWithSymbol(f.pAtom, name) },
			func() error { return f.fp.AddRuleWithSymbolAndBound(f.pAtom, name, 3) },
			func() error { return f.fp.UpdateRuleWithSymbol(f.pAtom, name) },
		} {
			if err := operation(); err == nil {
				t.Fatal("invalid rule symbol accepted")
			}
			checkAPIFixedpointRuleNameFormulas(t, f.m, f.fp.command, expected)
		}
	}
	astTestMust(t, f.fp.Close())
	for _, operation := range []func() error{
		func() error { return f.fp.AddRuleWithSymbol(f.pAtom, local) },
		func() error { return f.fp.AddRuleWithSymbolAndBound(f.pAtom, local, 3) },
		func() error { return f.fp.UpdateRuleWithSymbol(f.pAtom, local) },
	} {
		if err := operation(); err == nil {
			t.Fatal("closed fixedpoint accepted rule symbol")
		}
	}
}

func TestAPIFixedpointLoadPreservesRuleNameSymbols(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	queries, err := f.fp.LoadSMT2WithQueries(strings.NewReader(`
(declare-rel loaded-rule-symbol ())
(rule loaded-rule-symbol)
(rule loaded-rule-symbol ||)
(rule loaded-rule-symbol |k!7|)
(query loaded-rule-symbol)
`))
	astTestMust(t, err)
	defer func() { astTestMust(t, queries.Close()) }()
	values, err := queries.Values()
	astTestMust(t, err)
	if len(values) != 1 {
		t.Fatal("loaded query count", len(values))
	}
	names := apiFixedpointRuleNameSymbols(t, f.m)
	expected := map[symbol.Symbol]ast.ASTHandle{names[0]: values[0], names[1]: values[0], names[3]: values[0]}
	checkAPIFixedpointRuleNameFormulas(t, f.m, f.fp.command, expected)
	declaration, err := f.m.GetDecl(values[0])
	astTestMust(t, err)
	status, err := f.fp.Query(declaration)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("typed loaded rule query", status)
	}
	checkAPIFixedpointRuleNameFormulas(t, f.m, f.fp.command, expected)
}
