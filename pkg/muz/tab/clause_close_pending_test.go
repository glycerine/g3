package tab

// Source: src/muz/tab/tab_context.cpp clause::dec_ref ownership. Clause AST
// owners remain attached when the native manager refuses reentrant disposal.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestClauseClosePreservesASTOwnersOnNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var clause *Clause
	var originalHead, originalConstraint ast.ASTRef
	var originalPredicates []ast.ASTRef
	active, callbacks, intact := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := clause.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !clause.closed && clause.head == originalHead && clause.constraint == originalConstraint && len(clause.predicates) == len(originalPredicates)
				for j := range clause.predicates {
					intact = intact && clause.predicates[j] == originalPredicates[j]
				}
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := symbols.Intern("tab-clause-pending")
	must(err)
	declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	application, err := m.MkApp(declaration, nil)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	originalHead, err = m.NewRef(application)
	must(err)
	predicate, err := m.NewRef(application)
	must(err)
	originalPredicates = []ast.ASTRef{predicate}
	originalConstraint, err = m.NewRef(truth)
	must(err)
	clause = &Clause{manager: m, head: originalHead, predicates: append([]ast.ASTRef(nil), originalPredicates...), constraint: originalConstraint}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("refused clause close lost AST owners", body, callbacks, intact)
	}
	for _, ref := range append([]ast.ASTRef{clause.head, clause.constraint}, clause.predicates...) {
		h, e := m.RefGet(ref)
		must(e)
		_, e = m.GetKind(h)
		must(e)
	}
	must(clause.Close())
	must(clause.Close())
	if !clause.closed || clause.head != (ast.ASTRef{}) || clause.constraint != (ast.ASTRef{}) {
		t.Fatal("clause cleanup retry incomplete")
	}
	for _, ref := range clause.predicates {
		if ref != (ast.ASTRef{}) {
			t.Fatal("clause cleanup kept predicate owner")
		}
	}
}
