package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtInternalizerFixture(t testing.TB, m *ast.ASTManager, registry *SMTQuantifierRegistry) (*SMTCheckerContext, *SMTRelevancyPropagator, *SMTInternalizerState) {
	t.Helper()
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	relevancy, err := NewSMTRelevancyPropagator(context, true)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, registry)
	astTestMust(t, err)
	astTestMust(t, state.SetRelevancy(relevancy))
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	return context, relevancy, state
}

func TestSMTInternalizerGateProofReservedConstants(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a := smtRelevancyBool(t, m, "gate-proof-constant-a")
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Internalize(a, true))
	literal, err := state.Literal(a)
	astTestMust(t, err)
	for _, constant := range []SMTLiteral{SMTTrueLiteral, SMTFalseLiteral} {
		proof, err := state.gateClauseProof([]SMTLiteral{literal, constant, constant})
		astTestMust(t, err)
		fact, err := m.GetProofFact(proof)
		astTestMust(t, err)
		if op, err := m.GetDeclKind(fact); err != nil || op != ast.ASTOpOr {
			t.Fatal("definition proof must retain unsimplified OR", op, err)
		}
		value, err := m.GetArg(fact, 1)
		astTestMust(t, err)
		if (constant == SMTTrueLiteral && !m.IsTrue(value)) || (constant == SMTFalseLiteral && !m.IsFalse(value)) {
			t.Fatal("reserved literal proof expression", constant, value)
		}
	}
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	gate, err := m.MkIff(a, falsehood)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(gate, true))
	constantGate, err := m.MkIff(falsehood, falsehood)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(constantGate, true))
	constantLiteral, err := state.Literal(constantGate)
	astTestMust(t, err)
	builder := &smtConflictProofBuilder{state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
	proof, err := builder.literalProof(constantLiteral)
	astTestMust(t, err)
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	if fact != constantGate {
		t.Fatal("false disjunct elimination did not produce a unit proof", fact, constantGate)
	}
}

func TestSMTInternalizerScopedEnodeFlagOnExistingGate(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "scoped-enode-a")
	b := smtRelevancyBool(t, m, "scoped-enode-b")
	gate, err := m.MkAnd(a, b)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Internalize(gate, true))
	variable, found := state.BoolVar(gate)
	if !found {
		t.Fatal("gate was not internalized")
	}
	data, err := state.BooleanData(variable)
	astTestMust(t, err)
	if data.IsENode() {
		t.Fatal("gate already had an enode")
	}
	for i := 0; i < 2; i++ {
		astTestMust(t, state.PushScope())
		astTestMust(t, state.Internalize(gate, false))
		data, err = state.BooleanData(variable)
		astTestMust(t, err)
		if !data.IsENode() || state.context.enodes.get(gate) == nil {
			t.Fatal("scoped enode was not attached")
		}
		astTestMust(t, state.PopScopes(1))
		data, err = state.BooleanData(variable)
		astTestMust(t, err)
		if data.IsENode() || state.context.enodes.get(gate) != nil {
			t.Fatal("scoped enode flag survived rollback")
		}
		if v, ok := state.BoolVar(gate); !ok || v != variable {
			t.Fatal("base Boolean variable was lost")
		}
		resolver, err := NewSMTConflictResolver(state)
		astTestMust(t, err)
		astTestMust(t, resolver.Close())
	}
}

func TestSMTInternalizerBooleanGateClausesAndRootAssertions(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "internalize-gate-a")
	b := smtRelevancyBool(t, m, "internalize-gate-b")
	conjunction, err := m.MkAnd(a, b)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Internalize(conjunction, true))
	if _, found := state.BoolVar(a); !found {
		t.Fatal("first child was not internalized")
	}
	if _, found := state.BoolVar(b); !found {
		t.Fatal("second child was not internalized")
	}
	if _, found := state.BoolVar(conjunction); !found {
		t.Fatal("gate variable was not internalized")
	}
	if len(state.Clauses()) != 3 {
		t.Fatal("AND definition clauses", state.Clauses())
	}
	if state.context.enodes.count() != 0 {
		t.Fatal("gate-context formula unexpectedly created enodes")
	}

	c := smtRelevancyBool(t, m, "internalize-root-c")
	d := smtRelevancyBool(t, m, "internalize-root-d")
	disjunction, err := m.MkOr(c, d)
	astTestMust(t, err)
	before := len(state.Clauses())
	astTestMust(t, state.Assert(disjunction, ast.ASTHandle{}, 9))
	clauses := state.Clauses()
	if len(clauses) != before+1 || !clauses[len(clauses)-1].Root || len(clauses[len(clauses)-1].Literals) != 2 {
		t.Fatal("root OR clause", clauses)
	}
	if !state.context.relevant.get(disjunction) || state.Statistics().MaxGeneration != 9 {
		t.Fatal("root relevance or generation")
	}
}

func TestSMTInternalizerDefaultAssertionMarksLiteralRelevant(t *testing.T) {
	m := astTestManager(t)
	atom := smtRelevancyBool(t, m, "assert-default-literal")
	negated, err := m.MkNot(atom)
	astTestMust(t, err)
	ref, err := m.NewRef(negated)
	astTestMust(t, err)
	_, relevancy, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Assert(negated, ast.ASTHandle{}, 0))
	astTestMust(t, m.DelRef(&ref))
	if !relevancy.IsRelevant(atom) || relevancy.IsRelevant(negated) {
		t.Fatal("default assertion marked its temporary wrapper instead of its literal")
	}
	consistent, err := state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("single negative assertion became inconsistent")
	}
}

func TestSMTInternalizerBuildsGateAndRootClauseProofs(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a := smtRelevancyBool(t, m, "internalizer-proof-a")
	b := smtRelevancyBool(t, m, "internalizer-proof-b")
	conjunction, err := m.MkAnd(a, b)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Internalize(conjunction, true))
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	for index := range state.clauses {
		clause := state.clauses[index].clause
		proof, proofErr := builder.clauseProof(clause)
		astTestMust(t, proofErr)
		fact, proofErr := m.GetProofFact(proof)
		astTestMust(t, proofErr)
		want, proofErr := builder.theoryFact(clause.literals)
		astTestMust(t, proofErr)
		if fact != want {
			t.Fatal("gate clause proof fact", index, fact, want)
		}
	}

	before := len(state.clauses)
	iff, err := m.MkIff(a, b)
	astTestMust(t, err)
	asserted, err := m.MkProofAsserted(iff)
	astTestMust(t, err)
	astTestMust(t, state.Assert(iff, asserted, 0))
	for index := before; index < len(state.clauses); index++ {
		clause := state.clauses[index].clause
		proof, proofErr := builder.clauseProof(clause)
		astTestMust(t, proofErr)
		fact, proofErr := m.GetProofFact(proof)
		astTestMust(t, proofErr)
		want, proofErr := builder.theoryFact(clause.literals)
		astTestMust(t, proofErr)
		if fact != want {
			t.Fatal("root clause proof fact", index, fact, want)
		}
	}

	c := smtRelevancyBool(t, m, "internalizer-proof-unit")
	cProof, err := m.MkProofAsserted(c)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Assert(c, cProof, 0))
	cVariable, found := state.BoolVar(c)
	if !found {
		t.Fatal("missing unit assertion variable")
	}
	justification, ok := state.booleanData[cVariable].Justification().Justification()
	if !ok {
		t.Fatal("unit assertion did not retain proof justification")
	}
	unitProof, err := builder.objectProof(justification)
	astTestMust(t, err)
	unitFact, err := m.GetProofFact(unitProof)
	astTestMust(t, err)
	if unitFact != c {
		t.Fatal("unit assertion proof fact", unitFact, c)
	}
	astTestMust(t, state.PopScopes(1))

	d := smtRelevancyBool(t, m, "internalizer-proof-reduced-left")
	e := smtRelevancyBool(t, m, "internalizer-proof-reduced-right")
	astTestMust(t, state.Internalize(d, true))
	astTestMust(t, state.Internalize(e, true))
	dVariable, _ := state.BoolVar(d)
	eVariable, _ := state.BoolVar(e)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(dVariable, true), NewSMTBooleanAxiom()))
	reducedIFF, err := m.MkIff(d, e)
	astTestMust(t, err)
	reducedProof, err := m.MkProofAsserted(reducedIFF)
	astTestMust(t, err)
	astTestMust(t, state.Assert(reducedIFF, reducedProof, 0))
	eJustification, ok := state.booleanData[eVariable].Justification().Justification()
	if !ok {
		t.Fatal("simplified root clause lost proof justification")
	}
	eProof, err := builder.objectProof(eJustification)
	astTestMust(t, err)
	eFact, err := m.GetProofFact(eProof)
	astTestMust(t, err)
	notE, err := m.MkNot(e)
	astTestMust(t, err)
	if eFact != notE {
		t.Fatal("simplified root clause proof fact", eFact, notE)
	}
	astTestMust(t, state.PopScopes(1))
}

func TestSMTInternalizerBuildsTheoryAxiomProofs(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	_, _, state := smtInternalizerFixture(t, m, nil)
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	makeLiteral := func(name string) (ast.ASTHandle, SMTLiteral) {
		expression := smtRelevancyBool(t, m, name)
		astTestMust(t, state.Internalize(expression, true))
		variable, found := state.BoolVar(expression)
		if !found {
			t.Fatal("missing theory-axiom variable", name)
		}
		return expression, NewSMTLiteral(variable, false)
	}
	aExpression, a := makeLiteral("theory-axiom-a")
	bExpression, b := makeLiteral("theory-axiom-b")
	astTestMust(t, state.addTheoryAxiom(family, []SMTLiteral{a, b}))
	clause := state.clauses[0].clause
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	proof, err := builder.clauseProof(clause)
	astTestMust(t, err)
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	want, err := m.MkOr(aExpression, bExpression)
	astTestMust(t, err)
	if fact != want {
		t.Fatal("theory axiom clause fact", fact, want)
	}

	falseReducedExpression, falseReduced := makeLiteral("theory-axiom-false-reduced")
	astTestMust(t, state.addTheoryAxiom(family, []SMTLiteral{SMTFalseLiteral, SMTFalseLiteral, falseReduced}))
	falseReducedJustification, ok := state.booleanData[falseReduced.Var()].Justification().Justification()
	if !ok {
		t.Fatal("false-reduced theory axiom lost its justification")
	}
	falseReducedProof, err := builder.objectProof(falseReducedJustification)
	astTestMust(t, err)
	falseReducedFact, err := m.GetProofFact(falseReducedProof)
	astTestMust(t, err)
	if falseReducedFact != falseReducedExpression {
		t.Fatal("false-reduced theory axiom fact", falseReducedFact, falseReducedExpression)
	}

	_, c := makeLiteral("theory-axiom-c")
	dExpression, d := makeLiteral("theory-axiom-d")
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(c.Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, state.addTheoryAxiom(family, []SMTLiteral{c, d}))
	justification, ok := state.booleanData[d.Var()].Justification().Justification()
	if !ok {
		t.Fatal("simplified theory axiom lost its justification")
	}
	proof, err = builder.objectProof(justification)
	astTestMust(t, err)
	fact, err = m.GetProofFact(proof)
	astTestMust(t, err)
	if fact != dExpression {
		t.Fatal("simplified theory axiom fact", fact, dExpression)
	}
	astTestMust(t, state.PopScopes(1))
}

func TestSMTInternalizerTermITEEqualityClausesAndRelevancy(t *testing.T) {
	m := astTestManager(t)
	condition := smtRelevancyBool(t, m, "internalize-term-condition")
	sort, err := m.MkSort(astTestSymbol(t, m, "InternalizeTermSort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	thenTerm, _ := smtEnodeConstant(t, m, 2240, sort, 0)
	elseTerm, _ := smtEnodeConstant(t, m, 2241, sort, 0)
	term, err := m.MkIte(condition, thenTerm, elseTerm)
	astTestMust(t, err)
	context, relevancy, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Internalize(term, false))
	if context.enodes.get(term) == nil || context.enodes.get(thenTerm) == nil || context.enodes.get(elseTerm) == nil || len(state.Clauses()) != 2 {
		t.Fatal("term ITE enodes or clauses", context.enodes.count(), state.Clauses())
	}
	thenEquality, err := m.MkEq(term, thenTerm)
	astTestMust(t, err)
	elseEquality, err := m.MkEq(term, elseTerm)
	astTestMust(t, err)
	for _, equality := range []ast.ASTHandle{thenEquality, elseEquality} {
		variable, found := state.BoolVar(equality)
		if !found {
			t.Fatal("ITE equality variable")
		}
		data, err := state.BooleanData(variable)
		astTestMust(t, err)
		if !data.IsEquality() || !data.IsENode() {
			t.Fatal("equality metadata", *data)
		}
	}
	astTestMust(t, relevancy.MarkRelevant(term))
	astTestMust(t, relevancy.Propagate())
	conditionVar, _ := state.BoolVar(condition)
	astTestMust(t, state.SetAssignment(conditionVar, lbool.False))
	astTestMust(t, relevancy.AssignmentEvent(condition, false))
	astTestMust(t, relevancy.Propagate())
	if !relevancy.IsRelevant(elseEquality) || relevancy.IsRelevant(thenEquality) {
		t.Fatal("term ITE selected equality relevance")
	}
}

func TestSMTInternalizerQuantifierRegistrationAndDefaultAssertion(t *testing.T) {
	m := astTestManager(t)
	registry, _ := smtQuickRegistry(t, m)
	_, _, state := smtInternalizerFixture(t, m, registry)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "internalize-q-x")}, Body: variable,
	})
	astTestMust(t, err)
	astTestMust(t, state.SetCachedGeneration(quantifier, 12))
	astTestMust(t, state.Internalize(quantifier, true))
	if generation, err := registry.Generation(quantifier); err != nil || generation != 12 {
		t.Fatal("quantifier generation", generation, err)
	}
	atom := smtRelevancyBool(t, m, "internalize-asserted-atom")
	astTestMust(t, state.Assert(atom, ast.ASTHandle{}, 4))
	atomVar, found := state.BoolVar(atom)
	if !found {
		t.Fatal("asserted atom variable")
	}
	if value, err := state.Assignment(NewSMTLiteral(atomVar, false)); err != nil || value != lbool.True {
		t.Fatal("default assertion assignment", value, err)
	}
	if !state.context.relevant.get(atom) {
		t.Fatal("default assertion relevance")
	}
}

func TestSMTInternalizerConsumesMaterializedQuantifierInstance(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "internalize-instance-x")}, Body: variable,
	})
	astTestMust(t, err)
	_, binding := smtEnodeConstant(t, m, 2250, boolean, 2)
	checkerContext, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	builder := smtQIInstanceBuilder(t, m, checkerContext)
	prepared, err := builder.Prepare(SMTQIQueueEntry{Fingerprint: &SMTFingerprint{data: quantifier, args: []*SMTEnode{binding}}, Cost: 6, Generation: 2, Instantiated: true})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, prepared.Close()) })
	registry, _ := smtQuickRegistry(t, m)
	_, _, state := smtInternalizerFixture(t, m, registry)
	instance, err := builder.MaterializeAndInternalize(prepared, smtQuickTheoryRewriter(t, m), &SMTQuantifierStat{}, 0, state)
	astTestMust(t, err)
	if instance == nil {
		t.Fatal("missing materialized instance")
	}
	t.Cleanup(func() { astTestMust(t, instance.Close()) })
	if len(state.Clauses()) != 1 || !state.Clauses()[0].Root {
		t.Fatal("internalized quantifier lemma", state.Clauses())
	}
	if state.Statistics().MaxGeneration != instance.Generation() {
		t.Fatal("instance generation", state.Statistics().MaxGeneration, instance.Generation())
	}
}

func TestSMTInternalizerDeepAssertionUsesTopologicalPrepass(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	wrapper, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "internalize-deep-wrapper"), []ast.ASTHandle{boolean}, boolean, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	expression := smtRelevancyBool(t, m, "internalize-deep-leaf")
	for depth := uint32(0); depth <= smtDeepExpressionThreshold; depth++ {
		expression, err = m.MkApp(wrapper, []ast.ASTHandle{expression})
		astTestMust(t, err)
	}
	actualDepth, err := m.GetDepth(expression)
	astTestMust(t, err)
	if actualDepth <= smtDeepExpressionThreshold {
		t.Fatal("fixture is not deep", actualDepth)
	}
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Assert(expression, ast.ASTHandle{}, 3))
	if state.boolVar.count() != int(smtDeepExpressionThreshold)+2 {
		t.Fatal("deep assertion internalization count", state.boolVar.count())
	}
	rootVariable, found := state.BoolVar(expression)
	if !found {
		t.Fatal("deep assertion root was not internalized")
	}
	if value, err := state.Assignment(NewSMTLiteral(rootVariable, false)); err != nil || value != lbool.True {
		t.Fatal("deep assertion root assignment", value, err)
	}
}

func TestSMTInternalizerWatchedClausePropagationAndRollback(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "propagate-a")
	b := smtRelevancyBool(t, m, "propagate-b")
	c := smtRelevancyBool(t, m, "propagate-c")
	_, _, state := smtInternalizerFixture(t, m, nil)
	for _, expression := range []ast.ASTHandle{a, b, c} {
		astTestMust(t, state.Internalize(expression, true))
	}
	aLiteral, err := state.Literal(a)
	astTestMust(t, err)
	bLiteral, err := state.Literal(b)
	astTestMust(t, err)
	cLiteral, err := state.Literal(c)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addClause([]SMTLiteral{aLiteral, bLiteral, cLiteral}, false, ast.ASTHandle{}))
	if len(state.watches[aLiteral.Negated().Index()].Clauses()) != 1 || len(state.watches[bLiteral.Negated().Index()].Clauses()) != 1 {
		t.Fatal("initial clause watches")
	}
	astTestMust(t, state.AssignLiteral(aLiteral.Negated(), NewSMTBooleanAxiom()))
	consistent, err := state.Propagate()
	astTestMust(t, err)
	if !consistent || len(state.watches[aLiteral.Negated().Index()].Clauses()) != 0 || len(state.watches[cLiteral.Negated().Index()].Clauses()) != 1 {
		t.Fatal("moved clause watch", consistent)
	}
	astTestMust(t, state.AssignLiteral(bLiteral.Negated(), NewSMTBooleanAxiom()))
	consistent, err = state.Propagate()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("unexpected propagation conflict")
	}
	if value, err := state.Assignment(cLiteral); err != nil || value != lbool.True {
		t.Fatal("unit propagation", value, err)
	}
	if clause, ok := state.booleanData[cLiteral.Var()].Justification().Clause(); !ok || clause == nil {
		t.Fatal("clause propagation justification")
	}
	if state.Statistics().NumPropagations != 1 {
		t.Fatal("propagation statistics", state.Statistics())
	}
	astTestMust(t, state.PopScopes(1))
	for _, literal := range []SMTLiteral{aLiteral, bLiteral, cLiteral} {
		if value, err := state.Assignment(literal); err != nil || value != lbool.Undef {
			t.Fatal("assignment rollback", literal, value, err)
		}
		if len(state.watches[literal.Negated().Index()].Clauses()) != 0 {
			t.Fatal("watch rollback", literal)
		}
	}

	astTestMust(t, state.PushScope())
	astTestMust(t, state.addClause([]SMTLiteral{aLiteral, bLiteral}, false, ast.ASTHandle{}))
	astTestMust(t, state.AssignLiteral(aLiteral.Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, state.AssignLiteral(bLiteral.Negated(), NewSMTBooleanAxiom()))
	consistent, err = state.Propagate()
	astTestMust(t, err)
	conflict, found := state.Conflict()
	clause, clauseConflict := conflict.Clause()
	if consistent || !found || !clauseConflict || clause == nil || state.Statistics().NumConflicts != 1 {
		t.Fatal("watched clause conflict", consistent, found, conflict.Kind(), state.Statistics())
	}
	astTestMust(t, state.PopScopes(1))
	if state.Inconsistent() {
		t.Fatal("conflict scope rollback")
	}
}
