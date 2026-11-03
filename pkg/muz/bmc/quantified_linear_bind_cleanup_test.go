package bmc

// Source: dl_bmc_engine.cpp qlinear::eval_q and get_model. Substitution,
// evaluator and indexed-term owners outlive an interrupted binding extraction.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifiedLinearBindingCleanupPreservesOwnersAcrossNativeRefusal(t *testing.T) {
	for _, stage := range []string{"nested", "substitution", "evaluator", "terms", "outputs"} {
		t.Run(stage, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var manager *ast.ASTManager
			var owner *quantifiedLinearBindingOwner
			var childRefs []ast.ASTRef
			var failure error
			var published []ast.ASTRef
			active, callbacks := false, 0
			var operationCause error
			if stage == "nested" || stage == "outputs" {
				operationCause = errors.New("binding extraction failed before publication")
			}
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
				if active && failure == nil {
					callbacks++
					cause := operationCause
					if stage == "nested" {
						cause = disposeASTRefs(manager, childRefs, cause)
					}
					published, failure = finishQuantifiedLinearBindings(owner, cause)
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
					t.Fatal("quantified binding cleanup leaked bytes", budget.Used())
				}
			}()
			_, err = manager.RegisterArithmeticPlugin()
			must(err)
			owner = &quantifiedLinearBindingOwner{manager: manager}
			truth, err := manager.MkTrue()
			must(err)
			for i := 0; i < 2; i++ {
				ref, err := manager.NewRef(truth)
				must(err)
				owner.outputs = append(owner.outputs, ref)
			}
			if stage != "outputs" {
				owner.terms, err = newQuantifiedLinear(manager, 4)
				must(err)
			}
			if stage == "nested" || stage == "substitution" || stage == "evaluator" {
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
				boolean, err := manager.MkBoolSort()
				must(err)
				name, err := symbols.Intern("binding-cleanup-evaluation")
				must(err)
				term, err := owner.terms.hold(manager.MkConst(name, boolean))
				must(err)
				declaration, err := manager.GetDecl(term)
				must(err)
				must(md.RegisterConstant(declaration, truth))
				term, err = owner.terms.hold(manager.MkNot(term))
				must(err)
				value, err := owner.evaluator.EvaluateValue(term)
				must(err)
				must(manager.DelRef(&value))
				if stage != "evaluator" {
					owner.substitution, err = rewriter.NewVarSubstitution(manager, false)
					must(err)
					variable, err := owner.terms.hold(manager.MkVar(0, boolean))
					must(err)
					output, err := manager.NewRef(ast.ASTHandle{})
					must(err)
					owner.terms.refs = append(owner.terms.refs, output)
					must(owner.substitution.Substitute(variable, []ast.ASTHandle{truth}, output))
				}
			}
			if stage == "nested" {
				ref, err := manager.NewRef(truth)
				must(err)
				childRefs = []ast.ASTRef{ref}
			}
			beforeSub, beforeEvaluator, beforeTerms := owner.substitution, owner.evaluator, owner.terms
			beforeOutputs := append([]ast.ASTRef(nil), owner.outputs...)
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
			if body == nil || callbacks != 1 || !errors.Is(failure, ast.ErrASTPending) || published != nil {
				t.Fatal("native refusal did not suppress binding publication", body, callbacks, failure, published)
			}
			if operationCause != nil && !errors.Is(failure, operationCause) {
				t.Fatal("cleanup lost original binding failure", failure)
			}
			if owner.substitution != beforeSub || owner.evaluator != beforeEvaluator || owner.terms != beforeTerms || len(owner.outputs) != len(beforeOutputs) {
				t.Fatal("refusal advanced a dependent cleanup owner")
			}
			for i := range beforeOutputs {
				if owner.outputs[i] != beforeOutputs[i] {
					t.Fatal("refusal consumed an output root", i)
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
			var cleanup memory.ConstructionFailure
			if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
				t.Fatal("missing concrete binding cleanup receipt", failure)
			}
			original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
			must(cleanup.Cleanup())
			must(cleanup.Cleanup())
			if owner.nested != nil || owner.substitution != nil || owner.evaluator != nil || owner.terms != nil || owner.outputs != nil {
				t.Fatal("cleanup retry left a binding owner")
			}
			if stage == "nested" && childRefs[0] != (ast.ASTRef{}) {
				t.Fatal("cleanup retry left the nested root")
			}
			if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message || cleanup.CleanupCause() != nil {
				t.Fatal("cleanup retry changed stable binding diagnostics")
			}
		})
	}
}
