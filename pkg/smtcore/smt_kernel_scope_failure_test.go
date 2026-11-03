package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Use the native numeral transport while its AST invocation owns the manager.
// Cleanup must then refuse access to live roots instead of silently dropping
// them. No replacement solver or cleanup callback is installed in production.
func TestSMTKernelPopResumesNativeOwnerFrontiers(t *testing.T) {
	for _, test := range []struct {
		stage  smtKernelPopStage
		result string
	}{
		{smtKernelPopBVSharing, ""}, {smtKernelPopMacros, ""},
		{smtKernelPopDefinedNames, ""}, {smtKernelPopSubstitution, ""},
		{smtKernelPopAssertions, ""}, {smtKernelPopNRAInputs, ""},
		{smtKernelPopFinish, ""}, {smtKernelPopFinish, "assignments"},
		{smtKernelPopFinish, "relevant"}, {smtKernelPopFinish, "labeled"},
		{smtKernelPopFinish, "guessed"}, {smtKernelPopFinish, "unsat-core"},
	} {
		stage := test.stage
		for _, closeOnly := range []bool{false, true} {
			if test.result != "" && !closeOnly {
				continue
			}
			t.Run(fmt.Sprintf("stage=%d/close=%v/result=%s", stage, closeOnly, test.result), func(t *testing.T) {
				budget := arena.NewBudget(1 << 28)
				var s *SMTSolver
				var failure error
				var closeFailure error
				active := false
				runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
					if active && failure == nil {
						failure = s.Pop(1)
						if closeOnly {
							closeFailure = s.kernel.Close()
						}
					}
					return source
				}})
				astTestMust(t, err)
				numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
				astTestMust(t, err)
				symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
				astTestMust(t, err)
				m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
				astTestMust(t, err)
				params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
				astTestMust(t, err)
				t.Cleanup(func() {
					astTestMust(t, s.Close())
					astTestMust(t, params.Close())
					astTestMust(t, m.Close())
					astTestMust(t, numbers.Close())
					astTestMust(t, symbols.Close())
					astTestMust(t, runtime.Close())
					if budget.Used() != 0 {
						t.Fatal("kernel scope cleanup leaked bytes", budget.Used())
					}
				})
				_, err = m.RegisterArithmeticPlugin()
				astTestMust(t, err)
				if stage == smtKernelPopAssertions {
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				s, err = NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: params})
				astTestMust(t, err)
				astTestMust(t, s.Push())
				p := smtRelevancyBool(t, m, "kernel-pop-frontier")
				astTestMust(t, s.Assert(p))
				k := s.kernel
				root, err := m.NewRef(p)
				astTestMust(t, err)
				k.nraInputs = append(k.nraInputs, root)

				// Reach each exact boundary using the real child owners. Keeping
				// the parent scope published models a refusal between children.
				astTestMust(t, k.internalizer.PopScopes(1))
				if stage > smtKernelPopBVSharing {
					astTestMust(t, k.bvSharing.PopScope(1))
				}
				if stage > smtKernelPopMacros {
					astTestMust(t, k.assertedMacros.PopScope(1))
				}
				if stage > smtKernelPopDefinedNames {
					astTestMust(t, k.definedNames.Pop(1))
				}
				if stage > smtKernelPopSubstitution {
					astTestMust(t, m.ScopedExprSubstitutionPop(k.assertedScopedSubst, 1))
				}
				if stage == smtKernelPopAssertions {
					// A proof may be consumed before its formula refuses release.
					// The remaining assertion must keep only that live root.
					proof := &k.assertions[0].Proof
					if proof.IsNull() {
						t.Fatal("proof frontier has no owned proof")
					}
					consumed, err := m.DecRefConsumed(*proof)
					astTestMust(t, err)
					if !consumed {
						t.Fatal("proof frontier did not consume its proof")
					}
					*proof = ast.ASTHandle{}
				}
				if stage > smtKernelPopAssertions {
					astTestMust(t, k.restoreAssertions(0))
				}
				if stage > smtKernelPopNRAInputs {
					astTestMust(t, k.restoreNRAInputs(0))
					k.scopes = nil
					k.nraScopes = nil
					k.assertedScopeConflicts = nil
				}
				k.pendingPop = &smtKernelPendingPop{count: 1, stage: stage}
				s.pendingPop = &smtSolverPendingPop{count: 1, kernelPopped: stage == smtKernelPopFinish, kernelCleanup: stage == smtKernelPopFinish}
				var resultRoots *[]ast.ASTHandle
				var resultRoot ast.ASTHandle
				if test.result != "" {
					resultRoot = smtRelevancyBool(t, m, "pending-result-root")
					astTestMust(t, m.IncRef(resultRoot))
					switch test.result {
					case "assignments":
						resultRoots = &k.assignments
					case "relevant":
						resultRoots = &k.relevantLiterals
					case "labeled":
						resultRoots = &k.relevantLabeled
					case "guessed":
						resultRoots = &k.guessed
					case "unsat-core":
						resultRoots = &k.unsatCore
					}
					// The first owner was consumed before a later root refused.
					*resultRoots = []ast.ASTHandle{{}, resultRoot}
				}

				value, err := numbers.NewValue()
				astTestMust(t, err)
				astTestMust(t, numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
				astTestMust(t, runtime.SetMaxCount(uint64(runtime.State().GlobalCount)+1))
				var invocation ast.ASTArithmeticNumeralInvocation
				active = true
				_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
				active = false
				astTestMust(t, runtime.SetMaxCount(0))
				astTestMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
				astTestMust(t, numbers.Del(&value))
				if body == nil || !errors.Is(failure, ast.ErrASTPending) {
					t.Fatal("native scope cleanup did not refuse", body, failure)
				}
				level, err := s.ScopeLevel()
				astTestMust(t, err)
				if level != 1 || k.pendingPop == nil || k.pendingPop.stage != stage {
					t.Fatal("refused cleanup lost its published scope or frontier", level, k.pendingPop)
				}
				if err = k.Push(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("kernel accepted work during pending cleanup", err)
				}
				if err = k.Pop(2); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("kernel accepted a different pending pop", err)
				}
				if closeOnly {
					if !errors.Is(closeFailure, ast.ErrASTPending) {
						t.Fatal("native close did not refuse its live owners", closeFailure)
					}
					if err = s.Pop(1); !errors.Is(err, ast.ErrASTPending) {
						t.Fatal("solver resumed pop after kernel destruction began", err)
					}
					if resultRoots != nil {
						count, err := m.GetRefCount(resultRoot)
						astTestMust(t, err)
						if len(*resultRoots) != 2 || (*resultRoots)[1] != resultRoot || count != 1 {
							t.Fatal("refused result cleanup lost its remaining root", *resultRoots, count)
						}
					}
					astTestMust(t, s.Close())
					return
				}
				astTestMust(t, s.Pop(1))
				level, err = s.ScopeLevel()
				astTestMust(t, err)
				if level != 0 || k.ScopeLevel() != 0 || k.pendingPop != nil || len(k.assertions) != 0 || len(k.nraInputs) != 0 {
					t.Fatal("scope retry left owners or bookkeeping", level, k.ScopeLevel(), k.pendingPop)
				}
				astTestMust(t, s.Push())
				astTestMust(t, s.Pop(1))
			})
		}
	}
}

func TestSMTKernelPendingPopRejectsNRABeforeAcquiringInputs(t *testing.T) {
	m, s := smtSolverFixture(t)
	astTestMust(t, s.Push())
	k := s.kernel
	k.setupLogic = "QF_NRA"
	k.nraInputsValid = true
	k.pendingPop = &smtKernelPendingPop{count: 1}
	p := smtRelevancyBool(t, m, "pending-nra-input")
	root, err := m.NewRef(p)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&root)) }()
	before, err := m.GetRefCount(p)
	astTestMust(t, err)
	if err := k.Assert(p, ast.ASTHandle{}); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("pending pop accepted an NRA assertion", err)
	}
	after, err := m.GetRefCount(p)
	astTestMust(t, err)
	if !k.nraInputsValid || len(k.nraInputs) != 0 || before != after {
		t.Fatal("refused assertion changed NRA ownership", k.nraInputsValid, len(k.nraInputs), before, after)
	}
	astTestMust(t, s.Pop(1))
}
