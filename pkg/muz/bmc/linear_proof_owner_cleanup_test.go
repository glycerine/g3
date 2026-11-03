package bmc

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestLinearProofOwnerRetriesNativeRefusalBeforeRestoringProofMode(t *testing.T) {
	for _, stage := range []string{"nested", "normalizer", "scope"} {
		t.Run(stage, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var manager *ast.ASTManager
			var owner *linearProofOwner
			var childRefs []ast.ASTRef
			var failure error
			var published ast.ASTRef
			active, callbacks := false, 0
			original := errors.New("proof step interrupted")
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
				if active && failure == nil {
					callbacks++
					var cause error
					if stage == "nested" {
						cause = disposeASTRefs(manager, childRefs, original)
					}
					published, failure = finishLinearProof(owner, cause)
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
					t.Fatal("proof owner fixture leaked bytes", budget.Used())
				}
			}()
			must(manager.RegisterDeclarationPlugins())
			mode, err := manager.ProofMode()
			must(err)
			if mode != ast.ASTProofDisabled {
				t.Fatal("fixture must initially disable proofs", mode)
			}
			owner = &linearProofOwner{manager: manager}
			owner.scope, err = ast.NewScopedProof(manager)
			must(err)
			truth, err := manager.MkTrue()
			must(err)
			proof, err := manager.MkProofAsserted(truth)
			must(err)
			owner.proof, err = manager.NewRef(proof)
			must(err)
			if stage != "scope" {
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
				rules, err := base.NewRuleManager(manager)
				must(err)
				defer func() { must(rules.Close()) }()
				boolean, err := manager.MkBoolSort()
				must(err)
				name, err := symbols.Intern("proof-owner-goal")
				must(err)
				declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(rules.RegisterPredicate(declaration))
				head, err := manager.MkApp(declaration, nil)
				must(err)
				rule, err := rules.NewRule(head, nil, nil, "proof-owner-input")
				must(err)
				defer func() { must(rule.Close()) }()
				owner.normalizer = transforms.NewRuleInlinerPluginWithServices(manager, rules, nil, true, true, params, globals, 0)
				// Run real post-resolution normalization so Close must retire
				// its simplifier and QE helpers, not an empty plugin shell.
				normalized, err := owner.normalizer.NormalizeResolvedRule(rule)
				must(err)
				if normalized == nil {
					t.Fatal("normalization discarded the consistent fact")
				}
				if stage == "nested" {
					owner.previous = normalized
					step, err := rule.Retain()
					must(err)
					owner.steps = []quantifiedLinearStep{{rule: step, index: 1, level: 2}}
					child, err := manager.NewRef(truth)
					must(err)
					childRefs = []ast.ASTRef{child}
				} else {
					must(normalized.Close())
				}
			}
			beforeProof, beforeScope, beforeNormalizer := owner.proof, owner.scope, owner.normalizer
			beforePrevious := owner.previous
			beforeSteps := append([]quantifiedLinearStep(nil), owner.steps...)
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
			if body == nil || callbacks != 1 || !errors.Is(failure, ast.ErrASTPending) || published != (ast.ASTRef{}) {
				t.Fatal("native refusal did not suppress proof publication", body, callbacks, failure, published)
			}
			if owner.proof != beforeProof || owner.scope != beforeScope || owner.normalizer != beforeNormalizer || owner.previous != beforePrevious || len(owner.steps) != len(beforeSteps) {
				t.Fatal("refusal advanced a dependent proof owner")
			}
			for i := range beforeSteps {
				if owner.steps[i] != beforeSteps[i] {
					t.Fatal("refusal consumed a trace rule", i)
				}
			}
			if stage == "nested" && (owner.nested == nil || childRefs[0] != beforeChild || !errors.Is(failure, original)) {
				t.Fatal("proof cleanup lost its child owner or cause", failure)
			}
			mode, err = manager.ProofMode()
			must(err)
			if mode != ast.ASTProofEnabled {
				t.Fatal("proof mode restored before pending cleanup completed", mode)
			}
			var cleanup memory.ConstructionFailure
			if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
				t.Fatal("missing concrete proof cleanup receipt", failure)
			}
			cause, reported, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
			must(cleanup.Cleanup())
			must(cleanup.Cleanup())
			if owner.nested != nil || owner.previous != nil || owner.steps != nil || owner.normalizer != nil || owner.scope != nil || owner.proof != (ast.ASTRef{}) {
				t.Fatal("cleanup retry left a proof owner")
			}
			if stage == "nested" && childRefs[0] != (ast.ASTRef{}) {
				t.Fatal("cleanup retry left its child root")
			}
			mode, err = manager.ProofMode()
			must(err)
			if mode != ast.ASTProofDisabled {
				t.Fatal("cleanup did not restore original proof mode", mode)
			}
			if cleanup.OriginalCause() != cause || errors.Unwrap(cleanup) != reported || cleanup.Error() != message || cleanup.CleanupCause() != nil {
				t.Fatal("cleanup changed stable failure diagnostics")
			}
		})
	}
}
