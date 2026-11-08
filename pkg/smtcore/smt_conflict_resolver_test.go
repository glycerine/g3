package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTConflictResolverBuildsFirstUIPLemmaAndBackjumpLevel(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "resolve-a")
	b := smtRelevancyBool(t, m, "resolve-b")
	c := smtRelevancyBool(t, m, "resolve-c")
	for _, expression := range []ast.ASTHandle{a, b, c} {
		astTestMust(t, state.Internalize(expression, true))
	}
	av, _ := state.BoolVar(a)
	bv, _ := state.BoolVar(b)
	cv, _ := state.BoolVar(c)
	la, lb, lc := NewSMTLiteral(av, false), NewSMTLiteral(bv, false), NewSMTLiteral(cv, false)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(la, NewSMTBooleanAxiom()))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(lb, NewSMTBooleanAxiom()))
	reason, err := NewSMTClause(m, []SMTLiteral{lc, la.Negated()}, SMTClauseLearned, nil, nil, false, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, reason.Close()) })
	astTestMust(t, state.AssignLiteral(lc, NewSMTBooleanClause(reason)))
	conflict, err := NewSMTClause(m, []SMTLiteral{lc.Negated(), lb.Negated()}, SMTClauseAux, nil, nil, false, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, conflict.Close()) })
	resolver, err := NewSMTConflictResolver(state)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, resolver.Close()) })
	got, err := resolver.Resolve(NewSMTBooleanClause(conflict), SMTNullLiteral, 0)
	astTestMust(t, err)
	want := []SMTLiteral{lb.Negated(), la.Negated()}
	if !got.Resolved || got.ConflictLevel != 2 || got.NewScopeLevel != 1 || len(got.Lemma) != len(want) {
		t.Fatal("resolution metadata", got)
	}
	for i := range want {
		if got.Lemma[i] != want[i] {
			t.Fatal("lemma", got.Lemma, want)
		}
	}
	if activity, activityErr := reason.Activity(); activityErr != nil || activity != 2 {
		t.Fatal("resolved lemma activity", activity, activityErr)
	}
}

func TestSMTConflictResolverIgnoresTrueAntecedent(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	expression := smtRelevancyBool(t, m, "resolve-true-antecedent")
	astTestMust(t, state.Internalize(expression, true))
	variable, found := state.BoolVar(expression)
	if !found {
		t.Fatal("missing conflict variable")
	}
	literal := NewSMTLiteral(variable, false)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	justification, err := NewSMTSimpleJustification(true, []SMTLiteral{SMTTrueLiteral, literal})
	astTestMust(t, err)
	conflict, err := NewSMTBooleanJustification(justification)
	astTestMust(t, err)
	resolver, err := NewSMTConflictResolver(state)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, resolver.Close()) })
	resolution, err := resolver.Resolve(conflict, SMTNullLiteral, 0)
	astTestMust(t, err)
	if !resolution.Resolved || len(resolution.Lemma) != 1 || resolution.Lemma[0] != literal.Negated() {
		t.Fatal("true antecedent affected conflict resolution", resolution)
	}
}

func TestSMTConflictResolverSynchronizesLateBooleanEnodes(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	first := smtRelevancyBool(t, m, "resolve-initial-enode")
	astTestMust(t, state.Internalize(first, true))
	resolver, err := NewSMTConflictResolver(state)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, resolver.Close()) })
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "resolve-late-sort"), nil)
	astTestMust(t, err)
	left, err := m.MkConst(astTestSymbol(t, m, "resolve-late-left"), sort)
	astTestMust(t, err)
	right, err := m.MkConst(astTestSymbol(t, m, "resolve-late-right"), sort)
	astTestMust(t, err)
	late, err := m.MkEq(left, right)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(late, true))
	variable, found := state.BoolVar(late)
	if !found {
		t.Fatal("missing late Boolean variable")
	}
	literal, err := resolver.expander.LiteralForENode(state.context.enodes.get(late))
	astTestMust(t, err)
	if literal != NewSMTLiteral(variable, false) {
		t.Fatal("late Boolean enode literal", literal, variable)
	}
}

func TestSMTInternalizerInstallsLearnedClauseInCurrentScope(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	variables := make([]SMTLiteral, 3)
	for index := range variables {
		expression := smtRelevancyBool(t, m, "learned-clause-"+string(rune('a'+index)))
		astTestMust(t, state.Internalize(expression, true))
		variable, found := state.BoolVar(expression)
		if !found {
			t.Fatal("missing learned-clause variable", index)
		}
		variables[index] = NewSMTLiteral(variable, false)
	}
	smtTestUserPush(t, state)
	astTestMust(t, state.AddLearnedClause(variables))
	clauses := state.Clauses()
	if len(clauses) != 1 || state.clauses[0].clause.Kind() != SMTClauseLearned {
		t.Fatal("learned clause kind", clauses)
	}
	astTestMust(t, state.PopScopes(1))
	if len(state.Clauses()) != 0 {
		t.Fatal("learned clause survived its logical scope")
	}
}

func TestSMTInternalizerLearnedClausePreservesFalseAntecedents(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	literals := make([]SMTLiteral, 3)
	for index := range literals {
		expression := smtRelevancyBool(t, m, "learned-preserve-"+string(rune('a'+index)))
		astTestMust(t, state.Internalize(expression, true))
		variable, found := state.BoolVar(expression)
		if !found {
			t.Fatal("missing learned-clause variable", index)
		}
		literals[index] = NewSMTLiteral(variable, false)
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(literals[1].Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, state.AssignLiteral(literals[2].Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, state.AddLearnedClause(literals))
	if len(state.clauses) != 1 || len(state.clauses[0].clause.Literals()) != 3 {
		t.Fatal("learned clause discarded false antecedents", state.Clauses())
	}
	value, err := state.Assignment(literals[0])
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("learned clause did not propagate asserting literal", value)
	}
	justification := state.booleanData[literals[0].Var()].Justification()
	clause, ok := justification.Clause()
	if !ok || clause != state.clauses[0].clause {
		t.Fatal("asserting literal lost learned-clause justification")
	}
}

func TestSMTInternalizerLearnedClauseRetainsProof(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	_, _, state := smtInternalizerFixture(t, m, nil)
	literals := make([]SMTLiteral, 2)
	expressions := make([]ast.ASTHandle, 2)
	for index := range literals {
		expressions[index] = smtRelevancyBool(t, m, "learned-proof-"+string(rune('a'+index)))
		astTestMust(t, state.Internalize(expressions[index], true))
		variable, found := state.BoolVar(expressions[index])
		if !found {
			t.Fatal("missing learned-clause variable", index)
		}
		literals[index] = NewSMTLiteral(variable, false)
	}
	fact, err := m.MkOrArgs(expressions)
	astTestMust(t, err)
	proof, err := m.MkProofHypothesis(fact)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AddLearnedClauseWithProof(literals, proof))
	clause := state.clauses[0].clause
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	got, err := builder.clauseProof(clause)
	astTestMust(t, err)
	gotFact, err := m.GetProofFact(got)
	astTestMust(t, err)
	if gotFact != fact {
		t.Fatal("learned clause proof fact", gotFact, fact)
	}
}

func TestSMTSearchReinternalizesLearnedClauseAtomsAfterBackjump(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	expression := smtRelevancyBool(t, m, "learned-reinternalize")
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(expression, true))
	variable, found := state.BoolVar(expression)
	if !found {
		t.Fatal("missing scoped learned-clause atom")
	}
	literal := NewSMTLiteral(variable, true)
	astTestMust(t, state.relevancy.MarkRelevant(expression))
	atoms, err := state.retainLemmaAtoms([]SMTLiteral{literal})
	astTestMust(t, err)
	astTestMust(t, state.PopScopes(1))
	if _, found = state.BoolVar(expression); found {
		t.Fatal("scoped atom survived backjump")
	}
	restored, err := state.reinternalizeLemmaAtoms(atoms)
	astTestMust(t, err)
	astTestMust(t, state.releaseLemmaAtoms(atoms))
	if len(restored) != 1 || !restored[0].Sign() {
		t.Fatal("reinternalized learned literal", restored)
	}
	data, err := state.BooleanData(restored[0].Var())
	astTestMust(t, err)
	if data.InternalLevel() != 0 {
		t.Fatal("reinternalized atom scope", data.InternalLevel())
	}
	if !state.relevancy.IsRelevant(expression) {
		t.Fatal("reinternalized learned atom lost conflict relevancy")
	}
}

func TestSMTConflictResolverMinimizesImpliedLemmaLiteral(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "minimize-a")
	b := smtRelevancyBool(t, m, "minimize-b")
	c := smtRelevancyBool(t, m, "minimize-c")
	for _, expression := range []ast.ASTHandle{a, b, c} {
		astTestMust(t, state.Internalize(expression, true))
	}
	av, _ := state.BoolVar(a)
	bv, _ := state.BoolVar(b)
	cv, _ := state.BoolVar(c)
	la, lb, lc := NewSMTLiteral(av, false), NewSMTLiteral(bv, false), NewSMTLiteral(cv, false)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(la, NewSMTBooleanAxiom()))
	reason, err := NewSMTClause(m, []SMTLiteral{lc, la.Negated()}, SMTClauseAux, nil, nil, false, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, reason.Close()) })
	astTestMust(t, state.AssignLiteral(lc, NewSMTBooleanClause(reason)))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(lb, NewSMTBooleanAxiom()))
	resolver, err := NewSMTConflictResolver(state)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, resolver.Close()) })
	marks := make([]bool, len(state.booleanData))
	marks[av], marks[cv] = true, true
	lemma, err := resolver.minimizeLemma([]SMTLiteral{lb.Negated(), la.Negated(), lc.Negated()}, marks, resolver.levels(), 0)
	astTestMust(t, err)
	if len(lemma) != 2 || lemma[0] != lb.Negated() || lemma[1] != la.Negated() || state.Statistics().NumMinimizedLits != 1 {
		t.Fatal("minimized lemma", lemma, state.Statistics())
	}
}

func TestSMTConflictResolverOwnsFirstUIPProof(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "proof-resolve-a")
	b := smtRelevancyBool(t, m, "proof-resolve-b")
	c := smtRelevancyBool(t, m, "proof-resolve-c")
	for _, expression := range []ast.ASTHandle{a, b, c} {
		astTestMust(t, state.Internalize(expression, true))
	}
	av, _ := state.BoolVar(a)
	bv, _ := state.BoolVar(b)
	cv, _ := state.BoolVar(c)
	la, lb, lc := NewSMTLiteral(av, false), NewSMTLiteral(bv, false), NewSMTLiteral(cv, false)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))

	notA, err := m.MkNot(a)
	astTestMust(t, err)
	reasonFact, err := m.MkOr(c, notA)
	astTestMust(t, err)
	reasonProof, err := m.MkProofHypothesis(reasonFact)
	astTestMust(t, err)
	reasonJustification, err := NewSMTJustificationProofWrapper(m, reasonProof, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, reasonJustification.Release(m)) })

	notC, err := m.MkNot(c)
	astTestMust(t, err)
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	conflictFact, err := m.MkOr(notC, notB)
	astTestMust(t, err)
	conflictProof, err := m.MkProofHypothesis(conflictFact)
	astTestMust(t, err)
	conflictJustification, err := NewSMTJustificationProofWrapper(m, conflictProof, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, conflictJustification.Release(m)) })

	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(la, NewSMTBooleanAxiom()))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(lb, NewSMTBooleanAxiom()))
	reason, err := NewSMTClause(m, []SMTLiteral{lc, la.Negated()}, SMTClauseAux, reasonJustification, nil, false, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, reason.Close()) })
	astTestMust(t, state.AssignLiteral(lc, NewSMTBooleanClause(reason)))
	conflict, err := NewSMTClause(m, []SMTLiteral{lc.Negated(), lb.Negated()}, SMTClauseAux, conflictJustification, nil, false, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, conflict.Close()) })

	resolver, err := NewSMTConflictResolver(state)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, resolver.Close()) })
	resolution, err := resolver.Resolve(NewSMTBooleanClause(conflict), SMTNullLiteral, 0)
	astTestMust(t, err)
	if !resolution.Resolved {
		t.Fatal("conflict was not resolved")
	}
	proof, err := resolver.LemmaProof()
	astTestMust(t, err)
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	want, err := m.MkOr(notB, notA)
	astTestMust(t, err)
	if fact != want {
		t.Fatal("lemma proof fact", fact, want)
	}
}
