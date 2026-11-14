package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExternalIntervalScopeNativeCleanupPreservesOriginalOwners(t *testing.T) {
	for _, retirement := range []bool{false, true} {
		name := "helper-close"
		if retirement {
			name = "store-retirement"
		}
		t.Run(name, func(t *testing.T) {
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			budget := arena.NewBudget(1 << 28)
			var action func() error
			var refusal error
			active, fired := false, false
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
				if active && !fired {
					fired = true
					refusal = action()
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
			declare := func(label string) ast.ASTHandle {
				name, cause := symbols.Intern("external-scope-cleanup-" + label)
				must(cause)
				declaration, cause := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
				must(cause)
				return declaration
			}
			p, h := declare("P"), declare("H")
			relations, err := rel.NewRelationManager(manager)
			must(err)
			callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
			plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
			must(err)
			kinds := []string{"external_relation", "interval_relation"}
			source, err := newExternalIntervalDatalogActivation(manager, plugin, []externalIntervalDatalogPredicate{
				{declaration: p, sorts: []ast.ASTHandle{integer}, kinds: kinds},
				{declaration: h, sorts: []ast.ASTHandle{integer}, kinds: kinds},
			})
			must(err)
			thirteen, err := manager.ArithmeticIntegerNumeralText("13")
			must(err)
			must(source.addFact(p, []ast.ASTHandle{thirteen}))
			originalP, originalH := source.entries[p], source.entries[h]
			rootP, rootH := originalP.root, originalH.root
			oldP, oldH := originalP.product, originalH.product
			filter := declare("filter")
			program := &externalIntervalDatalogProgram{
				rules: []externalIntervalDatalogProgramRule{
					{identity: &externalIntervalDatalogIdentityRule{head: filter, tail: p}},
					{identity: &externalIntervalDatalogIdentityRule{head: h, tail: filter}},
				},
				declarations: []ast.ASTHandle{filter, p, h, filter}, schedule: []int{0, 1},
			}
			scope, err := newExternalIntervalDatalogScope(source, []externalIntervalDatalogPredicate{{declaration: filter, sorts: []ast.ASTHandle{integer}, kinds: kinds}}, program)
			must(err)
			var registers *externalIntervalDatalogRegisters
			if retirement {
				// Run the production rule continuations with their ordinary
				// deferred STORE bank, then interrupt that final STORE itself.
				registers, err = scope.loadExternalIntervalDatalogRegisters(program.declarations)
				must(err)
				registers.deferStore = true
				program.registers = registers
				for _, index := range program.schedule {
					scope.identity = program.rules[index].identity
					must(scope.executeExternalIntervalIdentity())
					program.next++
				}
				registers.deferStore = false
				program.storing = true
				action = registers.store
			} else {
				must(scope.executeExternalIntervalProgram())
				action = scope.close
			}
			callbackCount := len(callbacks.operations)
			number, err := numbers.NewValue()
			must(err)
			must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
			must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
			var invocation ast.ASTArithmeticNumeralInvocation
			active = true
			_, failure := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
			active = false
			must(runtime.SetMaxCount(0))
			must(manager.CleanupArithmeticNumeralInvocation(&invocation))
			if failure == nil || !fired || !errors.Is(refusal, ast.ErrASTPending) || scope.closed || len(callbacks.operations) != callbackCount {
				t.Fatal("native scoped cleanup refusal was not preserved", failure, fired, refusal)
			}
			if retirement {
				var receipt memory.ConstructionFailure
				if !errors.As(refusal, &receipt) || receipt.RuntimeOwner() != runtime || !registers.published || registers.closed {
					t.Fatal("scoped STORE lost its published retirement owner", refusal)
				}
				scope.pending = receipt
			}
			publishedP, publishedH := originalP.product, originalH.product
			if publishedP == oldP || publishedH == oldH || scope.entries[p] != originalP || scope.entries[h] != originalH || originalP.root != rootP || originalH.root != rootH {
				t.Fatal("scoped STORE replaced entry identity or failed to publish original products")
			}
			helper := scope.entries[filter].product
			if helper == nil {
				t.Fatal("refused cleanup discarded the helper product")
			}
			must(scope.close())
			must(scope.close())
			if len(callbacks.operations) != callbackCount || !scope.closed || source.closed || source.entries[p] != originalP || source.entries[h] != originalH || originalP.product != publishedP || originalH.product != publishedH {
				t.Fatal("scope cleanup replayed operations or consumed an original owner")
			}
			for _, root := range []ast.ASTRef{rootP, rootH} {
				if _, cause := manager.RefGet(root); cause != nil {
					t.Fatal("scope cleanup released an original declaration", cause)
				}
			}
			for _, retired := range []*rel.ExternalIntervalProduct{oldP, oldH, helper} {
				if _, cause := retired.Empty(); !errors.Is(cause, rel.ErrTableRelation) {
					t.Fatal("scope cleanup left a displaced/helper product live", cause)
				}
			}
			if manager.IsLiveHandle(filter) {
				t.Fatal("scope cleanup left the helper declaration live")
			}
			status, err := source.query([]ast.ASTHandle{h})
			must(err)
			if status != lbool.Undef {
				t.Fatal("source lost the published nonempty answer", status)
			}
			must(numbers.Del(&number))
			must(source.close())
			must(plugin.Close())
			must(relations.Close())
			must(manager.Close())
			must(numbers.Close())
			must(symbols.Close())
			must(runtime.Close())
			if budget.Used() != 0 {
				t.Fatal("scoped external cleanup leaked bytes", budget.Used())
			}
		})
	}
}
