package smtcore

// Source: src/smt/smt_context.cpp assert_expr, internalize_assertions and
// search (timeit), mk_proto_model (IF_VERBOSE(11, model_pp)); src/util/timeit.cpp.
// AUDIT.md FUZZ-F5. Every helper here is compiled only into g3trace builds
// (diagnostic.VerboseProgress, owner decision 2026-11-15).

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// startTimeit is `timeit tt(get_verbosity_level() >= 100, msg)`: the report
// goes to std::cerr, timeit's default stream. The command context binds
// verbose_stream() to its diagnostic channel, std::cerr by default, so g3
// writes to the current process verbose stream, which then stands for
// std::cerr. (Only with :diagnostic-output-channel redirected to a file
// does the C++ keep timeit on std::cerr while g3 follows the channel.) It
// returns nil when the scope is disabled; pass the result to closeTimeit.
func (k *SMTKernel) startTimeit(msg string) (*stopwatch.Timeit, error) {
	if !diagnostic.VerboseProgress {
		return nil, nil
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(100)
	if err != nil || !enabled {
		return nil, err
	}
	// memory::get_allocation_size() is the process allocation owner.
	memory := func() (uint64, error) { return 0, nil }
	if k.manager != nil {
		if runtime := k.manager.NumeralService().RuntimeOwner(); runtime != nil {
			memory = runtime.AllocationSize
		}
	}
	cerr, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return nil, err
	}
	t := &stopwatch.Timeit{}
	if err = t.Construct(true, stopwatch.TimeitConfig{Output: cerrWriter{cerr}, Message: msg, Memory: memory}); err != nil {
		return nil, errors.Join(err, t.Cleanup())
	}
	return t, nil
}

// closeTimeit is ~timeit: it prints "(msg :time ... :after-memory ...)".
// Like the C++ destructor it also reports on an error (exception) path.
func closeTimeit(t *stopwatch.Timeit, cause error) error {
	if !diagnostic.VerboseProgress || t == nil {
		return cause
	}
	if err := t.Close(); err != nil {
		return errors.Join(cause, err)
	}
	// ~timeit wrote `std::fixed << std::setprecision(2)` into std::cerr; the
	// flags stay set, so later doubles printed to std::cerr (the default
	// verbose_stream) are fixed with two decimals, as in the C++.
	cerr, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return errors.Join(cause, err)
	}
	return errors.Join(cause, cerr.SetFloatMode(utext.NumericFixedFloat), cerr.SetPrecision(2))
}

// cerrWriter writes to the stream that stands for std::cerr.
type cerrWriter struct{ out *utext.NumericTextStream }

func (w cerrWriter) Write(p []byte) (int, error) {
	if err := w.out.AppendCString(string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w cerrWriter) Flush() error { return w.out.Flush() }

// verboseStreamWriter writes to the process verbose_stream().
type verboseStreamWriter struct{}

func (verboseStreamWriter) Write(p []byte) (int, error) {
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return 0, err
	}
	if err = out.AppendCString(string(p)); err != nil {
		return 0, err
	}
	return len(p), out.Flush()
}

// setupCollectFeaturesVerbose is the verbose part of collecting static
// features in setup_auto_config and setup_arith: "(smt.collecting-features)"
// at level 100, then IF_VERBOSE(1000, st.display_primitive(verbose_stream())).
func (k *SMTKernel) setupCollectFeaturesVerbose() error {
	if err := progressVerbose(100, "(smt.collecting-features)\n"); err != nil {
		return err
	}
	if enabled, err := diagnostic.ProcessVerboseEnabled(1000); err != nil || !enabled {
		return err
	}
	features, err := k.collectSetupFeatures()
	if err != nil {
		return err
	}
	return errors.Join(features.DisplayPrimitive(verboseStreamWriter{}), features.Close())
}

// setupUnknownVerbose is the verbose output of setup::setup_unknown():
// setup_arith, then setup_seq_str(st), whose setup_str calls setup_arith
// again for z3str3, or for auto without non-string sequences.
func (k *SMTKernel) setupUnknownVerbose() (result error) {
	if err := k.setupCollectFeaturesVerbose(); err != nil {
		return err
	}
	return k.setupSeqStrVerbose()
}

func (k *SMTKernel) setupSeqStrVerbose() (result error) {
	choice, _, err := k.manager.Symbols.BareString(k.smtState.StringSolver)
	if err != nil {
		return err
	}
	str := choice == "z3str3"
	if choice == "auto" {
		features, err := k.collectSetupFeatures()
		if err != nil {
			return err
		}
		str = !features.HasNonStringSequence
		if err = features.Close(); err != nil {
			return err
		}
	}
	if str {
		return k.setupCollectFeaturesVerbose()
	}
	return nil
}

// setupVerbose prints what setup::operator() prints for the config mode
// context::get_config_mode selects: setup_unknown() for CFG_BASIC,
// setup_default for CFG_LOGIC and setup_auto_config for CFG_AUTO
// (smt_setup.cpp; AUDIT.md FUZZ-F5). The Go setup is split over several
// policies, so the lines are produced here, before any of them runs; the
// setup itself prints nothing else.
func (k *SMTKernel) setupVerbose(useStaticFeatures bool) (result error) {
	if !diagnostic.VerboseProgress {
		return nil
	}
	if enabled, err := diagnostic.ProcessVerboseEnabled(100); err != nil || !enabled {
		return err
	}
	logic := k.setupLogic
	if !k.smtState.AutoConfig {
		return k.setupUnknownVerbose()
	}
	if !useStaticFeatures {
		if smtLogicUsesUnknownSetup(logic) {
			return k.setupUnknownVerbose()
		}
		return nil
	}
	if err := progressVerbose(100, "(smt.configuring)\n"); err != nil {
		return err
	}
	switch logic {
	case "QF_BV", "QF_AUFBV", "QF_ABV", "QF_UFBV":
		return nil
	}
	if err := k.setupCollectFeaturesVerbose(); err != nil {
		return err
	}
	if !smtAutoConfigUsesUnknownSetup(logic) {
		return nil
	}
	// setup_unknown(st)
	features, err := k.collectSetupFeatures()
	if err != nil {
		return err
	}
	quantified := features.NumQuantifiers != 0
	unknown := smtUnknownSetupUsesSequences(features)
	if err = features.Close(); err != nil {
		return err
	}
	if quantified {
		return k.setupSeqStrVerbose()
	}
	if unknown {
		return k.setupUnknownVerbose()
	}
	return nil
}

// protoModelVerbose is mk_proto_model's
// IF_VERBOSE(11, model_pp(verbose_stream(), *m_proto_model);).
func (k *SMTKernel) protoModelVerbose(generator *SMTModelGenerator) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	if enabled, err := diagnostic.ProcessVerboseEnabled(11); err != nil || !enabled {
		return err
	}
	if k.paramContext == nil || k.paramGlobals == nil || generator == nil || generator.model == nil {
		return nil
	}
	return generator.model.WriteDebug(verboseStreamWriter{}, k.paramContext, k.paramGlobals)
}

// appendVerboseDouble writes v as `verbose_stream() << double` does: the
// stream's current (sticky) float format applies.
func appendVerboseDouble(out *utext.NumericTextStream, v float64) error {
	return out.AppendFloat64(v)
}

// restartVerbose is context::restart's IF_VERBOSE(2, "(smt.restarting ...")
// after inc_limits.
func (k *SMTKernel) restartVerbose(state *smtRestartState) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	if enabled, err := diagnostic.ProcessVerboseEnabled(2); err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	stats := &k.internalizer.statistics
	for _, step := range []func() error{
		func() error { return out.AppendCString("(smt.restarting :propagations ") },
		func() error { return out.AppendUint32(stats.NumPropagations) },
		func() error { return out.AppendCString(" :decisions ") },
		func() error { return out.AppendUint32(stats.NumDecisions) },
		func() error { return out.AppendCString(" :conflicts ") },
		func() error { return out.AppendUint32(stats.NumConflicts) },
		func() error { return out.AppendCString(" :restart ") },
		func() error { return out.AppendUint32(state.threshold) },
		func() error {
			if k.smtState.RestartStrategy != 1 { // RS_IN_OUT_GEOMETRIC
				return nil
			}
			if err := out.AppendCString(" :restart-outer "); err != nil {
				return err
			}
			return out.AppendUint32(state.outerThreshold)
		},
		func() error {
			if !k.smtState.RestartAdaptive {
				return nil
			}
			if err := out.AppendCString(" :agility "); err != nil {
				return err
			}
			return appendVerboseDouble(out, k.internalizer.searchAgility)
		},
		func() error { return out.AppendCString(")\n") },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return out.Flush()
}

// tickVerbose is context::tick: every m_tick conflicts of one
// bounded_search, IF_VERBOSE(3, "(smt.working :conflicts N :agility A)").
// m_num_conflicts counts from init_search (conflictBase).
func (k *SMTKernel) tickVerbose(counter *uint32, conflictBase uint32) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	*counter++
	if *counter <= k.smtState.Tick {
		return nil
	}
	*counter = 0
	if enabled, err := diagnostic.ProcessVerboseEnabled(3); err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(smt.working :conflicts "); err != nil {
		return err
	}
	if err = out.AppendUint32(k.internalizer.statistics.NumConflicts - conflictBase); err != nil {
		return err
	}
	if k.smtState.RestartAdaptive {
		if err = out.AppendCString(" :agility "); err != nil {
			return err
		}
		if err = appendVerboseDouble(out, k.internalizer.searchAgility); err != nil {
			return err
		}
	}
	if err = out.AppendCString(")\n"); err != nil {
		return err
	}
	return out.Flush()
}

// smtInfEpsText is inf_eps_rational<inf_rational>::to_string.
func smtInfEpsText(q *numeral.RationalManager, v *smtArithmeticValue) string {
	// to_string reads the rationals; abs(infinitesimal) is a temporary.
	text := func(x *numeral.Rational) string {
		s, _ := q.String(x)
		return s
	}
	r := text(&v.rational)
	if v.infinitesimal.Sign() != 0 {
		sign := " +e*"
		if v.infinitesimal.Sign() < 0 {
			sign = " -e*"
		}
		abs := smtQZero()
		_ = q.Copy(&abs, &v.infinitesimal)
		_ = q.Abs(&abs)
		r = "(" + r + sign + text(&abs) + ")"
		_ = smtQDel(q, &abs)
	}
	if v.infinity.Sign() == 0 {
		return r
	}
	var si string
	switch {
	case v.infinity.IsOne():
		si = "oo"
	case v.infinity.IsMinusOne():
		si = "-oo"
	default:
		si = text(&v.infinity) + "*oo"
	}
	if v.rational.Sign() == 0 && v.infinitesimal.Sign() == 0 {
		return si
	}
	return "(" + si + " + " + r + ")"
}

// smtPPText is mk_pp(h, m) as text.
func smtPPText(m *ast.ASTManager, h ast.ASTHandle) string {
	var out string
	if failure := tracePPText(m, h, func(text string) { out = text }); failure != "" {
		return failure
	}
	return out
}

// utvpiConflictVerbose is theory_utvpi::set_conflict's IF_VERBOSE(2,
// "conflict:\n", then mk_pp of each literal2expr and a blank line).
func (t *SMTUTVPITheory) utvpiConflictVerbose(literals []SMTLiteral) error {
	if enabled, err := diagnostic.ProcessVerboseEnabled(2); err != nil || !enabled {
		return err
	}
	var out strings.Builder
	out.WriteString("conflict:\n")
	for _, literal := range literals {
		e, err := t.state.literalExpression(literal)
		if err != nil {
			return err
		}
		out.WriteString(smtPPText(t.state.manager, e) + "\n")
	}
	out.WriteString("\n")
	return progressVerbose(2, out.String())
}

// smtPPIndentText is ast_smt2_pp(out, e, env, p, indent): mk_pp with every
// line after the first indented by indent columns.
func smtPPIndentText(m *ast.ASTManager, h ast.ASTHandle, indent int) string {
	return strings.ReplaceAll(smtPPText(m, h), "\n", "\n"+strings.Repeat(" ", indent))
}

// smtPPVectorText is `out << expr_ref_vector`: mk_seq of the elements, each
// preceded by a line break.
func smtPPVectorText(m *ast.ASTManager, hs []ast.ASTHandle) string {
	var out strings.Builder
	for _, h := range hs {
		out.WriteString("\n" + smtPPText(m, h))
	}
	return out.String()
}

// seqDisplayDeps is theory_seq::display_deps(out, dep).
func (t *SMTTheorySeq) seqDisplayDeps(out *strings.Builder, dep *seqDep) error {
	var eqs []SMTEnodePair
	var lits []SMTLiteral
	t.linearize(dep, &eqs, &lits)
	for _, eq := range eqs {
		out.WriteString("  (= " + smtPPIndentText(t.m, eq.First.Owner(), 5) + "\n     " + smtPPIndentText(t.m, eq.Second.Owner(), 5) + ")\n")
	}
	for _, l := range lits {
		switch l {
		case SMTTrueLiteral:
			out.WriteString("   true")
		case SMTFalseLiteral:
			out.WriteString("   false")
		default:
			e, err := t.state.BoolExpression(l.Var())
			if err != nil {
				return err
			}
			if l.Sign() {
				out.WriteString("  (not " + smtPPIndentText(t.m, e, 7) + ")")
			} else {
				out.WriteString("  " + smtPPIndentText(t.m, e, 2))
			}
		}
		out.WriteString("\n")
	}
	return nil
}

// seqGiveupVerbose is theory_seq::is_solved's IF_VERBOSE(10, "(seq.giveup
// ...") lines (AUDIT.md FUZZ-F5; g3trace builds only). kind: 0 equation,
// 1 regular expression, 2 non-contains.
func (t *SMTTheorySeq) seqGiveupVerbose(kind int) {
	if !diagnostic.VerboseProgress {
		return
	}
	if enabled, err := diagnostic.ProcessVerboseEnabled(10); err != nil || !enabled {
		return
	}
	var out strings.Builder
	switch kind {
	case 0:
		eq := t.eqs.at(0)
		out.WriteString("(seq.giveup " + smtPPVectorText(t.m, eq.ls.handles()) + " = " + smtPPVectorText(t.m, eq.rs.handles()) + " is unsolved)\n")
	case 1:
		out.WriteString("(seq.giveup regular expression did not compile to automaton)\n")
	case 2:
		// display_nc: "not " << mk_pp(contains) "\n" then "  <- " deps "\n".
		nc := t.ncs.at(0)
		out.WriteString("(seq.giveup not " + smtPPText(t.m, nc.contains) + "\n  <- ")
		if err := t.seqDisplayDeps(&out, nc.dep); err != nil {
			return
		}
		out.WriteString("\n is unsolved)\n")
	}
	_ = progressVerbose(10, out.String())
}
