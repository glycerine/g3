package duality

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Term trees retain their main term and additional interpolation constraints.
func TestTermTreeCloseAfterConsumedRelease(t *testing.T) {
	for _, extra := range []bool{false, true} {
		name := "main"
		if extra {
			name = "extra"
		}
		t.Run(name, func(t *testing.T) { termTreeConsumedRelease(t, extra) })
	}
}

func termTreeConsumedRelease(t *testing.T, extra bool) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 29)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	config, err := NewConfig(params, nil)
	must(err)
	context, err := NewContext(manager, config)
	must(err)
	scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
	must(err)
	integer, err := scoped.Get()
	must(err)
	words := make([]uint32, 262144)
	words[0], words[len(words)-1] = 17, 1
	must(numbers.Manager.Z.SetLimbs(integer, words))
	value, err := numbers.NewFromScopedInt(scoped)
	must(err)
	parameter, err := manager.NewParameterRational(&value)
	must(err)
	info, err := manager.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	label, err := symbols.Intern("prepared-background-consumed")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	expression, err := NewExpr(context, formula)
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	previous, err := NewExpr(context, truth)
	must(err)
	var tree *TermTree
	if extra {
		tree, err = NewTermTree(previous, nil)
		must(err)
		must(tree.AddTerm(expression))
	} else {
		tree, err = NewTermTree(expression, nil)
		must(err)
	}
	retained, err := NewTermTree(previous, nil)
	must(err)
	parent, err := NewTermTree(previous, []*TermTree{retained, tree})
	must(err)
	proposed, err := NewTermTree(previous, nil)
	must(err)
	must(expression.Close())
	must(previous.Close())
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	a, err := runtime.Allocate(8)
	must(err)
	b, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first boundary", err)
	}
	must(runtime.Deallocate(&a))
	delta = math.MinInt64 + 262144 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second boundary", err)
	}
	must(runtime.Deallocate(&b))
	blocks := runtime.Stats().LiveBlocks
	failure := parent.Close()
	if !errors.Is(failure, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("term cleanup did not fail after native consumption", failure)
	}
	if _, err := tree.Term(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("partially closed term tree accepted use", err)
	}
	if extra && (len(tree.terms) != 0 || tree.term == nil || tree.term.Closed()) {
		t.Fatal("extra-term cleanup did not retain the live main term")
	}
	if !extra && tree.term != nil {
		t.Fatal("main-term cleanup retained a consumed reference")
	}
	parent.SetChildren([]*TermTree{proposed})
	if len(parent.children) != 2 || parent.children[0] != retained || parent.children[1] != tree || retained.closed || proposed.closed {
		t.Fatal("mutation discarded children from a partially closed tree")
	}

	recovery, err := runtime.Allocate(4 << 20)
	must(err)
	must(parent.Close())
	must(parent.Close())
	if !retained.closed || !tree.closed {
		t.Fatal("cleanup retry did not close the original children")
	}
	must(proposed.Close())
	must(context.Close())
	must(config.Close())
	must(params.Close())
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	if err = runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
		t.Fatal(err)
	}
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("leak", budget.Used())
	}
}
