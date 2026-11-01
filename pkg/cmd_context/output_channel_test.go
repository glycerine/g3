package cmd_context

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestDiagnosticOutputChannelEchoAndReplacement(t *testing.T) {
	c := contextFixture(t)
	path := filepath.Join(t.TempDir(), "diagnostic.out")
	contextMust(t, c.SetStringOption("diagnostic-output-channel", path))
	name, ok, err := c.GetOption("diagnostic-output-channel")
	contextMust(t, err)
	if !ok || name != path {
		t.Fatal("diagnostic channel name", name, ok)
	}
	goal := echoGoal(t, c)
	probe, err := tactic.NewProbe(goal.Manager().Budget(), tactic.ProbeConstant, 2)
	contextMust(t, err)
	message := "probe"
	selected, err := NewProbeValueTactic(c, &message, probe, true)
	contextMust(t, err)
	applyEcho(t, selected, goal)
	content, err := os.ReadFile(path)
	contextMust(t, err)
	if string(content) != "probe 2\n" {
		t.Fatal("diagnostic file", string(content))
	}
	contextMust(t, c.SetStringOption("diagnostic-output-channel", "stderr"))
	name, ok, err = c.GetOption("diagnostic-output-channel")
	contextMust(t, err)
	if !ok || name != "stderr" {
		t.Fatal("replacement channel", name, ok)
	}
}

func TestShellStdoutDiagnosticChannelUsesBorrowedWriter(t *testing.T) {
	c := contextFixture(t)
	var output bytes.Buffer
	buffered := bufio.NewWriter(&output)
	contextMust(t, c.SetShellRegularOutput(buffered))
	contextMust(t, c.SetStringOption("diagnostic-output-channel", "stdout"))
	contextMust(t, diagnostic.BorrowProcessWarningService().Warning("warning to stdout"))
	if output.String() != "WARNING: warning to stdout\n" {
		t.Fatal("warning was not flushed", output.String())
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	contextMust(t, stream.AppendString("verbose to stdout\n"))
	contextMust(t, stream.Flush())
	if output.String() != "WARNING: warning to stdout\nverbose to stdout\n" {
		t.Fatal("shell diagnostic stdout", output.String())
	}
}

func TestDiagnosticChannelFormatterAllocationFailureRestoresProcessStreams(t *testing.T) {
	c := contextFixture(t)
	warnings := diagnostic.BorrowProcessWarningService()
	beforeWarning := warnings.WarningStream()
	beforeVerbose, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	dir := t.TempDir()
	contextMust(t, c.SetStringOption("diagnostic-output-channel", filepath.Join(dir, "first")))
	c.config.Params.BudgetField.FailAfter(0)
	err = c.SetStringOption("diagnostic-output-channel", filepath.Join(dir, "second"))
	c.config.Params.BudgetField.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("formatter allocation failure", err)
	}
	afterVerbose, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	if afterVerbose != beforeVerbose || warnings.WarningStream() != beforeWarning {
		t.Fatal("failed replacement left process streams selected")
	}
	contextMust(t, c.Close())
}

func TestShellDiagnosticOutputAllocationFailureKeepsProcessStreams(t *testing.T) {
	c := contextFixture(t)
	warnings := diagnostic.BorrowProcessWarningService()
	beforeWarning := warnings.WarningStream()
	beforeVerbose, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	var output bytes.Buffer
	c.config.Params.BudgetField.FailAfter(0)
	err = c.SetShellDiagnosticOutput(&output)
	c.config.Params.BudgetField.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("formatter allocation failure", err)
	}
	afterVerbose, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	if afterVerbose != beforeVerbose || warnings.WarningStream() != beforeWarning {
		t.Fatal("failed setup changed process streams")
	}
	contextMust(t, c.Close())
}

func TestDiagnosticChannelRoutesProcessStreamsAndRestores(t *testing.T) {
	c := contextFixture(t)
	warnings := diagnostic.BorrowProcessWarningService()
	beforeWarning := warnings.WarningStream()
	beforeVerbose, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	path := filepath.Join(t.TempDir(), "process.out")
	contextMust(t, c.SetStringOption("diagnostic-output-channel", path))
	contextMust(t, warnings.Warning("actual warning"))
	selected, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	contextMust(t, selected.AppendString("verbose message\n"))
	content, err := os.ReadFile(path)
	contextMust(t, err)
	if string(content) != "WARNING: actual warning\nverbose message\n" {
		t.Fatal("process output", string(content))
	}
	contextMust(t, c.Close())
	afterVerbose, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	if afterVerbose != beforeVerbose || warnings.WarningStream() != beforeWarning {
		t.Fatal("process streams were not restored")
	}
}

func TestAuxiliaryDiagnosticChannelDoesNotRedirectProcessStreams(t *testing.T) {
	c, err := NewAuxiliaryContext()
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, c.Close()) })
	warnings := diagnostic.BorrowProcessWarningService()
	beforeWarning := warnings.WarningStream()
	beforeVerbose, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	contextMust(t, c.SetStringOption("diagnostic-output-channel", filepath.Join(t.TempDir(), "aux.out")))
	afterVerbose, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	if afterVerbose != beforeVerbose || warnings.WarningStream() != beforeWarning {
		t.Fatal("auxiliary context changed process streams")
	}
}

func TestDiagnosticChannelNestedCloseDoesNotRestoreClosedStream(t *testing.T) {
	outer := contextFixture(t)
	inner := contextFixture(t)
	dir := t.TempDir()
	contextMust(t, outer.SetStringOption("diagnostic-output-channel", filepath.Join(dir, "outer")))
	contextMust(t, inner.SetStringOption("diagnostic-output-channel", filepath.Join(dir, "inner")))
	contextMust(t, outer.Close())
	contextMust(t, inner.Close())
	stream, err := diagnostic.BorrowProcessVerboseStream()
	contextMust(t, err)
	if err := stream.CheckOpen(); err != nil {
		t.Fatal("closed earlier context selected", err)
	}
	if _, ok := diagnostic.BorrowProcessWarningService().WarningStream().(contextWarningOutput); ok {
		t.Fatal("closed earlier context selected for warnings")
	}
}
