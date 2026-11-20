package smtcore

// Source: src/smt/smt_clause.h and src/smt/smt_clause.cpp.

import (
	"errors"
	"io"
	"math/bits"

	"github.com/glycerine/g3/pkg/ast"
)

type SMTClauseKind uint8

const (
	SMTClauseAux SMTClauseKind = iota
	SMTClauseLearned
	SMTClauseAuxLemma
)

type SMTClauseDeleteHandler interface {
	ClauseDeleted(*ast.ASTManager, *SMTClause) error
}

type smtClauseAtom struct {
	expression ast.ASTHandle
	sign       bool
}

type SMTClause struct {
	self               *SMTClause
	manager            *ast.ASTManager
	atomPage           *smtClauseAtomPage
	atomSlot           uint8
	kind               SMTClauseKind
	reinit             bool
	reinternalizeAtoms bool
	// reinitLevel and reinitSeq place the clause in
	// m_clauses_to_reinit[reinitLevel], in push order.
	reinitLevel uint32
	reinitSeq   uint64
	// lemmaIndex is the position in the internalizer's live lemma vector.
	lemmaIndex    int
	atoms         []smtClauseAtom
	deleteHandler SMTClauseDeleteHandler
	justification SMTJustification
	activity      uint32
	deleted       bool
	closePhase    uint8
	closed        bool
	// literals is last: smtNewClauseStorage places a short clause's
	// literal array right after it, in the same cache line or the next.
	literals []SMTLiteral
}

// Clause storage. The source allocates a clause and its literals as one
// object (clause::mk). Go places the literal array of a clause of up to 64
// literals in the same allocation, right after the header's literals field,
// so BCP's clause visit reads its literals from the same or the adjacent
// cache line rather than from a separate object. Longer clauses keep a separate literal array.
type smtClauseStorage4 struct {
	clause SMTClause
	lits   [4]SMTLiteral
}
type smtClauseStorage8 struct {
	clause SMTClause
	lits   [8]SMTLiteral
}
type smtClauseStorage16 struct {
	clause SMTClause
	lits   [16]SMTLiteral
}
type smtClauseStorage32 struct {
	clause SMTClause
	lits   [32]SMTLiteral
}
type smtClauseStorage64 struct {
	clause SMTClause
	lits   [64]SMTLiteral
}

// smtNewClauseStorage returns a zero clause whose literals field holds a
// copy of literals (length and capacity len(literals)).
func smtNewClauseStorage(literals []SMTLiteral) *SMTClause {
	n := len(literals)
	var clause *SMTClause
	var storage []SMTLiteral
	switch {
	case n <= 4:
		x := &smtClauseStorage4{}
		clause, storage = &x.clause, x.lits[:n:n]
	case n <= 8:
		x := &smtClauseStorage8{}
		clause, storage = &x.clause, x.lits[:n:n]
	case n <= 16:
		x := &smtClauseStorage16{}
		clause, storage = &x.clause, x.lits[:n:n]
	case n <= 32:
		x := &smtClauseStorage32{}
		clause, storage = &x.clause, x.lits[:n:n]
	case n <= 64:
		x := &smtClauseStorage64{}
		clause, storage = &x.clause, x.lits[:n:n]
	default:
		clause, storage = &SMTClause{}, make([]SMTLiteral, n)
	}
	copy(storage, literals)
	clause.literals = storage
	return clause
}

func NewSMTClause(manager *ast.ASTManager, literals []SMTLiteral, kind SMTClauseKind, justification SMTJustification, deleteHandler SMTClauseDeleteHandler, saveAtoms bool, expressions []ast.ASTHandle) (*SMTClause, error) {
	return newSMTClause(manager, literals, kind, justification, deleteHandler, saveAtoms, expressions, nil)
}

func newSMTClause(manager *ast.ASTManager, literals []SMTLiteral, kind SMTClauseKind, justification SMTJustification, deleteHandler SMTClauseDeleteHandler, saveAtoms bool, expressions []ast.ASTHandle, pool *smtClauseAtomPool) (*SMTClause, error) {
	if manager == nil || len(literals) < 2 || kind > SMTClauseAuxLemma {
		return nil, ast.ErrASTDomain
	}
	if kind != SMTClauseAux && justification != nil && justification.InRegion() {
		return nil, ast.ErrASTDomain
	}
	proofs, err := manager.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	if proofs && justification == nil {
		return nil, ast.ErrASTDomain
	}
	clause := smtNewClauseStorage(literals)
	*clause = SMTClause{
		manager:            manager,
		literals:           clause.literals,
		kind:               kind,
		reinit:             saveAtoms,
		reinternalizeAtoms: saveAtoms,
		deleteHandler:      deleteHandler,
		justification:      justification,
	}
	clause.self = clause
	if kind != SMTClauseAux {
		clause.activity = 1
	}
	if !saveAtoms {
		return clause, nil
	}
	pool.allocate(clause, len(literals))
	for i, literal := range literals {
		expression, e := smtLiteralExpression(literal, expressions)
		if e != nil {
			return nil, errors.Join(e, clause.Close())
		}
		if e = manager.IncRef(expression); e != nil {
			return nil, errors.Join(e, clause.Close())
		}
		clause.atoms[i] = smtClauseAtom{expression: expression, sign: literal.Sign()}
	}
	return clause, nil
}

func (c *SMTClause) open() error {
	if c == nil || c.self != c || c.manager == nil || c.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *SMTClause) Kind() SMTClauseKind { return c.kind }
func (c *SMTClause) IsLemma() bool       { return c != nil && c.kind != SMTClauseAux }
func (c *SMTClause) IsLearned() bool     { return c != nil && c.kind == SMTClauseLearned }
func (c *SMTClause) IsAuxLemma() bool    { return c != nil && c.kind == SMTClauseAuxLemma }
func (c *SMTClause) InReinitStack() bool { return c != nil && c.reinit }
func (c *SMTClause) ReinternalizeAtoms() bool {
	return c != nil && c.reinternalizeAtoms
}
func (c *SMTClause) Deleted() bool { return c != nil && c.deleted }

func (c *SMTClause) Literals() []SMTLiteral {
	if c == nil {
		return nil
	}
	return c.literals
}

func (c *SMTClause) Literal(index int) (SMTLiteral, error) {
	if err := c.open(); err != nil {
		return SMTNullLiteral, err
	}
	if index < 0 || index >= len(c.literals) {
		return SMTNullLiteral, ast.ErrASTDomain
	}
	return c.literals[index], nil
}

func (c *SMTClause) SetLiteral(index int, literal SMTLiteral) error {
	if err := c.open(); err != nil {
		return err
	}
	if index < 0 || index >= len(c.literals) {
		return ast.ErrASTDomain
	}
	c.literals[index] = literal
	return nil
}

func (c *SMTClause) SwapLiterals(first, second int) error {
	if err := c.open(); err != nil {
		return err
	}
	if first < 0 || first >= len(c.literals) || second < 0 || second >= len(c.literals) {
		return ast.ErrASTDomain
	}
	c.literals[first], c.literals[second] = c.literals[second], c.literals[first]
	return nil
}

func (c *SMTClause) IsWatch(literal SMTLiteral) bool {
	return c != nil && len(c.literals) >= 2 && (c.literals[0] == literal || c.literals[1] == literal)
}

func (c *SMTClause) SetNumLiterals(count int) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.reinit || count < 0 || count > len(c.literals) {
		return ast.ErrASTDomain
	}
	c.literals = c.literals[:count]
	return nil
}

func (c *SMTClause) Justification() SMTJustification {
	if c == nil {
		return nil
	}
	return c.justification
}

func (c *SMTClause) SetJustification(justification SMTJustification) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.reinit || c.justification == nil || c.IsLemma() && justification != nil && justification.InRegion() {
		return ast.ErrASTDomain
	}
	c.justification = justification
	return nil
}

func (c *SMTClause) Activity() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if !c.IsLemma() {
		return 0, ast.ErrASTDomain
	}
	return c.activity, nil
}

func (c *SMTClause) SetActivity(activity uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	if !c.IsLemma() {
		return ast.ErrASTDomain
	}
	c.activity = activity
	return nil
}

func (c *SMTClause) IncrementActivity() error {
	activity, err := c.Activity()
	if err != nil {
		return err
	}
	c.activity = activity + 1
	return nil
}

func (c *SMTClause) NumAtoms() int {
	if c == nil || !c.reinternalizeAtoms {
		return 0
	}
	return len(c.literals)
}

func (c *SMTClause) Atom(index int) (ast.ASTHandle, bool, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if index < 0 || index >= c.NumAtoms() || index >= len(c.atoms) {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	return c.atoms[index].expression, c.atoms[index].sign, nil
}

func (c *SMTClause) ReleaseAtoms() error {
	if err := c.open(); err != nil {
		return err
	}
	// clause::deallocate and release_atoms dec_ref the atoms first to last.
	for i := range c.atoms {
		handle := c.atoms[i].expression
		if handle.IsNull() {
			continue
		}
		consumed, err := c.manager.DecRefHeldConsumed(handle)
		if consumed {
			c.atoms[i].expression = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *SMTClause) MarkDeleted() error {
	if err := c.open(); err != nil {
		return err
	}
	if c.deleted {
		return ast.ErrASTDomain
	}
	c.deleted = true
	if c.deleteHandler != nil {
		if err := c.deleteHandler.ClauseDeleted(c.manager, c); err != nil {
			return err
		}
		c.deleteHandler = nil
	}
	return nil
}

func (c *SMTClause) Display(out io.Writer, expressions []ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if _, err := io.WriteString(out, "(clause"); err != nil {
		return err
	}
	for _, literal := range c.literals {
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if err := literal.Display(out, c.manager, expressions); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")")
	return err
}

func (c *SMTClause) DisplayCompact(out io.Writer, expressions []ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if _, err := io.WriteString(out, "(clause"); err != nil {
		return err
	}
	for _, literal := range c.literals {
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if err := literal.DisplayCompact(out, c.manager, expressions); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")")
	return err
}

func (c *SMTClause) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if c.closePhase == 0 {
		if c.deleteHandler != nil {
			if err := c.deleteHandler.ClauseDeleted(c.manager, c); err != nil {
				return err
			}
			c.deleteHandler = nil
		}
		c.closePhase = 1
	}
	if c.closePhase == 1 {
		if c.IsLemma() && c.justification != nil {
			if err := c.justification.Release(c.manager); err != nil {
				return err
			}
			c.justification = nil
		}
		c.closePhase = 2
	}
	if c.closePhase == 2 {
		if err := c.ReleaseAtoms(); err != nil {
			return err
		}
		c.closePhase = 3
	}
	c.literals = nil
	c.discardAtoms()
	c.manager = nil
	c.closed = true
	return nil
}

// Each occupied slot belongs to exactly one live clause. Each page contains
// only scalar handles and sign bits. Long clauses and overflow get ordinary
// heap storage; a clause of any lifetime can release its slot independently.
const smtShortClauseAtoms = 4

type smtClauseAtomPage struct {
	free  uint16
	slots [16][smtShortClauseAtoms]smtClauseAtom
}

type smtClauseAtomPool struct{ pages [4]*smtClauseAtomPage }

func (p *smtClauseAtomPool) allocate(c *SMTClause, n int) {
	if p != nil && n <= smtShortClauseAtoms {
		for i, page := range p.pages {
			if page == nil {
				page = &smtClauseAtomPage{free: ^uint16(0)}
				p.pages[i] = page
			}
			if page.free == 0 {
				continue
			}
			slot := bits.TrailingZeros16(page.free)
			page.free &^= 1 << slot
			c.atomPage, c.atomSlot = page, uint8(slot)
			c.atoms = page.slots[slot][:n:n]
			return
		}
	}
	c.atoms = make([]smtClauseAtom, n)
}

// All references in this span have been consumed by ReleaseAtoms.
func (c *SMTClause) discardAtoms() {
	c.atoms = nil
	if c.atomPage != nil {
		c.atomPage.free |= 1 << c.atomSlot
		c.atomPage = nil
	}
}
