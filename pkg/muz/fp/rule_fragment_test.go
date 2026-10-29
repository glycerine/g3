package fp

// Source: src/muz/base/dl_context.cpp check_rules and
// src/muz/base/rule_properties.cpp fragment diagnostics.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func fragmentMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFragmentTestEngine(t *testing.T, backend string) (*PropositionalEngine, *ast.ASTManager) {
	t.Helper()
	budget := arena.NewBudget(1 << 24)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	fragmentMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	fragmentMust(t, err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	fragmentMust(t, err)
	fragmentMust(t, manager.RegisterDeclarationPlugins())
	engine, err := NewPropositionalEngine(manager)
	fragmentMust(t, err)
	engine.backend = backend
	t.Cleanup(func() {
		fragmentMust(t, engine.Close())
		fragmentMust(t, manager.Close())
		fragmentMust(t, numerals.Close())
		fragmentMust(t, symbols.Close())
		if used := budget.Used(); used != 0 {
			t.Errorf("fragment fixture retained %d bytes", used)
		}
	})
	return engine, manager
}

func TestFixedpointRulePropertyValidationWiring(t *testing.T) {
	for _, test := range []struct {
		name    string
		backend string
		kind    base.RulePropertyKind
		body    func(*testing.T, *ast.ASTManager, ast.ASTHandle) ast.ASTHandle
	}{
		{
			name:    "datalog-nested-predicate",
			backend: "datalog",
			kind:    base.RulePropertyNested,
			body: func(t *testing.T, manager *ast.ASTManager, predicate ast.ASTHandle) ast.ASTHandle {
				falsity, err := manager.MkFalse()
				fragmentMust(t, err)
				result, err := manager.MkOr(predicate, falsity)
				fragmentMust(t, err)
				return result
			},
		},
		{
			name:    "pdr-existential-tail",
			backend: "pdr",
			kind:    base.RulePropertyExistentialTail,
			body: func(t *testing.T, manager *ast.ASTManager, predicate ast.ASTHandle) ast.ASTHandle {
				truth, err := manager.MkTrue()
				fragmentMust(t, err)
				result, err := manager.MkImplies(predicate, truth)
				fragmentMust(t, err)
				return result
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, manager := newFragmentTestEngine(t, test.backend)
			boolean, err := manager.MkBoolSort()
			fragmentMust(t, err)
			applications := make([]ast.ASTHandle, 2)
			for i, suffix := range []string{"head", "body"} {
				name, x := manager.Symbols.Intern("fp-property-" + test.backend + "-" + suffix)
				fragmentMust(t, x)
				declaration, x := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
				fragmentMust(t, x)
				applications[i], x = manager.MkApp(declaration, nil)
				fragmentMust(t, x)
				fragmentMust(t, engine.RegisterPredicate(declaration))
			}
			body := test.body(t, manager, applications[1])
			rule, err := engine.rules.NewRule(applications[0], []ast.ASTHandle{body}, nil, test.name)
			fragmentMust(t, err)
			fragmentMust(t, engine.set.Add(rule))
			fragmentMust(t, rule.Close())
			err = engine.validateFragment()
			var property *base.RulePropertyError
			if !errors.Is(err, ErrPropositionalHorn) || !errors.As(err, &property) || property.Kind != test.kind {
				t.Fatalf("fragment error = %v, property = %#v", err, property)
			}
			if !strings.Contains(err.Error(), "muz/fp:") || !strings.Contains(err.Error(), "muz/base:") {
				t.Fatal("fragment error lost ownership prefixes", err)
			}
		})
	}
}
