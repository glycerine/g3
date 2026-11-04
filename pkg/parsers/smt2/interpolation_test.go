package smt2

// Source differential: src/cmd_context/interpolant_cmds.cpp.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/interp"
)

func interpolationReference(t *testing.T, script string) string {
	t.Helper()
	path := os.Getenv("G3_CPP_REFERENCE")
	if path == "" {
		var err error
		path, err = filepath.Abs("../../../build/z3")
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("C++ reference executable unavailable:", err)
	}
	command := exec.Command(path, "-in")
	command.Stdin = strings.NewReader(script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	return string(output)
}

func TestSMT2ComputeAndGetInterpolantUseNativeBackend(t *testing.T) {
	marker := "command-interpolation-" + t.TempDir()
	profiler := interp.ProcessProfiler()
	if err := profiler.TimerStart(marker); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := profiler.TimerStop(marker); err != nil {
			t.Error(err)
		}
	}()
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(declare-const q Bool)(compute-interpolant q (not q))(reset)
(set-option :produce-interpolants true)(declare-const p Bool)
(compute-interpolant p)
(compute-interpolant p (not p))
(assert p)(assert (not p))(check-sat)(get-interpolant p (not p))
(reset)(declare-const x Int)(compute-interpolant (< x 0) (> x 2))`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	want := "unsat\n(interpolants\n q)\nsat\nunsat\n(interpolants\n p)\nunsat\n(interpolants\n p)\nunsat\n(interpolants\n (not (>= x 0)))\n"
	if output.String() != want {
		t.Fatal("interpolation output", output.String(), "want", want)
	}
	profile, _ := graph.ProcessProfilers()
	var report bytes.Buffer
	if err := profile.Print(&report); err != nil {
		t.Fatal(err)
	}
	if !interpolationScopeHasSolving(report.String(), marker) {
		t.Fatal("Duality report lacks command interpolation scope", report.String())
	}
}

func interpolationScopeHasSolving(report, marker string) bool {
	inside := false
	for _, line := range strings.Split(report, "\n") {
		if !inside {
			inside = strings.HasPrefix(line, "  "+marker)
			continue
		}
		if !strings.HasPrefix(line, "    ") {
			return false
		}
		if strings.HasPrefix(line, "    solving...") {
			return true
		}
	}
	return false
}

func TestSMT2InterpolantCommandGuards(t *testing.T) {
	for name, script := range map[string]string{
		"empty-compute": `(compute-interpolant)`,
		"disabled-get":  `(get-interpolant true)`,
		"no-proof":      `(set-option :produce-interpolants true)(get-interpolant true)`,
	} {
		t.Run(name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			})
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err == nil {
				t.Fatal("interpolation guard accepted", script, output.String())
			}
		})
	}
}

func TestSMT2InterpolationGlobalCheckAndProfileOptions(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	globals, err := context.GlobalParameters()
	if err != nil {
		t.Fatal(err)
	}
	if err = globals.SourceSetName("interp.check", "true"); err != nil {
		t.Fatal(err)
	}
	if err = globals.SourceSetName("interp.profile", "true"); err != nil {
		t.Fatal(err)
	}
	script := `(set-option :produce-interpolants true)(declare-const p Bool)
(assert p)(assert (not p))(check-sat)(get-interpolant p (not p))`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if !strings.Contains(output.String(), "(interpolants\n p)\ncorrect\n") || !strings.Contains(output.String(), "TOTAL") {
		t.Fatal("global interpolation check/profile options were not applied", output.String())
	}
}

func TestSMT2InterpolationReplayScriptMatchesCPP(t *testing.T) {
	script := `(set-option :produce-interpolants true)
(declare-fun replay-p () Bool)
(assert (! replay-p :named f!1))
(assert (! (not replay-p) :named f!2))
(check-sat)
(get-interpolant (and (interp f!1) f!2))
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	cpp := interpolationReference(t, script)
	if output.String() != cpp || cpp != "unsat\n(interpolants\n replay-p)\n" {
		t.Fatal("interpolation replay differential", output.String(), cpp)
	}
}
