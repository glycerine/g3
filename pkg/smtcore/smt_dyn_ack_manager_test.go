package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTDynamicAckManagerBuildsPairAndTripleClauses(t *testing.T) {
	manager := astTestManager(t)
	sort, values := smtUFConstants(t, manager, "dynamic-ack-manager-", 3)
	function, err := manager.MkFuncDecl(astTestSymbol(t, manager, "dynamic-ack-manager-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	applications := make([]ast.ASTHandle, len(values))
	for index, value := range values {
		applications[index], err = manager.MkApp(function, []ast.ASTHandle{value})
		astTestMust(t, err)
	}
	_, _, state := smtInternalizerFixture(t, manager, nil)
	config := defaultSMTDynAckState()
	config.Threshold = 1
	config.Factor = 1
	config.Equalities = true
	astTestMust(t, state.ConfigureDynamicAck(config))
	smtTestUserPush(t, state)
	astTestMust(t, state.dynamicAck.schedule.conflictCongruence(applications[0], applications[1], dynAckRoot))
	astTestMust(t, state.dynamicAck.schedule.usedEquality(applications[0], applications[1], applications[2], true))
	state.statistics.NumConflicts = 2
	astTestMust(t, state.dynamicAck.propagate())
	if state.statistics.NumDynAck != 2 || len(state.clauses) != 2 {
		t.Fatal("dynamic Ackermann propagation", state.statistics.NumDynAck, len(state.clauses))
	}
	if len(state.clauses[0].Literals) != 2 || len(state.clauses[1].Literals) != 3 {
		t.Fatal("dynamic Ackermann clause shapes", state.clauses[0].Literals, state.clauses[1].Literals)
	}
	pair := dynAckPair{applications[0], applications[1]}
	triple := dynAckTriple{applications[0], applications[1], applications[2]}
	if _, found := state.dynamicAck.pairs.instantiated[pair]; !found {
		t.Fatal("pair instantiation was not retained")
	}
	if _, found := state.dynamicAck.triples.instantiated[triple]; !found {
		t.Fatal("triple instantiation was not retained")
	}
	astTestMust(t, state.PopScopes(1))
	if _, found := state.dynamicAck.pairs.instantiated[pair]; found {
		t.Fatal("deleted pair clause retained its instantiation")
	}
	if _, found := state.dynamicAck.triples.instantiated[triple]; found {
		t.Fatal("deleted triple clause retained its instantiation")
	}
	if state.statistics.NumDelDynAck != 2 {
		t.Fatal("dynamic Ackermann deletion statistics", state.statistics.NumDelDynAck)
	}
}

func TestSMTDynamicAckManagerBuildsClauseProof(t *testing.T) {
	manager := astTestManager(t)
	astTestMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	sort, values := smtUFConstants(t, manager, "dynamic-ack-proof-", 4)
	function, err := manager.MkFuncDecl(astTestSymbol(t, manager, "dynamic-ack-proof-f"), []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	first, err := manager.MkApp(function, []ast.ASTHandle{values[0], values[1]})
	astTestMust(t, err)
	second, err := manager.MkApp(function, []ast.ASTHandle{values[2], values[3]})
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	config := defaultSMTDynAckState()
	config.Threshold = 1
	config.Factor = 1
	astTestMust(t, state.ConfigureDynamicAck(config))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.dynamicAck.schedule.conflictCongruence(first, second, dynAckRoot))
	state.statistics.NumConflicts = 1
	astTestMust(t, state.dynamicAck.propagate())
	if len(state.clauses) != 1 || state.clauses[0].clause == nil {
		t.Fatal("dynamic Ackermann proof clause", len(state.clauses))
	}
	clause := state.clauses[0].clause
	if _, ok := clause.justification.(*smtDynamicAckJustification); !ok {
		t.Fatalf("dynamic Ackermann justification %T", clause.justification)
	}
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	proof, err := builder.clauseProof(clause)
	astTestMust(t, err)
	fact, err := manager.GetProofFact(proof)
	astTestMust(t, err)
	firstArgumentEquality, err := manager.MkEq(values[0], values[2])
	astTestMust(t, err)
	firstArgumentDisequality, err := manager.MkNot(firstArgumentEquality)
	astTestMust(t, err)
	secondArgumentEquality, err := manager.MkEq(values[1], values[3])
	astTestMust(t, err)
	secondArgumentDisequality, err := manager.MkNot(secondArgumentEquality)
	astTestMust(t, err)
	conclusion, err := manager.MkEq(first, second)
	astTestMust(t, err)
	want, err := manager.MkOrArgs([]ast.ASTHandle{firstArgumentDisequality, secondArgumentDisequality, conclusion})
	astTestMust(t, err)
	if fact != want {
		t.Fatal("dynamic Ackermann proof fact", fact, want)
	}
	astTestMust(t, state.PopScopes(1))
}

func TestSMTDynamicAckClauseDeletionCallbackRunsDuringLemmaGC(t *testing.T) {
	manager := astTestManager(t)
	sort, values := smtUFConstants(t, manager, "dynamic-ack-gc-", 3)
	function, err := manager.MkFuncDecl(astTestSymbol(t, manager, "dynamic-ack-gc-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	applications := make([]ast.ASTHandle, len(values))
	for index, value := range values {
		applications[index], err = manager.MkApp(function, []ast.ASTHandle{value})
		astTestMust(t, err)
	}
	_, _, state := smtInternalizerFixture(t, manager, nil)
	dynamic := defaultSMTDynAckState()
	dynamic.Threshold = 1
	dynamic.Factor = 1
	dynamic.Equalities = true
	astTestMust(t, state.ConfigureDynamicAck(dynamic))
	maintenance := defaultSMTClauseMaintenanceConfig()
	maintenance.Half = true
	maintenance.Recent = 0
	astTestMust(t, state.ConfigureClauseMaintenance(maintenance))
	smtTestUserPush(t, state)
	astTestMust(t, state.dynamicAck.schedule.conflictCongruence(applications[0], applications[1], dynAckRoot))
	astTestMust(t, state.dynamicAck.schedule.usedEquality(applications[0], applications[1], applications[2], true))
	state.statistics.NumConflicts = 2
	astTestMust(t, state.dynamicAck.propagate())
	astTestMust(t, state.garbageCollectLearnedClauses())
	if len(state.clauses) != 1 || state.statistics.NumDelDynAck != 1 {
		t.Fatal("dynamic Ackermann lemma collection", len(state.clauses), state.statistics.NumDelDynAck)
	}
	astTestMust(t, state.PopScopes(1))
	if state.statistics.NumDelDynAck != 2 {
		t.Fatal("remaining dynamic Ackermann deletion", state.statistics.NumDelDynAck)
	}
}

// AUDIT.md smtcore_03-F4: with proofs enabled the triple instantiation
// reuses dyn_ack_justification for two arbitrary equal terms (here
// constants); its preconditions are SASSERTs only, and mk_proof builds a
// congruence with no premises.
func TestSMTDynamicAckTripleWithProofs(t *testing.T) {
	manager := astTestManager(t)
	astTestMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	_, values := smtUFConstants(t, manager, "dynamic-ack-triple-proof-", 3)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	config := defaultSMTDynAckState()
	config.Threshold = 1
	config.Factor = 1
	config.Equalities = true
	astTestMust(t, state.ConfigureDynamicAck(config))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.dynamicAck.schedule.usedEquality(values[0], values[1], values[2], true))
	state.statistics.NumConflicts = 1
	astTestMust(t, state.dynamicAck.propagate())
	if state.statistics.NumDynAck != 1 || len(state.clauses) != 1 || len(state.clauses[0].Literals) != 3 {
		t.Fatal("dynamic Ackermann triple with proofs", state.statistics.NumDynAck, len(state.clauses))
	}
	clause := state.clauses[0].clause
	if clause == nil {
		t.Fatal("dynamic Ackermann triple clause was not installed")
	}
	if _, ok := clause.justification.(*smtDynamicAckJustification); !ok {
		t.Fatalf("dynamic Ackermann triple justification %T", clause.justification)
	}
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	proof, err := builder.clauseProof(clause)
	if ast.ASTSourceDebug {
		// Debug builds check mk_monotonicity's SASSERT(f1->get_decl() ==
		// f2->get_decl()), which two distinct constants violate; the C++
		// debug build stops there too.
		if !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("debug build accepted a triple congruence over distinct decls", err)
		}
		astTestMust(t, state.PopScopes(1))
		return
	}
	astTestMust(t, err)
	fact, err := manager.GetProofFact(proof)
	astTestMust(t, err)
	conclusion, err := state.MkEqAtom(values[0], values[1])
	astTestMust(t, err)
	want, err := manager.MkOrArgs([]ast.ASTHandle{conclusion})
	astTestMust(t, err)
	if fact != want {
		t.Fatal("dynamic Ackermann triple proof fact", fact, want)
	}
	astTestMust(t, state.PopScopes(1))
}

// AUDIT.md smtcore_03-F5: dyn_ack_manager::mk_eq builds its atoms with
// context::mk_eq_atom, so argument pairs whose ids descend still give
// the id-ordered equality.
func TestSMTDynamicAckUsesCanonicalEqualityAtoms(t *testing.T) {
	manager := astTestManager(t)
	sort, values := smtUFConstants(t, manager, "dack-atom-", 2)
	function, err := manager.MkFuncDecl(astTestSymbol(t, manager, "dack-atom-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	app0, err := manager.MkApp(function, []ast.ASTHandle{values[1]})
	astTestMust(t, err)
	app1, err := manager.MkApp(function, []ast.ASTHandle{values[0]})
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	config := defaultSMTDynAckState()
	config.Threshold = 1
	config.Factor = 1
	astTestMust(t, state.ConfigureDynamicAck(config))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.dynamicAck.schedule.conflictCongruence(app0, app1, dynAckRoot))
	state.statistics.NumConflicts = 1
	astTestMust(t, state.dynamicAck.propagate())
	if state.statistics.NumDynAck != 1 {
		t.Fatal("dynamic Ackermann pair was not instantiated", state.statistics.NumDynAck)
	}
	ordered, err := manager.MkEq(values[0], values[1])
	astTestMust(t, err)
	reversed, err := manager.MkEq(values[1], values[0])
	astTestMust(t, err)
	if _, found := state.BoolVar(ordered); !found {
		t.Fatal("argument equality was not built by mk_eq_atom")
	}
	if _, found := state.BoolVar(reversed); found {
		t.Fatal("argument equality kept the descending operand order")
	}
	astTestMust(t, state.PopScopes(1))
}
