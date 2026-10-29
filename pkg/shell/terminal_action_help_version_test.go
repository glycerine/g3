package shell

import (
	"bytes"
	"strings"
	"testing"
)

func TestShellParseAndWriteHelpVersionTerminalActions(t *testing.T) {
	for _, tc := range []struct{ arg, prefix string }{{"-h", "Z3 [version"}, {"--help", "Z3 [version"}, {"-?", "Z3 [version"}, {"-version", "Z3 version"}} {
		var out bytes.Buffer
		command := NewCommandLine(nil, nil, nil)
		if err := command.ParseAndWrite([]string{tc.arg, "ignored.smt2"}, &out); err != nil {
			t.Fatal(tc.arg, err)
		}
		if !strings.HasPrefix(out.String(), tc.prefix) {
			t.Fatalf("%s output %q", tc.arg, out.String())
		}
		if _, selected := command.Input.Selected(); selected {
			t.Fatalf("terminal action scanned later input for %s", tc.arg)
		}
	}
}
