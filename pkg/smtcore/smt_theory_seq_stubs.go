package smtcore

// Temporary forward declarations for the theory_seq port, removed as each
// function is ported in source order. Every stub panics.

import (

	"github.com/glycerine/g3/pkg/ast"
)

func seqUnported(name string) { panic("theory_seq port: unported " + name) }

func (t *SMTTheorySeq) newEqEhDep(dep *seqDep, n1, n2 *SMTEnode) { seqUnported("new_eq_eh") }
func (t *SMTTheorySeq) closeAutomata()                           { seqUnported("automata") }

func (t *SMTTheorySeq) isStep6(e ast.ASTHandle) (s, idx, re, i, j, acc ast.ASTHandle, ok bool) {
	seqUnported("is_step")
	return
}
