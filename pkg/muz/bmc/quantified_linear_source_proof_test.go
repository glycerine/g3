package bmc

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// qlinear::get_model uses the original formula as the first proof conclusion,
// but carries the instantiated rule forward to the next resolution.
func TestQuantifiedLinearFirstProofUsesSourceConclusion(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 26)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("leaked budget", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("qlinear-source-proof")
	must(err)
	decl, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	rm, err := base.NewRuleManager(manager)
	must(err)
	defer func() { must(rm.Close()) }()
	must(rm.RegisterPredicate(decl))
	x, err := manager.MkVar(0, boolean)
	must(err)
	head, err := manager.MkApp(decl, []ast.ASTHandle{x})
	must(err)
	rule, err := rm.NewRule(head, nil, nil, "source-rule")
	must(err)
	defer func() { must(rule.Close()) }()
	original, err := rule.ToFormula()
	must(err)
	defer func() { must(manager.DelRef(&original)) }()
	truth, err := manager.MkTrue()
	must(err)
	binding, err := manager.NewRef(truth)
	must(err)
	compiler, err := NewCompiler(manager)
	must(err)
	defer func() { must(compiler.Close()) }()
	beforeMode, err := manager.ProofMode()
	must(err)
	next, proof, err := compiler.linearProofStepWithBindings(rm, rule, []ast.ASTRef{binding}, nil, ast.ASTRef{}, nil, ast.ASTProofPosition{First: 0, Second: 1}, true)
	must(err)
	afterMode, err := manager.ProofMode()
	must(err)
	if beforeMode != afterMode {
		t.Fatal("proof mode not restored", beforeMode, afterMode)
	}
	defer func() { must(next.Close()); must(manager.DelRef(&proof)) }()
	proofHandle, err := manager.RefGet(proof)
	must(err)
	fact, err := manager.GetProofFact(proofHandle)
	must(err)
	expected, err := manager.RefGet(original)
	must(err)
	if fact != expected {
		t.Fatal("first conclusion does not match source original rule", fact, expected)
	}
	instantiated, err := next.ToFormula()
	must(err)
	defer func() { must(manager.DelRef(&instantiated)) }()
	actual, err := manager.RefGet(instantiated)
	must(err)
	if actual == expected {
		t.Fatal("source continuation failed to carry instantiated rule")
	}
}
