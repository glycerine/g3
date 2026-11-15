package main

import (
	"bytes"
	"strings"
	"testing"
)

// replayLog runs a '/'-separated z3 log through the -log frontend.
func replayLog(t *testing.T, log string) (string, string) {
	t.Helper()
	var out, diagnostics bytes.Buffer
	text := strings.ReplaceAll(log, "/", "\n") + "\n"
	if err := runArgumentsWithDiagnostics([]string{"-log", "-in"}, strings.NewReader(text), &out, &diagnostics); err != nil {
		t.Fatal(err, out.String(), diagnostics.String())
	}
	return out.String(), diagnostics.String()
}

// AUDIT.md api_03-F2: check_arg failures inside a command are
// z3_replayer_exceptions: printed as [z3 exception] and replay continues;
// outside a command ('*') they are fatal with the bare message.
func TestZ3LogFrontendCommandArgumentException(t *testing.T) {
	out, diagnostics := replayLog(t, `R/P 0/C 6/= 10/R/I 5/C 33/M "after"/R/P 10/C 8`)
	if !strings.HasPrefix(out, "[z3 exception]: expecting object at position 0 but got int64\nafter\nmax. heap size:") || diagnostics != "" {
		t.Fatalf("command argument exception: %q %q", out, diagnostics)
	}
	out, diagnostics = replayLog(t, `R/P 0/C 6/= 10/R/P 10/I 3/* 21 1`)
	if diagnostics != "Error at line 8: expecting object at position 1 but got int64\n" || !strings.HasPrefix(out, "max. heap size:") {
		t.Fatalf("save-out argument error: %q %q", out, diagnostics)
	}
}

// AUDIT.md api_03-F4: Z3_inc_ref(c, NULL) is a no-op (ast_manager::inc_ref
// ignores null), so replay continues.
func TestZ3LogFrontendIncRefNull(t *testing.T) {
	out, diagnostics := replayLog(t, `R/P 0/C 7/= 10/R/P 10/P 0/C 9/M "after"/R/P 10/C 8`)
	if !strings.HasPrefix(out, "after\nmax. heap size:") || diagnostics != "" {
		t.Fatalf("inc_ref null: %q %q", out, diagnostics)
	}
}

// AUDIT.md api_03-F5: solver_from_stream and Z3_fixedpoint_from_stream keep
// the temporary cmd_context's regular stream, so the parser prints
// (error ...) before SET_ERROR_CODE(Z3_PARSER_ERROR) fires the handler.
// (The parser's message text is a separate divergence.)
func TestZ3LogFrontendFromStringParseError(t *testing.T) {
	for _, tc := range []struct{ name, log, prefix string }{
		{"solver", `R/P 0/C 6/= 10/R/P 10/C 408/= 11/R/P 10/P 11/S "(assert (foo"/C 426/M "after"/R/P 10/C 8`, `(error "line 1 column 10: `},
		{"fixedpoint", `R/P 0/C 6/= 10/R/P 10/C 511/= 11/R/P 10/P 11/S "(declare-rel q ())(rule q)(foo"/C 534/= 12/M "after"/R/P 10/C 8`, `(error "line 1 column 28: `},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, diagnostics := replayLog(t, tc.log)
			if !strings.HasPrefix(out, tc.prefix) || !strings.Contains(out, "\")\n[REPLAYER ERROR HANDLER]: parser error\nafter\nmax. heap size:") || diagnostics != "" {
				t.Fatalf("from_string parse error: %q %q", out, diagnostics)
			}
		})
	}
}
