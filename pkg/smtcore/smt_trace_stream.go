package smtcore

// Source: the m_manager.has_trace_stream() records of src/smt/smt_context.cpp,
// smt_context_pp.cpp, smt_internalizer.cpp and smt_conflict_resolution.cpp.
// They go to the AST manager's trace_file_name stream alongside [mk-app].

import (
	"strconv"

	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *SMTInternalizerState) traceStream() *utext.NumericTextStream {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.TraceStream
}

// traceStreamWriter appends each piece as soon as it is produced, so format
// terms created while pretty printing interleave as in the source stream.
func traceStreamWriter(out *utext.NumericTextStream) func(string) {
	return func(text string) { _ = out.AppendCString(text) }
}

// traceAssignStream is context::trace_assign.
func (s *SMTInternalizerState) traceAssignStream(l SMTLiteral, j SMTBooleanJustification, decision bool) {
	out := s.traceStream()
	if out == nil {
		return
	}
	w := traceStreamWriter(out)
	w("[assign] ")
	w(s.traceLiteralCompact(l))
	if decision {
		w(" decision")
	}
	w(" ")
	s.traceJustificationTo(j, w)
}

// traceAttachEnode is the [attach-enode] record ending context::mk_enode.
func (s *SMTInternalizerState) traceAttachEnode(node *SMTEnode) {
	out := s.traceStream()
	if out == nil {
		return
	}
	id, _ := s.manager.GetASTID(node.owner)
	traceStreamWriter(out)("[attach-enode] #" + strconv.FormatUint(uint64(id), 10) + " " + strconv.FormatUint(uint64(s.generation), 10) + "\n")
}

// tracePushStream is the [push] record opening context::push_scope.
func (s *SMTInternalizerState) tracePushStream() {
	if out := s.traceStream(); out != nil {
		traceStreamWriter(out)("[push] " + strconv.FormatUint(uint64(s.scopeLevel), 10) + "\n")
	}
}

// tracePopStream is the [pop] record opening context::pop_scope_core.
func (s *SMTInternalizerState) tracePopStream(count uint32) {
	if out := s.traceStream(); out != nil {
		traceStreamWriter(out)("[pop] " + strconv.FormatUint(uint64(count), 10) + " " + strconv.FormatUint(uint64(s.scopeLevel), 10) + "\n")
	}
}

// traceBeginCheckStream is the [begin-check] record of context::check_preamble.
func (s *SMTInternalizerState) traceBeginCheckStream() {
	if out := s.traceStream(); out != nil {
		traceStreamWriter(out)("[begin-check] " + strconv.FormatUint(uint64(s.scopeLevel), 10) + "\n")
	}
}

// traceConflictStream is the learned-lemma [conflict] record of
// context::resolve_conflict.
func (s *SMTInternalizerState) traceConflictStream(lits []SMTLiteral) {
	if out := s.traceStream(); out != nil {
		traceStreamWriter(out)("[conflict] " + s.traceLiteralsCompact(lits) + "\n")
	}
}

// traceResolveLitStream is conflict_resolution::process_antecedent's record.
func (s *SMTInternalizerState) traceResolveLitStream(levelDelta uint32, antecedent SMTLiteral) {
	if out := s.traceStream(); out != nil {
		traceStreamWriter(out)("[resolve-lit] " + strconv.FormatUint(uint64(levelDelta), 10) + " " + s.traceLiteralCompact(antecedent.Negated()) + "\n")
	}
}

// traceResolveProcessStream is conflict_resolution::resolve's record.
func (s *SMTInternalizerState) traceResolveProcessStream(consequent SMTLiteral) {
	if out := s.traceStream(); out != nil {
		traceStreamWriter(out)("[resolve-process] " + s.traceLiteralCompact(consequent.Negated()) + "\n")
	}
}
