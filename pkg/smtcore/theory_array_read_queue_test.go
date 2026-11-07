package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtArrayQueueIndices(t *testing.T, m *ast.ASTManager, prefix string, count int) (ast.ASTHandle, []ast.ASTHandle) {
	t.Helper()
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, prefix+"sort"), nil)
	astTestMust(t, err)
	values := make([]ast.ASTHandle, count)
	for i := range values {
		values[i], err = m.MkConst(astTestSymbol(t, m, fmt.Sprint(prefix, i)), sort)
		astTestMust(t, err)
		astTestPin(t, m, values[i])
	}
	return sort, values
}

func TestSMTArrayStoreReadQueuesProofBranchesAndRelevance(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m := astTestManager(t)
			if proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			family, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			theory, err := NewSMTArrayTheory(family)
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(theory))
			indexSort, indices := smtArrayQueueIndices(t, m, "queued-index-", 3)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort}), boolean)
			astTestMust(t, err)
			array, err := m.MkConst(astTestSymbol(t, m, "queued-store-base"), sort)
			astTestMust(t, err)
			value := smtRelevancyBool(t, m, "queued-store-value")
			store, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array, indices[0], value})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(store, false))
			if len(theory.storeTodo) != 1 || len(theory.selectTrail) != 0 {
				t.Fatal("store axiom was not deferred")
			}
			read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{store, indices[1]})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(read, true))
			if len(theory.readTodo) != 0 {
				t.Fatal("irrelevant read registered its parent eagerly")
			}
			astTestMust(t, state.markRelevant(read))
			astTestMust(t, state.propagateTheoryRelevancy())
			if len(theory.readTodo) != 1 || theory.statistics.axiom2 != 1 {
				t.Fatal("conditional read axiom was not deferred")
			}
			astTestMust(t, state.Internalize(indices[2], false))
			astTestMust(t, state.addEquality(state.context.enodes.get(indices[2]), state.context.enodes.get(indices[1]), NewSMTEqualityAxiom()))
			aliasRead, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{store, indices[2]})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(aliasRead, true))
			astTestMust(t, state.markRelevant(aliasRead))
			astTestMust(t, state.propagateTheoryRelevancy())
			if len(theory.readTodo) != 1 {
				t.Fatal("equal index roots did not suppress duplicate work")
			}
			astTestMust(t, theory.Propagate())
			if theory.CanPropagate() {
				t.Fatal("store/read propagation did not drain its queues")
			}
			sameIndexRead, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{store, indices[0]})
			astTestMust(t, err)
			unitEquality, err := state.mkEqualityAtom(sameIndexRead, value)
			astTestMust(t, err)
			_, unitVariable := state.BoolVar(unitEquality)
			if unitVariable != proofs {
				t.Fatal("store axiom did not select the source proof/non-proof branch", proofs, unitVariable)
			}
			_, err = state.PropagateCore()
			astTestMust(t, err)
			if state.context.enodes.get(sameIndexRead).root != state.context.enodes.get(value).root && !proofs {
				t.Fatal("non-proof store axiom did not merge read and value")
			}
			fallback, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, indices[1]})
			astTestMust(t, err)
			consequence, err := state.mkEqualityAtom(read, fallback)
			astTestMust(t, err)
			if state.relevancy.IsRelevant(consequence) {
				t.Fatal("read consequence was relevant before its index premise")
			}
			antecedent, err := theory.MkEqualityLiteral(indices[0], indices[1], true)
			astTestMust(t, err)
			if !state.relevancy.IsRelevant(state.boolExpr[antecedent.Var()]) {
				t.Fatal("index equality was not relevant")
			}
			astTestMust(t, state.AssignLiteral(antecedent.Negated(), NewSMTBooleanAxiom()))
			_, err = state.PropagateCore()
			astTestMust(t, err)
			if !state.relevancy.IsRelevant(consequence) {
				t.Fatal("false index equality did not activate the read consequence")
			}
		})
	}
}

func TestSMTArrayReadQueueRechecksIndexRootsAndClearsPoppedWork(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	indexSort, indices := smtArrayQueueIndices(t, m, "delayed-index-", 2)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort}), boolean)
	astTestMust(t, err)
	array, err := m.MkConst(astTestSymbol(t, m, "delayed-read-base"), sort)
	astTestMust(t, err)
	value := smtRelevancyBool(t, m, "delayed-read-value")
	store, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array, indices[0], value})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(store, false))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, state.PushScope())
		read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{store, indices[1]})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(read, true))
		astTestMust(t, state.markRelevant(read))
		astTestMust(t, state.propagateTheoryRelevancy())
		if len(theory.readTodo) != 1 || len(theory.readFingerprintTrail) != 1 {
			t.Fatal("child read was not queued")
		}
		if attempt == 0 {
			astTestMust(t, state.addEquality(state.context.enodes.get(indices[1]), state.context.enodes.get(indices[0]), NewSMTEqualityAxiom()))
			before := len(state.clauses)
			astTestMust(t, theory.Propagate())
			if len(state.clauses) != before {
				t.Fatal("queued read used stale unequal index roots")
			}
		}
		astTestMust(t, state.PopScopes(1))
		if theory.CanPropagate() || len(theory.readFingerprintTrail) != 0 || len(theory.readFingerprints) != 0 {
			t.Fatal("child work or fingerprint survived scope pop")
		}
	}
}

func TestSMTArrayReadFingerprintOriginalTupleAfterRootChange(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	_, indices := smtArrayQueueIndices(t, m, "fingerprint-root-", 4)
	for _, index := range indices {
		astTestMust(t, state.Internalize(index, false))
	}
	first, second, other := state.context.enodes.get(indices[0]), state.context.enodes.get(indices[1]), state.context.enodes.get(indices[2])
	// Constructor identity is opaque to the fingerprint table.
	constructor := indices[3]
	tuple := []*SMTEnode{first, other}
	if !theory.insertArrayReadFingerprint(constructor, tuple) {
		t.Fatal("initial tuple was rejected")
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addEquality(first, second, NewSMTEqualityAxiom()))
	if first.root != second || first.root == first {
		t.Fatal("test did not move the original fingerprint's root")
	}
	if theory.insertArrayReadFingerprint(constructor, tuple) || len(theory.readFingerprintTrail) != 1 {
		t.Fatal("original tuple was forgotten when its root changed")
	}
	// Native fingerprints are snapshots, not continually rehashed classes:
	// the new representative tuple can be inserted once in the child scope.
	if !theory.insertArrayReadFingerprint(constructor, []*SMTEnode{second, other}) {
		t.Fatal("new root tuple was rejected")
	}
	if theory.insertArrayReadFingerprint(constructor, tuple) || theory.insertArrayReadFingerprint(constructor, []*SMTEnode{second, other}) {
		t.Fatal("duplicate original or canonical tuple was accepted")
	}
	astTestMust(t, state.PopScopes(1))
	if first.root != first || second.root != second || len(theory.readFingerprintTrail) != 1 {
		t.Fatal("scope pop lost the original fingerprint or retained the child root tuple")
	}
	if theory.insertArrayReadFingerprint(constructor, tuple) || !theory.insertArrayReadFingerprint(constructor, []*SMTEnode{second, other}) {
		t.Fatal("fingerprint lookup did not recover after scope pop")
	}
}

func TestSMTArrayQueuedMultidimensionalReadConflictProof(t *testing.T) {
	m, solver := smtAssertedNNFFixture(t, true)
	solver.kernel.preprocess = false
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	indexSort, indices := smtArrayQueueIndices(t, m, "proof-read-index-", 4)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort, indexSort}), boolean)
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "proof-read-base"), sort)
	astTestMust(t, err)
	b, err := m.MkConst(astTestSymbol(t, m, "proof-read-alias"), sort)
	astTestMust(t, err)
	value := smtRelevancyBool(t, m, "proof-read-value")
	store, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, indices[0], indices[1], value})
	astTestMust(t, err)
	alias, err := m.MkEq(b, store)
	astTestMust(t, err)
	equalIndex, err := m.MkEq(indices[0], indices[2])
	astTestMust(t, err)
	differentIndex, err := m.MkEq(indices[1], indices[3])
	astTestMust(t, err)
	differentIndex, err = m.MkNot(differentIndex)
	astTestMust(t, err)
	for _, assertion := range []ast.ASTHandle{alias, equalIndex, differentIndex} {
		astTestMust(t, solver.Assert(assertion))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("base array constraints", status)
	}
	astTestMust(t, solver.Push())
	left, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, indices[2], indices[3]})
	astTestMust(t, err)
	right, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, indices[2], indices[3]})
	astTestMust(t, err)
	equalReads, err := m.MkEq(left, right)
	astTestMust(t, err)
	notEqualReads, err := m.MkNot(equalReads)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(notEqualReads))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("late alias read did not conflict", status)
	}
	proof, err := solver.Proof()
	astTestMust(t, err)
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	if !m.IsFalse(fact) {
		t.Fatal("queued read conflict lost its false proof fact")
	}
	astTestMust(t, solver.Pop(1))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("read conflict scope did not recover", status)
	}
}
