package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func smtTestKernel(t *testing.T, manager *ast.ASTManager) *SMTKernel {
	t.Helper()
	kernel, err := NewSMTKernel(manager, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, kernel.Close()) })
	return kernel
}

func TestSMTKernelDisplayStatisticsResetAndCancellation(t *testing.T) {
	m := astTestManager(t)
	kernel := smtTestKernel(t, m)
	a := smtRelevancyBool(t, m, "kernel-surface-a")
	b := smtRelevancyBool(t, m, "kernel-surface-b")
	astTestMust(t, kernel.Assert(a, ast.ASTHandle{}))
	astTestMust(t, kernel.Assert(b, ast.ASTHandle{}))

	var out strings.Builder
	astTestMust(t, kernel.Display(&out))
	if got := out.String(); got != "(kernel\n  kernel-surface-a\n  kernel-surface-b)" {
		t.Fatal("kernel display", got)
	}
	if err := kernel.Display(nil); err != ast.ErrASTHandle {
		t.Fatal("nil display writer", err)
	}

	status, err := kernel.Check()
	astTestMust(t, err)
	if status != lbool.True || kernel.Statistics().NumChecks != 1 {
		t.Fatal("statistics before reset", status, kernel.Statistics())
	}
	beforeReset := kernel.Statistics()
	astTestMust(t, kernel.ResetStatistics())
	if got := kernel.Statistics(); got != beforeReset {
		t.Fatal("source kernel statistics reset must be a no-op", got, beforeReset)
	}

	canceled, err := kernel.Canceled()
	astTestMust(t, err)
	if canceled {
		t.Fatal("uncanceled kernel reported cancellation")
	}
	m.Limit.Cancel()
	t.Cleanup(m.Limit.ResetCancel)
	canceled, err = kernel.Canceled()
	astTestMust(t, err)
	if !canceled {
		t.Fatal("kernel missed manager cancellation")
	}
}

func TestSMTKernelSearchReportsCancellationAndResourceLimit(t *testing.T) {
	m := astTestManager(t)
	kernel := smtTestKernel(t, m)
	m.Limit.Cancel()
	status, err := kernel.Check()
	astTestMust(t, err)
	if status != lbool.Undef || kernel.lastFailure != SMTFailureCanceled {
		t.Fatal("canceled search", status, kernel.lastFailure)
	}
	reason, err := kernel.ReasonUnknown()
	astTestMust(t, err)
	if reason != "canceled" {
		t.Fatal("canceled reason", reason)
	}
	m.Limit.ResetCancel()

	scope, err := control.NewLimitScope(&m.Limit, 1)
	astTestMust(t, err)
	defer scope.Close()
	status, err = kernel.Check()
	astTestMust(t, err)
	if status != lbool.Undef || kernel.lastFailure != SMTFailureResourceLimit {
		t.Fatal("resource-limited search", status, kernel.lastFailure)
	}
	reason, err = kernel.ReasonUnknown()
	astTestMust(t, err)
	if reason != "(resource limits reached)" {
		t.Fatal("resource-limit reason", reason)
	}
}

func TestSMTKernelSearchReportsTimeout(t *testing.T) {
	m := astTestManager(t)
	kernel := smtTestKernel(t, m)
	kernel.smtState.Timeout = 1
	reads := 0
	kernel.timerClock = func() (uint64, error) {
		reads++
		if reads == 1 {
			return 0, nil
		}
		return 2_000_000, nil
	}
	status, err := kernel.Check()
	astTestMust(t, err)
	if status != lbool.Undef || kernel.lastFailure != SMTFailureTimeout {
		t.Fatal("timed search", status, kernel.lastFailure)
	}
	reason, err := kernel.ReasonUnknown()
	astTestMust(t, err)
	if reason != "timeout" {
		t.Fatal("timeout reason", reason)
	}
}

func TestSMTKernelSearchReportsMemoryWatermark(t *testing.T) {
	m := astTestManager(t)
	kernel := smtTestKernel(t, m)
	runtime := m.NumeralService().RuntimeOwner()
	astTestMust(t, runtime.SetWatermark(1))
	t.Cleanup(func() { astTestMust(t, runtime.SetWatermark(0)) })
	status, err := kernel.Check()
	astTestMust(t, err)
	if status != lbool.Undef || kernel.lastFailure != SMTFailureMemory {
		t.Fatal("memory-limited search", status, kernel.lastFailure)
	}
	reason, err := kernel.ReasonUnknown()
	astTestMust(t, err)
	if reason != "memout" {
		t.Fatal("memory-limit reason", reason)
	}
}

func TestSMTKernelAssertedFormulaDisplayAndInconsistencyProof(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	kernel := smtTestKernel(t, m)
	a := smtRelevancyBool(t, m, "asserted-display-a")
	astTestMust(t, kernel.Assert(a, ast.ASTHandle{}))
	var before strings.Builder
	astTestMust(t, kernel.DisplayAssertedFormulas(&before))
	if got := before.String(); got != "asserted formulas:\n[HEAD] ==>\nasserted-display-a\ninconsistent: 0\n" {
		t.Fatal("pending asserted-formula display", got)
	}
	if proof, err := kernel.AssertionInconsistencyProof(); err != nil || !proof.IsNull() {
		t.Fatal("consistent assertion proof", proof, err)
	}
	if err := kernel.internalizePending(); err != nil {
		t.Fatal(err)
	}
	var committed strings.Builder
	astTestMust(t, kernel.DisplayAssertedFormulas(&committed))
	if strings.Contains(committed.String(), "[HEAD]") {
		t.Fatal("committed display retained head marker", committed.String())
	}

	falsity, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, kernel.Assert(falsity, ast.ASTHandle{}))
	proof, err := kernel.AssertionInconsistencyProof()
	astTestMust(t, err)
	if proof.IsNull() {
		t.Fatal("missing asserted inconsistency proof")
	}
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	if fact != falsity {
		t.Fatal("asserted inconsistency proof fact", fact, falsity)
	}
	var inconsistent strings.Builder
	astTestMust(t, kernel.DisplayAssertedFormulas(&inconsistent))
	if !strings.Contains(inconsistent.String(), "inconsistent: 1") {
		t.Fatal("inconsistent asserted-formula display", inconsistent.String())
	}
}

func TestSMTKernelAssertionCheckAndDecisionClause(t *testing.T) {
	m := astTestManager(t)
	atom := smtRelevancyBool(t, m, "kernel-atom")
	kernel := smtTestKernel(t, m)
	astTestMust(t, kernel.Assert(atom, ast.ASTHandle{}))
	result, err := kernel.Check()
	astTestMust(t, err)
	if result != lbool.True || len(kernel.Assertions()) != 1 {
		t.Fatal("unit assertion check", result, kernel.Assertions())
	}

	a := smtRelevancyBool(t, m, "kernel-or-a")
	b := smtRelevancyBool(t, m, "kernel-or-b")
	disjunction, err := m.MkOr(a, b)
	astTestMust(t, err)
	other := smtTestKernel(t, m)
	astTestMust(t, other.Assert(disjunction, ast.ASTHandle{}))
	result, err = other.Check()
	astTestMust(t, err)
	if result != lbool.True || other.Statistics().NumDecisions == 0 {
		t.Fatal("decision-requiring clause", result, other.Statistics())
	}
	internalizer, err := other.Internalizer()
	astTestMust(t, err)
	// context::search leaves its decision scopes in place; the next push
	// restores the base level through pop_to_base_lvl.
	if internalizer.scopeLevel == 0 {
		t.Fatal("search scopes were popped by the check itself")
	}
	astTestMust(t, other.Push())
	for _, expression := range []ast.ASTHandle{a, b} {
		literal, literalErr := internalizer.Literal(expression)
		astTestMust(t, literalErr)
		value, assignmentErr := internalizer.Assignment(literal)
		astTestMust(t, assignmentErr)
		if value != lbool.Undef {
			t.Fatal("search decision survived pop_to_base_lvl", expression, value)
		}
	}
	astTestMust(t, other.Pop(1))

	falseExpression, err := m.MkFalse()
	astTestMust(t, err)
	unsat := smtTestKernel(t, m)
	astTestMust(t, unsat.Assert(falseExpression, ast.ASTHandle{}))
	result, err = unsat.Check()
	astTestMust(t, err)
	if result != lbool.False || !unsat.Inconsistent() {
		t.Fatal("false assertion check", result, unsat.Inconsistent())
	}
}

func TestSMTKernelCapturesAssignmentsRelevantAndGuessedLiterals(t *testing.T) {
	m := astTestManager(t)
	kernel, err := NewSMTKernel(m, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, kernel.Close()) })
	a := smtRelevancyBool(t, m, "kernel-result-a")
	b := smtRelevancyBool(t, m, "kernel-result-b")
	clause, err := m.MkOr(a, b)
	astTestMust(t, err)
	astTestMust(t, kernel.Assert(clause, ast.ASTHandle{}))
	status, err := kernel.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("kernel result status", status)
	}
	assignments, err := kernel.Assignments()
	astTestMust(t, err)
	relevant, err := kernel.RelevantLiterals()
	astTestMust(t, err)
	guessed, err := kernel.GuessedLiterals()
	astTestMust(t, err)
	if len(assignments) == 0 || len(relevant) == 0 || len(guessed) == 0 {
		t.Fatal("captured kernel literals", assignments, relevant, guessed)
	}
	labeled, err := kernel.RelevantLabeledLiterals(false)
	astTestMust(t, err)
	if len(labeled) != 0 {
		t.Fatal("unlabeled formula produced labeled literals", labeled)
	}
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, kernel.Assert(falsehood, ast.ASTHandle{}))
	status, err = kernel.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("second kernel result status", status)
	}
	assignments, err = kernel.Assignments()
	astTestMust(t, err)
	if len(assignments) != 0 {
		t.Fatal("stale satisfiable assignments survived unsat check", assignments)
	}
}

func TestSMTKernelCapturesRelevantLabeledLiterals(t *testing.T) {
	m := astTestManager(t)
	kernel, err := NewSMTKernel(m, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, kernel.Close()) })
	literal, err := m.MkLabelLit([]usymbol.Symbol{
		astTestSymbol(t, m, "kernel-label@selected"),
		astTestSymbol(t, m, "kernel-label-plain"),
	})
	astTestMust(t, err)
	astTestMust(t, kernel.Assert(literal, ast.ASTHandle{}))
	status, err := kernel.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("labeled kernel result", status)
	}
	all, err := kernel.RelevantLabeledLiterals(false)
	astTestMust(t, err)
	atOnly, err := kernel.RelevantLabeledLiterals(true)
	astTestMust(t, err)
	if len(all) != 1 || all[0] != literal || len(atOnly) != 1 || atOnly[0] != literal {
		t.Fatal("relevant labeled literals", all, atOnly)
	}
}

func TestSMTKernelConflictLearningProvesBooleanUnsat(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "kernel-unsat-a")
	b := smtRelevancyBool(t, m, "kernel-unsat-b")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	clauses := make([]ast.ASTHandle, 4)
	clauses[0], err = m.MkOr(a, b)
	astTestMust(t, err)
	clauses[1], err = m.MkOr(notA, b)
	astTestMust(t, err)
	clauses[2], err = m.MkOr(a, notB)
	astTestMust(t, err)
	clauses[3], err = m.MkOr(notA, notB)
	astTestMust(t, err)
	formula, err := m.MkAndArgs(clauses)
	astTestMust(t, err)
	kernel := smtTestKernel(t, m)
	astTestMust(t, kernel.Assert(formula, ast.ASTHandle{}))
	result, err := kernel.Check()
	astTestMust(t, err)
	if result != lbool.False || kernel.Statistics().NumDecisions == 0 || kernel.Statistics().NumConflicts < 2 {
		t.Fatal("search did not learn from contradictory branches", result, kernel.Statistics())
	}
}

func TestSMTKernelFlattensNegatedDisjunctionAssertion(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "kernel-not-or-a")
	b := smtRelevancyBool(t, m, "kernel-not-or-b")
	disjunction, err := m.MkOr(a, b)
	astTestMust(t, err)
	formula, err := m.MkNot(disjunction)
	astTestMust(t, err)
	kernel := smtTestKernel(t, m)
	astTestMust(t, kernel.Assert(formula, ast.ASTHandle{}))
	result, err := kernel.Check()
	astTestMust(t, err)
	if result != lbool.True || kernel.Statistics().NumDecisions != 0 {
		t.Fatal("negated disjunction assertion", result, kernel.Statistics())
	}
	state, err := kernel.Internalizer()
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{a, b} {
		literal, literalErr := state.Literal(expression)
		astTestMust(t, literalErr)
		value, assignmentErr := state.Assignment(literal)
		astTestMust(t, assignmentErr)
		if value != lbool.False {
			t.Fatal("negated disjunction child", expression, value)
		}
	}
}

func TestSMTKernelFlattenedAssertionsOwnDerivedProofs(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a := smtRelevancyBool(t, m, "kernel-proof-a")
	b := smtRelevancyBool(t, m, "kernel-proof-b")
	formula, err := m.MkAnd(a, b)
	astTestMust(t, err)
	kernel := smtTestKernel(t, m)
	astTestMust(t, kernel.Assert(formula, ast.ASTHandle{}))
	assertions := kernel.Assertions()
	if len(assertions) != 2 {
		t.Fatal("flattened proof assertions", assertions)
	}
	for _, assertion := range assertions {
		andElim, proofErr := m.IsProofKind(assertion.Proof, ast.ASTPrAndElim)
		astTestMust(t, proofErr)
		if !andElim {
			t.Fatal("missing conjunction-elimination proof", assertion.Proof)
		}
	}
	result, err := kernel.Check()
	astTestMust(t, err)
	if result != lbool.True {
		t.Fatal("proved conjunction", result)
	}
}

func TestSMTKernelAssumptionsValidateCheckAndRollback(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "kernel-assumption-a")
	b := smtRelevancyBool(t, m, "kernel-assumption-b")
	clause, err := m.MkOr(a, b)
	astTestMust(t, err)
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	kernel := smtTestKernel(t, m)
	astTestMust(t, kernel.Assert(clause, ast.ASTHandle{}))
	result, err := kernel.CheckAssumptions([]ast.ASTHandle{notA, notB})
	astTestMust(t, err)
	if result != lbool.False {
		t.Fatal("contradictory assumptions", result)
	}
	core := kernel.UnsatCore()
	if len(core) != 2 || core[0] != notA || core[1] != notB {
		t.Fatal("assumption unsat core", core, notA, notB)
	}
	state, err := kernel.Internalizer()
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{a, b} {
		literal, literalErr := state.Literal(expression)
		astTestMust(t, literalErr)
		value, assignmentErr := state.Assignment(literal)
		astTestMust(t, assignmentErr)
		data, dataErr := state.BooleanData(literal.Var())
		astTestMust(t, dataErr)
		if value != lbool.Undef || data.Assumption {
			t.Fatal("assumption state leaked", expression, value, data.Assumption)
		}
	}
	result, err = kernel.Check()
	astTestMust(t, err)
	if result != lbool.True {
		t.Fatal("base formula after assumption rollback", result)
	}
	if len(kernel.UnsatCore()) != 0 {
		t.Fatal("ordinary check did not clear unsat core")
	}
	if _, err = kernel.CheckAssumptions([]ast.ASTHandle{clause}); err == nil {
		t.Fatal("compound assumption accepted")
	}
}

func TestSMTKernelUnsatCoreIncludesContradictingAssumptionLiteral(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "kernel-core-a")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	kernel := smtTestKernel(t, m)
	astTestMust(t, kernel.Assert(a, ast.ASTHandle{}))
	result, err := kernel.CheckAssumptions([]ast.ASTHandle{notA})
	astTestMust(t, err)
	core := kernel.UnsatCore()
	if result != lbool.False || len(core) != 1 || core[0] != notA {
		t.Fatal("directly contradicting assumption core", result, core)
	}
}

func TestSMTKernelScopesRestoreAssertionsAndContext(t *testing.T) {
	m := astTestManager(t)
	atom := smtRelevancyBool(t, m, "kernel-scope-atom")
	negation, err := m.MkNot(atom)
	astTestMust(t, err)
	kernel := smtTestKernel(t, m)
	astTestMust(t, kernel.Assert(atom, ast.ASTHandle{}))
	astTestMust(t, kernel.Push())
	astTestMust(t, kernel.Assert(negation, ast.ASTHandle{}))
	result, err := kernel.Check()
	astTestMust(t, err)
	if result != lbool.False || kernel.ScopeLevel() != 1 {
		t.Fatal("scoped contradiction", result, kernel.ScopeLevel())
	}
	astTestMust(t, kernel.Pop(1))
	if kernel.ScopeLevel() != 0 || len(kernel.Assertions()) != 1 || kernel.Inconsistent() {
		t.Fatal("scope rollback", kernel.ScopeLevel(), kernel.Assertions(), kernel.Inconsistent())
	}
	result, err = kernel.Check()
	astTestMust(t, err)
	if result != lbool.True {
		t.Fatal("base assertion after rollback", result)
	}
}
