package smtcore

// Source: TRACE records of src/smt/smt_context.cpp (assign_core:293,
// add_eq:497, decide:1829,1885), src/smt/smt_internalizer.cpp (mk_bool_var:825),
// and the display helpers they use: src/smt/smt_context_pp.cpp:99-105 and
// 572-601 (display_literal_verbose, display(b_justification)) and
// src/smt/smt_literal.cpp:24-56 (literal::display, operator<<).
//
// The records are written in the z3 TRACE format to .z3-trace when the build
// has -tags g3trace and the tag is enabled with -tr:tag, so a trace of g3 can
// be compared with a trace of a z3 TRACE build of the same input.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// smtTraceEnabled is the TRACE gate: the compile gate and the tag test.
func smtTraceEnabled(tag string) bool {
	return diagnostic.TraceCompiled() && diagnostic.BorrowProcessTraceTags().ShouldTrace(tag)
}

// emitSMTTrace writes one TRACE(tag, ...) record. Callers test
// smtTraceEnabled first so the body is built only when it is written.
func emitSMTTrace(tag, function, file string, line int, body string) {
	_ = diagnostic.BorrowProcessTraceTags().Emit(tag, function, file, line, body)
}

// tracePP is mk_pp(e, m).
func tracePP(m *ast.ASTManager, h ast.ASTHandle) string {
	var text string
	tracePPTo(m, h, func(t string) { text = t })
	return text
}

// tracePPTo is operator<<(mk_pp): the text is written while the format is
// still alive, and the format is released afterwards.
func tracePPTo(m *ast.ASTManager, h ast.ASTHandle, w func(string)) {
	text := tracePPText(m, h, w)
	if text != "" {
		w(text)
	}
}

// tracePPText returns its text only on early failure; otherwise it hands the
// rendered text to w before the deferred format release.
func tracePPText(m *ast.ASTManager, h ast.ASTHandle, w func(string)) string {
	if h == (ast.ASTHandle{}) {
		return "null"
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	if err != nil {
		return "<" + err.Error() + ">"
	}
	defer stream.Close()
	f, err := m.SMT2PPExprTree(h, nil)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	fm, err := m.GetFormatManager()
	if err != nil {
		return "<" + err.Error() + ">"
	}
	ref, err := fm.NewRef(f)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	defer fm.DelRef(&ref)
	if err = m.PrintFormat(stream, f, nil, nil, nil); err != nil {
		return "<" + err.Error() + ">"
	}
	text, err := stream.String()
	if err != nil {
		return "<" + err.Error() + ">"
	}
	w(text)
	return ""
}

// traceBoundedPP is mk_bounded_pp(e, m) with the default depth 3.
func traceBoundedPP(m *ast.ASTManager, h ast.ASTHandle) string {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	if err != nil {
		return "<" + err.Error() + ">"
	}
	defer stream.Close()
	if err = m.DisplayBoundedLowLevel(stream, h, 3); err != nil {
		return "<" + err.Error() + ">"
	}
	text, err := stream.String()
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return text
}

// traceLiteral is operator<<(ostream, literal).
func traceLiteral(l SMTLiteral) string {
	switch {
	case l == SMTTrueLiteral:
		return "true"
	case l == SMTFalseLiteral:
		return "false"
	case l.Sign():
		return "(not p" + strconv.Itoa(int(l.Var())) + ")"
	}
	return "p" + strconv.Itoa(int(l.Var()))
}

// traceLiteralVerbose is literal::display(out, m, bool_var2expr).
func (s *SMTInternalizerState) traceLiteralVerbose(l SMTLiteral) string {
	var b strings.Builder
	s.traceLiteralVerboseTo(l, func(t string) { b.WriteString(t) })
	return b.String()
}

// traceLiteralVerboseTo writes in source stream order: "(not " precedes the
// mk_pp formatting, whose format terms the AST log records in between.
func (s *SMTInternalizerState) traceLiteralVerboseTo(l SMTLiteral, w func(string)) {
	switch {
	case l == SMTTrueLiteral:
		w("true")
		return
	case l == SMTFalseLiteral:
		w("false")
		return
	case l == SMTNullLiteral:
		w("null")
		return
	}
	if l.Sign() {
		w("(not ")
		tracePPTo(s.manager, s.boolExpr[l.Var()], w)
		w(")")
		return
	}
	tracePPTo(s.manager, s.boolExpr[l.Var()], w)
}

// traceLiteralsVerbose is display_literals_verbose: separated by newlines.
func (s *SMTInternalizerState) traceLiteralsVerbose(lits []SMTLiteral) string {
	var b strings.Builder
	s.traceLiteralsVerboseTo(lits, func(t string) { b.WriteString(t) })
	return b.String()
}

func (s *SMTInternalizerState) traceLiteralsVerboseTo(lits []SMTLiteral, w func(string)) {
	for i, l := range lits {
		if i > 0 {
			w("\n")
		}
		s.traceLiteralVerboseTo(l, w)
	}
}

// traceJustification is context::display(out, b_justification).
func (s *SMTInternalizerState) traceJustification(j SMTBooleanJustification) string {
	var b strings.Builder
	s.traceJustificationTo(j, func(t string) { b.WriteString(t) })
	return b.String()
}

func (s *SMTInternalizerState) traceJustificationTo(j SMTBooleanJustification, w func(string)) {
	switch j.kind {
	case SMTBooleanJustificationAxiom:
		w("axiom")
	case SMTBooleanJustificationBinaryClause:
		w("bin-clause ")
		s.traceLiteralVerboseTo(j.literal, w)
	case SMTBooleanJustificationClause:
		w("clause ")
		if j.clause != nil {
			s.traceLiteralsVerboseTo(j.clause.literals, w)
		}
	case SMTBooleanJustificationObject:
		theory := SMTNullTheoryID
		if j.justification != nil {
			theory = j.justification.FromTheory()
		}
		w("justification " + strconv.Itoa(int(theory)) + ": ")
		if j.justification != nil {
			s.traceLiteralsVerboseTo(s.traceJustificationLiterals(j.justification), w)
		}
	}
	w("\n")
}

// traceJustificationLiterals is conflict_resolution::justification2literals.
func (s *SMTInternalizerState) traceJustificationLiterals(j SMTJustification) []SMTLiteral {
	e := NewSMTAntecedentExpander()
	e.literalOf = (&SMTConflictResolver{state: s}).enodeLiteral
	lits, err := e.ExpandJustification(j)
	if err != nil {
		return nil
	}
	return lits
}

func (s *SMTInternalizerState) traceAssignCore(l SMTLiteral, j SMTBooleanJustification, decision bool) {
	if !smtTraceEnabled("assign_core") {
		return
	}
	kind := "propagating: "
	if decision {
		kind = "decision: "
	}
	emitSMTTrace("assign_core", "assign_core", "src/smt/smt_context.cpp", 293,
		kind+traceLiteral(l)+" "+s.traceLiteralVerbose(l)+" level: "+strconv.Itoa(int(s.scopeLevel))+"\n"+s.traceJustification(j))
}

func (s *SMTInternalizerState) traceAddEqDetail(n1, n2 *SMTEnode, j SMTEqualityJustification) {
	if !smtTraceEnabled("add_eq_detail") {
		return
	}
	emitSMTTrace("add_eq_detail", "add_eq", "src/smt/smt_context.cpp", 497,
		"assigning\n"+tracePP(s.manager, n1.owner)+"\n"+tracePP(s.manager, n2.owner)+"\nkind: "+strconv.Itoa(int(j.kind))+"\n")
}

// traceAddEqMerging is add_eq's "merging" record.
func (s *SMTInternalizerState) traceAddEqMerging(r1, r2, n1 *SMTEnode) {
	if !smtTraceEnabled("add_eq") {
		return
	}
	id := func(n *SMTEnode) string { v, _ := n.OwnerID(); return strconv.Itoa(int(v)) }
	emitSMTTrace("add_eq", "add_eq", "src/smt/smt_context.cpp", 528,
		"merging: #"+id(r1)+" #"+id(r2)+" n1: #"+id(n1)+"\n")
}

func (s *SMTInternalizerState) traceDecide(variable int32, positive bool) {
	if !smtTraceEnabled("decide") {
		return
	}
	emitSMTTrace("decide", "decide", "src/smt/smt_context.cpp", 1829, "splitting, lvl: "+strconv.Itoa(int(s.scopeLevel))+"\n")
	pos := "0"
	if positive {
		pos = "1"
	}
	emitSMTTrace("decide", "decide", "src/smt/smt_context.cpp", 1885,
		"case split pos: "+pos+" p"+strconv.Itoa(int(variable))+"\nactivity: "+strconv.FormatFloat(s.activity[variable], 'g', 6, 64)+"\n")
}

func (s *SMTInternalizerState) traceMkBoolVar(variable int32, expression ast.ASTHandle) {
	if !smtTraceEnabled("mk_bool_var") {
		return
	}
	emitSMTTrace("mk_bool_var", "mk_bool_var", "src/smt/smt_internalizer.cpp", 825,
		"creating boolean variable: "+strconv.Itoa(int(variable))+" for:\n"+tracePP(s.manager, expression)+"\n")
}

func traceBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// traceMkEnode is the mk_enode record (src/smt/smt_internalizer.cpp:939).
func (s *SMTInternalizerState) traceMkEnode(node *SMTEnode, count uint32, congruenceEnabled bool) {
	id, err := node.OwnerID()
	if err != nil {
		return
	}
	body := "created enode: #" + strconv.Itoa(int(id)) + " for:\n" + tracePP(s.manager, node.owner) + "\n"
	if count > 0 {
		inTable := congruenceEnabled && node.congruence == node && !node.IsTrueEquality()
		body += "is_true_eq: " + traceBool(node.IsTrueEquality()) + " in cg_table: " + traceBool(inTable) + " is_cgr: " + traceBool(node.congruence == node) + "\n"
	}
	emitSMTTrace("mk_enode", "mk_enode", "src/smt/smt_internalizer.cpp", 939, body)
}

// tracePropagate is the TRACE pair at the head of relevancy_propagator::
// propagate (src/smt/smt_relevancy.cpp:462-463).
func (p *SMTRelevancyPropagator) tracePropagate(n ast.ASTHandle) {
	if smtTraceEnabled("propagate_relevancy_to_args") {
		id, _ := p.context.manager.GetASTID(n)
		emitSMTTrace("propagate_relevancy_to_args", "propagate", "src/smt/smt_relevancy.cpp", 462,
			"propagating relevancy to args of #"+strconv.FormatUint(uint64(id), 10)+"\n")
	}
	if smtTraceEnabled("propagate_relevancy") {
		emitSMTTrace("propagate_relevancy", "propagate", "src/smt/smt_relevancy.cpp", 463,
			"marking as relevant:\n"+traceBoundedPP(p.context.manager, n)+"\n")
	}
}

// traceUnmark is the TRACE in relevancy_propagator_imp::unmark_relevant_exprs
// (src/smt/smt_relevancy.cpp:279).
func (p *SMTRelevancyPropagator) traceUnmark(n ast.ASTHandle) {
	if smtTraceEnabled("propagate_relevancy") {
		emitSMTTrace("propagate_relevancy", "unmark_relevant_exprs", "src/smt/smt_relevancy.cpp", 279,
			"unmarking:\n"+tracePP(p.context.manager, n)+"\n")
	}
}

// traceRelevantEh is the TRACE in context::relevant_eh
// (src/smt/smt_context.cpp:1614).
func (s *SMTInternalizerState) traceRelevantEh(n ast.ASTHandle) {
	if smtTraceEnabled("propagate_relevancy") {
		emitSMTTrace("propagate_relevancy", "relevant_eh", "src/smt/smt_context.cpp", 1614,
			"marking as relevant:\n"+traceBoundedPP(s.manager, n)+" "+strconv.FormatUint(uint64(s.scopeLevel), 10)+"\n")
	}
}

// tracePropagateAtom is the TRACE in context::propagate_atoms
// (src/smt/smt_context.cpp:1380).
func (s *SMTInternalizerState) tracePropagateAtom(l SMTLiteral, d *SMTBoolVarData) {
	if !smtTraceEnabled("propagate_atoms") {
		return
	}
	id, _ := s.manager.GetASTID(s.boolExpr[l.Var()])
	tag := ""
	if d.IsEquality() {
		tag += "eq"
	}
	if d.IsTheoryAtom() {
		tag += "th"
	}
	if d.IsQuantifier() {
		tag += "q"
	}
	enode := "0"
	if d.IsENode() {
		enode = "1"
	}
	emitSMTTrace("propagate_atoms", "propagate_atoms", "src/smt/smt_context.cpp", 1380,
		"propagating atom, #"+strconv.FormatUint(uint64(id), 10)+", is_enode(): "+enode+" tag: "+tag+" "+traceLiteral(l)+"\n")
}

// traceLiteralCompact is literal::display_compact (context::display_literal).
func (s *SMTInternalizerState) traceLiteralCompact(l SMTLiteral) string {
	switch {
	case l == SMTTrueLiteral:
		return "true"
	case l == SMTFalseLiteral:
		return "false"
	}
	id, _ := s.manager.GetASTID(s.boolExpr[l.Var()])
	if l.Sign() {
		return "(not #" + strconv.FormatUint(uint64(id), 10) + ")"
	}
	return "#" + strconv.FormatUint(uint64(id), 10)
}

// traceLiteralsCompact is context::display_literals: space separated.
func (s *SMTInternalizerState) traceLiteralsCompact(lits []SMTLiteral) string {
	var b strings.Builder
	for i, l := range lits {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(s.traceLiteralCompact(l))
	}
	return b.String()
}

// traceConflict emits a TRACE("conflict", ...) record of
// src/smt/smt_conflict_resolution.cpp.
func (s *SMTInternalizerState) traceConflict(function string, line int, body func() string) {
	if smtTraceEnabled("conflict") {
		emitSMTTrace("conflict", function, "src/smt/smt_conflict_resolution.cpp", line, body())
	}
}

// traceBJustificationKind is operator<<(ostream, b_justification::kind).
func traceBJustificationKind(k SMTBooleanJustificationKind) string {
	switch k {
	case SMTBooleanJustificationClause:
		return "clause"
	case SMTBooleanJustificationBinaryClause:
		return "bin_clause"
	case SMTBooleanJustificationAxiom:
		return "axiom"
	case SMTBooleanJustificationObject:
		return "theory"
	}
	return ""
}

// traceLBool is operator<<(ostream, lbool).
func traceLBool(v lbool.LBool) string {
	switch v {
	case lbool.True:
		return "l_true"
	case lbool.False:
		return "l_false"
	}
	return "l_undef"
}

// traceClauseDetail is context::display_clause_detail.
func (s *SMTInternalizerState) traceClauseDetail(c *SMTClause, levels []uint32) string {
	var b strings.Builder
	b.WriteString("lemma: " + traceBool(c.IsLemma()) + "\n")
	for _, l := range c.literals {
		b.WriteString(s.traceLiteralCompact(l))
		b.WriteString(", val: " + traceLBool(s.assignment[l.Index()]))
		b.WriteString(", lvl: " + strconv.FormatUint(uint64(levels[l.Var()]), 10))
		b.WriteString(", ilvl: " + strconv.FormatUint(uint64(s.booleanData[l.Var()].InternalLevel()), 10))
		b.WriteString(", var: " + strconv.Itoa(int(l.Var())) + "\n")
		b.WriteString(tracePP(s.manager, s.boolExpr[l.Var()]) + "\n\n")
	}
	return b.String()
}

// traceAssignRelevancy is assign_core's TRACE("relevancy") record.
func (s *SMTInternalizerState) traceAssignRelevancy(l SMTLiteral) {
	if !smtTraceEnabled("relevancy") {
		return
	}
	bit := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	relevant := s.relevancy != nil && s.relevancy.IsRelevant(s.boolExpr[l.Var()])
	emitSMTTrace("relevancy", "assign_core", "src/smt/smt_context.cpp", 312,
		"is_atom: "+bit(s.booleanData[l.Var()].IsAtom())+" is relevant: "+bit(relevant)+"\n")
}
