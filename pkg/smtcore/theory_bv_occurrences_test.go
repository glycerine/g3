package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func bvOccurrenceFixture(t *testing.T) (*ast.ASTManager, *SMTInternalizerState, *SMTBitVectorTheory) {
	t.Helper()
	m, _ := smtModelGeneratorFixture(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	family, err := m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	bv, err := NewSMTBitVectorTheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(bv))
	return m, s, bv
}

func TestSMTBVBitOccurrencesStaticDisequalitiesPerPosition(t *testing.T) {
	for _, tc := range []struct {
		name    string
		indices []int
		want    uint64
	}{
		{"two-complementary-positions", []int{0, 1}, 2},
		{"different-positions", []int{1, 0}, 0},
		{"different-widths", []int{0, 1, 0}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, s, bv := bvOccurrenceFixture(t)
			s.relevancy.enabled = true
			p := smtRelevancyBool(t, m, "diseq-p")
			q := smtRelevancyBool(t, m, "diseq-q")
			base, err := m.MkBitVectorFromBits([]ast.ASTHandle{p, q})
			astTestMust(t, err)
			astTestMust(t, s.Internalize(base, false))
			np, err := m.MkNot(p)
			astTestMust(t, err)
			nq, err := m.MkNot(q)
			astTestMust(t, err)
			negated := []ast.ASTHandle{np, nq}
			args := make([]ast.ASTHandle, len(tc.indices))
			for i, index := range tc.indices {
				args[i] = negated[index]
			}
			other, err := m.MkBitVectorFromBits(args)
			astTestMust(t, err)
			astTestMust(t, s.Internalize(other, false))
			if bv.statistics.diseqStatic != tc.want {
				t.Fatal("source emits one static disequality for each matching complementary position", bv.statistics.diseqStatic, tc.want)
			}
			if tc.want == 0 {
				return
			}
			eq, err := bv.MkEqualityLiteral(other, base, true)
			astTestMust(t, err)
			value, err := s.Assignment(eq)
			astTestMust(t, err)
			if value != lbool.False {
				t.Fatal("static disequality was not asserted", value)
			}
			eqExpr, err := s.BoolExpression(eq.Var())
			astTestMust(t, err)
			for _, owner := range []ast.ASTHandle{base, other} {
				count := 0
				for _, h := range s.relevancy.handlers[owner] {
					if h.kind == smtRelevancyPair && h.target == eqExpr {
						count++
					}
				}
				if uint64(count) != tc.want {
					t.Fatal("source pair relevancy registrations", count, tc.want)
				}
			}
		})
	}
}

func TestSMTBVBitOccurrencesPreservePositionsSignsAndSourceHeadOrder(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p := smtRelevancyBool(t, m, "occurrence-p")
	q := smtRelevancyBool(t, m, "occurrence-q")
	// Boolean order deliberately differs from AST construction order.
	astTestMust(t, s.Internalize(q, true))
	notQ, err := m.MkNot(q)
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	word, err := m.MkBitVectorFromBits([]ast.ASTHandle{p, q, p, notQ, truth, falsity})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(word, false))
	variable := s.context.enodes.get(word).TheoryVar(bv.FamilyID())
	pv, _ := s.BoolVar(p)
	qv, _ := s.BoolVar(q)
	wantBits := []SMTLiteral{NewSMTLiteral(pv, false), NewSMTLiteral(qv, false), NewSMTLiteral(pv, false), NewSMTLiteral(qv, true), SMTTrueLiteral, SMTFalseLiteral}
	if fmt.Sprint(bv.bitLiterals[variable]) != fmt.Sprint(wantBits) {
		t.Fatal("actual signed/constant literals", bv.bitLiterals[variable], wantBits)
	}
	pid, err := m.GetASTID(p)
	astTestMust(t, err)
	qid, err := m.GetASTID(q)
	astTestMust(t, err)
	wid, err := m.GetASTID(word)
	astTestMust(t, err)
	want := fmt.Sprintf("atoms:\n#%d -> #%d[3] #%d[1]\n#%d -> #%d[2] #%d[0]\n", qid, wid, wid, pid, wid, wid)
	var out bytes.Buffer
	astTestMust(t, bv.displayBitAtoms(&out))
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	if _, exists := bv.bitAtoms[SMTTrueBoolVar]; exists {
		t.Fatal("reserved true/false literals became bit atoms")
	}
	astTestMust(t, s.AssignLiteral(NewSMTLiteral(pv, false), NewSMTBooleanAxiom()))
	astTestMust(t, s.AssignLiteral(NewSMTLiteral(qv, false), NewSMTBooleanAxiom()))
	_, err = s.PropagateCore()
	astTestMust(t, err)
	value, fixed, err := bv.fixedLiteralValue(variable)
	astTestMust(t, err)
	if !fixed || value.String() != "23" {
		t.Fatal("actual assignment callback lost a repeated position", value, fixed)
	}
	before := m.Stats()
	rng := s.random
	for cut := 0; cut < len(want); cut++ {
		w := &smtLRAOutputPrefix{remaining: cut}
		if err = bv.displayBitAtoms(w); !errors.Is(err, io.ErrClosedPipe) || w.text.String() != want[:cut] {
			t.Fatal(cut, err, w.text.String(), want[:cut])
		}
	}
	if err = bv.displayBitAtoms(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if before != m.Stats() || s.random != rng {
		t.Fatal("atom display changed owner state")
	}
}

func TestSMTBVBitOccurrencesScopeUndoAfterCoreNodeRetirement(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p := smtRelevancyBool(t, m, "scope-occ-p")
	q := smtRelevancyBool(t, m, "scope-occ-q")
	base, err := m.MkBitVectorFromBits([]ast.ASTHandle{p, q, p})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(base, false))
	pv, _ := s.BoolVar(p)
	qv, _ := s.BoolVar(q)
	var before bytes.Buffer
	astTestMust(t, bv.displayBitAtoms(&before))
	baseTrail := len(bv.bitOccurrenceTrail)
	baseAtoms := len(bv.bitAtomTrail)
	astTestMust(t, s.PushScope())
	notP, err := m.MkNot(p)
	astTestMust(t, err)
	child, err := m.MkBitVectorFromBits([]ast.ASTHandle{q, notP})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(child, false))
	childVar := s.context.enodes.get(child).TheoryVar(bv.FamilyID())
	if got := bv.bitAtoms[pv]; got[len(got)-1] != (smtBVBitOccurrence{childVar, 1}) {
		t.Fatal("new head occurrence", got)
	}
	if got := bv.bitAtoms[qv]; got[len(got)-1] != (smtBVBitOccurrence{childVar, 0}) {
		t.Fatal("shared Boolean owner index", got)
	}
	fresh := smtRelevancyBool(t, m, "fresh-scope-bit")
	extra, err := m.MkBitVectorFromBits([]ast.ASTHandle{fresh})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(extra, false))
	fv, _ := s.BoolVar(fresh)
	astTestMust(t, s.PopScopes(1))
	if s.context.enodes.get(child) != nil || s.context.enodes.get(extra) != nil {
		t.Fatal("core nodes did not retire before theory rollback")
	}
	if _, exists := bv.bitAtoms[fv]; exists {
		t.Fatal("scoped bit atom survived Boolean ID retirement")
	}
	if len(bv.bitOccurrenceTrail) != baseTrail || len(bv.bitAtomTrail) != baseAtoms || len(bv.bitLiterals) != bv.NumVariables() {
		t.Fatal("bit metadata scope limits")
	}
	var after bytes.Buffer
	astTestMust(t, bv.displayBitAtoms(&after))
	if after.String() != before.String() {
		t.Fatal("scope rollback lost old heads", after.String(), before.String())
	}
	// Boolean slots are reused; a fresh scoped atom must have no stale owners.
	astTestMust(t, s.PushScope())
	other := smtRelevancyBool(t, m, "reused-scope-bit")
	again, err := m.MkBitVectorFromBits([]ast.ASTHandle{other})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(again, false))
	ov, _ := s.BoolVar(other)
	if len(bv.bitAtoms[ov]) != 1 {
		t.Fatal("reused slot inherited occurrences", bv.bitAtoms[ov])
	}
	astTestMust(t, s.PopScopes(1))
}

func TestSMTBVBitProjectionOwnerFirstInitializationAndNumeralExpansion(t *testing.T) {
	for _, numeral := range []bool{false, true} {
		t.Run(fmt.Sprint(numeral), func(t *testing.T) {
			m, s, bv := bvOccurrenceFixture(t)
			var word ast.ASTHandle
			var err error
			if numeral {
				word, err = m.BitVectorNumeralUint64(5, 3)
			} else {
				sort, e := m.BitVectorSort(3)
				astTestMust(t, e)
				word, err = m.MkConst(astTestSymbol(t, m, "projection-first-word"), sort)
			}
			astTestMust(t, err)
			high, err := m.MkBitVectorBit2Bool(2, word)
			astTestMust(t, err)
			root, err := m.NewRef(high)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&root)) })
			astTestMust(t, s.Internalize(high, true))
			variable := s.context.enodes.get(word).TheoryVar(bv.FamilyID())
			var previous int32
			for i := uint32(0); i < 3; i++ {
				atom, err := m.MkBitVectorBit2Bool(i, word)
				astTestMust(t, err)
				boolean, exists := s.BoolVar(atom)
				if !exists || i > 0 && boolean <= previous {
					t.Fatal("requested high bit allocated before owner initialization", i, boolean, previous, exists)
				}
				previous = boolean
				got := bv.bitAtoms[boolean]
				if len(got) != 1 || got[0] != (smtBVBitOccurrence{variable, i}) {
					t.Fatal("projection occurrence", i, got)
				}
			}
			if len(bv.bitLiterals[variable]) != 3 {
				t.Fatal("missing native literal vector")
			}
			if numeral {
				want := []SMTLiteral{SMTTrueLiteral, SMTFalseLiteral, SMTTrueLiteral}
				if fmt.Sprint(bv.bitLiterals[variable]) != fmt.Sprint(want) {
					t.Fatal("numeral projection replaced constant bits", bv.bitLiterals[variable])
				}
			}
			count := len(bv.bitOccurrenceTrail)
			astTestMust(t, s.Internalize(high, true))
			if len(bv.bitOccurrenceTrail) != count {
				t.Fatal("reentrant projection duplicated occurrence")
			}
		})
	}
}

func TestSMTBVBitProjectionExistingNumeralCreatesOnlyRequestedAlias(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	word, err := m.BitVectorNumeralUint64(5, 3)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(word, false))
	high, err := m.MkBitVectorBit2Bool(2, word)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(high, true))
	hv, exists := s.BoolVar(high)
	if !exists || len(bv.bitAtoms[hv]) != 1 {
		t.Fatal("requested alias missing")
	}
	low, err := m.MkBitVectorBit2Bool(0, word)
	astTestMust(t, err)
	if _, exists = s.BoolVar(low); exists {
		t.Fatal("existing numeral eagerly created unrelated aliases")
	}
}

func TestSMTBVBitAtomsScopedCreationOnExistingBooleanAndCloseOwners(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	p := smtRelevancyBool(t, m, "base-Boolean-scoped-atom")
	astTestMust(t, s.Internalize(p, true))
	pv, _ := s.BoolVar(p)
	if _, exists := bv.bitAtoms[pv]; exists {
		t.Fatal("ordinary Boolean started as a bit atom")
	}
	astTestMust(t, s.PushScope())
	word, err := m.MkBitVectorFromBits([]ast.ASTHandle{p})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(word, false))
	if len(bv.bitAtoms[pv]) != 1 {
		t.Fatal("scoped bit use not registered")
	}
	astTestMust(t, s.PopScopes(1))
	if _, exists := bv.bitAtoms[pv]; exists {
		t.Fatal("scoped bit_atom survived on a still-live Boolean")
	}
	if got, exists := s.BoolVar(p); !exists || got != pv {
		t.Fatal("bit atom rollback removed the base Boolean")
	}
	data, err := s.BooleanData(pv)
	astTestMust(t, err)
	if data.IsTheoryAtom() {
		t.Fatal("scoped subscription survived pop")
	}
	var out bytes.Buffer
	astTestMust(t, bv.displayBitAtoms(&out))
	if out.String() != "atoms:\n" {
		t.Fatal(out.String())
	}
	if err = bv.displayBitAtoms(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	astTestMust(t, bv.Close())
	if bv.bitAtoms != nil || bv.bitOccurrenceTrail != nil || bv.bitAtomTrail != nil || bv.bitLiterals != nil {
		t.Fatal("bit occurrence owners survived close")
	}
}

func TestSMTBVBitProjectionAllocationRefusalScopeRetirement(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	sort, err := m.BitVectorSort(257)
	astTestMust(t, err)
	word, err := m.MkConst(astTestSymbol(t, m, "cold-projection-owner"), sort)
	astTestMust(t, err)
	high, err := m.MkBitVectorBit2Bool(256, word)
	astTestMust(t, err)
	root, err := m.NewRef(high)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&root)) })
	before := len(bv.bitOccurrenceTrail)
	atoms := len(bv.bitAtomTrail)
	astTestMust(t, s.PushScope())
	m.Budget().FailAfter(0)
	err = s.Internalize(high, true)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected actual projection owner/circuit allocation refusal", err)
	}
	astTestMust(t, s.PopScopes(1))
	if len(bv.bitOccurrenceTrail) != before || len(bv.bitAtomTrail) != atoms {
		t.Fatal("refusal left scoped bit-atom owners")
	}
	astTestMust(t, s.Internalize(high, true))
	boolean, exists := s.BoolVar(high)
	if !exists || len(bv.bitAtoms[boolean]) != 1 {
		t.Fatal("retry after scope retirement lost alias owner")
	}
}
