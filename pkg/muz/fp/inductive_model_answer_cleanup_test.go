package fp

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source pdr_manager.cpp:113-118 and spacer_manager.cpp:119-125 keep a
// model_ref across model2expr. Predicate declarations remain independently
// owned by the query context; only the model's numeric Else expressions are
// exclusive here. Real TLS accounting makes destruction consume one numeric
// payload before reporting an error, leaving the second model entry pending.
func TestInductiveModelAnswerPreservesConsumedNativeModelCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	var md *model.Model
	var pending memory.ConstructionFailure
	engine := &PropositionalEngine{manager: manager}
	var padding []ast.ASTRef
	var recovery memory.Block
	t.Cleanup(func() {
		finish := func(close func() error) {
			t.Helper()
			for i := 0; i < 128; i++ {
				before := runtime.Stats().LiveBlocks
				err := close()
				if err == nil {
					return
				}
				if !errors.Is(err, arena.ErrBounds) || runtime.Stats().LiveBlocks >= before {
					t.Fatal("cleanup made no physical progress", err, runtime.State())
				}
			}
			t.Fatal("cleanup retry limit")
		}
		if pending != nil {
			finish(pending.Cleanup)
		}
		for i := range padding {
			must(manager.DelRef(&padding[i]))
		}
		finish(manager.Close)
		finish(numbers.Close)
		finish(symbols.Close)
		if recovery != (memory.Block{}) {
			if err := runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
				t.Fatal("recovery storage cleanup", err)
			}
		}
		finish(runtime.Close)
		if budget.Used() != 0 {
			t.Fatal("inductive model answer cleanup leaked budget", budget.Used())
		}
	})
	boolean, err := manager.MkBoolSort()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	family, err := manager.RegisterArithmeticPlugin()
	must(err)
	integerSort, err := manager.MkFamilySort(family, 1, nil)
	must(err)
	var declarations []ast.ASTHandle
	for i := 0; i < 2; i++ {
		name, err := symbols.Intern(fmt.Sprintf("inductive-model-P%d", i))
		must(err)
		declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{integerSort}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		pin, err := manager.NewRef(declaration)
		must(err)
		padding = append(padding, pin)
		declarations = append(declarations, declaration)
	}
	makeModel := func() (*model.Model, []ast.ASTHandle, []ast.ASTHandle) {
		result, err := model.NewModel(manager)
		must(err)
		variable, err := manager.MkVar(0, integerSort)
		must(err)
		var numerals, bodies []ast.ASTHandle
		for i, declaration := range declarations {
			scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
			must(err)
			integer, err := scoped.Get()
			must(err)
			digits := make([]uint32, 32768)
			digits[0], digits[len(digits)-1] = uint32(17+i), 1
			must(numbers.Manager.Z.SetLimbs(integer, digits))
			value, err := numbers.NewFromScopedInt(scoped)
			must(err)
			number, err := manager.ArithmeticNumeral(&value, true)
			must(err)
			body, err := manager.MkEq(variable, number)
			must(err)
			fi, err := model.NewFunctionInterpretation(manager, 1)
			must(err)
			must(fi.SetElse(body))
			must(result.Core().RegisterFunction(declaration, fi))
			must(numbers.Del(&value))
			must(scoped.Close())
			numerals = append(numerals, number)
			bodies = append(bodies, body)
			if rc, err := manager.GetRefCount(declaration); err != nil || rc != 2 {
				t.Fatal("predicate lacks external and model owners", rc, err)
			}
			if rc, err := manager.GetRefCount(body); err != nil || rc != 1 {
				t.Fatal("Else is not exclusively model-owned", rc, err)
			}
			if rc, err := manager.GetRefCount(number); err != nil || rc != 1 {
				t.Fatal("numeric payload is not exclusively Else-owned", rc, err)
			}
		}
		return result, numerals, bodies
	}
	var numerals []ast.ASTHandle
	md, numerals, _ = makeModel()

	firstBlock, err := runtime.Allocate(8)
	must(err)
	secondBlock, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&firstBlock, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first signed accounting adjustment", err)
	}
	must(runtime.Deallocate(&firstBlock))
	state := runtime.State()
	delta = math.MinInt64 + 1000 - state.GlobalSize + 16
	if err = runtime.Reallocate(&secondBlock, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second signed accounting adjustment", err)
	}
	must(runtime.Deallocate(&secondBlock))
	if state = runtime.State(); state.GlobalSize != math.MinInt64+1000 || state.LocalSize != 0 {
		t.Fatal("failed real TLS accounting setup", state)
	}
	// Fill reference pages using actual caller-owned roots, never internal state.
	for manager.Stats().LiveReferences%64 != 0 {
		ref, err := manager.NewRef(truth)
		must(err)
		padding = append(padding, ref)
	}
	before, blocks := manager.Stats(), runtime.Stats().LiveBlocks
	budget.FailAfter(0)
	out, cause := engine.inductiveModelAnswer(md)
	budget.FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) || !errors.Is(cause, arena.ErrBounds) || out != (ast.ASTRef{}) {
		t.Fatal("missed allocation and consumed cleanup failure", cause, out)
	}
	if !errors.As(cause, &pending) {
		t.Fatal("failed model conversion lost its cleanup receipt", cause)
	}
	receipt, ok := pending.(*inductiveModelAnswerCleanupFailure)
	if !ok || receipt.owner.model != md || md.Closed || receipt.owner.builder != nil || receipt.owner.output != (ast.ASTRef{}) {
		t.Fatal("cleanup receipt lost the exact pending model scope", pending)
	}
	if pending.RuntimeOwner() != runtime || !errors.Is(pending.OriginalCause(), arena.ErrInjectedFailure) || !errors.Is(pending.CleanupCause(), arena.ErrBounds) {
		t.Fatal("receipt changed failure causes or native runtime", pending.OriginalCause(), pending.CleanupCause())
	}
	original, message := pending.OriginalCause(), cause.Error()
	if after := manager.Stats(); after.LiveReferences != before.LiveReferences || runtime.Stats().LiveBlocks+1 != blocks {
		t.Fatal("failure did not consume exactly one model-only native payload", before.LiveReferences, after.LiveReferences, blocks, runtime.Stats().LiveBlocks)
	}
	alive, consumed := 0, 0
	for _, n := range numerals {
		rc, err := manager.GetRefCount(n)
		if err == nil && rc == 1 {
			alive++
		} else if errors.Is(err, ast.ErrASTHandle) || err == nil && rc == 0 {
			consumed++
		} else {
			t.Fatal("unexpected numeral frontier", rc, err)
		}
	}
	if alive != 1 || consumed != 1 {
		t.Fatal("model lost its unvisited numeric Else payload", alive, consumed)
	}
	for _, d := range declarations {
		if rc, err := manager.GetRefCount(d); err != nil || rc < 1 {
			t.Fatal("external predicate owner lost", rc, err)
		}
	}
	recovery, err = runtime.Allocate(400000)
	must(err)
	blocks = runtime.Stats().LiveBlocks
	must(pending.Cleanup())
	if !md.Closed || receipt.owner.model != nil || pending.CleanupCause() != nil || runtime.Stats().LiveBlocks+1 != blocks {
		t.Fatal("receipt did not drain exactly the remaining model payload", md.Closed, receipt.owner.model, pending.CleanupCause(), blocks, runtime.Stats().LiveBlocks)
	}
	for _, d := range declarations {
		if rc, err := manager.GetRefCount(d); err != nil || rc != 1 {
			t.Fatal("model cleanup damaged external predicate ownership", rc, err)
		}
	}
	for _, n := range numerals {
		if rc, err := manager.GetRefCount(n); err == nil && rc != 0 {
			t.Fatal("remaining numeric payload not drained", rc)
		}
	}
	state, blocks = runtime.State(), runtime.Stats().LiveBlocks
	must(pending.Cleanup())
	must(pending.Cleanup())
	if pending.OriginalCause() != original || cause.Error() != message || !errors.Is(cause, arena.ErrInjectedFailure) || !errors.Is(cause, arena.ErrBounds) || runtime.State() != state || runtime.Stats().LiveBlocks != blocks || manager.Stats().LiveReferences != before.LiveReferences {
		t.Fatal("cleanup retry changed error identity or replayed native destruction")
	}
	// Retry the same numeric interpretation shapes; the new output owns the
	// formula after the consumed model has been completely closed.
	var bodies []ast.ASTHandle
	md, numerals, bodies = makeModel()
	out, err = engine.inductiveModelAnswer(md)
	if err != nil {
		var failed memory.ConstructionFailure
		if errors.As(err, &failed) {
			pending = failed
		}
		t.Fatal("model answer retry", err)
	}
	t.Cleanup(func() {
		if out != (ast.ASTRef{}) {
			must(manager.DelRef(&out))
		}
	})
	if !md.Closed {
		t.Fatal("successful answer left its input model open")
	}
	formula, err := manager.RefGet(out)
	must(err)
	count, err := manager.GetNumArgs(formula)
	must(err)
	op, err := manager.GetDeclKind(formula)
	must(err)
	if op != ast.ASTOpAnd || count != 2 {
		t.Fatal("retry lost model conjunction", op, count)
	}
	for i, declaration := range declarations {
		quantified, err := manager.GetArg(formula, uint32(i))
		must(err)
		forall, err := manager.IsForall(quantified)
		must(err)
		arity, err := manager.GetNumDeclarations(quantified)
		must(err)
		equality, err := manager.GetQuantifierBody(quantified)
		must(err)
		lhs, err := manager.GetArg(equality, 0)
		must(err)
		rhs, err := manager.GetArg(equality, 1)
		must(err)
		actual, err := manager.GetDecl(lhs)
		must(err)
		if !forall || arity != 1 || actual != declaration || rhs != bodies[i] {
			t.Fatal("retry changed quantified predicate interpretation", i, forall, arity, actual, rhs, bodies[i])
		}
	}
	must(manager.DelRef(&out))
	if manager.Stats().LiveReferences != before.LiveReferences {
		t.Fatal("successful retry leaked output references")
	}
	for _, d := range declarations {
		if rc, err := manager.GetRefCount(d); err != nil || rc != 1 {
			t.Fatal("retry damaged caller predicate ownership", rc, err)
		}
	}
}
