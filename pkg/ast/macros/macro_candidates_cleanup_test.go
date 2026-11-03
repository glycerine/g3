package macros

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type candidateCleanupFixture struct {
	manager  *ast.ASTManager
	numbers  *numeral.RationalService
	runtime  *memory.Runtime
	owner    *Candidates
	recovery memory.Block
}

func newCandidateCleanupFixture(t *testing.T, threadLocal bool, transport func(memory.Failure) error) *candidateCleanupFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: threadLocal, SizeTBits: 64, Transport: transport})
	macroCleanupMust(t, err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	macroCleanupMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	macroCleanupMust(t, err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	macroCleanupMust(t, err)
	_, err = manager.RegisterArithmeticPlugin()
	macroCleanupMust(t, err)
	owner, err := NewCandidates(manager)
	macroCleanupMust(t, err)
	f := &candidateCleanupFixture{manager: manager, numbers: numbers, runtime: runtime, owner: owner}
	t.Cleanup(func() {
		macroCleanupMust(t, owner.Close())
		macroCleanupMust(t, manager.Close())
		macroCleanupMust(t, numbers.Close())
		macroCleanupMust(t, symbols.Close())
		if f.recovery != (memory.Block{}) {
			if err := runtime.Deallocate(&f.recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || f.recovery != (memory.Block{})) {
				t.Fatal("candidate recovery allocation cleanup", err)
			}
		}
		macroCleanupMust(t, runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("candidate cleanup leaked storage", budget.Used())
		}
	})
	return f
}

func (f *candidateCleanupFixture) add(t *testing.T, index int, definition ast.ASTHandle) Candidate {
	t.Helper()
	m := f.manager
	sort, err := m.GetSort(definition)
	macroCleanupMust(t, err)
	name, err := m.Symbols.Intern(fmt.Sprintf("cleanup-candidate-%d", index))
	macroCleanupMust(t, err)
	decl, err := m.MkFuncDecl(name, nil, sort, ast.ASTFuncDeclInfo{})
	macroCleanupMust(t, err)
	boolean, err := m.MkBoolSort()
	macroCleanupMust(t, err)
	name, err = m.Symbols.Intern(fmt.Sprintf("cleanup-condition-%d", index))
	macroCleanupMust(t, err)
	condition, err := m.MkConst(name, boolean)
	macroCleanupMust(t, err)
	row := Candidate{Declaration: decl, Definition: definition, Condition: condition, SatisfyAtom: true}
	macroCleanupMust(t, f.owner.Insert(row))
	return row
}

func TestMacroCandidatesCleanupPreservesUnconsumedRows(t *testing.T) {
	for _, operation := range []string{"reset", "close"} {
		t.Run(operation, func(t *testing.T) {
			var owner *Candidates
			callbacks, intact := 0, true
			perform := func() error {
				if operation == "reset" {
					return owner.Reset()
				}
				return owner.Close()
			}
			f := newCandidateCleanupFixture(t, false, func(failure memory.Failure) error {
				callbacks++
				before := append([]Candidate(nil), owner.rows...)
				for retry := 0; retry < 2; retry++ {
					err := perform()
					intact = intact && errors.Is(err, ast.ErrASTPending) && len(owner.rows) == len(before) && !owner.closed
					if len(owner.rows) == len(before) {
						for i := range before {
							intact = intact && owner.rows[i] == before[i]
						}
					}
				}
				return failure
			})
			owner = f.owner
			truth, err := f.manager.MkTrue()
			macroCleanupMust(t, err)
			f.add(t, 0, truth)
			f.add(t, 1, truth)
			value, err := f.numbers.NewValue()
			macroCleanupMust(t, err)
			macroCleanupMust(t, f.numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
			macroCleanupMust(t, f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount)+1))
			var invocation ast.ASTArithmeticNumeralInvocation
			_, cause := f.manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
			macroCleanupMust(t, f.runtime.SetMaxCount(0))
			macroCleanupMust(t, f.manager.CleanupArithmeticNumeralInvocation(&invocation))
			macroCleanupMust(t, f.numbers.Del(&value))
			if cause == nil || callbacks == 0 || !intact {
				t.Fatal("refused cleanup lost candidate owners", cause, callbacks, intact)
			}
			if _, err := owner.At(0); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("cleanup-pending candidates accepted row access", err)
			}
			if err := owner.Insert(Candidate{}); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("cleanup-pending candidates accepted insertion", err)
			}
			macroCleanupMust(t, perform())
			if len(owner.rows) != 0 {
				t.Fatal("cleanup retry left candidate rows")
			}
			if operation == "reset" {
				f.add(t, 2, truth)
			}
			macroCleanupMust(t, owner.Close())
			macroCleanupMust(t, owner.Close())
		})
	}
}

func TestMacroCandidatesCleanupAdvancesConsumedFields(t *testing.T) {
	for _, operation := range []string{"reset", "reset-close", "close"} {
		t.Run(operation, func(t *testing.T) {
			f := newCandidateCleanupFixture(t, true, nil)
			integer, err := numeral.NewScopedInt(f.numbers.Manager.Z)
			macroCleanupMust(t, err)
			macroCleanupMust(t, integer.SetInt64(1))
			raw, err := integer.Get()
			macroCleanupMust(t, err)
			macroCleanupMust(t, f.numbers.Manager.Z.Mul2kInPlace(raw, 1600000))
			value, err := f.numbers.NewFromScopedInt(integer)
			macroCleanupMust(t, err)
			wide, err := f.manager.ArithmeticNumeral(&value, true)
			macroCleanupMust(t, err)
			macroCleanupMust(t, f.numbers.Del(&value))
			macroCleanupMust(t, integer.Close())
			truth, err := f.manager.MkTrue()
			macroCleanupMust(t, err)
			first := f.add(t, 0, truth)
			middle := f.add(t, 1, wide)
			f.add(t, 2, truth)
			before, err := f.manager.GetRefCount(first.Declaration)
			macroCleanupMust(t, err)
			perform := func() error {
				if operation == "close" {
					return f.owner.Close()
				}
				return f.owner.Reset()
			}
			macroCleanupNearMinimum(t, f.runtime)
			blocks := f.runtime.Stats().LiveBlocks
			cause := perform()
			f.recovery, err = f.runtime.Allocate(400000)
			macroCleanupMust(t, err)
			if !errors.Is(cause, arena.ErrBounds) || f.runtime.Stats().LiveBlocks > blocks {
				t.Fatal("cleanup did not reach consumed numerical deallocation", cause)
			}
			if len(f.owner.rows) != 2 || !f.owner.rows[1].Condition.IsNull() || !f.owner.rows[1].Definition.IsNull() || f.owner.rows[1].Declaration != middle.Declaration || f.owner.closed || f.owner.header == 0 {
				t.Fatal("cleanup did not preserve exact remaining fields", f.owner.rows)
			}
			if after, err := f.manager.GetRefCount(first.Declaration); err != nil || after != before {
				t.Fatal("cleanup touched an earlier candidate", before, after, err)
			}
			if _, err := f.owner.At(0); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("partial candidate row was exposed", err)
			}
			if operation == "close" {
				if err := f.owner.Reset(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("reset reopened a closing owner", err)
				}
			}
			if operation == "reset-close" {
				macroCleanupMust(t, f.owner.Close())
			} else {
				macroCleanupMust(t, perform())
			}
			macroCleanupMust(t, f.owner.Close())
			macroCleanupMust(t, f.owner.Close())
		})
	}
}
