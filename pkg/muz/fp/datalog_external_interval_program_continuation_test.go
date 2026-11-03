package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExternalIntervalDatalogProgramFinalContinuationNativeRetry(t *testing.T) {
	for _, frontier := range []string{"final-rule-cleanup", "final-store-retirement"} {
		t.Run(frontier, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var interrupted func() error
			var cleanupFailure error
			active, fired := false, false
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
				if active && !fired {
					fired = true
					cleanupFailure = interrupted()
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
			boolean, err := manager.MkBoolSort()
			must(err)
			declare := func(label string, sorts []ast.ASTHandle) ast.ASTHandle {
				name, cause := symbols.Intern("external-interval-program-continuation-" + label)
				must(cause)
				declaration, cause := manager.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
				must(cause)
				return declaration
			}
			p := declare("p", []ast.ASTHandle{integer})
			q := declare("q", []ast.ASTHandle{integer})
			r := declare("r", []ast.ASTHandle{integer})
			h := declare("h", []ast.ASTHandle{integer, integer})
			relations, err := rel.NewRelationManager(manager)
			must(err)
			callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
			plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
			must(err)
			kinds := []string{"external_relation", "interval_relation"}
			activation, err := newExternalIntervalDatalogActivation(manager, plugin, []externalIntervalDatalogPredicate{
				{declaration: p, sorts: []ast.ASTHandle{integer}, kinds: kinds},
				{declaration: q, sorts: []ast.ASTHandle{integer}, kinds: kinds},
				{declaration: r, sorts: []ast.ASTHandle{integer}, kinds: kinds},
				{declaration: h, sorts: []ast.ASTHandle{integer, integer}, kinds: kinds},
			})
			must(err)
			one, err := manager.ArithmeticIntegerNumeralText("1")
			must(err)
			must(activation.addFact(p, []ast.ASTHandle{one}))
			two, err := manager.ArithmeticIntegerNumeralText("2")
			must(err)
			must(activation.addFact(r, []ast.ASTHandle{two}))
			first := &externalIntervalDatalogIdentityRule{head: q, tail: p}
			last := &externalIntervalDatalogCartesianRule{head: h, left: q, right: r}
			program := &externalIntervalDatalogProgram{
				rules:        []externalIntervalDatalogProgramRule{{identity: first}, {cartesian: last}},
				declarations: []ast.ASTHandle{q, p, h, q, r}, schedule: []int{0, 1},
			}
			must(activation.installExternalIntervalProgram(program))
			oldHead, oldQ := activation.entries[h].product, activation.entries[q].product
			callbacks.operations = nil
			registers, err := activation.loadExternalIntervalDatalogRegisters(program.declarations)
			must(err)
			registers.deferStore = true
			program.registers = registers
			activation.identity = first
			must(activation.executeExternalIntervalIdentity())
			program.next = 1
			activation.identity, activation.cartesian = nil, last

			// Stage real final JOIN/UNION outputs at the existing continuation
			// boundary, so native refusal can target cleanup after commitment.
			left, err := registers.relation(q)
			must(err)
			right, err := registers.relation(r)
			must(err)
			joined, err := rel.JoinExternalIntervalProducts(left, right, nil, nil)
			must(err)
			target, err := registers.ensureRelation(h)
			must(err)
			must(target.Union(joined, nil, false))
			continuation := &externalIntervalDatalogCartesianContinuation{
				registers: registers, temporary: joined, runtime: runtime, publish: true,
			}
			if activation.entries[q].product != oldQ || activation.entries[h].product != oldHead {
				t.Fatal("individual rules published the shared bank before final STORE")
			}
			if frontier == "final-rule-cleanup" {
				interrupted = continuation.resume
			} else {
				must(continuation.resume())
				program.next++
				// Invoke the actual program final STORE while the native AST
				// mutation guard is active; publication succeeds, retirement waits.
				interrupted = activation.executeExternalIntervalProgram
			}
			callbacksBeforeRetry := len(callbacks.operations)
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
			if body == nil || !fired || !errors.Is(cleanupFailure, ast.ErrASTPending) || registers.closed {
				t.Fatal("native program cleanup refusal was not preserved", body, fired, cleanupFailure)
			}
			if frontier == "final-rule-cleanup" {
				if registers.published || program.storing || continuation.temporary != joined || activation.entries[h].product != oldHead {
					t.Fatal("last rule cleanup refusal reached final STORE")
				}
				activation.setCartesianPending(newExternalIntervalDatalogCartesianFailure(cleanupFailure, continuation), true)
			} else if !registers.published || !program.storing || activation.entries[h].product != target {
				t.Fatal("final STORE refusal did not preserve published replacements")
			}
			if len(callbacks.operations) != callbacksBeforeRetry {
				t.Fatal("cleanup refusal replayed a relation callback", callbacks.operations)
			}
			receipt := activation.pending
			if receipt == nil || receipt.RuntimeOwner() != runtime || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
				t.Fatal("program cleanup lost its pending owner", receipt)
			}
			shown, original := receipt.Error(), receipt.OriginalCause()
			exposed := errors.Unwrap(receipt)
			must(activation.executeExternalIntervalProgram())
			if activation.pending != nil || activation.pendingCompletesRule || program.registers != nil || program.storing || program.next != 0 || !registers.closed {
				t.Fatal("program continuation did not finish the final STORE")
			}
			if activation.entries[h].product != target || activation.entries[q].product != left || len(callbacks.operations) != callbacksBeforeRetry ||
				operationCount(callbacks.operations, ast.DatalogOpClone) != 2 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 2 || operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 {
				t.Fatal("program continuation replayed a rule or replaced a committed register", callbacks.operations)
			}
			if receipt.Error() != shown || errors.Unwrap(receipt) != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil {
				t.Fatal("program retry changed the cleanup receipt identity")
			}
			must(receipt.Cleanup())
			if len(callbacks.operations) != callbacksBeforeRetry {
				t.Fatal("completed program receipt replayed relation callbacks")
			}
			for _, product := range []*rel.ExternalIntervalProduct{oldHead, oldQ, joined} {
				if _, cause := product.Empty(); !errors.Is(cause, rel.ErrTableRelation) {
					t.Fatal("program did not retire a displaced or temporary product", cause)
				}
			}
			empty, err := target.Empty()
			must(err)
			if empty {
				t.Fatal("program final UNION result became empty")
			}
			must(numbers.Del(&number))
			must(activation.close())
			must(plugin.Close())
			must(relations.Close())
			must(manager.Close())
			must(numbers.Close())
			must(symbols.Close())
			must(runtime.Close())
			if used := budget.Used(); used != 0 {
				t.Fatal("program continuation fixture leaked bytes", used)
			}
		})
	}
}
