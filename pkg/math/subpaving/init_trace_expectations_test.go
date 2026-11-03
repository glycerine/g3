package subpaving

import (
	"strings"
	"testing"
)

func requireRootInitPrinterPrefix(t *testing.T, output *polynomialTraceFlushingBuffer, calls, stopAt int) {
	t.Helper()
	want := "-------- [subpaving_init] init src/math/subpaving/subpaving_t_def.h:1797 ---------\n" +
		"7 <= candidate"
	if stopAt == 2 {
		want += "\n" +
			"------------------------------------------------\n" +
			"-------- [subpaving_init] assert_units src/math/subpaving/subpaving_t_def.h:1780 ---------\n" +
			"asserting: 7 <= candidate"
	}
	if output.String() != want || output.flushes != stopAt-1 || calls != stopAt {
		t.Fatalf("printer interruption prefix: got %q, flushes %d, calls %d; want %q, %d flushes, %d calls", output.String(), output.flushes, calls, want, stopAt-1, stopAt)
	}
}

func requireExplicitRootUnitTrace(t *testing.T, output *polynomialTraceFlushingBuffer, before string, stopAt int) {
	t.Helper()
	const unit = "-------- [subpaving_init] assert_units src/math/subpaving/subpaving_t_def.h:1780 ---------\n" +
		"asserting: 7 <= x0, axiom: 1\n" +
		"------------------------------------------------\n"
	if !strings.HasPrefix(output.String(), before+unit) || output.flushes != stopAt+1 ||
		strings.Count(output.String(), "subpaving_t_def.h:1797") != 1 ||
		strings.Count(output.String(), "subpaving_t_def.h:1780") != stopAt ||
		strings.Count(output.String(), "subpaving_t_def.h:1785") != 1 ||
		!strings.Contains(output.String()[len(before):], "bounds after init\n7 <= x0") ||
		strings.Contains(output.String(), "subpaving_t_def.h:1801") {
		t.Fatalf("explicit unit assertion trace: got %q, flushes %d", output.String(), output.flushes)
	}
}
