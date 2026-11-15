package spacer

// Source: src/muz/spacer/spacer_context.cpp solve_core, check_reachability,
// is_reachable, expand_node, propagate and derivation::create_next_child
// IF_VERBOSE(1, ...) progress lines. AUDIT.md FUZZ-F5: compiled only into
// g3trace builds (diagnostic.VerboseProgress).

import (
	"errors"
	"strconv"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// spacerProgressEnabled is the IF_VERBOSE(1) test, false in untagged builds.
func spacerProgressEnabled() bool {
	if !diagnostic.VerboseProgress {
		return false
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	return err == nil && enabled
}

// spacerProgress appends text to verbose_stream() and flushes it.
func spacerProgress(text string) error {
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = stream.AppendCString(text); err != nil {
		return err
	}
	return stream.Flush()
}

// obligationLabel is `name << " (" << level << ", " << depth << ") " <<
// (FAR|SUB)` followed, for expand_node, by `" w(" << weakness << ") "`, and
// the post's AST id.
func (o *ProofObligation) verboseLabel(depth uint32, weakness bool) (string, error) {
	t := o.transformer
	head, err := t.Head()
	if err != nil {
		return "", err
	}
	name, err := t.manager.GetASTName(head)
	if err != nil {
		return "", err
	}
	text, err := t.manager.Symbols.String(name)
	if err != nil {
		return "", err
	}
	post, err := t.manager.RefGet(o.post)
	if err != nil {
		return "", err
	}
	id, err := t.manager.GetASTID(post)
	if err != nil {
		return "", err
	}
	s := text + " (" + strconv.FormatUint(uint64(o.level), 10) + ", " + strconv.FormatUint(uint64(depth), 10) + ") "
	if o.useFarkas {
		s += "FAR "
	} else {
		s += "SUB "
	}
	if weakness {
		s += " w(" + strconv.FormatUint(uint64(o.weakness), 10) + ") "
	}
	return s + strconv.FormatUint(uint64(id), 10), nil
}

// spacerNodeWatch is expand_node's / is_reachable's local stopwatch; the
// zero value is a disabled watch.
type spacerNodeWatch struct {
	start   time.Time
	enabled bool
}

// begin prints the header (prefix + label) and starts the watch.
func (o *ProofObligation) verboseBegin(prefix string, depth uint32, weakness bool) (spacerNodeWatch, error) {
	if !spacerProgressEnabled() {
		return spacerNodeWatch{}, nil
	}
	label, err := o.verboseLabel(depth, weakness)
	if err != nil {
		return spacerNodeWatch{}, err
	}
	if err = spacerProgress(prefix + label); err != nil {
		return spacerNodeWatch{}, err
	}
	return spacerNodeWatch{start: time.Now(), enabled: true}, nil
}

// end prints `verdict << std::fixed << std::setprecision(2) << seconds <<
// "\n"`. The fixed/precision flags stay set on the stream, as in C++.
func (w spacerNodeWatch) end(verdict string) error {
	if !w.enabled || !spacerProgressEnabled() {
		return nil
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = stream.AppendCString(verdict); err != nil {
		return err
	}
	if err = stream.SetFloatMode(utext.NumericFixedFloat); err != nil {
		return err
	}
	if err = stream.SetPrecision(2); err != nil {
		return err
	}
	if err = stream.AppendFloat64(time.Since(w.start).Seconds()); err != nil {
		return err
	}
	if err = stream.AppendCString("\n"); err != nil {
		return err
	}
	return stream.Flush()
}

// dot is expand_node's IF_VERBOSE(1, "." << std::flush).
func (w spacerNodeWatch) dot() error {
	if !w.enabled || !spacerProgressEnabled() {
		return nil
	}
	return spacerProgress(".")
}

// spacerTimeit is `timeit _timer(get_verbosity_level() >= 1, msg,
// verbose_stream())`; nil when disabled.
func spacerTimeit(msg string) (*stopwatch.Timeit, error) {
	if !spacerProgressEnabled() {
		return nil, nil
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return nil, err
	}
	t := &stopwatch.Timeit{}
	memory := func() (uint64, error) { return 0, nil }
	if err = t.Construct(true, stopwatch.TimeitConfig{Output: spacerStreamWriter{stream}, Message: msg, Memory: memory}); err != nil {
		_ = t.Cleanup()
		return nil, err
	}
	return t, nil
}

// closeSpacerTimeit is ~timeit, which also leaves std::fixed and
// setprecision(2) on the stream.
func closeSpacerTimeit(t *stopwatch.Timeit) error {
	if t == nil {
		return nil
	}
	if err := t.Close(); err != nil {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = stream.SetFloatMode(utext.NumericFixedFloat); err != nil {
		return err
	}
	return stream.SetPrecision(2)
}

type spacerStreamWriter struct{ out *utext.NumericTextStream }

func (w spacerStreamWriter) Write(p []byte) (int, error) {
	if err := w.out.AppendCString(string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w spacerStreamWriter) Flush() error { return w.out.Flush() }

// closedLabel is check_reachability's `name << "(" << level << ", " <<
// depth << ")" << " " << post id`.
func (o *ProofObligation) closedLabel() (string, error) {
	t := o.transformer
	head, err := t.Head()
	if err != nil {
		return "", err
	}
	name, err := t.manager.GetASTName(head)
	if err != nil {
		return "", err
	}
	text, err := t.manager.Symbols.String(name)
	if err != nil {
		return "", err
	}
	post, err := t.manager.RefGet(o.post)
	if err != nil {
		return "", err
	}
	id, err := t.manager.GetASTID(post)
	if err != nil {
		return "", err
	}
	return text + "(" + strconv.FormatUint(uint64(o.level), 10) + ", " + strconv.FormatUint(uint64(o.depth), 10) + ") " + strconv.FormatUint(uint64(id), 10), nil
}

// spacerPPText is mk_pp(h, m) through the SMT2 printer.
func spacerPPText(m *ast.ASTManager, h ast.ASTHandle) (_ string, resultErr error) {
	document, err := m.SMT2PPExprTree(h, nil)
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

// itpCoreVerbose is itp_solver::get_itp_core's IF_VERBOSE(2, "Itp Core:\n"
// << mk_pp(mk_and(core), m)).
func itpCoreVerbose(m *ast.ASTManager, core []ast.ASTRef) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	if enabled, err := diagnostic.ProcessVerboseEnabled(2); err != nil || !enabled {
		return err
	}
	values := make([]ast.ASTHandle, len(core))
	for i := range core {
		h, err := m.RefGet(core[i])
		if err != nil {
			return err
		}
		values[i] = h
	}
	conjunction, err := m.MkAndArray(values)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(conjunction)
	if err != nil {
		return err
	}
	text, err := spacerPPText(m, conjunction)
	err = errors.Join(err, m.DelRef(&ref))
	if err != nil {
		return err
	}
	return spacerProgress("Itp Core:\n" + text + "\n")
}

// inductivePropertyVerbose is context::solve's IF_VERBOSE(1, verbose_stream()
// << inductive_property(m, null mc, rs).to_string()) after an l_false.
func (c *Context) inductivePropertyVerbose() (resultErr error) {
	if !spacerProgressEnabled() {
		return nil
	}
	property, err := c.inductiveProperty()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, property.Close()) }()
	text, err := property.SMT2(c.config.Itp.Params, c.config.Itp.Globals)
	if err != nil {
		return err
	}
	return spacerProgress(text)
}
