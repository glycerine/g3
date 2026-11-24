package smtcore

// Source: src/smt/smt_b_justification.h, src/smt/smt_eq_justification.h,
// and src/smt/smt_bool_var_data.h compact dependency records.

import (
	"unsafe"

	"github.com/glycerine/g3/pkg/ast"
)

// SMTJustification is the typed base implemented by retained concrete SMT
// justification objects. It mirrors the source abstract justification class.
type SMTJustification interface {
	smtJustification()
	InRegion() bool
	IsMarked() bool
	SetMarked(bool) error
	MarkAntecedents(SMTAntecedentMarker) error
	FromTheory() SMTTheoryID
	Name() string
	Release(*ast.ASTManager) error
}

type SMTBooleanJustificationKind uint8

const (
	SMTBooleanJustificationClause SMTBooleanJustificationKind = iota
	SMTBooleanJustificationBinaryClause
	SMTBooleanJustificationAxiom
	SMTBooleanJustificationObject
)

func (k SMTBooleanJustificationKind) String() string {
	switch k {
	case SMTBooleanJustificationClause:
		return "clause"
	case SMTBooleanJustificationBinaryClause:
		return "bin_clause"
	case SMTBooleanJustificationAxiom:
		return "axiom"
	case SMTBooleanJustificationObject:
		return "theory"
	default:
		return ""
	}
}

// SMTBooleanJustification is one GC-traced word. The two low address bits
// select a variant. Every reference, including a tagged one, points inside
// its allocation. Clauses use their aligned address directly. Literal cells,
// the axiom token and boxed interfaces have at least four-byte alignment and
// size, so byte offsets 1, 2 and 3 respectively are valid interior pointers.
// uintptr is used only to read the tag. The stored pointer keeps the target live.
type SMTBooleanJustification struct {
	_   [0]func() // Use Equal for value equality of independently constructed reasons.
	ref unsafe.Pointer
}

var smtAxiomToken uint32

func NewSMTBooleanAxiom() SMTBooleanJustification {
	return SMTBooleanJustification{ref: unsafe.Add(unsafe.Pointer(&smtAxiomToken), SMTBooleanJustificationAxiom)}
}
func NewSMTBooleanClause(value *SMTClause) SMTBooleanJustification {
	return SMTBooleanJustification{ref: unsafe.Pointer(value)}
}

// Small binary antecedents share immutable, four-byte-aligned cells across
// solvers. A detached larger literal owns its cell. Solver propagation reuses
// pages for larger literals as well.
const smtBinaryReasonPageSize = 256

type smtBinaryReasonPage [smtBinaryReasonPageSize]SMTLiteral

func (p *smtBinaryReasonPage) init(base uint32) {
	for i := range p {
		p[i] = SMTLiteral(base + uint32(i))
	}
}

var smtSmallBinaryReasons = func() (p smtBinaryReasonPage) { p.init(0); return }()

func smtBinaryReasonRef(l *SMTLiteral) SMTBooleanJustification {
	return SMTBooleanJustification{ref: unsafe.Add(unsafe.Pointer(l), SMTBooleanJustificationBinaryClause)}
}
func NewSMTBooleanBinaryClause(value SMTLiteral) SMTBooleanJustification {
	if uint32(value) < smtBinaryReasonPageSize {
		return smtBinaryReasonRef(&smtSmallBinaryReasons[value])
	}
	cell := value
	return smtBinaryReasonRef(&cell)
}
func NewSMTBooleanJustification(value SMTJustification) (SMTBooleanJustification, error) {
	if value == nil {
		return SMTBooleanJustification{}, ast.ErrASTHandle
	}
	return SMTBooleanJustification{ref: unsafe.Add(unsafe.Pointer(&value), SMTBooleanJustificationObject)}, nil
}
func NullSMTBooleanJustification() SMTBooleanJustification { return SMTBooleanJustification{} }

func (j SMTBooleanJustification) Kind() SMTBooleanJustificationKind {
	return SMTBooleanJustificationKind(uintptr(j.ref) & 3)
}
func (j SMTBooleanJustification) Clause() (*SMTClause, bool) {
	if j.Kind() != SMTBooleanJustificationClause {
		return nil, false
	}
	return (*SMTClause)(j.ref), true
}
func (j SMTBooleanJustification) Literal() (SMTLiteral, bool) {
	if j.Kind() != SMTBooleanJustificationBinaryClause {
		return 0, false
	}
	return *(*SMTLiteral)(unsafe.Add(j.ref, -int(SMTBooleanJustificationBinaryClause))), true
}
func (j SMTBooleanJustification) Justification() (SMTJustification, bool) {
	if j.Kind() != SMTBooleanJustificationObject {
		return nil, false
	}
	return *(*SMTJustification)(unsafe.Add(j.ref, -int(SMTBooleanJustificationObject))), true
}
func (j SMTBooleanJustification) Equal(k SMTBooleanJustification) bool {
	if j.ref == k.ref {
		return true
	}
	if j.Kind() != k.Kind() {
		return false
	}
	switch j.Kind() {
	case SMTBooleanJustificationBinaryClause:
		a, _ := j.Literal()
		b, _ := k.Literal()
		return a == b
	case SMTBooleanJustificationObject:
		a, _ := j.Justification()
		b, _ := k.Justification()
		return a == b
	default:
		return false
	}
}

type SMTEqualityJustificationKind uint8

const (
	SMTEqualityJustificationAxiom SMTEqualityJustificationKind = iota
	SMTEqualityJustificationCongruence
	SMTEqualityJustificationEquation
	SMTEqualityJustificationObject
)

// SMTEqualityJustification replaces the source tagged pointer with a concrete
// discriminant and typed payload fields.
type SMTEqualityJustification struct {
	kind          SMTEqualityJustificationKind
	commutativity bool
	literal       SMTLiteral
	justification SMTJustification
}

func NewSMTEqualityAxiom() SMTEqualityJustification { return SMTEqualityJustification{} }
func NewSMTEqualityCongruence(commutativity bool) SMTEqualityJustification {
	return SMTEqualityJustification{kind: SMTEqualityJustificationCongruence, commutativity: commutativity}
}
func NewSMTEqualityEquation(literal SMTLiteral) SMTEqualityJustification {
	return SMTEqualityJustification{kind: SMTEqualityJustificationEquation, literal: literal}
}
func NewSMTEqualityJustification(value SMTJustification) SMTEqualityJustification {
	return SMTEqualityJustification{kind: SMTEqualityJustificationObject, justification: value}
}
func NullSMTEqualityJustification() SMTEqualityJustification {
	return NewSMTEqualityJustification(nil)
}
func (j SMTEqualityJustification) Kind() SMTEqualityJustificationKind { return j.kind }
func (j SMTEqualityJustification) Literal() (SMTLiteral, bool) {
	return j.literal, j.kind == SMTEqualityJustificationEquation
}
func (j SMTEqualityJustification) UsedCommutativity() (bool, bool) {
	return j.commutativity, j.kind == SMTEqualityJustificationCongruence
}
func (j SMTEqualityJustification) Justification() (SMTJustification, bool) {
	return j.justification, j.kind == SMTEqualityJustificationObject
}

// SMTBoolVarData follows smt_bool_var_data.h: a reason and two packed words.
type SMTBoolVarData struct {
	justification SMTBooleanJustification
	scopeFlags    uint32
	internalFlags uint32
}

const (
	smtBoolMark uint32 = 1 << (24 + iota)
	smtBoolAssumption
	smtBoolPhaseAvailable
	smtBoolPhase
	smtBoolEquality
	smtBoolTrueFirst
	smtBoolENode
	smtBoolQuantifier
)
const (
	smtBoolScopeMask    = 1<<24 - 1
	smtBoolInternalMask = 1<<23 - 1
	smtBoolAtom         = 1 << 23
	smtBoolTheoryShift  = 24
)

// AND and OR cache their argument array and its literal bindings.
type smtBoolGate struct {
	args     []ast.ASTHandle
	literals []SMTLiteral
}

func newSMTBoolGate(n uint32) *smtBoolGate {
	if n <= 4 {
		block := new(struct {
			gate     smtBoolGate
			children [4]ast.ASTHandle
		})
		block.gate.args = block.children[:n:n]
		return &block.gate
	}
	return &smtBoolGate{args: make([]ast.ASTHandle, n)}
}

// Initialize resets the record at a 23-bit internalization level.
func (d *SMTBoolVarData) Initialize(internalLevel uint32) error {
	if d == nil {
		return ast.ErrASTHandle
	}
	if internalLevel > smtBoolInternalMask {
		return ast.ErrASTDomain
	}
	*d = SMTBoolVarData{internalFlags: internalLevel}
	return nil
}
func (d *SMTBoolVarData) InternalLevel() uint32 { return d.internalFlags & smtBoolInternalMask }
func (d *SMTBoolVarData) ScopeLevel() uint32    { return d.scopeFlags & smtBoolScopeMask }

// SetScopeLevel records the 24-bit assignment level.
func (d *SMTBoolVarData) SetScopeLevel(level uint32) error {
	if level > smtBoolScopeMask {
		return ast.ErrASTDomain
	}
	d.scopeFlags = d.scopeFlags&^smtBoolScopeMask | level
	return nil
}
func (d *SMTBoolVarData) Justification() SMTBooleanJustification         { return d.justification }
func (d *SMTBoolVarData) SetAxiom()                                      { d.justification = NewSMTBooleanAxiom() }
func (d *SMTBoolVarData) SetNullJustification()                          { d.justification = NullSMTBooleanJustification() }
func (d *SMTBoolVarData) SetJustification(value SMTBooleanJustification) { d.justification = value }
func (d *SMTBoolVarData) IsAtom() bool                                   { return d.internalFlags&smtBoolAtom != 0 }
func (d *SMTBoolVarData) updateAtom() {
	if d.scopeFlags&(smtBoolEquality|smtBoolENode|smtBoolQuantifier) == 0 && !d.IsTheoryAtom() {
		d.internalFlags &^= smtBoolAtom
	}
}
func (d *SMTBoolVarData) Theory() SMTTheoryID {
	if !d.IsTheoryAtom() {
		return SMTNullTheoryID
	}
	return SMTTheoryID(d.internalFlags >> smtBoolTheoryShift)
}
func (d *SMTBoolVarData) IsTheoryAtom() bool { return d.internalFlags>>smtBoolTheoryShift != 0 }
func (d *SMTBoolVarData) SetNotifyTheory(value SMTTheoryID) error {
	if value <= 0 || value > 255 {
		return ast.ErrASTDomain
	}
	d.internalFlags = d.internalFlags&(1<<smtBoolTheoryShift-1) | uint32(value)<<smtBoolTheoryShift | smtBoolAtom
	return nil
}
func (d *SMTBoolVarData) ResetNotifyTheory() {
	d.internalFlags &= 1<<smtBoolTheoryShift - 1
	d.updateAtom()
}
func (d *SMTBoolVarData) Mark() bool { return d.scopeFlags&smtBoolMark != 0 }
func (d *SMTBoolVarData) SetMark(value bool) {
	if value {
		d.scopeFlags |= smtBoolMark
	} else {
		d.scopeFlags &^= smtBoolMark
	}
}
func (d *SMTBoolVarData) Assumption() bool { return d.scopeFlags&smtBoolAssumption != 0 }
func (d *SMTBoolVarData) SetAssumption(value bool) {
	if value {
		d.scopeFlags |= smtBoolAssumption
	} else {
		d.scopeFlags &^= smtBoolAssumption
	}
}
func (d *SMTBoolVarData) PhaseAvailable() bool { return d.scopeFlags&smtBoolPhaseAvailable != 0 }
func (d *SMTBoolVarData) SetPhaseAvailable(value bool) {
	if value {
		d.scopeFlags |= smtBoolPhaseAvailable
	} else {
		d.scopeFlags &^= smtBoolPhaseAvailable
	}
}
func (d *SMTBoolVarData) Phase() bool { return d.scopeFlags&smtBoolPhase != 0 }
func (d *SMTBoolVarData) SetPhase(value bool) {
	if value {
		d.scopeFlags |= smtBoolPhase
	} else {
		d.scopeFlags &^= smtBoolPhase
	}
}
func (d *SMTBoolVarData) IsEquality() bool { return d.scopeFlags&smtBoolEquality != 0 }
func (d *SMTBoolVarData) SetEquality() {
	d.scopeFlags |= smtBoolEquality
	d.internalFlags |= smtBoolAtom
}
func (d *SMTBoolVarData) ResetEquality() {
	d.scopeFlags &^= smtBoolEquality
	d.updateAtom()
}
func (d *SMTBoolVarData) TryTrueFirst() bool { return d.scopeFlags&smtBoolTrueFirst != 0 }
func (d *SMTBoolVarData) SetTrueFirst()      { d.scopeFlags |= smtBoolTrueFirst }
func (d *SMTBoolVarData) ResetTrueFirst()    { d.scopeFlags &^= smtBoolTrueFirst }
func (d *SMTBoolVarData) IsENode() bool      { return d.scopeFlags&smtBoolENode != 0 }
func (d *SMTBoolVarData) SetENode() {
	d.scopeFlags |= smtBoolENode
	d.internalFlags |= smtBoolAtom
}
func (d *SMTBoolVarData) ResetENode() {
	d.scopeFlags &^= smtBoolENode
	d.updateAtom()
}
func (d *SMTBoolVarData) IsQuantifier() bool { return d.scopeFlags&smtBoolQuantifier != 0 }
func (d *SMTBoolVarData) SetQuantifier() {
	d.scopeFlags |= smtBoolQuantifier
	d.internalFlags |= smtBoolAtom
}
