package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func bvNativeWord(t *testing.T, m *ast.ASTManager, s *SMTInternalizerState, bv *SMTBitVectorTheory, bits ...ast.ASTHandle) (*SMTEnode, SMTTheoryVar) {
	t.Helper()
	word, err := m.MkBitVectorFromBits(bits)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(word, false))
	node := s.context.enodes.get(word)
	return node, node.TheoryVar(bv.FamilyID())
}

func bvNativeCoreMerge(t *testing.T, s *SMTInternalizerState, left, right *SMTEnode) {
	t.Helper()
	astTestMust(t, s.addEquality(left, right, NewSMTEqualityAxiom()))
	astTestMust(t, s.PropagateTheoryEvents())
}

func TestSMTBVNativeForestSizeTiesCircularOrderAndScopedRetirement(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	var variables []SMTTheoryVar
	for i := 0; i < 3; i++ {
		p := smtRelevancyBool(t, m, fmt.Sprintf("forest-%d", i))
		_, v := bvNativeWord(t, m, s, bv, p)
		variables = append(variables, v)
	}
	a, b, c := variables[0], variables[1], variables[2]
	astTestMust(t, s.PushScope())
	astTestMust(t, bv.NewEquality(a, b))
	astTestMust(t, bv.NewEquality(a, c))
	for _, v := range variables {
		r, err := bv.privateRoot(v)
		astTestMust(t, err)
		if r != b {
			t.Fatal("source union-by-size/tie root", v, r, b)
		}
	}
	if bv.privateSize[b] != 3 || bv.privateNext[a] != b || bv.privateNext[b] != c || bv.privateNext[c] != a {
		t.Fatal("source circular class order", bv.privateNext, bv.privateSize)
	}
	before := len(bv.privateTrail)
	astTestMust(t, bv.NewEquality(c, a))
	if len(bv.privateTrail) != before {
		t.Fatal("same-root merge trailed")
	}
	p := smtRelevancyBool(t, m, "forest-retired")
	child, childVar := bvNativeWord(t, m, s, bv, p)
	astTestMust(t, bv.NewEquality(childVar, a))
	childOwner := child.owner
	astTestMust(t, s.PopScopes(1))
	if s.context.enodes.get(childOwner) != nil {
		t.Fatal("child enode survived pop")
	}
	if len(bv.privateFind) != 3 || len(bv.bitLiterals) != 3 || len(bv.zeroOneBits) != 3 || len(bv.watchedPosition) != 3 {
		t.Fatal("private state not truncated")
	}
	for _, v := range variables {
		if bv.privateFind[v] != v || bv.privateNext[v] != v || bv.privateSize[v] != 1 {
			t.Fatal("forest undo", v)
		}
	}
	q := smtRelevancyBool(t, m, "forest-reused")
	_, reused := bvNativeWord(t, m, s, bv, q)
	if reused != childVar || bv.privateFind[reused] != reused || bv.privateSize[reused] != 1 || len(bv.zeroOneBits[reused]) != 0 {
		t.Fatal("stale forest slot")
	}
	astTestMust(t, bv.Close())
	if bv.privateFind != nil || bv.privateNext != nil || bv.privateSize != nil || bv.privateTrail != nil || bv.zeroOneBits != nil || bv.watchedPosition != nil || bv.propagationQueue != nil || bv.mergeAux[0] != nil || bv.mergeAux[1] != nil {
		t.Fatal("private owners survived Close")
	}
}

func TestSMTBVNativeMergeRepeatedPropagationAndSourceJustifications(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	var p []ast.ASTHandle
	for i := 0; i < 5; i++ {
		p = append(p, smtRelevancyBool(t, m, fmt.Sprintf("chain-%d", i)))
	}
	left, lv := bvNativeWord(t, m, s, bv, p[3], p[2], p[1], p[0])
	right, rv := bvNativeWord(t, m, s, bv, p[4], p[3], p[2], p[1])
	astTestMust(t, s.PushScope())
	first, err := s.Literal(p[0])
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(first, NewSMTBooleanAxiom()))
	bvNativeCoreMerge(t, s, left, right)
	for i, expr := range p {
		literal, err := s.Literal(expr)
		astTestMust(t, err)
		value, err := s.Assignment(literal)
		astTestMust(t, err)
		if value != lbool.True {
			t.Fatal("repeated merge propagation missed chain bit", i, value)
		}
		if i == 0 {
			continue
		}
		owner, ok := s.booleanData[literal.Var()].Justification().Justification()
		j, typed := owner.(*smtBVBitEqualityJustification)
		if !ok || !typed || j.Name() != "bv-bit-eq" || j.FromTheory() != bv.FamilyID() || !j.InRegion() {
			t.Fatalf("native BV justification for bit%d: %T", i, owner)
		}
		collector := &smtAntecedentCollector{}
		astTestMust(t, j.MarkAntecedents(collector))
		if len(collector.equalities) != 1 || len(collector.literals) != 1 || collector.literals[0] != j.antecedent {
			t.Fatal("native bit antecedents", collector)
		}
	}
	if bv.statistics.bit2core != 4 || len(bv.propagationQueue) != 0 {
		t.Fatal("native assignment count/queue", bv.statistics, len(bv.propagationQueue))
	}
	lroot, err := bv.privateRoot(lv)
	astTestMust(t, err)
	rroot, err := bv.privateRoot(rv)
	astTestMust(t, err)
	if lroot != rroot {
		t.Fatal("native classes not linked after propagation")
	}
	astTestMust(t, s.PopScopes(1))
	for _, expr := range p {
		literal, err := s.Literal(expr)
		astTestMust(t, err)
		value, err := s.Assignment(literal)
		astTestMust(t, err)
		if value != lbool.Undef {
			t.Fatal("assignment survived pop")
		}
	}
	if bv.privateFind[lv] != lv || bv.privateFind[rv] != rv {
		t.Fatal("scoped private link survived")
	}
}

func TestSMTBVNativeConstantSummaryConflictCommitsAndUnmerges(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p, q, r := smtRelevancyBool(t, m, "summary-p"), smtRelevancyBool(t, m, "summary-q"), smtRelevancyBool(t, m, "summary-r")
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	left, lv := bvNativeWord(t, m, s, bv, truth, p)
	right, rv := bvNativeWord(t, m, s, bv, q, truth)
	astTestMust(t, s.PushScope())
	bvNativeCoreMerge(t, s, left, right)
	root, err := bv.privateRoot(lv)
	astTestMust(t, err)
	if root != lv || len(bv.zeroOneBits[root]) != 2 || bv.zeroOneBits[root][0].variable != lv || bv.zeroOneBits[root][1].variable != rv {
		t.Fatal("source summary append order", bv.zeroOneBits[root])
	}
	astTestMust(t, s.PushScope())
	other, ov := bvNativeWord(t, m, s, bv, falsity, r)
	before := bv.statistics.diseqStatic
	bvNativeCoreMerge(t, s, left, other)
	otherRoot, err := bv.privateRoot(ov)
	astTestMust(t, err)
	if !s.inconsistent || bv.statistics.diseqStatic != before+1 || otherRoot != root {
		t.Fatal("source logical conflict must still commit private link", s.inconsistent, bv.statistics, otherRoot, root)
	}
	for _, aux := range bv.mergeAux {
		for _, v := range aux {
			if v != SMTNullTheoryVar {
				t.Fatal("merge auxiliary not reset", aux)
			}
		}
	}
	astTestMust(t, s.PopScopes(1))
	if s.inconsistent || len(bv.zeroOneBits[root]) != 2 {
		t.Fatal("child summary/conflict undo")
	}
	astTestMust(t, s.PopScopes(1))
	if len(bv.zeroOneBits[lv]) != 1 || len(bv.zeroOneBits[rv]) != 1 || bv.privateFind[lv] != lv || bv.privateFind[rv] != rv {
		t.Fatal("source summary suffix undo", bv.zeroOneBits)
	}
}

func TestSMTBVNativeAssignUsesClassAndRepeatedPositions(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p, q, r := smtRelevancyBool(t, m, "assign-p"), smtRelevancyBool(t, m, "assign-q"), smtRelevancyBool(t, m, "assign-r")
	left, lv := bvNativeWord(t, m, s, bv, p, p)
	right, rv := bvNativeWord(t, m, s, bv, q, r)
	bvNativeCoreMerge(t, s, left, right)
	literal, err := s.Literal(p)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(literal.Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, bv.Assign(literal.Var(), false))
	for _, expr := range []ast.ASTHandle{q, r} {
		lit, err := s.Literal(expr)
		astTestMust(t, err)
		value, err := s.Assignment(lit)
		astTestMust(t, err)
		if value != lbool.False {
			t.Fatal("reused Boolean occurrence not propagated", value)
		}
	}
	if bv.statistics.bit2core != 2 || bv.watchedPosition[lv] != 0 || bv.watchedPosition[rv] != 0 {
		t.Fatal("source count/watched positions", bv.statistics, bv.watchedPosition)
	}
}

func TestSMTBVNativeMergeCallbackErrorStopsBeforeLink(t *testing.T) {
	_, report := newLRASourceAssertionContext(t)
	m, s, bv := bvOccurrenceFixture(t)
	p, q := smtRelevancyBool(t, m, "error-p"), smtRelevancyBool(t, m, "error-q")
	_, lv := bvNativeWord(t, m, s, bv, p)
	_, rv := bvNativeWord(t, m, s, bv, p, q)
	before := len(bv.privateTrail)
	err := bv.NewEquality(lv, rv)
	if diagnostic.DebugBuild {
		bvInvariantFailure(t, err, report, 1394, "bits1.size() == bits2.size()")
	} else if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("callback refusal", err)
	}
	if len(bv.privateTrail) != before || bv.privateFind[lv] != lv || bv.privateFind[rv] != rv {
		t.Fatal("callback refusal linked forest", err, bv.privateFind)
	}
}

func TestSMTBVNativeWatchedPositionWrapsAfterPop(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p, q, r := smtRelevancyBool(t, m, "watch-p"), smtRelevancyBool(t, m, "watch-q"), smtRelevancyBool(t, m, "watch-r")
	_, v := bvNativeWord(t, m, s, bv, p, q, r)
	astTestMust(t, s.PushScope())
	for _, expr := range []ast.ASTHandle{p, q} {
		literal, err := s.Literal(expr)
		astTestMust(t, err)
		astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
		astTestMust(t, bv.Assign(literal.Var(), true))
	}
	if bv.watchedPosition[v] != 2 {
		t.Fatal("watch did not advance", bv.watchedPosition)
	}
	astTestMust(t, s.PopScopes(1))
	if bv.watchedPosition[v] != 2 {
		t.Fatal("source watcher should remain untrailed")
	}
	literal, err := s.Literal(r)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	astTestMust(t, bv.Assign(literal.Var(), true))
	if bv.watchedPosition[v] != 0 {
		t.Fatal("watch did not wrap to restored unassigned prefix")
	}
}

func TestSMTBVNativeBitEqualityProofParentsAndConstantAntecedent(t *testing.T) {
	for _, constant := range []bool{false, true} {
		t.Run(fmt.Sprint(constant), func(t *testing.T) {
			m, s, bv := bvOccurrenceFixture(t)
			astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			p := smtRelevancyBool(t, m, "proof-bit-p")
			if constant {
				var err error
				p, err = m.MkTrue()
				astTestMust(t, err)
			}
			q := smtRelevancyBool(t, m, "proof-bit-q")
			left, _ := bvNativeWord(t, m, s, bv, p)
			right, _ := bvNativeWord(t, m, s, bv, q)
			astTestMust(t, s.PushScope())
			if !constant {
				lit, err := s.Literal(p)
				astTestMust(t, err)
				astTestMust(t, s.AssignLiteral(lit, NewSMTBooleanAxiom()))
			}
			fact, err := m.MkEq(left.owner, right.owner)
			astTestMust(t, err)
			hyp, err := m.MkProofHypothesis(fact)
			astTestMust(t, err)
			wrapper, err := NewSMTJustificationProofWrapper(m, hyp, true)
			astTestMust(t, err)
			s.ownedJustifications = append(s.ownedJustifications, wrapper)
			astTestMust(t, s.addEquality(left, right, NewSMTEqualityJustification(wrapper)))
			astTestMust(t, s.PropagateTheoryEvents())
			qlit, err := s.Literal(q)
			astTestMust(t, err)
			owner, ok := s.booleanData[qlit.Var()].Justification().Justification()
			j, typed := owner.(*smtBVBitEqualityJustification)
			if !ok || !typed {
				t.Fatalf("expected actual bit equality propagation: %T", owner)
			}
			collector := &smtAntecedentCollector{}
			astTestMust(t, j.MarkAntecedents(collector))
			wantParents := uint32(2)
			wantLiterals := 1
			if constant {
				wantParents = 1
				wantLiterals = 0
			}
			if len(collector.equalities) != 1 || len(collector.literals) != wantLiterals {
				t.Fatal("source constant antecedent filter", collector)
			}
			builder := &smtConflictProofBuilder{state: s, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
			proof, err := builder.objectProof(owner)
			astTestMust(t, err)
			result, err := m.GetProofFact(proof)
			astTestMust(t, err)
			parents, err := m.GetProofNumParents(proof)
			astTestMust(t, err)
			if result != q || parents != wantParents {
				t.Fatal("native bit proof fact/parent count", result, parents, wantParents)
			}
			first, err := m.GetProofParent(proof, 0)
			astTestMust(t, err)
			firstFact, err := m.GetProofFact(first)
			astTestMust(t, err)
			eq, err := m.MkEq(j.first.owner, j.second.owner)
			astTestMust(t, err)
			if firstFact != eq {
				t.Fatal("source proof must visit equality before literal")
			}
			if !constant {
				second, err := m.GetProofParent(proof, 1)
				astTestMust(t, err)
				secondFact, err := m.GetProofFact(second)
				astTestMust(t, err)
				if secondFact != p {
					t.Fatal("source antecedent proof fact")
				}
			}
			astTestMust(t, s.PopScopes(1))
			for _, owned := range s.ownedJustifications {
				if owned == owner || owned == wrapper {
					t.Fatal("scoped bit-equality proof owner survived pop")
				}
			}
		})
	}
}

func TestSMTBVNativeFalseConsequentConflictAndRollback(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p := smtRelevancyBool(t, m, "conflict-bit-p")
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	left, lv := bvNativeWord(t, m, s, bv, p)
	right, rv := bvNativeWord(t, m, s, bv, falsity)
	astTestMust(t, s.PushScope())
	plit, err := s.Literal(p)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(plit, NewSMTBooleanAxiom()))
	bvNativeCoreMerge(t, s, left, right)
	conflict, exists := s.Conflict()
	owner, object := conflict.Justification()
	j, typed := owner.(*smtBVBitEqualityJustification)
	if !exists || !object || !typed || j.consequent != SMTFalseLiteral || bv.statistics.conflicts != 1 || bv.statistics.bit2core != 1 {
		t.Fatalf("source false-consequent conflict: %T, stats%v", owner, bv.statistics)
	}
	lroot, err := bv.privateRoot(lv)
	astTestMust(t, err)
	rroot, err := bv.privateRoot(rv)
	astTestMust(t, err)
	if lroot != rroot {
		t.Fatal("logical bit conflict prevented source link commit")
	}
	astTestMust(t, s.PopScopes(1))
	if s.inconsistent || bv.privateFind[lv] != lv || bv.privateFind[rv] != rv {
		t.Fatal("conflicting merge did not undo")
	}
}

func TestSMTBVNativeDisplayUsesPrivateRootsAndActualLiteralVectors(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	var empty bytes.Buffer
	astTestMust(t, bv.Display(&empty))
	if empty.Len() != 0 {
		t.Fatal("empty BV display")
	}
	p, q := smtRelevancyBool(t, m, "display-native-p"), smtRelevancyBool(t, m, "display-native-q")
	nq, err := m.MkNot(q)
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	left, lv := bvNativeWord(t, m, s, bv, p, nq, truth)
	right, rv := bvNativeWord(t, m, s, bv, q, p, truth)
	// Direct source callback deliberately leaves core enode roots separate,
	// proving that the printer reads the BV forest rather than the core root.
	astTestMust(t, bv.NewEquality(lv, rv))
	if left.root == right.root {
		t.Fatal("fixture lost distinct core roots")
	}
	leftID, err := left.OwnerID()
	astTestMust(t, err)
	rightID, err := right.OwnerID()
	astTestMust(t, err)
	pid, err := m.GetASTID(p)
	astTestMust(t, err)
	qid, err := m.GetASTID(q)
	astTestMust(t, err)
	want := fmt.Sprintf("Theory bv:\nv%-4d #%-4d -> #%-4d, bits: #%d (not #%d) true\nv%-4d #%-4d -> #%-4d, bits: #%d #%d true\natoms:\n#%d -> #%d[1] #%d[0]\n#%d -> #%d[0] #%d[1]\n", lv, leftID, rightID, pid, qid, rv, rightID, rightID, qid, pid, pid, rightID, leftID, qid, rightID, leftID)
	var out bytes.Buffer
	astTestMust(t, bv.Display(&out))
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	var dispatch bytes.Buffer
	astTestMust(t, s.DisplayTheories(&dispatch))
	if dispatch.String() != want {
		t.Fatal("context did not dispatch native BV display", dispatch.String())
	}
	before := m.Stats()
	rng := s.random
	for cut := 0; cut < len(want); cut++ {
		w := &smtLRAOutputPrefix{remaining: cut}
		if err = bv.Display(w); !errors.Is(err, io.ErrClosedPipe) || w.text.String() != want[:cut] {
			t.Fatal(cut, err, w.text.String())
		}
	}
	if err = bv.Display(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if m.Stats() != before || s.random != rng {
		t.Fatal("native display mutated state")
	}
	if err = bv.Display(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if err = bv.DisplayVariable(&out, SMTTheoryVar(100)); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	numeral, err := m.BitVectorNumeralUint64(5, 3)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(numeral, false))
	nv := s.context.enodes.get(numeral).TheoryVar(bv.FamilyID())
	id, err := m.GetASTID(numeral)
	astTestMust(t, err)
	out.Reset()
	astTestMust(t, bv.DisplayVariable(&out, nv))
	want = fmt.Sprintf("v%-4d #%-4d -> #%-4d, bits: true false true, value: 5\n", nv, id, id)
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
}

func TestSMTBVNativeDisequalitySkipsComplementaryPositions(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p, q := smtRelevancyBool(t, m, "static-skip-p"), smtRelevancyBool(t, m, "static-skip-q")
	np, err := m.MkNot(p)
	astTestMust(t, err)
	_, left := bvNativeWord(t, m, s, bv, p, q)
	_, right := bvNativeWord(t, m, s, bv, np, q)
	before := len(s.clauses)
	astTestMust(t, bv.NewDisequality(left, right))
	if bv.statistics.diseqStatic != 1 || bv.statistics.diseqDynamic != 0 || len(s.clauses) != before {
		t.Fatal("static complementary bit expanded dynamically", bv.statistics, len(s.clauses), before)
	}
}

func TestSMTBVNativeEqualityInternalizationIsSourceNoop(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p, q := smtRelevancyBool(t, m, "no-expand-p"), smtRelevancyBool(t, m, "no-expand-q")
	left, lv := bvNativeWord(t, m, s, bv, p)
	right, rv := bvNativeWord(t, m, s, bv, q)
	expression, err := m.MkEq(left.owner, right.owner)
	astTestMust(t, err)
	clauses, booleans := len(s.clauses), len(s.boolExpr)
	astTestMust(t, s.Internalize(expression, true))
	if len(s.clauses) != clauses || len(s.boolExpr) != booleans+1 || bv.statistics.eqDynamic != 0 {
		t.Fatal("source no-op equality callback expanded a Boolean definition", len(s.clauses), clauses, len(s.boolExpr), booleans, bv.statistics)
	}
	if bv.privateFind[lv] != lv || bv.privateFind[rv] != rv {
		t.Fatal("unasserted equality merged private classes")
	}
	literal, err := s.Literal(expression)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	consistent, err := s.PropagateCore()
	astTestMust(t, err)
	lroot, err := bv.privateRoot(lv)
	astTestMust(t, err)
	rroot, err := bv.privateRoot(rv)
	astTestMust(t, err)
	if !consistent || lroot != rroot {
		t.Fatal("asserted equality failed to reach native class merge")
	}
}
