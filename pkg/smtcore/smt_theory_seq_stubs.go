package smtcore

// Temporary forward declarations for the theory_seq port, removed as each
// function is ported in source order. Every stub panics.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func seqUnported(name string) { panic("theory_seq port: unported " + name) }

func (t *SMTTheorySeq) closeAutomata() { seqUnported("automata") }

func (t *SMTTheorySeq) isStep6(e ast.ASTHandle) (s, idx, re, i, j, acc ast.ASTHandle, ok bool) {
	seqUnported("is_step")
	return
}

func (t *SMTTheorySeq) addPrefix2prefix(e ast.ASTHandle, change *bool) bool {
	seqUnported("add_prefix2prefix")
	return false
}
func (t *SMTTheorySeq) propagateNotSuffix(e ast.ASTHandle) { seqUnported("propagate_not_suffix") }
func (t *SMTTheorySeq) canonizes(sign bool, e ast.ASTHandle) bool {
	seqUnported("canonizes")
	return false
}
func (t *SMTTheorySeq) isAccept(e ast.ASTHandle) bool { seqUnported("is_accept"); return false }
func (t *SMTTheorySeq) isReject(e ast.ASTHandle) bool { seqUnported("is_reject"); return false }
func (t *SMTTheorySeq) isStep(e ast.ASTHandle) bool   { seqUnported("is_step"); return false }
func (t *SMTTheorySeq) propagateAccRejLength(lit SMTLiteral, e ast.ASTHandle) {
	seqUnported("propagate_acc_rej_length")
}
func (t *SMTTheorySeq) addAccept2step(e ast.ASTHandle, change *bool) bool {
	seqUnported("add_accept2step")
	return false
}
func (t *SMTTheorySeq) addStep2accept(e ast.ASTHandle, change *bool) bool {
	seqUnported("add_step2accept")
	return false
}
func (t *SMTTheorySeq) propagateInRe(e ast.ASTHandle, isTrue bool) { seqUnported("propagate_in_re") }
