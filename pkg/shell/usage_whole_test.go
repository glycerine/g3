package shell

import (
	"bytes"
	"strings"
	"testing"
)

func TestShellUsageWholeSourceSectionOrder(t *testing.T) {
	var out bytes.Buffer
	if err := WriteUsage(&out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	previous := -1
	for _, marker := range []string{"Z3 [version 4.7.1", "Usage: z3", "Input format:", "Miscellaneous:", "Resources:", "Output:", "Parameter setting:"} {
		at := strings.Index(text, marker)
		if at <= previous {
			t.Fatalf("source section %q missing/out of order in %q", marker, text)
		}
		previous = at
	}
	for _, option := range []string{"-smt2", "-dl", "-dimacs", "-log", "-in", "-version", "-memory:Megabytes", "module_name.param_name=value"} {
		if !strings.Contains(text, option) {
			t.Fatalf("source option %q missing", option)
		}
	}
	if strings.Contains(text, "-tr:tag") != shellTraceOptions || strings.Contains(text, "-dbg:tag") != shellDebugOptions {
		t.Fatal("compiled trace/debug help gates", shellTraceOptions, shellDebugOptions)
	}
}
