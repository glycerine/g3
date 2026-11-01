package shell_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSMTLIBFrontendWritesFinalStatisticsWithTotalDuration(t *testing.T) {
	frontend, _, output := newBorrowedFrontend(t, "(assert false)(check-sat)", true, steppedClock(1_000_000_000, 3_500_000_000))
	if err := frontend.Run(); err != nil {
		t.Fatal(err, output.String())
	}
	got := output.String()
	if !strings.HasPrefix(got, "unsat\n") || !strings.Contains(got, ":total-time   2.50") || !strings.Contains(got, ":rlimit-count") {
		t.Fatalf("statistics output %q", got)
	}
	result, err := frontend.Result()
	if err != nil || result.DurationSeconds != 2.5 || result.StatisticsError != nil {
		t.Fatalf("result %#v, error %v", result, err)
	}
}

func TestSMTLIBFrontendStatisticsRestoreStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output")
	script := fmt.Sprintf(`(set-option :regular-output-channel %q)(echo "file")`, path)
	frontend, context, output := newBorrowedFrontend(t, script, true, steppedClock(0, 1_000_000_000))
	if err := frontend.Run(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "file\n" {
		t.Fatal("redirected output", string(data), err)
	}
	if !strings.Contains(output.String(), ":total-time") {
		t.Fatal("statistics not written to stdout", output.String())
	}
	name, ok, err := context.GetOption("regular-output-channel")
	if err != nil || !ok || name != "stdout" {
		t.Fatal("statistics did not restore stdout", name, ok, err)
	}
}

func TestSMTLIBFrontendExplicitStatisticsUsesElapsedDuration(t *testing.T) {
	frontend, _, _ := newBorrowedFrontend(t, "(check-sat)", false, steppedClock(0, 750_000_000))
	if err := frontend.Run(); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := frontend.WriteStatistics(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), ":total-time   0.75") {
		t.Fatalf("statistics %q", output.String())
	}
}
