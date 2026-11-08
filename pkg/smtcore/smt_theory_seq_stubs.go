package smtcore

// Temporary forward declarations for the theory_seq port, removed as each
// function is ported in source order. Every stub panics.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func seqUnported(name string) { panic("theory_seq port: unported " + name) }

func (t *SMTTheorySeq) addAxiom(lits ...SMTLiteral)   { seqUnported("add_axiom") }
func (t *SMTTheorySeq) enforceLengthNode(n *SMTEnode) { seqUnported("enforce_length") }
func (t *SMTTheorySeq) ensureEnode(e ast.ASTHandle) *SMTEnode {
	seqUnported("ensure_enode")
	return nil
}
func (t *SMTTheorySeq) getLengthValue(e ast.ASTHandle) (*big.Rat, bool) {
	seqUnported("get_length")
	return nil, false
}
func (t *SMTTheorySeq) isBinaryEq(l, r []ast.ASTHandle) (x ast.ASTHandle, xs, ys []ast.ASTHandle, y ast.ASTHandle, ok bool) {
	seqUnported("is_binary_eq")
	return
}
func (t *SMTTheorySeq) isSkolem(s usymbol.Symbol, e ast.ASTHandle) bool {
	seqUnported("is_skolem")
	return false
}
func (t *SMTTheorySeq) lowerBound(e ast.ASTHandle) (*big.Rat, bool) {
	seqUnported("lower_bound")
	return nil, false
}
func (t *SMTTheorySeq) upperBound(e ast.ASTHandle) (*big.Rat, bool) {
	seqUnported("upper_bound")
	return nil, false
}
func (t *SMTTheorySeq) mkEqEmpty(e ast.ASTHandle, phase bool) SMTLiteral {
	seqUnported("mk_eq_empty")
	return SMTNullLiteral
}
func (t *SMTTheorySeq) mkLiteral(e ast.ASTHandle) SMTLiteral {
	seqUnported("mk_literal")
	return SMTNullLiteral
}
func (t *SMTTheorySeq) mkSeqEq(a, b ast.ASTHandle) SMTLiteral {
	seqUnported("mk_seq_eq")
	return SMTNullLiteral
}
func (t *SMTTheorySeq) mkSkolem(s usymbol.Symbol, e1, e2, e3, rng ast.ASTHandle) ast.ASTHandle {
	seqUnported("mk_skolem")
	return ast.ASTHandle{}
}
func (t *SMTTheorySeq) mkSub(a, b ast.ASTHandle) ast.ASTHandle {
	seqUnported("mk_sub")
	return ast.ASTHandle{}
}
func (t *SMTTheorySeq) newEqEhDep(dep *seqDep, n1, n2 *SMTEnode) { seqUnported("new_eq_eh") }
func (t *SMTTheorySeq) propagateEqDep(dep *seqDep, e1, e2 ast.ASTHandle, addToEqs bool) {
	seqUnported("propagate_eq")
}
func (t *SMTTheorySeq) propagateEqLits(dep *seqDep, lits []SMTLiteral, e1, e2 ast.ASTHandle, addToEqs bool) {
	seqUnported("propagate_eq")
}
func (t *SMTTheorySeq) propagateEqLit(dep *seqDep, lit SMTLiteral, e1, e2 ast.ASTHandle, addToEqs bool) {
	seqUnported("propagate_eq")
}
func (t *SMTTheorySeq) propagateLit(dep *seqDep, lits []SMTLiteral, lit SMTLiteral) {
	seqUnported("propagate_lit")
}
func (t *SMTTheorySeq) reduceLengthEq(ls, rs []ast.ASTHandle, dep *seqDep) bool {
	seqUnported("reduce_length_eq")
	return false
}
func (t *SMTTheorySeq) setConflict(dep *seqDep, lits []SMTLiteral) { seqUnported("set_conflict") }
