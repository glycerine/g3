package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func bvFixedSymbolic(t *testing.T, m *ast.ASTManager, s *SMTInternalizerState, bv *SMTBitVectorTheory, name string, width uint32) (*SMTEnode, SMTTheoryVar) {
	t.Helper()
	sort, err := m.BitVectorSort(width)
	astTestMust(t, err)
	word, err := m.MkConst(astTestSymbol(t, m, name), sort)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(word, false))
	node := s.context.enodes[word]
	return node, node.TheoryVar(bv.FamilyID())
}

// Assign without delivering atom notifications: the test invokes the exact
// fixed_var_eh callback independently of the context propagation scheduler.
func bvFixedAssign(t *testing.T, s *SMTInternalizerState, bv *SMTBitVectorTheory, v SMTTheoryVar, value uint64) {
	t.Helper()
	for index, literal := range bv.bitLiterals[v] {
		if literal.Var() == SMTTrueBoolVar {
			continue
		}
		bit := lbool.False
		if value&(uint64(1)<<uint(index)) != 0 {
			bit = lbool.True
		}
		if literal.Sign() {
			bit = bit.Negated()
		}
		astTestMust(t, s.AssignLiteral(NewSMTLiteral(literal.Var(), bit == lbool.False), NewSMTBooleanAxiom()))
	}
}

func bvFixedRepresentative(t *testing.T, bv *SMTBitVectorTheory, key smtBVFixedKey) SMTTheoryVar {
	t.Helper()
	value, found := bv.fixedVariables[key]
	if !found {
		t.Fatal("missing exact width/value cache key", key)
	}
	return value
}

func TestSMTBVFixedCacheCallbackOrderQueueAndProofParentOrder(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	left, lv := bvFixedSymbolic(t, m, s, bv, "fixed-old", 2)
	right, rv := bvFixedSymbolic(t, m, s, bv, "fixed-new", 2)
	bvFixedAssign(t, s, bv, lv, 1)
	bvFixedAssign(t, s, bv, rv, 1)
	before := len(s.pending)
	astTestMust(t, bv.fixedVariable(lv))
	key := smtBVFixedKey{width: 2, value: "1"}
	if bvFixedRepresentative(t, bv, key) != lv || len(s.pending) != before || bv.statistics.theoryToCoreEq != 0 {
		t.Fatal("first notification scanned another fixed word")
	}
	astTestMust(t, bv.fixedVariable(rv))
	if bvFixedRepresentative(t, bv, key) != lv || bv.statistics.theoryToCoreEq != 1 || bv.statistics.eqDynamic != 1 {
		t.Fatal("cache representative/counters", bv.fixedVariables, bv.statistics)
	}
	var owner *smtBVFixedEqualityJustification
	for _, entry := range s.pending[before:] {
		j, ok := entry.justification.Justification()
		if !ok {
			continue
		}
		if fixed, ok := j.(*smtBVFixedEqualityJustification); ok {
			owner = fixed
		}
	}
	if owner == nil || owner.first != rv || owner.second != lv || owner.Name() != "bv-fixed-eq" || owner.FromTheory() != bv.FamilyID() || !owner.InRegion() {
		t.Fatal("missing source queued equality owner", owner)
	}
	if left.root == right.root {
		t.Fatal("source assign_eq should enqueue, not merge immediately")
	}
	collector := &smtAntecedentCollector{}
	astTestMust(t, owner.MarkAntecedents(collector))
	wantMarks := []SMTLiteral{bv.bitLiterals[rv][0], bv.bitLiterals[rv][1].Negated(), bv.bitLiterals[lv][0], bv.bitLiterals[lv][1].Negated()}
	if fmt.Sprint(collector.literals) != fmt.Sprint(wantMarks) || len(collector.equalities) != 0 {
		t.Fatal("source antecedents visit whole first word then whole second", collector.literals, wantMarks)
	}
	builder := &smtConflictProofBuilder{state: s, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
	proof, err := builder.objectProof(owner)
	astTestMust(t, err)
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	wantFact, err := s.mkEqualityAtom(right.owner, left.owner)
	astTestMust(t, err)
	count, err := m.GetProofNumParents(proof)
	astTestMust(t, err)
	if fact != wantFact || count != 4 {
		t.Fatal("source fixed equality proof fact/count", fact, wantFact, count)
	}
	wantProofs := []SMTLiteral{wantMarks[0], wantMarks[2], wantMarks[1], wantMarks[3]}
	for i, literal := range wantProofs {
		parent, err := m.GetProofParent(proof, uint32(i))
		astTestMust(t, err)
		got, err := m.GetProofFact(parent)
		astTestMust(t, err)
		want, err := s.literalExpression(literal)
		astTestMust(t, err)
		if got != want {
			t.Fatal("source proof interleaves words at each bit", i, got, want)
		}
	}
	// Conflict resolution can ask before every parent is available. Source
	// still visits the remaining interleaved parents before returning null.
	missing := &smtConflictProofBuilder{state: s, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
	missing.literal[wantProofs[0]] = ast.ASTHandle{}
	deferred, err := missing.bvFixedEqualityProof(owner)
	astTestMust(t, err)
	if !deferred.IsNull() || len(missing.literal) != 4 {
		t.Fatal("missing parent short-circuited later proof visits", deferred, len(missing.literal))
	}
	astTestMust(t, s.propagateCoreEqualities())
	if left.root != right.root || bv.privateFind[lv] != lv || bv.privateFind[rv] != rv {
		t.Fatal("fixed equality must merge core classes without notifying its own BV forest")
	}
	beforeStats := bv.statistics
	astTestMust(t, bv.fixedVariable(rv))
	if bv.statistics != beforeStats {
		t.Fatal("same core class emitted another fixed equality")
	}
}

func TestSMTBVFixedCacheStaleAssignmentsAndDeletedVariables(t *testing.T) {
	for _, reason := range []string{"unfixed", "changed-value", "deleted"} {
		t.Run(reason, func(t *testing.T) {
			m, s, bv := bvOccurrenceFixture(t)
			_, base := bvFixedSymbolic(t, m, s, bv, "stale-base", 2)
			var old SMTTheoryVar
			if reason != "deleted" {
				_, old = bvFixedSymbolic(t, m, s, bv, "stale-old", 2)
			}
			astTestMust(t, s.PushScope())
			if reason == "deleted" {
				_, old = bvFixedSymbolic(t, m, s, bv, "stale-deleted", 2)
			}
			bvFixedAssign(t, s, bv, old, 1)
			astTestMust(t, bv.fixedVariable(old))
			key := smtBVFixedKey{width: 2, value: "1"}
			astTestMust(t, s.PopScopes(1))
			if bvFixedRepresentative(t, bv, key) != old {
				t.Fatal("source cache was incorrectly backtracked")
			}
			if reason == "changed-value" {
				bvFixedAssign(t, s, bv, old, 2)
			}
			bvFixedAssign(t, s, bv, base, 1)
			astTestMust(t, bv.fixedVariable(base))
			if bvFixedRepresentative(t, bv, key) != base || bv.statistics.theoryToCoreEq != 0 {
				t.Fatal("stale cache entry not replaced", reason, bv.fixedVariables, bv.statistics)
			}
		})
	}
}

func TestSMTBVFixedCacheReusedSlotWidthAndMatchingValue(t *testing.T) {
	for _, width := range []uint32{2, 3} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m, s, bv := bvOccurrenceFixture(t)
			astTestMust(t, s.PushScope())
			_, old := bvFixedSymbolic(t, m, s, bv, "old-slot", 2)
			bvFixedAssign(t, s, bv, old, 1)
			astTestMust(t, bv.fixedVariable(old))
			astTestMust(t, s.PopScopes(1))
			_, reused := bvFixedSymbolic(t, m, s, bv, "reused-slot", width)
			if reused != old {
				t.Fatal("fixture did not reuse actual theory slot")
			}
			bvFixedAssign(t, s, bv, reused, 1)
			_, current := bvFixedSymbolic(t, m, s, bv, "current-fixed", 2)
			bvFixedAssign(t, s, bv, current, 1)
			astTestMust(t, bv.fixedVariable(current))
			key := smtBVFixedKey{width: 2, value: "1"}
			if width == 2 {
				if bvFixedRepresentative(t, bv, key) != reused || bv.statistics.theoryToCoreEq != 1 {
					t.Fatal("source accepts a reused slot whose width/value still match")
				}
			} else if bvFixedRepresentative(t, bv, key) != current || bv.statistics.theoryToCoreEq != 0 {
				t.Fatal("wrong-width slot accepted as an equality")
			}
		})
	}
}

func TestSMTBVFixedEqualitySourceClauseSchedule(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	left, lv := bvFixedSymbolic(t, m, s, bv, "clause-first", 2)
	right, rv := bvFixedSymbolic(t, m, s, bv, "clause-second", 2)
	wordEq, err := bv.MkEqualityLiteral(left.owner, right.owner, true)
	astTestMust(t, err)
	before := len(s.clauses)
	astTestMust(t, bv.addFixedEquality(rv, lv))
	added := s.clauses[before:]
	if len(added) != 19 {
		t.Fatal("two Boolean equality gates, five native axioms per bit, final word clause", len(added))
	}
	var final []SMTLiteral
	for index := uint32(0); index < 2; index++ {
		e1, err := m.MkBitVectorBit2Bool(index, left.owner)
		astTestMust(t, err)
		e2, err := m.MkBitVectorBit2Bool(index, right.owner)
		astTestMust(t, err)
		eq, err := bv.MkEqualityLiteral(e1, e2, true)
		astTestMust(t, err)
		l1, l2 := bv.bitLiterals[lv][index], bv.bitLiterals[rv][index]
		want := [][]SMTLiteral{{l1, l2.Negated(), eq.Negated()}, {l1.Negated(), l2, eq.Negated()}, {l1, l2, eq}, {l1.Negated(), l2.Negated(), eq}, {eq, wordEq.Negated()}}
		for i, clause := range want {
			got := added[int(index)*9+4+i].Literals
			if boundClauseSet(got) != boundClauseSet(clause) {
				t.Fatal("source five-axiom order", index, i, got, clause)
			}
		}
		final = append(final, eq.Negated())
	}
	final = append(final, wordEq)
	got := added[len(added)-1].Literals
	if boundClauseSet(got) != boundClauseSet(final) {
		t.Fatal("source final word equality clause", got, final)
	}
}

func TestSMTBVFixedEqualityConstantAntecedentsAndClose(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	number, err := m.BitVectorNumeralUint64(1, 2)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(number, false))
	_, current := bvFixedSymbolic(t, m, s, bv, "constant-proof-word", 2)
	bvFixedAssign(t, s, bv, current, 1)
	astTestMust(t, bv.fixedVariable(current))
	var owner *smtBVFixedEqualityJustification
	for _, owned := range s.ownedJustifications {
		if j, ok := owned.(*smtBVFixedEqualityJustification); ok {
			owner = j
		}
	}
	if owner == nil {
		t.Fatal("missing actual numeral/word equality owner")
	}
	collector := &smtAntecedentCollector{}
	astTestMust(t, owner.MarkAntecedents(collector))
	if len(collector.literals) != 2 {
		t.Fatal("constant numeral bits became proof premises", collector.literals)
	}
	builder := &smtConflictProofBuilder{state: s, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
	proof, err := builder.objectProof(owner)
	astTestMust(t, err)
	count, err := m.GetProofNumParents(proof)
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("reserved constant variables must be excluded", count)
	}
	astTestMust(t, bv.Close())
	if bv.fixedVariables != nil {
		t.Fatal("fixed-value cache survived Close")
	}
}

func TestSMTBVFixedCacheKeyIncludesWidth(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	for _, width := range []uint32{1, 2, 3} {
		_, v := bvFixedSymbolic(t, m, s, bv, fmt.Sprintf("width-%d", width), width)
		bvFixedAssign(t, s, bv, v, 1)
		astTestMust(t, bv.fixedVariable(v))
		if bvFixedRepresentative(t, bv, smtBVFixedKey{width: width, value: "1"}) != v {
			t.Fatal("wrong width representative")
		}
	}
	if len(bv.fixedVariables) != 3 || bv.statistics.theoryToCoreEq != 0 {
		t.Fatal("cross-width values collided", bv.fixedVariables)
	}
}

func TestSMTBVFixedCacheReusedSlotForBVToInt(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	astTestMust(t, s.PushScope())
	_, _ = bvFixedSymbolic(t, m, s, bv, "retired-prefix", 1)
	_, old := bvFixedSymbolic(t, m, s, bv, "retired-cached", 2)
	bvFixedAssign(t, s, bv, old, 1)
	astTestMust(t, bv.fixedVariable(old))
	astTestMust(t, s.PopScopes(1))
	argument, _ := bvFixedSymbolic(t, m, s, bv, "conversion-argument", 3)
	conversion, err := m.MkBitVectorToInt(argument.owner)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(conversion, false))
	node := s.context.enodes[conversion]
	if node == nil || node.TheoryVar(bv.FamilyID()) != old {
		t.Fatal("native BV-to-Int consumer did not reuse cached theory slot")
	}
	isBV, err := m.IsBitVector(node.owner)
	astTestMust(t, err)
	if isBV {
		t.Fatal("fixture did not reach non-BV slot guard")
	}
	_, current := bvFixedSymbolic(t, m, s, bv, "conversion-current-fixed", 2)
	bvFixedAssign(t, s, bv, current, 1)
	astTestMust(t, bv.fixedVariable(current))
	if bvFixedRepresentative(t, bv, smtBVFixedKey{width: 2, value: "1"}) != current || bv.statistics.theoryToCoreEq != 0 {
		t.Fatal("non-BV cache representative accepted")
	}
}

func TestSMTBVFixedEqualityAllocationRefusalRetainsRepresentativeAndRetries(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	_, left := bvFixedSymbolic(t, m, s, bv, "refusal-first", 257)
	_, right := bvFixedSymbolic(t, m, s, bv, "refusal-second", 257)
	bvFixedAssign(t, s, bv, left, 0)
	bvFixedAssign(t, s, bv, right, 0)
	astTestMust(t, bv.fixedVariable(left))
	booleans := len(s.boolExpr)
	astTestMust(t, s.PushScope())
	m.Budget().FailAfter(0)
	err := bv.fixedVariable(right)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected actual cold equality/bit-clause allocation refusal", err)
	}
	key := smtBVFixedKey{width: 257, value: "0"}
	if bvFixedRepresentative(t, bv, key) != left || bv.statistics.theoryToCoreEq != 1 || bv.statistics.eqDynamic != 1 {
		t.Fatal("source failure prefix changed cache/counters", bv.statistics)
	}
	for _, entry := range s.pending {
		if j, ok := entry.justification.Justification(); ok {
			if _, fixed := j.(*smtBVFixedEqualityJustification); fixed {
				t.Fatal("fixed owner published before all equality axioms succeeded")
			}
		}
	}
	astTestMust(t, s.PopScopes(1))
	if len(s.boolExpr) != booleans || bv.NumVariables() != 2 {
		t.Fatal("failed prefix survived scoped retirement")
	}
	astTestMust(t, bv.fixedVariable(right))
	if bvFixedRepresentative(t, bv, key) != left || bv.statistics.theoryToCoreEq != 2 || bv.statistics.eqDynamic != 2 {
		t.Fatal("retry did not preserve native representative/counters")
	}
}
