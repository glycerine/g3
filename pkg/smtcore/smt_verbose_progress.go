package smtcore

// Source: src/smt/smt_context.cpp assert_expr, internalize_assertions and
// search (timeit), mk_proto_model (IF_VERBOSE(11, model_pp)); src/util/timeit.cpp.
// AUDIT.md FUZZ-F5. Every helper here is compiled only into g3trace builds
// (diagnostic.VerboseProgress, owner decision 2026-11-15).

import (
	"errors"

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
	features, err := k.setupStaticFeatures(true)
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
		features, err := k.setupStaticFeatures(true)
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
	features, err := k.setupStaticFeatures(true)
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
