package smt2_test

// Source: dl_rule_cmd defaults to symbol::null; a supplied CPK_SYMBOL token
// is a string symbol, including || and the string spelling k!7.

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func fixedpointSymbolFixture(t *testing.T) (*cmd_context.Context, *ast.ASTManager) {
	t.Helper()
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	})
	c.PrintSuccess = false
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	return c, m
}

func fixedpointParsedNameSymbols(t *testing.T, m *ast.ASTManager) []symbol.Symbol {
	t.Helper()
	names := []symbol.Symbol{{}}
	for _, text := range []string{"", "k!7", "k!7", "7"} {
		name, err := m.Symbols.Intern(text)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func releaseFixedpointSymbolRefs(t *testing.T, m *ast.ASTManager, refs []ast.ASTRef) {
	t.Helper()
	for i := range refs {
		if err := m.DelRef(&refs[i]); err != nil {
			t.Fatal(err)
		}
	}
}

const fixedpointSymbolRules = `(rule P) (rule P || 0) (rule P k!7 7) (rule P |k!7|) (rule P |7|)`

func TestFixedpointParserRuleSymbolsPreserveEmptyAndStringIdentity(t *testing.T) {
	c, m := fixedpointSymbolFixture(t)
	script := `(declare-rel P ()) ` + fixedpointSymbolRules + ` (fixedpoint-push) (rule P discarded) (fixedpoint-pop)`
	if err := smt2.ParseCommands(c, strings.NewReader(script), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	refs, names, err := c.FixedpointNamedRulesWithSymbols()
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFixedpointSymbolRefs(t, m, refs)
	if len(refs) != 5 || !reflect.DeepEqual(names, fixedpointParsedNameSymbols(t, m)) {
		t.Fatal("parsed rule label identities or scope order changed", len(refs), names)
	}
	if names[0] == names[1] || names[2] == symbol.Numerical(7) || names[4] == symbol.Numerical(7) {
		t.Fatal("parser collapsed absent/empty or invented a numerical symbol", names)
	}
	var display bytes.Buffer
	if err := c.WriteFixedpointSMT2(&display); err != nil {
		t.Fatal(err)
	}
	if strings.Count(display.String(), ":named ") != 4 || !strings.Contains(display.String(), "(rule (! P :named ))") {
		t.Fatal("context display omitted the explicit empty label", display.String())
	}
}

func TestFixedpointCollectorRuleSymbolsPreserveScopesAndLegacyNames(t *testing.T) {
	c, m := fixedpointSymbolFixture(t)
	script := `(declare-rel P ()) ` + fixedpointSymbolRules + ` (fixedpoint-push) (rule P discarded) (fixedpoint-pop) (query P)`
	program, err := smt2.ParseFixedpointProgram(c, strings.NewReader(script))
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFixedpointSymbolRefs(t, m, program.Rules)
	defer releaseFixedpointSymbolRefs(t, m, program.Queries)
	defer releaseFixedpointSymbolRefs(t, m, program.Relations)
	if len(program.Rules) != 5 || len(program.Queries) != 1 || !reflect.DeepEqual(program.NameSymbols, fixedpointParsedNameSymbols(t, m)) {
		t.Fatal("collector lost label identity or scoped alignment", len(program.Rules), len(program.Queries), program.NameSymbols)
	}
	if !reflect.DeepEqual(program.Names, []string{"", "", "k!7", "k!7", "7"}) {
		t.Fatal("collector changed legacy name text", program.Names)
	}
	refs, names, err := c.FixedpointNamedRulesWithSymbols()
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFixedpointSymbolRefs(t, m, refs)
	if len(refs) != 0 || len(names) != 0 {
		t.Fatal("collection ingested rules into the parser engine", len(refs), names)
	}
}

func TestFixedpointParserRuleNameRejectsNumeralToken(t *testing.T) {
	c, m := fixedpointSymbolFixture(t)
	err := smt2.ParseCommands(c, strings.NewReader(`(declare-rel P ()) (rule P 7)`), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "symbol expected") {
		t.Fatal("numeral accepted as a CPK_SYMBOL argument", err)
	}
	refs, names, err := c.FixedpointNamedRulesWithSymbols()
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFixedpointSymbolRefs(t, m, refs)
	if len(refs) != 0 || len(names) != 0 {
		t.Fatal("invalid rule name published a partial command", len(refs), names)
	}
}
