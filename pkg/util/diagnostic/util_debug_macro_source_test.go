package diagnostic

// Source: src/util/debug.h:63-92; src/util/heap.h:57-66,79-239,258-265.
// Source: src/util/vector.h:303-344,389-403.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func debugActionFixture(t *testing.T, text string) (*umemory.Runtime, *DebugContext, *DebugActionContext, *NumericCharInput, *MemoryStandardStreams) {
	t.Helper()
	b := arena.NewBudget(1 << 24)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b})
	paramsMust(t, e)
	region, e := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: b}})
	paramsMust(t, e)
	d, e := NewDebugContext(region, b)
	paramsMust(t, e)
	inputFile, e := os.CreateTemp(t.TempDir(), "debug-action-input-*")
	paramsMust(t, e)
	_, e = inputFile.WriteString(text)
	paramsMust(t, e)
	_, e = inputFile.Seek(0, 0)
	paramsMust(t, e)
	out, e := os.CreateTemp(t.TempDir(), "debug-action-output-*")
	paramsMust(t, e)
	streams, e := NewMemoryStandardStreams(MemoryStandardStreamsConfig{Budget: b, Stdout: out, Stderr: out})
	paramsMust(t, e)
	input, e := NewDebuggerCharInput(streams, inputFile)
	paramsMust(t, e)
	action, e := NewDebugActionContext(DebugActionConfig{Budget: b, Streams: streams, Input: input})
	paramsMust(t, e)
	paramsMust(t, d.BindAssertionStreams(streams))
	paramsMust(t, d.BindAssertionActions(action))
	t.Cleanup(func() {
		b.FailAfter(-1)
		paramsMust(t, action.Close())
		paramsMust(t, input.Close())
		paramsMust(t, streams.Close())
		paramsMust(t, inputFile.Close())
		paramsMust(t, out.Close())
		paramsMust(t, d.Close())
		paramsMust(t, region.Close())
		paramsMust(t, r.Close())
		if b.Used() != 0 {
			t.Fatal("unnamed debug fixture owner", b.Used())
		}
	})
	return r, d, action, input, streams
}

func TestDebugMacroUnreachableIgnoresAssertionSwitch(t *testing.T) {
	_, d, _, in, s := debugActionFixture(t, "T")
	paramsMust(t, d.EnableAssertions(false))
	e := d.UnreachableSource("macro_fixture.cpp", 41)
	if sourceDebug {
		var ex z3exception.DefaultException
		if !errors.As(e, &ex) {
			t.Fatal(e)
		}
		state, e := in.State()
		paramsMust(t, e)
		if state.Position != 1 {
			t.Fatal(state)
		}
		data, e := os.ReadFile(s.StderrField.File.Name())
		paramsMust(t, e)
		want := "ASSERTION VIOLATION\nFile: macro_fixture.cpp\nLine: 41\nUNREACHABLE CODE WAS REACHED.\n" + DebugSourcePrompt
		if string(data) != want {
			t.Fatal(string(data))
		}
	} else if !errors.Is(e, ErrDebugNativeUnreachable) {
		t.Fatal("GNU source undefined-domain guard", e)
	}
}
func TestDebugMacroVerifyFalsePrefixAndContinue(t *testing.T) {
	_, d, _, in, s := debugActionFixture(t, "C")
	paramsMust(t, d.EnableAssertions(false))
	e := d.VerifyFailure("macro_fixture.cpp", 52, "value == expected")
	data, re := os.ReadFile(s.StderrField.File.Name())
	paramsMust(t, re)
	want := "Failed to verify: value == expected\n"
	if sourceDebug {
		paramsMust(t, e)
		want += "ASSERTION VIOLATION\nFile: macro_fixture.cpp\nLine: 52\nUNREACHABLE CODE WAS REACHED.\n" + DebugSourcePrompt
		state, e := in.State()
		paramsMust(t, e)
		if state.Position != 1 {
			t.Fatal(state)
		}
	} else if !errors.Is(e, ErrDebugNativeUnreachable) {
		t.Fatal(e)
	}
	if string(data) != want {
		t.Fatal(string(data), want)
	}
}

func TestDebugAssertionFailureUsesBoundInteractiveAction(t *testing.T) {
	_, d, _, in, s := debugActionFixture(t, "C")
	e := DebugAssertionFailure(d, "src/tactic/arith/bv2real_rewriter.cpp", 51, "sig.m_d.is_pos()")
	if e != nil {
		t.Fatal(e)
	}
	data, err := os.ReadFile(s.StderrField.File.Name())
	paramsMust(t, err)
	want := "ASSERTION VIOLATION\nFile: src/tactic/arith/bv2real_rewriter.cpp\nLine: 51\nsig.m_d.is_pos()\n" + DebugSourcePrompt
	if string(data) != want {
		t.Fatalf("assertion action output mismatch:\n got %q\nwant %q", data, want)
	}
	state, err := in.State()
	paramsMust(t, err)
	if state.Position != 1 {
		t.Fatalf("continue action consumed %d bytes", state.Position)
	}
}

func TestDebugMacroXAssertionLiteralExtraBeforeAction(t *testing.T) {
	_, d, _, in, s := debugActionFixture(t, "T")
	var reached uint32
	// Exact definition-only expansion: gate, then condition, notify, actual EXTRA,
	// debugger. No upstream XASSERT invocation exists and no callback is used.
	on, e := SourceAssertionsEnabled(d)
	paramsMust(t, e)
	if on {
		reached++
		condition := false
		if !condition {
			paramsMust(t, d.NotifyAssertionViolation("macro_fixture.cpp", 65, "condition"))
			out, e := s.Stderr()
			paramsMust(t, e)
			paramsMust(t, out.AppendCString("EXTRA\n"))
			// EXTRA can change the switch; invoke_gdb itself does not read it.
			paramsMust(t, d.EnableAssertions(false))
			e = d.InvokeAssertionDebugger()
			var ex z3exception.DefaultException
			if !errors.As(e, &ex) {
				t.Fatal(e)
			}
		}
	}
	data, e := os.ReadFile(s.StderrField.File.Name())
	paramsMust(t, e)
	state, e := in.State()
	paramsMust(t, e)
	if sourceDebug {
		want := "ASSERTION VIOLATION\nFile: macro_fixture.cpp\nLine: 65\ncondition\nEXTRA\n" + DebugSourcePrompt
		if reached != 1 || state.Position != 1 || string(data) != want {
			t.Fatal(reached, state, string(data))
		}
	} else if reached != 0 || state.Position != 0 || len(data) != 0 {
		t.Fatal("release erased condition/EXTRA/actions", reached, state, string(data))
	}
}
func TestDebugMacroExitSubprocessHelper(t *testing.T) {
	route := os.Getenv("G3_DEBUG_MACRO_CHILD")
	if route == "" {
		return
	}
	b := arena.NewBudget(1 << 24)
	region, e := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: b}})
	paramsMust(t, e)
	d, e := NewDebugContext(region, b)
	paramsMust(t, e)
	cfg := MemoryStandardStreamsConfig{Budget: b}
	if route == "ensure-bad-file" {
		cfg.Stderr = os.NewFile(3, "borrowed-parent-descriptor")
	}
	s, e := NewMemoryStandardStreams(cfg)
	paramsMust(t, e)
	input, e := NewDebuggerCharInput(s, os.Stdin)
	paramsMust(t, e)
	a, e := NewDebugActionContext(DebugActionConfig{Budget: b, Streams: s, Input: input})
	paramsMust(t, e)
	paramsMust(t, d.BindAssertionStreams(s))
	paramsMust(t, d.BindAssertionActions(a))
	b.FailAfter(0)
	if route == "ensure-bad-file" {
		paramsMust(t, cfg.Stderr.Close())
	}
	e = nil
	if route == "ensure" || route == "ensure-bad-file" {
		e = d.EnsureFailure("condition")
	} else {
		e = d.NotImplementedYet("macro_fixture.cpp", 80)
	}
	if errors.Is(e, ErrDebugNativeUnreachable) {
		out, _ := s.Stdout()
		_ = out.AppendCString("undefined-release-guard\n")
		paramsMust(t, a.Close())
		paramsMust(t, input.Close())
		paramsMust(t, s.Close())
		paramsMust(t, d.Close())
		paramsMust(t, region.Close())
		if b.Used() != 0 {
			os.Exit(97)
		}
		os.Exit(0)
	}
	os.Exit(96)
}
func TestDebugMacroActualEnsureAndNotImplementedExits(t *testing.T) {
	for _, route := range []string{"ensure", "ensure-bad-file", "not-implemented"} {
		t.Run(route, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDebugMacroExitSubprocessHelper$")
			cmd.Env = append(os.Environ(), "G3_DEBUG_MACRO_CHILD="+route)
			f, e := os.CreateTemp(".", "debug-macro-exit-input-*")
			paramsMust(t, e)
			defer func() { f.Close(); os.Remove(f.Name()) }()
			_, e = f.WriteString("C")
			paramsMust(t, e)
			_, e = f.Seek(0, 0)
			paramsMust(t, e)
			cmd.Stdin = f
			if route == "ensure-bad-file" {
				cmd.ExtraFiles = []*os.File{f}
			}
			var out, errout strings.Builder
			cmd.Stdout = &out
			cmd.Stderr = &errout
			e = cmd.Run()
			if ctx.Err() != nil {
				t.Fatal("bounded exit child", ctx.Err())
			}
			code := 0
			if e != nil {
				var exit *exec.ExitError
				if !errors.As(e, &exit) {
					t.Fatal(e)
				}
				code = exit.ExitCode()
			}
			wantOut := ""
			wantErr := "Failed to verify: condition\n"
			wantCode := 255
			if route == "ensure-bad-file" {
				wantErr = ""
			}
			if route == "not-implemented" {
				wantErr = "NOT IMPLEMENTED YET!\n"
				if sourceDebug {
					wantCode = 107
					wantErr += "ASSERTION VIOLATION\nFile: macro_fixture.cpp\nLine: 80\nUNREACHABLE CODE WAS REACHED.\n" + DebugSourcePrompt
				} else {
					wantCode = 0
					wantOut = "undefined-release-guard\n"
				}
			}
			if code != wantCode || out.String() != wantOut || errout.String() != wantErr {
				t.Fatal(code, out.String(), errout.String(), wantCode, wantOut, wantErr)
			}
		})
	}
}

func TestDebugMacroCassertLiteralTagShortCircuitAndUninitializedCondition(t *testing.T) {
	r, d, _, _, _ := debugActionFixture(t, "C")
	word, e := r.Allocate(1)
	paramsMust(t, e)
	t.Cleanup(func() { paramsMust(t, r.Deallocate(&word)) })
	paramsMust(t, d.EnableAssertions(false))
	on := false
	// Release erases this whole gate; assertion-off erases tag and payload reads.
	if sourceDebug {
		on, e = d.ConditionalAssertionEnabled("heap")
		paramsMust(t, e)
	}
	if on {
		_, e = r.Read(word, 0)
		t.Fatal("erased uninitialized condition was read", e)
	}
	paramsMust(t, d.EnableAssertions(true))
	if sourceDebug {
		on, e = d.ConditionalAssertionEnabled("heap")
		paramsMust(t, e)
	}
	if on {
		t.Fatal("disabled source tag read condition")
	}
	paramsMust(t, d.EnableDebug(debugTag(t, d.region, "heap")))
	if sourceDebug {
		on, e = d.ConditionalAssertionEnabled("heap")
		paramsMust(t, e)
	}
	if on {
		_, e = r.Read(word, 0)
		if !errors.Is(e, umemory.ErrRuntimeUninitialized) {
			t.Fatal("actual reached condition word guard", e)
		}
	}
}

func TestDebugMacroVerifyConditionIsNotErasedByAssertionSwitch(t *testing.T) {
	r, d, _, _, _ := debugActionFixture(t, "C")
	paramsMust(t, d.EnableAssertions(false))
	word, e := r.Allocate(1)
	paramsMust(t, e)
	t.Cleanup(func() {
		if word != (umemory.Block{}) {
			paramsMust(t, r.Deallocate(&word))
		}
	})
	paramsMust(t, r.Write(word, 0, 7))
	// VERIFY evaluates this reached operand in release and with the assertion
	// switch false. A true condition performs no notification or input action.
	value, e := r.Read(word, 0)
	paramsMust(t, e)
	if !(value == 7) {
		paramsMust(t, d.VerifyFailure("macro_fixture.cpp", 121, "value == 7"))
	}
	paramsMust(t, r.Deallocate(&word))
	// This actual guarded read proves the same operand is still reached; VERIFY
	// must not erase it behind the source assertions switch in either profile.
	_, e = r.Read(word, 0)
	if e == nil {
		t.Fatal("destroyed operand unexpectedly readable")
	}
}
