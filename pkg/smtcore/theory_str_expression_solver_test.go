package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
)

func stringExpressionSolverFixture(t *testing.T) (*ast.ASTManager, *SMTKernel, *SMTStringTheory) {
	t.Helper()
	m, config := smtModelGeneratorFixture(t)
	key := "random_seed"
	astTestMust(t, config.Context.SetUIntName(config.Params, &key, 81))
	parent := smtTestKernel(t, m)
	if err := parent.ConfigurePreprocessing(config.Context, config.Globals, config.Params); err != nil {
		t.Fatal("parent configuration", err)
	}
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTStringTheory(family, &parent.smtParams.str)
	if err != nil {
		t.Fatal("String construction", err)
	}
	if err := parent.internalizer.RegisterTheory(theory); err != nil {
		t.Fatal("String initialization", err)
	}
	// theory_str::init runs at setup_context.
	if err := theory.Setup(); err != nil {
		t.Fatal("String setup", err)
	}
	if theory.regexExpressionSolver == nil {
		t.Fatal("configured theory initialization omitted its nested solver")
	}
	child := theory.regexExpressionSolver.kernel
	if child == parent || child.manager != m || child.smtParams != parent.smtParams || !child.smtParamsBorrowed || child.smtState.RandomSeed != 81 {
		t.Fatal("nested kernel did not borrow the actual parent configuration")
	}
	return m, parent, theory
}

func TestSMTStringExpressionSolverScopedQueriesAndOwnership(t *testing.T) {
	m, parent, theory := stringExpressionSolverFixture(t)
	solver := theory.regexExpressionSolver
	a := smtRelevancyBool(t, m, "regex-transition-a")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestPin(t, m, notA)
	contradiction, err := m.MkAnd(a, notA)
	astTestMust(t, err)
	astTestPin(t, m, contradiction)
	trueValue, err := m.MkTrue()
	astTestMust(t, err)
	falseValue, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, parent.Assert(a, ast.ASTHandle{}))
	for _, query := range []struct {
		expression ast.ASTHandle
		want       lbool.LBool
	}{
		{a, lbool.True}, {notA, lbool.True}, {contradiction, lbool.False},
		{falseValue, lbool.False}, {trueValue, lbool.True}, {notA, lbool.True},
	} {
		got, err := solver.CheckSatisfiable(query.expression)
		astTestMust(t, err)
		if got != query.want {
			t.Fatal("nested query result changed or scopes leaked", got, query.want)
		}
		if len(solver.kernel.scopes) != 0 || len(solver.kernel.assertions) != 0 {
			t.Fatal("query left an assertion scope behind")
		}
	}
	if len(parent.assertions) != 1 || len(parent.scopes) != 0 {
		t.Fatal("nested query changed parent assertions")
	}
	child := solver.kernel
	astTestMust(t, solver.Close())
	astTestMust(t, solver.Close())
	if !child.closed {
		t.Fatal("nested solver close did not retire its kernel")
	}
	if _, err := parent.smtParams.State(); err != nil {
		t.Fatal("nested close retired borrowed parent parameters", err)
	}
	if _, err := solver.CheckSatisfiable(a); err == nil {
		t.Fatal("closed solver accepted a query")
	}
	got, err := parent.Check()
	astTestMust(t, err)
	if got != lbool.True {
		t.Fatal("Boolean-only parent with no free String variables did not complete", got)
	}
}

func TestSMTStringExpressionSolverErrorsAndCancellationRestoreScope(t *testing.T) {
	m, _, theory := stringExpressionSolverFixture(t)
	solver := theory.regexExpressionSolver
	trueValue, err := m.MkTrue()
	astTestMust(t, err)
	number, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	astTestPin(t, m, number)
	if got, err := solver.CheckSatisfiable(number); err == nil || got != lbool.Undef {
		t.Fatal("nonboolean query was not refused", got, err)
	}
	if len(solver.kernel.scopes) != 0 || len(solver.kernel.assertions) != 0 {
		t.Fatal("assertion error leaked a query scope")
	}
	m.Limit.Cancel()
	got, err := solver.CheckSatisfiable(trueValue)
	m.Limit.ResetCancel()
	if err == nil || err.Error() != messages.Canceled || got != lbool.Undef {
		t.Fatal("canceled query did not propagate cancellation", got, err)
	}
	if len(solver.kernel.scopes) != 0 || len(solver.kernel.assertions) != 0 {
		t.Fatal("unknown query leaked a scope")
	}
	got, err = solver.CheckSatisfiable(trueValue)
	astTestMust(t, err)
	if got != lbool.True {
		t.Fatal("query failed after unknown/error cleanup", got)
	}
}

func TestSMTStringExpressionSolverUnknownRestoresScope(t *testing.T) {
	m, parent, theory := stringExpressionSolverFixture(t)
	solver := theory.regexExpressionSolver
	params, err := parent.paramContext.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parent.paramContext.Del(&params)) })
	key := "max_conflicts"
	astTestMust(t, parent.paramContext.SetUIntName(&params, &key, 0))
	preprocessKey := "preprocess"
	astTestMust(t, parent.paramContext.SetBoolName(&params, &preprocessKey, false))
	astTestMust(t, parent.UpdateParams(&params))
	// Keep the conflict in the actual Boolean search instead of allowing the
	// asserted-formula preprocessing pass to discharge this small CNF.
	a, b := smtRelevancyBool(t, m, "regex-limit-a"), smtRelevancyBool(t, m, "regex-limit-b")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestPin(t, m, notA)
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	astTestPin(t, m, notB)
	var clauses []ast.ASTHandle
	for _, pair := range [][2]ast.ASTHandle{{a, b}, {notA, b}, {a, notB}, {notA, notB}} {
		clause, err := m.MkOr(pair[0], pair[1])
		astTestMust(t, err)
		astTestPin(t, m, clause)
		clauses = append(clauses, clause)
	}
	formula, err := m.MkAndArgs(clauses)
	astTestMust(t, err)
	astTestPin(t, m, formula)
	got, err := solver.CheckSatisfiable(formula)
	astTestMust(t, err)
	if got != lbool.Undef || solver.kernel.lastFailure != SMTFailureConflictLimit {
		t.Fatal("nested solver lost unknown result", got, solver.kernel.lastFailure)
	}
	if len(solver.kernel.scopes) != 0 || len(solver.kernel.assertions) != 0 {
		t.Fatal("unknown query leaked a scope")
	}
	astTestMust(t, parent.paramContext.SetUIntName(&params, &key, ^uint32(0)))
	astTestMust(t, parent.UpdateParams(&params))
	got, err = solver.CheckSatisfiable(formula)
	astTestMust(t, err)
	if got != lbool.False {
		t.Fatal("nested solver failed after an unknown query", got)
	}
}

func TestSMTStringExpressionSolverBitVectorPredicates(t *testing.T) {
	m, _, theory := stringExpressionSolverFixture(t)
	solver := theory.regexExpressionSolver
	numeral, err := m.BitVectorNumeralUint64(255, 8)
	astTestMust(t, err)
	astTestPin(t, m, numeral)
	sort, err := m.GetSort(numeral)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "regex-byte-predicate"), sort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	zero, err := m.BitVectorNumeralUint64(0, 8)
	astTestMust(t, err)
	astTestPin(t, m, zero)
	eqZero, err := m.MkEq(x, zero)
	astTestMust(t, err)
	astTestPin(t, m, eqZero)
	eqMax, err := m.MkEq(x, numeral)
	astTestMust(t, err)
	astTestPin(t, m, eqMax)
	both, err := m.MkAnd(eqZero, eqMax)
	astTestMust(t, err)
	astTestPin(t, m, both)
	for _, query := range []struct {
		expression ast.ASTHandle
		want       lbool.LBool
	}{{eqZero, lbool.True}, {both, lbool.False}, {eqMax, lbool.True}} {
		got, err := solver.CheckSatisfiable(query.expression)
		astTestMust(t, err)
		if got != query.want {
			t.Fatal("symbolic byte predicate query failed", got, query.want)
		}
	}
}

func TestSMTStringExpressionSolverAutomatonConsumer(t *testing.T) {
	m, _, theory := stringExpressionSolverFixture(t)
	pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(t, err)
		astTestPin(t, m, value)
		return value
	}
	numeral := pin(m.BitVectorNumeralUint64(0, 8))
	sort, err := m.GetSort(numeral)
	astTestMust(t, err)
	character := pin(m.MkConst(astTestSymbol(t, m, "regex-symbolic-character"), sort))
	symbolic := pin(m.MkSequenceToRegex(pin(m.MkSequenceUnit(character))))
	complement := pin(m.MkRegexComplement(symbolic))
	// Symbolic unit labels force the actual solver hook, while literal byte
	// labels and their complements can use the concrete byte-set fast path.
	for _, test := range []struct {
		regex       ast.ASTHandle
		acceptsByte bool
	}{
		{pin(m.MkRegexIntersect(symbolic, complement)), false},
		{pin(m.MkRegexIntersect(stringRegexLiteral(t, m, "b"), complement)), true},
	} {
		before := theory.regexExpressionSolver.kernel.Statistics().NumChecks
		owner, err := rewriter.NewStringRegexAutomaton(m, test.regex, theory.regexExpressionSolver)
		astTestMust(t, err)
		graph, err := owner.Graph()
		astTestMust(t, err)
		accepts := false
		for _, state := range graph.EpsilonClosure(graph.Initial()) {
			for _, move := range graph.ExpandedMovesFrom(state, true) {
				accepts = accepts || graph.IsFinal(move.Destination)
			}
		}
		astTestMust(t, owner.Close())
		if accepts != test.acceptsByte {
			t.Fatal("solver-backed product language changed", accepts, test.acceptsByte)
		}
		if theory.regexExpressionSolver.kernel.Statistics().NumChecks <= before {
			t.Fatal("automaton product bypassed the concrete expression solver")
		}
	}
}

func TestSMTStringExpressionSolverConstructionFailureKeepsParent(t *testing.T) {
	m, parent, _ := stringExpressionSolverFixture(t)
	success := false
	for stop := int64(0); stop < 8192 && !success; stop++ {
		m.Budget().FailAfter(stop)
		solver, err := newSMTStringExpressionSolver(m, parent.smtParams)
		m.Budget().FailAfter(-1)
		if solver != nil {
			astTestMust(t, solver.Close())
		}
		if _, stateErr := parent.smtParams.State(); stateErr != nil {
			t.Fatal("failed child construction closed parent parameters", stop, stateErr)
		}
		if err == nil {
			success = true
			break
		}
		if !errors.Is(err, arena.ErrInjectedFailure) {
			var memoryFailure umemory.Failure
			if !errors.As(err, &memoryFailure) || memoryFailure.Kind != umemory.FailureMemout {
				t.Fatal("unexpected construction failure", stop, err)
			}
		}
	}
	if !success {
		t.Fatal("no successful nested solver construction frontier")
	}
}

func TestSMTStringExpressionSolverTheoryCloseAndConstructorGuards(t *testing.T) {
	m, parent, theory := stringExpressionSolverFixture(t)
	child := theory.regexExpressionSolver.kernel
	if _, err := newSMTStringExpressionSolver(nil, parent.smtParams); err == nil {
		t.Fatal("nil manager accepted")
	}
	if _, err := newSMTStringExpressionSolver(m, nil); err == nil {
		t.Fatal("missing parent parameters accepted")
	}
	if _, err := newSMTStringExpressionSolver(m, &SMTParams{}); err == nil {
		t.Fatal("uninitialized parameters accepted")
	}
	other := astTestManager(t)
	if _, err := newSMTStringExpressionSolver(other, parent.smtParams); err == nil {
		t.Fatal("foreign parameter services accepted")
	}
	astTestMust(t, theory.Close())
	if !child.closed || theory.regexExpressionSolver != nil {
		t.Fatal("String close did not retire its nested solver")
	}
	if _, err := parent.smtParams.State(); err != nil {
		t.Fatal("String close retired parent parameters", err)
	}
	astTestMust(t, theory.Close())
}
