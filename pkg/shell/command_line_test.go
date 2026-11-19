package shell

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestShellCommandLineFilenameWarningAndOptionOrder(t *testing.T) {
	var output bytes.Buffer
	warnings := diagnostic.NewWarningService(&output)
	c := NewCommandLine(warnings, nil)
	if err := c.Parse([]string{"-smt2", "first.smt2", "-file:second.smt2", "-nw", "third.smt2", "--", "-fourth.smt2", "tail"}); err != nil {
		t.Fatal(err)
	}
	if name, selected := c.Input.Selected(); !selected || name != "second.smt2" {
		t.Fatal("source filename replacement and terminal duplicate", name, selected)
	}
	if c.Kind != InputSMT2 || warnings.Enabled() || output.Len() != 0 {
		t.Fatal("source option effects", c.Kind, warnings.Enabled(), output.String())
	}

	output.Reset()
	warnings = diagnostic.NewWarningService(&output)
	c = NewCommandLine(warnings, nil)
	if err := c.Parse([]string{"-file", "first", "--", "-strange name"}); err != nil {
		t.Fatal(err)
	}
	if name, selected := c.Input.Selected(); !selected || name != "first" || output.String() != "WARNING: input file was already specified.\n" {
		t.Fatal("null -file followed by terminal duplicate", name, selected, output.String())
	}
}

func TestShellCommandLineFixedTimeout(t *testing.T) {
	cmd := NewCommandLine(nil, nil)
	if err := cmd.Parse([]string{"-T:1000", "-T:1"}); !errors.Is(err, ErrShellCommandLine) || cmd.TimeoutMilliseconds != 1_000_000 {
		t.Fatal("duplicate -T moved the deadline", cmd.TimeoutMilliseconds, err)
	}
}

func TestShellCommandLineTerminalActionsAndKinds(t *testing.T) {
	c := NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), nil)
	if err := c.Parse([]string{"-dl", "-dimacs", "-wcnf", "-bpo", "-log", "-in", "-st", "-ist", "-pm:arith", "ignored"}); err != nil {
		t.Fatal(err)
	}
	if c.Kind != InputZ3Log || !c.StandardInput || !c.DisplayStatistics || !c.DisplayIStatistics || c.Action != CommandDisplayModule || c.ActionArgument != "arith" {
		t.Fatal("source option sequence", c)
	}
	if _, selected := c.Input.Selected(); selected {
		t.Fatal("source early exit scanned later filename")
	}
	for _, test := range []struct {
		arg    string
		action CommandAction
	}{
		{"-h", CommandHelp}, {"--help", CommandHelp}, {"-?", CommandHelp},
		{"-version", CommandVersion}, {"-p", CommandDisplayParams},
		{"-pd", CommandDisplayParamDescriptions}, {"-pm", CommandDisplayModules},
		{"-pp:arith.solver", CommandDisplayParameter},
	} {
		c = NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), nil)
		if err := c.Parse([]string{test.arg, "later"}); err != nil || c.Action != test.action {
			t.Fatal(test.arg, c.Action, err)
		}
		if _, selected := c.Input.Selected(); selected {
			t.Fatal("terminal option scanned later filename", test.arg)
		}
	}
}

func TestShellCommandLineSourceDecimalAndRefusals(t *testing.T) {
	previous, err := diagnostic.ProcessVerbosityLevel()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := diagnostic.SetProcessVerbosityLevel(previous); err != nil {
			t.Error(err)
		}
	})
	c := NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), nil)
	if err := c.Parse([]string{"-v:  -12tail"}); err != nil {
		t.Fatal(err)
	}
	if level, err := diagnostic.ProcessVerbosityLevel(); err != nil || level != ^uint32(11) {
		t.Fatal("source signed-long to unsigned conversion", level, err)
	}
	for _, test := range []struct {
		arg  string
		want error
	}{
		{"-v", ErrShellOptionArgument}, {"-T", ErrMissingGlobalTimeout},
		{"-t", ErrShellOptionArgument}, {"-memory", ErrShellOptionArgument},
		{"-pp", ErrShellOptionArgument}, {"-bogus", ErrShellCommandLine},
		{"name=value", ErrShellOptionService},
	} {
		c = NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), nil)
		if err := c.Parse([]string{test.arg}); !errors.Is(err, test.want) {
			t.Fatal(test.arg, err, test.want)
		}
	}
}

func TestShellCommandLineDebugAndTraceBuildGates(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	r, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: b}})
	if err != nil {
		t.Fatal(err)
	}
	d, err := diagnostic.NewDebugContext(r, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
		if err := r.Close(); err != nil {
			t.Error(err)
		}
		if b.Used() != 0 {
			t.Error("debug owner retained bytes", b.Used())
		}
	})
	tags := diagnostic.NewTraceTagsWithWriter(&bytes.Buffer{})
	c := NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), nil)
	c.Trace, c.Debug = tags, d
	traceError := c.Parse([]string{"-tr:arith\x00ignored"})
	if shellTraceOptions {
		if traceError != nil || !tags.IsEnabled("arith") || !tags.IsEnabled("arith\x00ignored-more") {
			t.Fatal("compiled source trace option", traceError)
		}
	} else if !errors.Is(traceError, ErrShellCommandLine) || tags.IsEnabled("arith") {
		t.Fatal("erased source trace option", traceError)
	}
	debugError := c.Parse([]string{"-dbg:solver\x00ignored"})
	if shellDebugOptions {
		enabled, e := d.IsDebugEnabled("solver")
		if debugError != nil || e != nil || !enabled {
			t.Fatal("compiled source debug option", debugError, e, enabled)
		}
	} else if !errors.Is(debugError, ErrShellCommandLine) {
		t.Fatal("erased source debug option", debugError)
	}
	for _, test := range []struct {
		arg     string
		enabled bool
	}{
		{"-tr", shellTraceOptions}, {"-dbg", shellDebugOptions},
	} {
		want := ErrShellCommandLine
		if test.enabled {
			want = ErrShellOptionArgument
		}
		if err := c.Parse([]string{test.arg}); !errors.Is(err, want) {
			t.Fatal("source gate before argument check", test.arg, err, want)
		}
	}
}

func TestShellCommandLineReachesGlobalParameterAndTimeoutOwners(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	pc, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := params.NewGlobalParamsCatalog(sy, params.GlobalParamsCatalogConfig{
		Budget: b, Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}},
		Text: arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}},
	})
	if err != nil {
		t.Fatal(err)
	}
	globals, err := params.NewGlobalParamsService(sy, pc, params.GlobalParamsConfig{
		Budget: b, Catalog: catalog, DescriptorPageSize: 2, DescriptorWordsPageSize: 16,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := globals.InitializeSource(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, e := range []error{globals.Close(), catalog.Close(), pc.Close(), q.Close(), sy.Close()} {
			if e != nil {
				t.Error(e)
			}
		}
		if b.Used() != 0 {
			t.Error("shell command owners retained bytes", b.Used())
		}
	})
	c := NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), globals)
	if err := c.Parse([]string{"-T:0", "-t:50", "-memory:64", "verbose=2", "input.smt2"}); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"timeout", "50"}, {"memory_max_size", "64"}, {"verbose", "2"}} {
		value, e := globals.SourceGetValueName(pair[0])
		if e != nil || value != pair[1] {
			t.Fatal("source global parameter", pair, value, e)
		}
	}
	if name, ok := c.Input.Selected(); !ok || name != "input.smt2" {
		t.Fatal(name, ok)
	}
	if err := c.WriteTerminalAction(nil); err != nil {
		t.Fatal("ordinary solver path must not need terminal output", err)
	}
	for _, test := range []struct {
		argument string
		want     string
	}{
		{"-p", "Global parameters\n"},
		{"-pd", "Global parameters\n"},
		{"-pm", "Use -pm:name to display all parameters"},
		{"-pm:sat", "[module] sat"},
		{"-pp:verbose", "  name:           verbose\n"},
	} {
		var output bytes.Buffer
		choice := NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), globals)
		if err := choice.ParseAndWrite([]string{test.argument, "not-scanned"}, &output); err != nil {
			t.Fatal(test.argument, err)
		}
		if !strings.Contains(output.String(), test.want) {
			t.Fatal("source terminal output", test.argument, output.String())
		}
		if test.argument == "-p" && strings.Contains(output.String(), "be verbose, where the value is the verbosity level") {
			t.Fatal("-p included descriptor prose")
		}
		if test.argument == "-pd" && !strings.Contains(output.String(), "be verbose, where the value is the verbosity level") {
			t.Fatal("-pd omitted descriptor prose")
		}
		if test.argument == "-pp:verbose" && !strings.HasPrefix(output.String(), "verbose null verbose\n") {
			t.Fatal("-pp stdout diagnostic preceded descriptor", output.String())
		}
		if _, selected := choice.Input.Selected(); selected {
			t.Fatal("terminal action scanned later filename", test.argument)
		}
	}
	choice := NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), globals)
	var missing bytes.Buffer
	if err := choice.ParseAndWrite([]string{"-pp:no_such_parameter"}, &missing); err == nil || !strings.HasPrefix(missing.String(), "no_such_parameter ") {
		t.Fatal("-pp source diagnostic before lookup failure", err, missing.String())
	}
	choice = NewCommandLine(diagnostic.NewWarningService(&bytes.Buffer{}), globals)
	if err := choice.ParseAndWrite([]string{"-p"}, refusedWarningWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("terminal borrowed stdout refusal", err)
	}
}
