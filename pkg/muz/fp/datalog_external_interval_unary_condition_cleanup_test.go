package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExternalIntervalUnaryConditionAllocationFailureRetry(t *testing.T) {
	for _, frontier := range []struct {
		name string
		free int
	}{{name: "binding", free: 2}, {name: "conjunction", free: 4}} {
		t.Run(frontier.name, func(t *testing.T) {
			failed, completed := 0, false
			for allowance := int64(0); allowance < 256 && !completed; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					var budget *arena.Budget
					t.Cleanup(func() {
						if budget != nil && budget.Used() != 0 {
							t.Fatal("condition allocation fixture leaked bytes", budget.Used())
						}
					})
					manager, _ := newPDRLifecycleServices(t)
					budget = manager.Budget()
					own := func(handle ast.ASTHandle, err error) ast.ASTHandle {
						t.Helper()
						fragmentMust(t, err)
						root, cause := manager.NewRef(handle)
						fragmentMust(t, cause)
						t.Cleanup(func() { fragmentMust(t, manager.DelRef(&root)) })
						return handle
					}
					family, err := manager.RegisterArithmeticPlugin()
					fragmentMust(t, err)
					integer := own(manager.MkFamilySort(family, 1, nil))
					x := own(manager.MkVar(0, integer))
					y := own(manager.MkVar(1, integer))
					seven := own(manager.ArithmeticIntegerNumeralText("7"))
					eleven := own(manager.ArithmeticIntegerNumeralText("11"))
					first := own(manager.MkArithmeticLE(x, seven))
					second := own(manager.MkArithmeticLT(y, eleven))
					// Warm only the persistent AND declaration. The conjunction
					// under test and both replacement variables remain fresh.
					warm, err := manager.MkAndArgs([]ast.ASTHandle{first, first})
					fragmentMust(t, err)
					warmRoot, err := manager.NewRef(warm)
					fragmentMust(t, err)
					fragmentMust(t, manager.DelRef(&warmRoot))
					condition := &externalIntervalUnaryCondition{
						source: []ast.ASTHandle{first, second},
						bindings: []externalIntervalUnaryConditionBinding{
							{variable: 0, column: 2, sort: integer},
							{variable: 1, column: 3, sort: integer},
						},
					}
					t.Cleanup(func() { fragmentMust(t, condition.close(manager)) })
					// Exhaust actual ref records, then release exactly the source
					// pins and (for conjunction) binding slots. The next ref growth
					// occurs at the fresh-node publication boundary under test.
					var pins []ast.ASTRef
					for i := 0; i < 4096; i++ {
						budget.FailAfter(0)
						pin, cause := manager.NewRef(ast.ASTHandle{})
						budget.FailAfter(-1)
						if errors.Is(cause, arena.ErrInjectedFailure) {
							break
						}
						fragmentMust(t, cause)
						pins = append(pins, pin)
						if i == 4095 {
							t.Fatal("condition reference-page frontier not reached")
						}
					}
					if len(pins) < frontier.free {
						t.Fatal("condition fixture has too few reference slots", len(pins))
					}
					for i := len(pins) - frontier.free; i < len(pins); i++ {
						fragmentMust(t, manager.DelRef(&pins[i]))
					}
					t.Cleanup(func() {
						for i := range pins {
							if pins[i] != (ast.ASTRef{}) {
								fragmentMust(t, manager.DelRef(&pins[i]))
							}
						}
					})
					before := publicUDocProductLiveCounts(manager.Stats())
					budget.FailAfter(allowance)
					cause := condition.prepare(manager)
					budget.FailAfter(-1)
					if cause != nil {
						failed++
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal("unexpected preparation allocation failure", cause)
						}
						var receipt memory.ConstructionFailure
						if errors.As(cause, &receipt) {
							fragmentMust(t, receipt.Cleanup())
						}
						if condition.mapped != (ast.ASTRef{}) {
							t.Fatal("failed condition construction published a guard")
						}
						want := before
						want[3] += uint64(len(condition.sourceRoots))
						if actual := publicUDocProductLiveCounts(manager.Stats()); actual != want {
							t.Fatal("failed condition construction lost fresh AST ownership", want, actual, cause)
						}
					} else {
						completed = true
					}
					fragmentMust(t, condition.prepare(manager))
					mapped, err := condition.handle(manager)
					fragmentMust(t, err)
					for i, bound := range []ast.ASTHandle{seven, eleven} {
						guard, err := manager.GetArg(mapped, uint32(i))
						fragmentMust(t, err)
						argument, err := manager.GetArg(guard, 0)
						fragmentMust(t, err)
						column, err := manager.GetVarIndex(argument)
						fragmentMust(t, err)
						value, err := manager.GetArg(guard, 1)
						fragmentMust(t, err)
						if column != uint32(i+2) || value != bound {
							t.Fatal("allocation retry changed guard order or mapping", i, column, value)
						}
						rooted, err := manager.RefGet(condition.sourceRoots[i])
						fragmentMust(t, err)
						if rooted != condition.source[i] {
							t.Fatal("allocation retry changed source root", i)
						}
					}
					fragmentMust(t, condition.close(manager))
					if actual := publicUDocProductLiveCounts(manager.Stats()); actual != before {
						t.Fatal("condition retry and close leaked AST ownership", before, actual)
					}
				}) {
					return
				}
			}
			if failed == 0 || !completed {
				t.Fatal("condition allocation campaign missed failure or success", failed, completed)
			}
		})
	}
}

func TestExternalIntervalUnaryConditionPreparationCleanupNativeRetry(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var condition *externalIntervalUnaryCondition
	var cleanupFailure error
	active, fired := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			cleanupFailure = condition.failPreparation(nil)
		}
		return failure
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	family, err := manager.RegisterArithmeticPlugin()
	must(err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	must(err)
	x, err := manager.MkVar(0, integer)
	must(err)
	y, err := manager.MkVar(1, integer)
	must(err)
	seven, err := manager.ArithmeticIntegerNumeralText("7")
	must(err)
	eleven, err := manager.ArithmeticIntegerNumeralText("11")
	must(err)
	first, err := manager.MkArithmeticLE(x, seven)
	must(err)
	firstRoot, err := manager.NewRef(first)
	must(err)
	second, err := manager.MkArithmeticLT(y, eleven)
	must(err)
	secondRoot, err := manager.NewRef(second)
	must(err)
	condition = &externalIntervalUnaryCondition{
		source: []ast.ASTHandle{first, second},
		bindings: []externalIntervalUnaryConditionBinding{
			{variable: 1, column: 0, sort: integer},
			{variable: 0, column: 2, sort: integer},
		},
	}
	checkMapped := func() {
		t.Helper()
		mapped, cause := condition.handle(manager)
		must(cause)
		declaration, cause := manager.GetDecl(mapped)
		must(cause)
		op, cause := manager.GetDeclKind(declaration)
		must(cause)
		count, cause := manager.GetNumArgs(mapped)
		must(cause)
		if op != ast.ASTOpAnd || count != 2 {
			t.Fatal("guard preparation lost the ordered conjunction", op, count)
		}
		for i, want := range []uint32{2, 0} {
			guard, cause := manager.GetArg(mapped, uint32(i))
			must(cause)
			variable, cause := manager.GetArg(guard, 0)
			must(cause)
			column, cause := manager.GetVarIndex(variable)
			must(cause)
			bound, cause := manager.GetArg(guard, 1)
			must(cause)
			if column != want || bound != []ast.ASTHandle{seven, eleven}[i] {
				t.Fatal("guard preparation changed source order or column mapping", i, column, want, bound)
			}
		}
	}
	must(condition.prepare(manager))
	checkMapped()
	prepared := condition.mapped
	must(condition.prepare(manager))
	if condition.mapped != prepared || condition.attempt != nil || len(condition.sourceRoots) != 2 {
		t.Fatal("repeated preparation replaced the compiled condition")
	}
	must(condition.close(manager))

	// Recreate the real construction frontier after substitution, before
	// temporary owners retire. The adopted condition owns raw source roots;
	// its concrete attempt owns the rewriter, bindings, conjunction and output.
	condition.manager = manager
	for _, source := range condition.source {
		root, cause := manager.NewRef(source)
		must(cause)
		condition.sourceRoots = append(condition.sourceRoots, root)
	}
	attempt := &externalIntervalUnaryConditionAttempt{manager: manager}
	condition.attempt = attempt
	bindings := make([]ast.ASTHandle, 2)
	for _, binding := range condition.bindings {
		mapped, cause := manager.MkVar(binding.column, binding.sort)
		must(cause)
		root, cause := manager.NewRef(mapped)
		must(cause)
		attempt.roots = append(attempt.roots, root)
		bindings[binding.variable] = mapped
	}
	conjunction, err := manager.MkAndArgs(condition.source)
	must(err)
	conjunctionRoot, err := manager.NewRef(conjunction)
	must(err)
	attempt.roots = append(attempt.roots, conjunctionRoot)
	attempt.substitution, err = astrewriter.NewVarSubstitution(manager, false)
	must(err)
	attempt.mapped, err = manager.NewRef(ast.ASTHandle{})
	must(err)
	must(attempt.substitution.Substitute(conjunction, bindings, attempt.mapped))
	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !fired || !errors.Is(cleanupFailure, ast.ErrASTPending) || condition.attempt != attempt || len(condition.sourceRoots) != 2 || attempt.mapped == (ast.ASTRef{}) {
		t.Fatal("native preparation cleanup lost its owners", body, fired, cleanupFailure)
	}
	if _, cause := condition.handle(manager); !errors.Is(cause, ErrExternalIntervalDatalogFragment) {
		t.Fatal("failed preparation published a mapped guard", cause)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(cleanupFailure, &receipt) || receipt.RuntimeOwner() != runtime {
		t.Fatal("guard preparation lost its cleanup receipt", cleanupFailure)
	}
	shown, exposed, original := receipt.Error(), errors.Unwrap(receipt), receipt.OriginalCause()
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if attempt.substitution != nil || attempt.mapped != (ast.ASTRef{}) || len(attempt.roots) != 0 || receipt.CleanupCause() != nil || receipt.Error() != shown || errors.Unwrap(receipt) != exposed || receipt.OriginalCause() != original {
		t.Fatal("guard preparation cleanup did not drain a stable receipt")
	}
	// The next installation must accept an explicitly cleaned old receipt,
	// preserve its source pins, and construct a fresh usable mapped guard.
	must(condition.prepare(manager))
	checkMapped()
	if condition.attempt != nil || len(condition.sourceRoots) != 2 {
		t.Fatal("guard preparation retry changed source-root ownership")
	}
	must(condition.close(manager))
	must(condition.close(manager))
	for _, source := range condition.source {
		refs, cause := manager.GetRefCount(source)
		must(cause)
		if refs != 1 {
			t.Fatal("guard plan close did not release its source pins", refs)
		}
	}
	single := &externalIntervalUnaryCondition{
		source:   []ast.ASTHandle{first},
		bindings: []externalIntervalUnaryConditionBinding{{variable: 0, column: 0, sort: integer}},
	}
	must(single.prepare(manager))
	sole, err := single.handle(manager)
	must(err)
	if sole != first {
		t.Fatal("sole guard was wrapped in an extra conjunction")
	}
	must(single.close(manager))
	must(numbers.Del(&number))
	must(manager.DelRef(&secondRoot))
	must(manager.DelRef(&firstRoot))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("guard condition fixture leaked bytes", used)
	}
}
