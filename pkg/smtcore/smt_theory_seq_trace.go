package smtcore

// Source: TRACE("seq", ...) records of src/smt/theory_seq.cpp add_axiom:3865,
// deque_axiom:3098 and get_automaton:4174, with automaton::display (math/automata/automaton.h
// 549-566) and sym_expr::display (ast/rewriter/seq_rewriter.cpp:63-70).

import (
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (t *SMTTheorySeq) traceAddAxiom(lits []SMTLiteral) {
	if !smtTraceEnabled("seq") {
		return
	}
	emitSMTTrace("seq", "add_axiom", "src/smt/theory_seq.cpp", 3865, "assert:\n"+t.state.traceLiteralsVerbose(lits)+"\n")
}

func (t *SMTTheorySeq) traceDequeAxiom(n ast.ASTHandle) {
	if !smtTraceEnabled("seq") {
		return
	}
	emitSMTTrace("seq", "deque_axiom", "src/smt/theory_seq.cpp", 3098, "deque: "+tracePP(t.m, n)+"\n")
}

func (t *SMTTheorySeq) traceAutomaton(a *rewriter.StringRegexAutomaton) {
	if !smtTraceEnabled("seq") {
		return
	}
	g, err := a.Graph()
	if err != nil {
		return
	}
	var b strings.Builder
	b.WriteString("init: " + strconv.Itoa(int(g.Initial())) + "\nfinal: ")
	for _, state := range g.FinalStates() {
		b.WriteString(strconv.Itoa(int(state)) + " ")
	}
	b.WriteString("\n")
	for state := uint32(0); state < g.NumStates(); state++ {
		for _, mv := range g.MovesFrom(state) {
			b.WriteString(strconv.Itoa(int(state)) + " -> " + strconv.Itoa(int(mv.Destination)) + " ")
			if !mv.Epsilon {
				b.WriteString("if ")
				b.WriteString(t.traceCondition(a, mv.Condition))
			}
			b.WriteString("\n")
		}
	}
	emitSMTTrace("seq", "get_automaton", "src/smt/theory_seq.cpp", 4174, b.String())
}

// traceCondition is sym_expr::display.
func (t *SMTTheorySeq) traceCondition(a *rewriter.StringRegexAutomaton, id uint32) string {
	c, err := a.Condition(id)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	switch c.Kind {
	case rewriter.StringRegexCharacter:
		return tracePP(t.m, c.Character)
	case rewriter.StringRegexRange:
		return tracePP(t.m, c.Lower) + ":" + tracePP(t.m, c.Upper)
	}
	return tracePP(t.m, c.Predicate)
}
