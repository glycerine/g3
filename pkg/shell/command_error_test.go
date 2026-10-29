package shell

import (
	"bytes"
	"testing"
)

func TestShellCommandErrorExactSourceLines(t *testing.T) {
	var out bytes.Buffer
	if err := WriteCommandError(&out, "option argument (-v:level) is missing."); err != nil {
		t.Fatal(err)
	}
	want := "Error: option argument (-v:level) is missing.\nFor usage information: z3 -h\n"
	if out.String() != want {
		t.Fatalf("command error %q", out.String())
	}
}
