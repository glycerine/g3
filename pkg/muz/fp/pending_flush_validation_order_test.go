package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// dl_context::flush_add_rules publishes all clauses and checks their
// properties before ensure_engine, including when no formulas remain pending.
func TestExpressionFlushValidatesBeforeEngineConstruction(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			f := newFPRuleNameFixture(t)
			fragmentMust(t, f.e.SelectEngine(backend))
			fragmentMust(t, f.e.Push())
			_, head := f.predicate("validation-order-head")
			boolean := f.own(f.m.MkBoolSort())
			name, err := f.m.Symbols.Intern("validation-order-opaque")
			fragmentMust(t, err)
			decl := f.own(f.m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{}))
			body := f.own(f.m.MkApp(decl, nil))
			formula := f.own(f.m.MkImplies(body, head))
			fragmentMust(t, f.e.addRuleEager(formula, "validation-order"))
			query := f.own(f.m.MkFalse())
			for attempt := 0; attempt < 2; attempt++ {
				status, cause := f.e.QueryExpression(query)
				var property *base.RulePropertyError
				if status != lbool.Undef || !errors.As(cause, &property) || property.Kind != base.RulePropertyUninterpreted {
					t.Fatalf("attempt %d: status %v, error %v", attempt, status, cause)
				}
				if len(f.e.rawRules) != 1 || f.e.rawRules[0].pending {
					t.Fatalf("attempt %d: rule not published before validation", attempt)
				}
				if f.e.engineInstantiated || f.e.pdr != nil || f.e.spacer != nil || f.e.queryClone != nil {
					t.Fatalf("attempt %d: validation constructed an engine or query clone", attempt)
				}
			}
			fragmentMust(t, f.e.Pop())
			status, err := f.e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatalf("query after restoring scope = %v", status)
			}
		})
	}
}

func TestPredicateQueriesValidatePublishedRules(t *testing.T) {
	for _, test := range []struct {
		backend string
		level   bool
	}{
		{backend: "pdr"}, {backend: "qpdr"}, {backend: "spacer"},
		{backend: "spacer", level: true},
	} {
		t.Run(test.backend+map[bool]string{false: "", true: "/from-level"}[test.level], func(t *testing.T) {
			f := newFPRuleNameFixture(t)
			fragmentMust(t, f.e.SelectEngine(test.backend))
			pred, head := f.predicate("predicate-validation-head")
			boolean := f.own(f.m.MkBoolSort())
			name, err := f.m.Symbols.Intern("predicate-validation-opaque")
			fragmentMust(t, err)
			decl := f.own(f.m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{}))
			body := f.own(f.m.MkApp(decl, nil))
			fragmentMust(t, f.e.addRuleEager(f.own(f.m.MkImplies(body, head)), "predicate-validation"))
			for attempt := 0; attempt < 2; attempt++ {
				var status lbool.LBool
				var cause error
				if test.level {
					status, cause = f.e.QueryPredicateFromLevel(pred, 0)
				} else {
					status, cause = f.e.QueryPredicate(pred)
				}
				var property *base.RulePropertyError
				if status != lbool.Undef || !errors.As(cause, &property) || property.Kind != base.RulePropertyUninterpreted {
					t.Fatalf("attempt %d: status %v, error %v", attempt, status, cause)
				}
				if len(f.e.rawRules) != 1 || f.e.rawRules[0].pending {
					t.Fatalf("attempt %d: validation ran before publication", attempt)
				}
				if test.level {
					if f.e.engineInstantiated || f.e.pdr != nil || f.e.spacer != nil {
						t.Fatalf("attempt %d: level query constructed an engine before validation", attempt)
					}
				} else if !f.e.engineInstantiated || (f.e.pdr == nil && f.e.spacer == nil) {
					t.Fatalf("attempt %d: relation query validated before constructing its engine", attempt)
				}
			}
		})
	}
}

func TestEmptyFlushChecksExistingRulesForNonDatalogEngines(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "spacer", "bmc", "qbmc", "duality", "tab", "clp", "ddnf"} {
		t.Run(backend, func(t *testing.T) {
			e, m := newFragmentTestEngine(t, backend)
			boolean, err := m.MkBoolSort()
			fragmentMust(t, err)
			var apps []ast.ASTHandle
			for _, suffix := range []string{"head", "tail"} {
				name, err := m.Symbols.Intern("empty-flush-" + backend + "-" + suffix)
				fragmentMust(t, err)
				decl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
				fragmentMust(t, err)
				fragmentMust(t, e.RegisterPredicate(decl))
				app, err := m.MkApp(decl, nil)
				fragmentMust(t, err)
				apps = append(apps, app)
			}
			rule, err := e.rules.NewRule(apps[0], []ast.ASTHandle{apps[1]}, []bool{true}, "negative")
			fragmentMust(t, err)
			fragmentMust(t, e.set.Add(rule))
			fragmentMust(t, rule.Close())
			for attempt := 0; attempt < 2; attempt++ {
				err = e.flushPendingRules()
				if backend == "ddnf" {
					fragmentMust(t, err)
					continue
				}
				var property *base.RulePropertyError
				if !errors.As(err, &property) || property.Kind != base.RulePropertyNegativePredicate {
					t.Fatalf("attempt %d: expected negation diagnostic, got %v", attempt, err)
				}
			}
		})
	}
}

func TestEmptyFlushChecksRulesWithRequestedProofMode(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := map[bool]string{false: "disabled", true: "enabled"}[enabled]
		t.Run(name, func(t *testing.T) {
			e, m := newFragmentTestEngine(t, "pdr")
			e.forceProofTrace = enabled
			boolean, err := m.MkBoolSort()
			fragmentMust(t, err)
			symbol, err := m.Symbols.Intern("empty-flush-proof-" + name)
			fragmentMust(t, err)
			decl, err := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			fragmentMust(t, e.RegisterPredicate(decl))
			head, err := m.MkApp(decl, nil)
			fragmentMust(t, err)
			rule, err := e.rules.NewRule(head, nil, nil, "fact")
			fragmentMust(t, err)
			defer func() { fragmentMust(t, rule.Close()) }()
			fragmentMust(t, e.set.Add(rule))
			before, err := m.ProofMode()
			fragmentMust(t, err)
			fragmentMust(t, e.flushPendingRules())
			after, err := m.ProofMode()
			fragmentMust(t, err)
			if before != after {
				t.Fatalf("proof mode changed across flush: %v -> %v", before, after)
			}
			_, found, err := rule.Proof()
			fragmentMust(t, err)
			if found != enabled {
				t.Fatalf("proof generated = %t, want %t", found, enabled)
			}
		})
	}
}
