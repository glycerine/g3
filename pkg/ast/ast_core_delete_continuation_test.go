package ast

// Source: src/ast/ast.cpp:36-39,196-199,1781-1844. Native deletion may
// consume its reference before a rational parameter's destructor refuses.

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type deletionContinuationFixture struct {
	t        *testing.T
	m        *ASTManager
	runtime  *memory.Runtime
	recovery memory.Block
}

func newDeletionContinuationFixture(t *testing.T) *deletionContinuationFixture {
	t.Helper()
	f := &deletionContinuationFixture{t: t}
	budget := arena.NewBudget(1 << 28)
	var err error
	f.runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	astTestMust(t, err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: f.runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	astTestMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	astTestMust(t, err)
	f.m, err = NewASTManager(ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	astTestMust(t, err)
	t.Cleanup(func() {
		if f.recovery == (memory.Block{}) {
			f.recovery, err = f.runtime.Allocate(400000)
			astTestMust(t, err)
		}
		astTestMust(t, f.m.Close())
		astTestMust(t, numbers.Close())
		astTestMust(t, symbols.Close())
		if err := f.runtime.Deallocate(&f.recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || f.recovery != (memory.Block{})) {
			t.Fatal("recovery allocation cleanup", err)
		}
		astTestMust(t, f.runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("deletion continuation leaked storage", budget.Used())
		}
	})
	return f
}

func (f *deletionContinuationFixture) nearMinimum() {
	f.t.Helper()
	first, err := f.runtime.Allocate(8)
	astTestMust(f.t, err)
	second, err := f.runtime.Allocate(8)
	astTestMust(f.t, err)
	delta := int64(-1000000)
	if err = f.runtime.Reallocate(&first, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		f.t.Fatal("first native accounting boundary", err)
	}
	astTestMust(f.t, f.runtime.Deallocate(&first))
	delta = math.MinInt64 + 1000 - f.runtime.State().GlobalSize + 16
	if err = f.runtime.Reallocate(&second, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		f.t.Fatal("second native accounting boundary", err)
	}
	astTestMust(f.t, f.runtime.Deallocate(&second))
	if state := f.runtime.State(); state.GlobalSize != math.MinInt64+1000 || state.LocalSize != 0 {
		f.t.Fatal("native accounting fixture", state)
	}
}

func deletionContinuationCounts(m *ASTManager) [4]uint32 {
	return [4]uint32{m.liveNodes, m.liveInfos, m.liveParameters, m.liveReferences}
}

func TestASTDeletionContinuationParameterPrefixAndRemainingLIFO(t *testing.T) {
	for _, resume := range []string{"lock", "close"} {
		t.Run(resume, func(t *testing.T) {
			f := newDeletionContinuationFixture(t)
			m := f.m
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			sentinel, err := m.MkVar(1000, boolean)
			astTestMust(t, err)
			sentinelRef, err := m.NewRef(sentinel)
			astTestMust(t, err)
			sentinelNode, err := m.internal(sentinel)
			astTestMust(t, err)
			before := deletionContinuationCounts(m)

			integer, err := numeral.NewScopedInt(m.numerals.Manager.Z)
			astTestMust(t, err)
			astTestMust(t, integer.SetInt64(1))
			raw, err := integer.Get()
			astTestMust(t, err)
			astTestMust(t, m.numerals.Manager.Z.Mul2kInPlace(raw, 1600000))
			value, err := m.numerals.NewFromScopedInt(integer)
			astTestMust(t, err)
			prefix, err := m.NewParameterInt(17)
			astTestMust(t, err)
			borrow, err := m.NewParameterAST(sentinel)
			astTestMust(t, err)
			wide, err := m.NewParameterRational(&value)
			astTestMust(t, err)
			tail, err := m.NewParameterInt(23)
			astTestMust(t, err)
			parameters := []ASTParameter{prefix, borrow, wide, tail}
			info, err := m.NewFuncDeclInfo(4, 717, parameters, ASTFuncFlags{}, false)
			astTestMust(t, err)
			declaration, err := m.MkFuncDecl(astTestSymbol(t, m, "deletion-continuation"), nil, boolean, info)
			astTestMust(t, err)
			declNode, err := m.internal(declaration)
			astTestMust(t, err)
			declID := m.nodeAt(declNode.slot).id
			ownedInfo, err := m.funcInfo(m.declAt(declNode.slot).info)
			astTestMust(t, err)
			ownedParameters := append([]ASTParameter(nil), ownedInfo.parameters...)
			astTestMust(t, m.DelFuncDeclInfo(&info))
			for i := range parameters {
				astTestMust(t, m.DelParameter(&parameters[i]))
			}
			astTestMust(t, m.numerals.Del(&value))
			astTestMust(t, integer.Close())

			var arguments []ASTHandle
			var siblingIDs []uint32
			for i := uint32(0); i < 40; i++ {
				argument, err := m.MkVar(2000+i, boolean)
				astTestMust(t, err)
				id, err := m.GetASTID(argument)
				astTestMust(t, err)
				arguments = append(arguments, argument)
				siblingIDs = append(siblingIDs, id)
			}
			last, err := m.MkApp(declaration, nil)
			astTestMust(t, err)
			lastID, err := m.GetASTID(last)
			astTestMust(t, err)
			arguments = append(arguments, last)
			root, err := m.MkAndArray(arguments)
			astTestMust(t, err)
			rootID, err := m.GetASTID(root)
			astTestMust(t, err)
			ref, err := m.NewRef(root)
			astTestMust(t, err)
			freeExpr, freeDecl := m.ExprIDs.FreeLen(), m.declIDs.FreeLen()
			if m.nodeAt(sentinelNode.slot).rc != 2 {
				t.Fatal("fixture missed info-owned AST parameter")
			}
			f.nearMinimum()
			blocks := f.runtime.Stats().LiveBlocks
			cause := m.DelRef(&ref)
			if !errors.Is(cause, arena.ErrBounds) || ref != (ASTRef{}) || f.runtime.Stats().LiveBlocks >= blocks {
				t.Fatal("missed consumed reference and physical native deallocation", cause, ref)
			}
			if m.nodeAt(sentinelNode.slot).rc != 1 || m.parameterAt(ownedParameters[0].slot).Live || m.parameterAt(ownedParameters[1].slot).Live {
				t.Fatal("deletion did not consume the AST callback and scalar parameter prefix exactly once")
			}
			if !m.parameterAt(ownedParameters[2].slot).Live || !m.parameterAt(ownedParameters[3].slot).Live || !m.nodeAt(declNode.slot).live {
				t.Fatal("native refusal lost unfinished info or declaration")
			}
			f.recovery, err = f.runtime.Allocate(400000)
			astTestMust(t, err)
			if resume == "close" {
				astTestMust(t, m.Close())
				astTestMust(t, m.Close())
				return
			}
			count, err := m.GetRefCount(sentinel)
			astTestMust(t, err)
			if count != 1 {
				t.Fatal("resume repeated the completed AST parameter callback", count)
			}
			if after := deletionContinuationCounts(m); after != before {
				t.Fatal("ordinary manager operation did not reclaim the complete stopped graph", before, after)
			}
			want := []uint32{rootID, lastID}
			for i := len(siblingIDs) - 1; i >= 0; i-- {
				want = append(want, siblingIDs[i])
			}
			if got := m.ExprIDs.FreeIDs()[freeExpr:]; !reflect.DeepEqual(got, want) {
				t.Fatal("resumed deletion changed ascending-push/LIFO-pop recycling", got, want)
			}
			declRecycles := 0
			for _, id := range m.declIDs.FreeIDs()[freeDecl:] {
				if id == declID {
					declRecycles++
				}
			}
			if declRecycles != 1 {
				t.Fatal("resume repeated or omitted declaration ID recycling", declRecycles)
			}
			astTestMust(t, m.DelRef(&sentinelRef))
		})
	}
}
