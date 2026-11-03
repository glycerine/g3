package bmc

// Source: dl_bmc_engine.cpp qlinear::get_model. A failed trace owns its
// evaluator, indexed terms and rule references until each destructor finishes.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifiedLinearTraceCleanupPreservesOwnersAcrossNativeRefusal(t *testing.T) {
	for _, stage := range []string{"nested", "group", "evaluator", "terms", "selected", "steps"} {
		t.Run(stage, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var owner *quantifiedLinearTraceOwner
			var manager *ast.ASTManager
			var childRefs []ast.ASTRef
			var failure error
			var published []quantifiedLinearStep
			active, callbacks := false, 0
			var operationCause error
			if stage == "nested" || stage == "selected" || stage == "steps" {
				operationCause = errors.New("quantified trace failed before publication")
			}
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
				if active && failure == nil {
					callbacks++
					cause := operationCause
					if stage == "nested" {
						cause = disposeASTRefs(manager, childRefs, cause)
					}
					published, failure = finishQuantifiedLinearTrace(owner, cause)
				}
				return source
			}})
			must(err)
			numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
			must(err)
			symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
			must(err)
			manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
			must(err)
			defer func() {
				must(manager.Close())
				must(numbers.Close())
				must(symbols.Close())
				must(runtime.Close())
				if budget.Used() != 0 {
					t.Fatal("quantified trace cleanup leaked bytes", budget.Used())
				}
			}()
			_, err = manager.RegisterArithmeticPlugin()
			must(err)
			owner = &quantifiedLinearTraceOwner{manager: manager}
			rules, err := base.NewRuleManager(manager)
			must(err)
			defer func() { must(rules.Close()) }()
			boolean, err := manager.MkBoolSort()
			must(err)
			var ownedRules []*base.Rule
			newRule := func(name string) *base.Rule {
				t.Helper()
				sym, err := symbols.Intern(name)
				must(err)
				declaration, err := manager.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(rules.RegisterPredicate(declaration))
				head, err := manager.MkApp(declaration, nil)
				must(err)
				rule, err := rules.NewRule(head, nil, nil, name)
				must(err)
				// Do not add the rule to a set or copy its owner: Close must
				// reach final destruction and encounter the native refusal.
				ownedRules = append(ownedRules, rule)
				return rule
			}
			owner.steps = []quantifiedLinearStep{
				{rule: newRule("trace-cleanup-earlier"), index: 2, level: 3},
				{rule: newRule("trace-cleanup-later"), index: 0, level: 2},
			}
			if stage != "steps" {
				owner.selected = newRule("trace-cleanup-selected")
			}
			if stage == "nested" || stage == "group" {
				owner.group = []*base.Rule{newRule("trace-cleanup-group-first"), newRule("trace-cleanup-group-last")}
			}
			if stage != "selected" && stage != "steps" {
				owner.terms, err = newQuantifiedLinear(manager, 4)
				must(err)
			}
			if stage == "nested" || stage == "group" || stage == "evaluator" {
				params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
				must(err)
				defer func() { must(params.Close()) }()
				catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
				must(err)
				defer func() { must(catalog.Close()) }()
				globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
				must(err)
				defer func() { must(globals.Close()) }()
				must(globals.InitializeSource())
				cooperation, err := cooperate.NewCooperation(budget)
				must(err)
				defer func() { must(cooperation.Close()) }()
				must(manager.RegisterDeclarationPlugins())
				md, err := model.NewModelCore(manager)
				must(err)
				defer func() { must(md.Close()) }()
				owner.evaluator, err = model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: params, Globals: globals, Memory: runtime, Cooperation: cooperation})
				must(err)
				name, err := symbols.Intern("trace-cleanup-evaluation")
				must(err)
				term, err := owner.terms.hold(manager.MkConst(name, boolean))
				must(err)
				decl, err := manager.GetDecl(term)
				must(err)
				truth, err := manager.MkTrue()
				must(err)
				must(md.RegisterConstant(decl, truth))
				term, err = owner.terms.hold(manager.MkNot(term))
				must(err)
				value, err := owner.evaluator.EvaluateValue(term)
				must(err)
				must(manager.DelRef(&value))
			}
			if stage == "nested" {
				truth, err := manager.MkTrue()
				must(err)
				child, err := manager.NewRef(truth)
				must(err)
				childRefs = []ast.ASTRef{child}
			}
			beforeEvaluator, beforeTerms, beforeSelected := owner.evaluator, owner.terms, owner.selected
			beforeGroup := append([]*base.Rule(nil), owner.group...)
			beforeSteps := append([]quantifiedLinearStep(nil), owner.steps...)
			var beforeRefs []ast.ASTRef
			if owner.terms != nil {
				beforeRefs = append(beforeRefs, owner.terms.refs...)
			}
			var beforeChild ast.ASTRef
			if len(childRefs) != 0 {
				beforeChild = childRefs[0]
			}
			value, err := numbers.NewValue()
			must(err)
			defer func() { must(numbers.Del(&value)) }()
			must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
			must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
			var invocation ast.ASTArithmeticNumeralInvocation
			active = true
			_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
			active = false
			must(runtime.SetMaxCount(0))
			must(manager.CleanupArithmeticNumeralInvocation(&invocation))
			if body == nil || callbacks == 0 || !errors.Is(failure, ast.ErrASTPending) || published != nil {
				t.Fatal("native refusal did not suppress trace publication", body, callbacks, failure, published)
			}
			if operationCause != nil && !errors.Is(failure, operationCause) {
				t.Fatal("cleanup lost original trace failure", failure)
			}
			if owner.evaluator != beforeEvaluator || owner.terms != beforeTerms || owner.selected != beforeSelected || len(owner.group) != len(beforeGroup) || len(owner.steps) != len(beforeSteps) {
				t.Fatal("refusal advanced a dependent cleanup owner")
			}
			for i := range beforeGroup {
				if owner.group[i] != beforeGroup[i] {
					t.Fatal("refusal consumed a group owner", i)
				}
			}
			for i := range beforeSteps {
				if owner.steps[i] != beforeSteps[i] {
					t.Fatal("refusal consumed a trace step", i)
				}
			}
			for i := range beforeRefs {
				if owner.terms.refs[i] != beforeRefs[i] {
					t.Fatal("refusal consumed an indexed term", i)
				}
			}
			if stage == "nested" && (owner.nested == nil || childRefs[0] != beforeChild) {
				t.Fatal("nested refusal lost its child root")
			}
			for _, rule := range ownedRules {
				_, err := rule.Head()
				must(err)
			}
			var cleanup memory.ConstructionFailure
			if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
				t.Fatal("missing concrete trace cleanup receipt", failure)
			}
			original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
			must(cleanup.Cleanup())
			must(cleanup.Cleanup())
			if owner.nested != nil || owner.evaluator != nil || owner.terms != nil || owner.group != nil || owner.selected != nil || owner.steps != nil {
				t.Fatal("cleanup retry left a trace owner")
			}
			if stage == "nested" && childRefs[0] != (ast.ASTRef{}) {
				t.Fatal("cleanup retry left the nested root")
			}
			for _, rule := range ownedRules {
				if _, err := rule.Head(); err == nil {
					t.Fatal("cleanup retry left a rule open")
				}
			}
			if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message || cleanup.CleanupCause() != nil {
				t.Fatal("cleanup retry changed stable trace diagnostics")
			}
		})
	}
}
