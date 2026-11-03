package macros

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func macroCleanupMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Public reallocations reach the real signed accounting boundary without
// modifying private state or injecting a synthetic destructor failure.
func macroCleanupNearMinimum(t *testing.T, runtime *memory.Runtime) {
	t.Helper()
	a, err := runtime.Allocate(8)
	macroCleanupMust(t, err)
	b, err := runtime.Allocate(8)
	macroCleanupMust(t, err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first accounting boundary", err)
	}
	macroCleanupMust(t, runtime.Deallocate(&a))
	delta = math.MinInt64 + 1000 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second accounting boundary", err)
	}
	macroCleanupMust(t, runtime.Deallocate(&b))
	if state := runtime.State(); state.GlobalSize != math.MinInt64+1000 || state.LocalSize != 0 {
		t.Fatal("native accounting fixture", state)
	}
}

func TestMacroManagerCleanupResumesConsumedDependencyAndScope(t *testing.T) {
	for _, scenario := range []string{"pop/dependency", "reset/dependency", "close/dependency", "pop/quantifier", "reset/quantifier", "close/quantifier"} {
		t.Run(scenario, func(t *testing.T) {
			parts := strings.Split(scenario, "/")
			operation, field := parts[0], parts[1]
			budget := arena.NewBudget(1 << 28)
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
			macroCleanupMust(t, err)
			numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
			macroCleanupMust(t, err)
			symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
			macroCleanupMust(t, err)
			manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
			macroCleanupMust(t, err)
			_, err = manager.RegisterArithmeticPlugin()
			macroCleanupMust(t, err)
			macros, err := NewManager(manager)
			macroCleanupMust(t, err)
			var recovery memory.Block
			defer func() {
				macroCleanupMust(t, macros.Close())
				macroCleanupMust(t, manager.Close())
				macroCleanupMust(t, numbers.Close())
				macroCleanupMust(t, symbols.Close())
				if recovery != (memory.Block{}) {
					if err := runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
						t.Fatal("recovery allocation cleanup", err)
					}
				}
				macroCleanupMust(t, runtime.Close())
				if budget.Used() != 0 {
					t.Fatal("macro cleanup leaked storage", budget.Used())
				}
			}()
			integer, err := numeral.NewScopedInt(numbers.Manager.Z)
			macroCleanupMust(t, err)
			macroCleanupMust(t, integer.SetInt64(1))
			raw, err := integer.Get()
			macroCleanupMust(t, err)
			macroCleanupMust(t, numbers.Manager.Z.Mul2kInPlace(raw, 1600000))
			value, err := numbers.NewFromScopedInt(integer)
			macroCleanupMust(t, err)
			atom, err := manager.ArithmeticNumeral(&value, true)
			macroCleanupMust(t, err)
			var dependency ast.ASTDependency
			var numericDefinition ast.ASTHandle
			if field == "dependency" {
				dependency, err = manager.DependencyLeaf(atom)
				macroCleanupMust(t, err)
			} else {
				zero, err := numbers.NewValue()
				macroCleanupMust(t, err)
				zeroAtom, err := manager.ArithmeticNumeral(&zero, true)
				macroCleanupMust(t, err)
				numericDefinition, err = manager.MkEq(atom, zeroAtom)
				macroCleanupMust(t, err)
				macroCleanupMust(t, numbers.Del(&zero))
			}
			macroCleanupMust(t, numbers.Del(&value))
			macroCleanupMust(t, integer.Close())
			boolean, err := manager.MkBoolSort()
			macroCleanupMust(t, err)
			variable, err := manager.MkVar(0, boolean)
			macroCleanupMust(t, err)
			var declarations []ast.ASTHandle
			for i := 0; i < 3; i++ {
				if i < 2 {
					macroCleanupMust(t, macros.PushScope())
				}
				name, err := symbols.Intern(fmt.Sprintf("cleanup-macro-%d", i))
				macroCleanupMust(t, err)
				decl, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				macroCleanupMust(t, err)
				head, err := manager.MkApp(decl, []ast.ASTHandle{variable})
				macroCleanupMust(t, err)
				definition := variable
				if i == 1 && field == "quantifier" {
					definition = numericDefinition
				}
				equality, err := manager.MkEq(head, definition)
				macroCleanupMust(t, err)
				formula, err := manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{name}, Body: equality})
				macroCleanupMust(t, err)
				dep := ast.ASTDependency{}
				if i == 1 {
					dep = dependency
				}
				inserted, err := macros.Insert(decl, formula, ast.ASTHandle{}, dep)
				macroCleanupMust(t, err)
				if !inserted {
					t.Fatal("macro insertion")
				}
				macroCleanupMust(t, macros.MarkForbidden([]ast.ASTHandle{head}))
				declarations = append(declarations, decl)
			}
			beforeRefs, err := manager.GetRefCount(declarations[0])
			macroCleanupMust(t, err)
			perform := func() error {
				switch operation {
				case "pop":
					return macros.PopScope(1)
				case "reset":
					return macros.Reset()
				default:
					return macros.Close()
				}
			}
			macroCleanupNearMinimum(t, runtime)
			blocks := runtime.Stats().LiveBlocks
			stopped := perform()
			if !errors.Is(stopped, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
				t.Fatal("did not stop after physical deallocation", stopped, runtime.State())
			}
			if len(macros.entries) != 2 || len(macros.scopes) != 2 || macros.dependencies == nil || macros.closed || len(macros.forbidden) != 3 {
				t.Fatal("cleanup lost remaining entry, dependency or scope", len(macros.entries), len(macros.scopes), len(macros.forbidden))
			}
			entry := macros.entries[1]
			if field == "dependency" && (!entry.dependencyPending || entry.dependency != dependency || entry.quantifier.IsNull()) {
				t.Fatal("consumed dependency lost its continuation or next AST owner", entry)
			}
			if field == "quantifier" && (!entry.quantifier.IsNull() || entry.declaration.IsNull()) {
				t.Fatal("consumed AST reference was not retired before refusal", entry)
			}
			if refs, err := manager.GetRefCount(declarations[0]); err != nil || refs != beforeRefs {
				t.Fatal("cleanup changed untouched macro", refs, beforeRefs, err)
			}
			if err := macros.PushScope(); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("scope mutation accepted during cleanup", err)
			}
			if operation == "pop" {
				if err := macros.PopScope(2); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("different pop replaced pending request", err)
				}
			}
			recovery, err = runtime.Allocate(400000)
			macroCleanupMust(t, err)
			macroCleanupMust(t, perform())
			if operation == "pop" {
				if len(macros.scopes) != 1 || len(macros.entries) != 1 || len(macros.forbidden) != 1 || !macros.IsForbidden(declarations[0]) {
					t.Fatal("pop retry did not preserve outer scope")
				}
				macroCleanupMust(t, macros.PopScope(1))
			} else if len(macros.scopes) != 0 || len(macros.entries) != 0 || len(macros.forbidden) != 0 {
				t.Fatal("cleanup retry left owned state")
			}
			macroCleanupMust(t, macros.Close())
			macroCleanupMust(t, macros.Close())
		})
	}
}
