package shell

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestShellResolveSourceInputExtension(t *testing.T) {
	for _, tc := range []struct {
		filename string
		kind     InputKind
	}{
		{"query.smt2", InputSMT2},
		{"query.a.cnf", InputDIMACS},
		{"query.dl", InputDatalog},
		{"query.wcnf", InputWCNF},
		{"query.opb", InputOPB},
		{"query.log", InputZ3Log},
		{"query.MPS", InputMPS},
		{"query.SIF", InputMPS},
		{"query.CNF", InputSMT2},
		{"query.", InputSMT2},
	} {
		c := NewCommandLine(nil, nil)
		if err := c.Input.AcceptPositional(tc.filename); err != nil {
			t.Fatal(err)
		}
		kind, err := c.ResolveInputKind()
		if err != nil || kind != tc.kind || c.Kind != tc.kind {
			t.Fatalf("%q: kind %v, err %v", tc.filename, kind, err)
		}
	}
	c := NewCommandLine(nil, nil)
	c.Kind = InputDatalog
	if err := c.Input.AcceptPositional("query.smt2"); err != nil {
		t.Fatal(err)
	}
	if kind, err := c.ResolveInputKind(); err != nil || kind != InputDatalog {
		t.Fatal("explicit input kind replaced by extension", kind, err)
	}
}

func TestShellMemoryDispatchExitChild(t *testing.T) {
	filename := os.Getenv("G3_SHELL_MEMORY_CHILD")
	if filename == "" {
		return
	}
	b := arena.NewBudget(1 << 20)
	streams, err := diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := diagnostic.NewMemoryFatalContext(diagnostic.MemoryFatalContextConfig{Budget: b, Streams: streams})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: b, FatalPolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	c := NewCommandLine(nil, nil)
	if err := c.BeginMemoryPolicy(policy); err != nil {
		t.Fatal(err)
	}
	if err := c.Input.AcceptPositional(filename); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ConfigureMemoryPolicyForDispatch(policy); err != nil {
		t.Fatal(err)
	}
	b.FailAfter(0)
	_, err = runtime.Allocate(8)
	t.Fatal("source fatal memory route returned", err)
}

func TestShellMemoryDispatchSourceExitText(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ filename, stderr string }{
		{"query.smt2", "(error \"out of memory\")\n"},
		{"query.cnf", "ERROR: out of memory\n"},
	} {
		cmd := exec.Command(exe, "-test.run=^TestShellMemoryDispatchExitChild$")
		cmd.Env = append(os.Environ(), "G3_SHELL_MEMORY_CHILD="+tc.filename)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 101 || stdout.Len() != 0 || stderr.String() != tc.stderr {
			t.Fatalf("%s: exit %v, stdout %q, stderr %q", tc.filename, err, stdout.String(), stderr.String())
		}
	}
}
