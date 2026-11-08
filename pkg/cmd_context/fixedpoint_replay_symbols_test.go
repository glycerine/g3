package cmd_context_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func replaySymbolMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func replaySymbolContext(t *testing.T) (*cmd_context.Context, *ast.ASTManager) {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	replaySymbolMust(t, err)
	t.Cleanup(func() { replaySymbolMust(t, context.Close()) })
	manager, err := context.Manager()
	replaySymbolMust(t, err)
	return context, manager
}

func replaySymbolNames(t *testing.T, c *cmd_context.Context, m *ast.ASTManager) []symbol.Symbol {
	t.Helper()
	rules, names, err := c.FixedpointNamedRulesWithSymbols()
	replaySymbolMust(t, err)
	for i := range rules {
		replaySymbolMust(t, m.DelRef(&rules[i]))
	}
	return names
}

type replaySymbolsRefusingWriter struct{}

func (replaySymbolsRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestFixedpointReplayPreservesPresentTextualRuleSymbols(t *testing.T) {
	context, manager := replaySymbolContext(t)
	empty, err := manager.Symbols.Intern("")
	replaySymbolMust(t, err)
	quoted, err := manager.Symbols.Intern("label with space")
	replaySymbolMust(t, err)
	spelling, err := manager.Symbols.Intern("k!7")
	replaySymbolMust(t, err)
	labels := []symbol.Symbol{{}, empty, quoted, spelling}
	for i, predicate := range []string{"replay-null", "replay-empty", "replay-quoted", "replay-spelling"} {
		decl, err := context.DeclareRelation(predicate, nil)
		replaySymbolMust(t, err)
		formula, err := manager.MkApp(decl, nil)
		replaySymbolMust(t, err)
		replaySymbolMust(t, context.AddFixedpointRuleWithSymbol(formula, labels[i]))
	}
	// AddRule only queues (dl_context::add_rule); the first reader builds and
	// keeps the Go eager rule view, so prime it before counting references.
	var primed bytes.Buffer
	replaySymbolMust(t, context.WriteFixedpointReplaySMT2(&primed))
	before := manager.Stats().LiveReferences
	var out bytes.Buffer
	replaySymbolMust(t, context.WriteFixedpointReplaySMT2(&out))
	for _, fragment := range []string{"(rule replay-null)", "(rule replay-empty ||)", "(rule replay-quoted |label with space|)", "(rule replay-spelling k!7)"} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("missing replay label %q in %s", fragment, out.String())
		}
	}
	if strings.Contains(out.String(), ":named") {
		t.Fatal("display annotation used as replay rule label", out.String())
	}
	if err = context.WriteFixedpointReplaySMT2(replaySymbolsRefusingWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("writer refusal lost", err)
	}
	if manager.Stats().LiveReferences != before {
		t.Fatal("replay left owning references")
	}
	original := replaySymbolNames(t, context, manager)
	for i := range labels {
		if original[i] != labels[i] {
			t.Fatal("replay changed source symbol", i)
		}
	}

	loaded, loadedManager := replaySymbolContext(t)
	replaySymbolMust(t, smt2.ParseCommands(loaded, strings.NewReader(out.String()), io.Discard))
	names := replaySymbolNames(t, loaded, loadedManager)
	if len(names) != len(labels) {
		t.Fatal("replayed rule count", len(names))
	}
	for i, name := range names {
		wantText, err := manager.Symbols.String(labels[i])
		replaySymbolMust(t, err)
		text, err := loadedManager.Symbols.String(name)
		replaySymbolMust(t, err)
		if text != wantText || name.IsNull() != labels[i].IsNull() || name.IsNumerical() {
			t.Fatalf("rule %d label text=%q null=%t numeric=%t", i, text, name.IsNull(), name.IsNumerical())
		}
		if !name.IsNull() {
			interned, err := loadedManager.Symbols.Intern(wantText)
			replaySymbolMust(t, err)
			if name != interned {
				t.Fatal("replay changed string symbol identity", i)
			}
		}
	}
}

func TestFixedpointReplayNumericalRenderingDoesNotMutateIdentity(t *testing.T) {
	context, manager := replaySymbolContext(t)
	textual, err := manager.Symbols.Intern("k!7")
	replaySymbolMust(t, err)
	labels := []symbol.Symbol{symbol.Numerical(7), textual}
	for i, name := range []string{"replay-numeric", "replay-textual"} {
		decl, err := context.DeclareRelation(name, nil)
		replaySymbolMust(t, err)
		formula, err := manager.MkApp(decl, nil)
		replaySymbolMust(t, err)
		replaySymbolMust(t, context.AddFixedpointRuleWithSymbol(formula, labels[i]))
	}
	var out bytes.Buffer
	replaySymbolMust(t, context.WriteFixedpointReplaySMT2(&out))
	// Source SMT-LIB identifiers render both as k!7; only API replay tokens
	// encode numerical symbol identity. Rendering must leave both owners intact.
	if !strings.Contains(out.String(), "(rule replay-numeric k!7)") || !strings.Contains(out.String(), "(rule replay-textual k!7)") {
		t.Fatal("source numerical rendering", out.String())
	}
	names := replaySymbolNames(t, context, manager)
	if len(names) != 2 || names[0] != labels[0] || names[1] != labels[1] || names[0] == names[1] {
		t.Fatal("rendering collapsed numeric and textual identity", names)
	}
}
