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
	engine.engineSelectionSet = true // Fixture explicitly selects the fragment, without solver services.
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

func TestFixedpointRulePropertiesFollowEngineLifetime(t *testing.T) {
	engine, manager := newFragmentTestEngine(t, "datalog")
	properties := engine.ruleProperties
	if properties == nil {
		t.Fatal("context did not construct its rule-property owner")
	}
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	name, err := manager.Symbols.Intern("persistent-rule-property")
	fragmentMust(t, err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(predicate))
	fact, err := manager.MkApp(predicate, nil)
	fragmentMust(t, err)
	rule, err := engine.rules.NewRule(fact, nil, nil, "persistent-rule-property")
	fragmentMust(t, err)
	fragmentMust(t, engine.set.Add(rule))
	fragmentMust(t, rule.Close())

	for pass := 0; pass < 2; pass++ {
		if err = engine.checkDatalogEarlyRuleProperties(); err != nil {
			t.Fatal("source rule preflight", pass, err)
		}
		if err = engine.checkSourceRules(); err != nil {
			t.Fatal("source context validation", pass, err)
		}
		if engine.ruleProperties != properties {
			t.Fatal("query replaced context-owned rule properties")
		}
		if err = properties.SetGenerateProof(false); err != nil {
			t.Fatal("query destroyed context-owned rule properties", pass, err)
		}
	}
	if err = engine.Close(); err != nil {
		t.Fatal("context close", err)
	}
	if err = properties.SetGenerateProof(false); err == nil {
		t.Fatal("context close left rule-property owner open")
	}
}

func TestQPDRAllowsExistentialRuleTail(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "qbmc"} {
		t.Run(backend, func(t *testing.T) {
			engine, manager := newFragmentTestEngine(t, backend)
			boolean, err := manager.MkBoolSort()
			fragmentMust(t, err)
			var applications []ast.ASTHandle
			for _, suffix := range []string{"head", "tail"} {
				name, cause := manager.Symbols.Intern("qpdr-fragment-" + backend + "-" + suffix)
				fragmentMust(t, cause)
				decl, cause := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
				fragmentMust(t, cause)
				fragmentMust(t, engine.RegisterPredicate(decl))
				app, cause := manager.MkApp(decl, nil)
				fragmentMust(t, cause)
				applications = append(applications, app)
			}
			truth, err := manager.MkTrue()
			fragmentMust(t, err)
			body, err := manager.MkImplies(applications[1], truth)
			fragmentMust(t, err)
			rule, err := engine.rules.NewRule(applications[0], []ast.ASTHandle{body}, nil, "existential-tail")
			fragmentMust(t, err)
			fragmentMust(t, engine.set.Add(rule))
			fragmentMust(t, rule.Close())
			err = engine.validateFragment()
			if backend == "qpdr" {
				fragmentMust(t, err)
			} else {
				var property *base.RulePropertyError
				if !errors.As(err, &property) || property.Kind != base.RulePropertyExistentialTail {
					t.Fatal("engine accepted an existential tail", backend, err)
				}
			}
		})
	}
}

func TestFixedpointRuleValidationGeneratesRequestedProof(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			engine, manager := newFragmentTestEngine(t, "pdr")
			engine.forceProofTrace = enabled
			boolean, err := manager.MkBoolSort()
			fragmentMust(t, err)
			symbol, err := manager.Symbols.Intern("fragment-proof-" + name)
			fragmentMust(t, err)
			declaration, err := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			fragmentMust(t, engine.RegisterPredicate(declaration))
			head, err := manager.MkApp(declaration, nil)
			fragmentMust(t, err)
			rule, err := engine.rules.NewRule(head, nil, nil, "fact")
			fragmentMust(t, err)
			defer func() { fragmentMust(t, rule.Close()) }()
			fragmentMust(t, engine.set.Add(rule))
			fragmentMust(t, engine.validateFragment())
			proof, found, err := rule.Proof()
			fragmentMust(t, err)
			if found != enabled {
				t.Fatal("proof tracing did not control rule proof creation", found, enabled)
			}
			if found {
				fact, e := manager.GetProofFact(proof)
				fragmentMust(t, e)
				if fact != head {
					t.Fatal("generated proof has wrong fact", fact)
				}
			}
		})
	}
}

func TestFixedpointRuleValidationProofParameterPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, global string
		local        *bool
		want         bool
	}{
		{"default", "false", nil, false},
		{"global", "true", nil, true},
		{"local-enable", "false", magicQueryBool(true), true},
		{"local-disable", "true", magicQueryBool(false), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, globals := newPDRLifecycleServices(t)
			fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", tc.global))
			engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
			fragmentMust(t, err)
			t.Cleanup(func() { fragmentMust(t, engine.Close()) })
			fragmentMust(t, engine.SelectEngine("qbmc"))
			if tc.local != nil {
				params := globals.Context.EmptyValue()
				key := "generate_proof_trace"
				fragmentMust(t, globals.Context.SetBoolName(&params, &key, *tc.local))
				fragmentMust(t, engine.SetQueryParameters(&params))
				fragmentMust(t, globals.Context.Del(&params))
			}
			boolean, err := manager.MkBoolSort()
			fragmentMust(t, err)
			name, err := manager.Symbols.Intern("proof-param-" + tc.name)
			fragmentMust(t, err)
			declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			fragmentMust(t, engine.RegisterPredicate(declaration))
			head, err := manager.MkApp(declaration, nil)
			fragmentMust(t, err)
			fragmentMust(t, engine.addRuleEager(head, "fact"))
			fragmentMust(t, engine.validateFragment())
			rule, err := engine.set.Rule(0)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, rule.Close()) }()
			_, found, err := rule.Proof()
			fragmentMust(t, err)
			if found != tc.want {
				t.Fatal("unexpected rule proof state", found, tc.want)
			}
		})
	}
}
