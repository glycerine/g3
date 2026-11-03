package tab

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type tautologyScopeRefusalSolver struct {
	solver.Solver
	pushFailure, popFailure error
	pushes, pops            uint32
}

func (s *tautologyScopeRefusalSolver) Push() error {
	s.pushes++
	if err := s.Solver.Push(); err != nil {
		return err
	}
	if s.pushFailure != nil {
		err := s.pushFailure
		s.pushFailure = nil
		return err
	}
	return nil
}

func (s *tautologyScopeRefusalSolver) Pop(count uint32) error {
	s.pops++
	if s.popFailure != nil {
		err := s.popFailure
		s.popFailure = nil
		return err
	}
	return s.Solver.Pop(count)
}

func TestTautologyCheckSuppressesResultOnNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var attempt *tautologyCheckAttempt
	var result bool
	var failure error
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && failure == nil {
			result, failure = finishTautologyCheck(attempt, true, nil)
		}
		return source
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
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
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	formula, err := manager.NewRef(truth)
	must(err)
	attempt = &tautologyCheckAttempt{manager: manager, formula: formula}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || result || !errors.Is(failure, ast.ErrASTPending) || attempt.formula != formula {
		t.Fatal("tautology cleanup published result or lost formula", body, result, failure)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("tautology cleanup failure is not retryable")
	}
	original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if attempt.formula != (ast.ASTRef{}) || cleanup.CleanupCause() != nil {
		t.Fatal("tautology cleanup retry incomplete")
	}
	if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message {
		t.Fatal("tautology cleanup changed receipt")
	}
}

func TestTautologyCheckOwnsPartiallyPublishedSolverScopes(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	manager, err := ctx.Manager()
	must(err)
	globals, err := ctx.GlobalParameters()
	must(err)
	rw, err := rewriter.NewTheoryRewriter(manager, globals.Context, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	native, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(native.Close()) }()
	truth, err := manager.MkTrue()
	must(err)
	clause, err := NewClause(manager, rw, truth, nil, truth)
	must(err)
	defer func() { must(clause.Close()) }()

	pushRefused := errors.New("tautology push refused after publishing scope")
	partial := &tautologyScopeRefusalSolver{Solver: native, pushFailure: pushRefused}
	result, err := (&tabSearch{manager: manager, querySolver: partial}).isTautology(clause)
	if result || !errors.Is(err, pushRefused) {
		t.Fatal("partial push changed its result or cause", result, err)
	}
	level, err := native.ScopeLevel()
	must(err)
	if level != 0 || partial.pushes != 1 || partial.pops != 1 {
		t.Fatal("partial push scope was not reconciled", level, partial.pushes, partial.pops)
	}

	popRefused := errors.New("tautology pop refused")
	blocked := &tautologyScopeRefusalSolver{Solver: native, pushFailure: pushRefused, popFailure: popRefused}
	result, failure := (&tabSearch{manager: manager, querySolver: blocked}).isTautology(clause)
	if result || !errors.Is(failure, pushRefused) || !errors.Is(failure, popRefused) {
		t.Fatal("partial push cleanup did not preserve both failures", result, failure)
	}
	var receipt *tautologyCheckFailure
	if !errors.As(failure, &receipt) || receipt.attempt.formula == (ast.ASTRef{}) || !receipt.attempt.pushed {
		t.Fatal("partial push cleanup lost its scope or formula owner")
	}
	level, err = native.ScopeLevel()
	must(err)
	if level != 1 || blocked.pushes != 1 || blocked.pops != 1 {
		t.Fatal("refused partial-push cleanup surrendered its frontier", level, blocked.pushes, blocked.pops)
	}
	original, exposed, message := receipt.OriginalCause(), errors.Unwrap(receipt), receipt.Error()
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	level, err = native.ScopeLevel()
	must(err)
	if level != 0 || blocked.pops != 2 || receipt.attempt.formula != (ast.ASTRef{}) || receipt.attempt.pushed || receipt.CleanupCause() != nil {
		t.Fatal("partial-push cleanup retry did not finish", level, blocked.pops, receipt.attempt.formula, receipt.attempt.pushed, receipt.CleanupCause())
	}
	if receipt.OriginalCause() != original || errors.Unwrap(receipt) != exposed || receipt.Error() != message {
		t.Fatal("partial-push cleanup retry changed its receipt")
	}

	blocked = &tautologyScopeRefusalSolver{Solver: native, popFailure: popRefused}
	result, failure = (&tabSearch{manager: manager, querySolver: blocked}).isTautology(clause)
	if result || !errors.Is(failure, popRefused) {
		t.Fatal("pop refusal published a successful tautology result", result, failure)
	}
	receipt = nil
	if !errors.As(failure, &receipt) || receipt.attempt.formula == (ast.ASTRef{}) || !receipt.attempt.pushed {
		t.Fatal("pop refusal lost its cleanup frontier")
	}
	level, err = native.ScopeLevel()
	must(err)
	if level != 1 || blocked.pushes != 1 || blocked.pops != 1 {
		t.Fatal("pop refusal released solver scope early", level, blocked.pushes, blocked.pops)
	}
	must(receipt.Cleanup())
	level, err = native.ScopeLevel()
	must(err)
	if level != 0 || blocked.pops != 2 || receipt.attempt.formula != (ast.ASTRef{}) || receipt.attempt.pushed {
		t.Fatal("pop-refusal retry did not release exactly one scope", level, blocked.pops, receipt.attempt.formula, receipt.attempt.pushed)
	}
}
