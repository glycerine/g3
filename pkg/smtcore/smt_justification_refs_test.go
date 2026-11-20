package smtcore

import (
	"errors"
	"runtime"
	"testing"
	"unsafe"

	"github.com/glycerine/g3/pkg/ast"
)

type smtTestJustification struct{ SMTJustificationBase }

func TestSMTCompactBooleanAndEqualityJustifications(t *testing.T) {
	if NullSMTBooleanJustification().Kind() != SMTBooleanJustificationClause || NewSMTBooleanAxiom().Kind() != SMTBooleanJustificationAxiom {
		t.Fatal("Boolean source defaults")
	}
	clause := &SMTClause{}
	clauseValue := NewSMTBooleanClause(clause)
	if got, ok := clauseValue.Clause(); !ok || got != clause {
		t.Fatal("clause payload", got, ok)
	}
	literal := NewSMTLiteral(7, true)
	binary := NewSMTBooleanBinaryClause(literal)
	if got, ok := binary.Literal(); !ok || got != literal || binary.Kind().String() != "bin_clause" {
		t.Fatal("binary payload", got, ok)
	}
	object := &smtTestJustification{}
	theory, err := NewSMTBooleanJustification(object)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := theory.Justification(); !ok || got != object || theory.Kind().String() != "theory" {
		t.Fatal("theory payload", got, ok)
	}
	if _, err = NewSMTBooleanJustification(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil Boolean justification", err)
	}
	for kind, text := range map[SMTBooleanJustificationKind]string{SMTBooleanJustificationClause: "clause", SMTBooleanJustificationBinaryClause: "bin_clause", SMTBooleanJustificationAxiom: "axiom", SMTBooleanJustificationObject: "theory"} {
		if kind.String() != text {
			t.Fatal(kind, kind.String(), text)
		}
	}
	if SMTBooleanJustificationKind(255).String() != "" {
		t.Fatal("unknown Boolean justification display")
	}
	if NewSMTEqualityAxiom().Kind() != SMTEqualityJustificationAxiom || NullSMTEqualityJustification().Kind() != SMTEqualityJustificationObject {
		t.Fatal("equality source defaults")
	}
	congruence := NewSMTEqualityCongruence(true)
	if commuted, ok := congruence.UsedCommutativity(); !ok || !commuted {
		t.Fatal("congruence payload", commuted, ok)
	}
	equation := NewSMTEqualityEquation(literal)
	if got, ok := equation.Literal(); !ok || got != literal {
		t.Fatal("equation payload", got, ok)
	}
	fallback := NewSMTEqualityJustification(object)
	if got, ok := fallback.Justification(); !ok || got != object {
		t.Fatal("equality fallback", got, ok)
	}
}

func TestSMTBoolVarDataSourceFlagTransitions(t *testing.T) {
	var data SMTBoolVarData
	data.Initialize(19)
	if data.InternalLevel() != 19 || data.IsAtom() || data.Justification().Kind() != SMTBooleanJustificationClause || data.ScopeLevel() != 0 || data.Mark() || data.Assumption() || data.PhaseAvailable() || data.Phase() {
		t.Fatal("initial state", data)
	}
	data.SetAxiom()
	if data.Justification().Kind() != SMTBooleanJustificationAxiom {
		t.Fatal("axiom")
	}
	data.SetEquality()
	data.SetENode()
	data.SetQuantifier()
	if !data.IsAtom() || !data.IsEquality() || !data.IsENode() || !data.IsQuantifier() {
		t.Fatal("atom flags")
	}
	data.ResetEquality()
	data.ResetENode()
	if !data.IsAtom() {
		t.Fatal("quantifier must retain atom")
	}
	if err := data.SetNotifyTheory(7); err != nil || data.Theory() != 7 || !data.IsTheoryAtom() {
		t.Fatal("theory atom", data.Theory(), err)
	}
	if err := data.SetNotifyTheory(0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("zero theory", err)
	}
	if err := data.SetNotifyTheory(256); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("wide theory", err)
	}
	data.ResetNotifyTheory()
	if data.IsTheoryAtom() || data.Theory() != SMTNullTheoryID || !data.IsAtom() {
		t.Fatal("theory reset with quantifier")
	}
	data.SetTrueFirst()
	if !data.TryTrueFirst() {
		t.Fatal("true first")
	}
	data.ResetTrueFirst()
	if data.TryTrueFirst() {
		t.Fatal("true first reset")
	}
	data.Initialize(3)
	if data.InternalLevel() != 3 || data.IsAtom() || data.TryTrueFirst() {
		t.Fatal("reinitialize", data)
	}
}

func TestSMTBooleanRecordLayoutAndPackedLevels(t *testing.T) {
	word := unsafe.Sizeof(uintptr(0))
	if size := unsafe.Sizeof(SMTBooleanJustification{}); size != word {
		t.Fatalf("Boolean justification is %d bytes, want %d", size, word)
	}
	if size := unsafe.Sizeof(SMTBoolVarData{}); size != word+8 {
		t.Fatalf("Boolean record is %d bytes, want %d", size, word+8)
	}
	var d SMTBoolVarData
	astTestMust(t, d.Initialize(smtBoolInternalMask))
	astTestMust(t, d.SetScopeLevel(smtBoolScopeMask))
	d.SetMark(true)
	d.SetAssumption(true)
	d.SetPhaseAvailable(true)
	d.SetPhase(true)
	d.SetTrueFirst()
	d.SetEquality()
	d.SetENode()
	d.SetQuantifier()
	astTestMust(t, d.SetNotifyTheory(255))
	if !errors.Is(d.Initialize(smtBoolInternalMask+1), ast.ErrASTDomain) ||
		!errors.Is(d.SetScopeLevel(smtBoolScopeMask+1), ast.ErrASTDomain) {
		t.Fatal("out-of-range level accepted")
	}
	if d.InternalLevel() != smtBoolInternalMask || d.ScopeLevel() != smtBoolScopeMask || d.Theory() != 255 ||
		!d.Mark() || !d.Assumption() || !d.PhaseAvailable() || !d.Phase() ||
		!d.TryTrueFirst() || !d.IsEquality() || !d.IsENode() || !d.IsQuantifier() {
		t.Fatal("a packed field overwrote its neighbor")
	}
	astTestMust(t, d.SetScopeLevel(0))
	d.ResetNotifyTheory()
	if d.InternalLevel() != smtBoolInternalMask || !d.IsQuantifier() || !d.Mark() {
		t.Fatal("reset damaged adjacent flags")
	}
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	s.scopeLevel = smtBoolInternalMask
	err := s.PushScope()
	s.scopeLevel = 0
	if !errors.Is(err, ast.ErrASTDomain) || len(s.scopes) != 0 {
		t.Fatal("pushed past internalization level limit", err)
	}
}

// Returning only the tagged references removes the creator's stack frame
// before the forced collections. Each reference must retain its own payload.
func smtDetachedTestReasons(t *testing.T) []SMTBooleanJustification {
	clause := &SMTClause{literals: []SMTLiteral{4141}}
	theory, err := NewSMTSimpleJustification(false, []SMTLiteral{9191})
	astTestMust(t, err)
	j, err := NewSMTBooleanJustification(theory)
	astTestMust(t, err)
	s := SMTInternalizerState{boolExpr: make([]ast.ASTHandle, 1024)}
	first := s.binaryReason(511)
	last := s.binaryReason(2047)
	if len(s.binaryReasons) != 7 || s.binaryReasons[0] == nil || s.binaryReasons[1] != nil || s.binaryReasons[6] == nil {
		t.Fatal("unexpected allocation for sparse binary literal pages")
	}
	return []SMTBooleanJustification{NullSMTBooleanJustification(), NewSMTBooleanClause(clause), NewSMTBooleanAxiom(), j, first, last,
		NewSMTBooleanBinaryClause(0), NewSMTBooleanBinaryClause(-1 << 31), NewSMTBooleanBinaryClause(1<<31 - 1)}
}

func TestSMTBooleanTaggedReferencesSurviveGC(t *testing.T) {
	js := smtDetachedTestReasons(t)
	runtime.GC()
	runtime.GC()
	if c, ok := js[0].Clause(); !ok || c != nil {
		t.Fatal("null clause")
	}
	if c, ok := js[1].Clause(); !ok || len(c.literals) != 1 || c.literals[0] != 4141 {
		t.Fatal("detached clause")
	}
	if !js[2].Equal(NewSMTBooleanAxiom()) {
		t.Fatal("axiom tag")
	}
	obj, ok := js[3].Justification()
	if !ok || obj.(*SMTSimpleJustification).Literals[0] != 9191 {
		t.Fatal("detached theory justification")
	}
	another, err := NewSMTBooleanJustification(obj)
	astTestMust(t, err)
	if !js[3].Equal(another) || js[3].Equal(js[1]) {
		t.Fatal("theory equality")
	}
	for index, want := range []SMTLiteral{511, 2047, 0, -1 << 31, 1<<31 - 1} {
		reason := js[4+index]
		got, ok := reason.Literal()
		if !ok || got != want || !reason.Equal(NewSMTBooleanBinaryClause(want)) || reason.Equal(NewSMTBooleanBinaryClause(want.Negated())) {
			t.Fatal("literal payload or equality", got, want)
		}
		if c, ok := reason.Clause(); ok || c != nil {
			t.Fatal("binary decoded as a clause")
		}
	}
}
