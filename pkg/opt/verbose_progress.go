package opt

// Source: src/opt/maxsmt.cpp trace_bounds, find_mutexes, maxsmt::operator()
// and display_answer; src/opt/maxres.cpp, wmax.cpp, sortmax.cpp and
// optsmt.cpp IF_VERBOSE progress lines. AUDIT.md FUZZ-F5: compiled only
// into g3trace builds (diagnostic.VerboseProgress, owner decision
// 2026-11-15).

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// optProgress is IF_VERBOSE(level, verbose_stream() << text).
func optProgress(level uint32, text string) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	return diagnostic.ProgressVerbose(level, text)
}

// optProgressEnabled is the IF_VERBOSE level test, false in untagged builds.
func optProgressEnabled(level uint32) (bool, error) {
	if !diagnostic.VerboseProgress {
		return false, nil
	}
	return diagnostic.ProcessVerboseEnabled(level)
}

// ratText is `out << rational`: an integer, or numerator/denominator.
func (s *ArithmeticSolver) ratText(r *numeral.Rational) string {
	o := newRatOps(s.manager)
	return o.text(r)
}

// adjustSoftBound is maxsmt_solver_base::m_adjust_value for objective
// index into out. The engines' bounds already include the objective's soft
// offset; an objective converted from minimize/maximize also negates and
// offsets (from_fmls' set_negate/set_offset, kept as its verification).
func (s *ArithmeticSolver) adjustSoftBound(o *ratOps, index int, x, out *numeral.Rational) {
	o.set(out, x)
	if index >= 0 && index < len(s.verifications) && s.verifications[index] != nil {
		v := s.verifications[index]
		if v.negate {
			o.neg(out)
		}
		o.add(out, out, &v.offset)
	}
}

// traceBounds is maxsmt_solver_base::trace_bounds(solver):
// IF_VERBOSE(1, l = adjust(m_lower); u = adjust(m_upper); if (l > u)
// swap; "(opt.<solver> [l:u])").
func (s *ArithmeticSolver) traceBounds(solver string, index int, lower, upper *numeral.Rational) error {
	if enabled, err := optProgressEnabled(1); err != nil || !enabled {
		return err
	}
	o := newRatOps(s.manager)
	l, u := o.zero(), o.zero()
	defer o.del(&u)
	defer o.del(&l)
	s.adjustSoftBound(&o, index, lower, &l)
	s.adjustSoftBound(&o, index, upper, &u)
	lt, ut := o.text(&l), o.text(&u)
	if o.cmp(&l, &u) > 0 {
		lt, ut = ut, lt
	}
	if !o.ok() {
		return o.err
	}
	return optProgress(1, "(opt."+solver+" ["+lt+":"+ut+"])\n")
}

// mutexVerbose is find_mutexes/process_mutex's
// IF_VERBOSE(1, "(opt.maxsat mutex size: " << i + 1 << " weight: " << weight << ")").
func (s *ArithmeticSolver) mutexVerbose(size int, weight *numeral.Rational) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	return mutexVerboseText(size, s.ratText(weight))
}

func mutexVerboseText(size int, weight string) error {
	return optProgress(1, "(opt.maxsat mutex size: "+strconv.Itoa(size)+" weight: "+weight+")\n")
}

// mkPPString is mk_pp(e, m) through the SMT2 printer.
func (s *ArithmeticSolver) mkPPString(term ast.ASTHandle) (string, error) {
	return mkPPStringOf(s.manager, term)
}

func mkPPStringOf(m *ast.ASTManager, term ast.ASTHandle) (_ string, resultErr error) {
	document, err := m.SMT2PPExprTree(term, nil)
	if err != nil {
		return "", err
	}
	formats, err := m.GetFormatManager()
	if err != nil {
		return "", err
	}
	ref, err := formats.NewRef(document)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, stream.Close()) }()
	context, globals, err := m.RewriterParameterServices()
	if err != nil {
		return "", err
	}
	if context != nil && globals != nil {
		empty := context.EmptyValue()
		err = m.PrintFormat(stream, document, context, &empty, globals)
	} else {
		err = m.PrintFormat(stream, document, nil, nil, nil)
	}
	if err != nil {
		return "", err
	}
	return stream.String()
}

// maxsmtBeginVerbose is maxsmt::operator()'s IF_VERBOSE(1, "(maxsmt)").
func maxsmtBeginVerbose() error { return optProgress(1, "(maxsmt)\n") }

// maxsmtEndVerbose is maxsmt::operator()'s closing
// IF_VERBOSE(1, "is-sat: " << is_sat; if l_true: "Satisfying soft
// constraints" and display_answer).
func (s *ArithmeticSolver) maxsmtEndVerbose(index int, status lbool.LBool, result *ArithmeticResult) error {
	if enabled, err := optProgressEnabled(1); err != nil || !enabled {
		return err
	}
	var out strings.Builder
	out.WriteString("is-sat: " + status.String() + "\n")
	if status == lbool.True && index >= 0 && index < len(s.objectives) {
		out.WriteString("Satisfying soft constraints\n")
		objective := &s.objectives[index]
		var assignment []bool
		if result != nil && index < len(result.Values) {
			assignment = result.Values[index].SoftAssignment
		}
		for i := range objective.softTerms {
			term, err := s.manager.RefGet(objective.softTerms[i])
			if err != nil {
				return err
			}
			// bool is_not = m.is_not(e, e);
			isNot, arg, err := s.manager.IsNot(term)
			if err != nil {
				return err
			}
			if isNot {
				term = arg
			}
			text, err := s.mkPPString(term)
			if err != nil {
				return err
			}
			assigned := i < len(assignment) && assignment[i]
			truth := " |-> false "
			if isNot != assigned {
				truth = " |-> true "
			}
			out.WriteString(s.ratText(&objective.softWeights[i]) + ": " + text + truth + "\n")
		}
	}
	return optProgress(1, out.String())
}

// traceID is maxres::m_trace_id.
func (e *maxResEngine) traceID() string {
	if e.strategy == maxResPrimalDual {
		return "pd-maxres"
	}
	return "maxres"
}

// trace is maxres::trace(): trace_bounds(m_trace_id).
func (e *maxResEngine) trace() error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	return e.solver.traceBounds(e.traceID(), e.index, &e.lower, &e.upper)
}

// finishVerbose is the end of mus_solver / primal_dual_solver. mus_solver
// prints found_optimum's "found optimum" and, when it leaves the
// lower < upper loop, trace(); primal_dual_solver traces after its loop.
func (e *maxResEngine) finishVerbose(loopExit bool) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	if e.strategy == maxResPrimalDual {
		return e.trace()
	}
	if err := optProgress(1, "found optimum\n"); err != nil {
		return err
	}
	if loopExit {
		return e.trace()
	}
	return nil
}

// coreVerbose is process_unsat(core)'s IF_VERBOSE(3, "(maxres cs model
// valid: ...") before remove_core.
func (e *maxResEngine) coreStartVerbose(core []ast.ASTHandle) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	valid := "0"
	if e.correctionModel != nil {
		valid = "1"
	}
	return optProgress(3, "(maxres cs model valid: "+valid+" cs size:"+strconv.Itoa(e.correctionSize)+
		" core: "+strconv.Itoa(len(core))+")\n")
}

// coreDisplayVerbose is IF_VERBOSE(10, display_vec(verbose_stream() <<
// "core: ", core)) after split_core: mk_pp(e) " : " weight " " each.
func (e *maxResEngine) coreDisplayVerbose(core []ast.ASTHandle) error {
	if enabled, err := optProgressEnabled(10); err != nil || !enabled {
		return err
	}
	var out strings.Builder
	out.WriteString("core: ")
	for _, expression := range core {
		text, err := e.solver.mkPPString(expression)
		if err != nil {
			return err
		}
		weight := "0"
		if w := e.weights[expression]; w != nil {
			weight = e.solver.ratText(w)
		}
		out.WriteString(text + " : " + weight + " ")
	}
	out.WriteString("\n")
	return optProgress(10, out.String())
}

// infEpsText is inf_eps_rational<inf_rational>::to_string.
func infEpsText(o *ratOps, v *simplex.Infinitesimal) string {
	if !o.ok() {
		return ""
	}
	text, err := smtcore.SMTInfEpsString(o.q, v)
	o.fail(err)
	return text
}

// lowerLexVerbose is optsmt::update_lower_lex's IF_VERBOSE(1, maximize ?
// "(optsmt lower bound: " << v : "(optsmt upper bound: " << -v).
func (s *ArithmeticSolver) lowerLexVerbose(idx int, v *simplex.Infinitesimal) error {
	if !diagnostic.VerboseProgress || idx < 0 || idx >= len(s.objectives) {
		return nil
	}
	o := newRatOps(s.manager)
	if s.objectives[idx].maximize {
		text := infEpsText(&o, v)
		return o.joined(optProgress(1, "(optsmt lower bound: "+text+")\n"))
	}
	// -v is a temporary.
	n := o.infClone(v)
	o.infNeg(&n)
	text := infEpsText(&o, &n)
	o.infDel(&n)
	return o.joined(optProgress(1, "(optsmt upper bound: "+text+")\n"))
}
