package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptimizationFrontendArguments(t *testing.T) {
	for _, tc := range []struct{ name, option, input, value string }{
		{"opb", "-bpo", "min: 1 x1 +2 x2;\n1 x1 +1 x2 >= 1;\n", " 1)"},
		{"wcnf", "-wcnf", "p wcnf 1 3 10\n10 1 0\n2 -1 0\n3 1 0\n", " 2)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := runArguments([]string{tc.option, "-in"}, strings.NewReader(tc.input), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if text := output.String(); !strings.HasPrefix(text, "sat\n(objectives\n") || !strings.Contains(text, tc.value) {
				t.Fatalf("optimization output: %q", text)
			}
		})
	}
}

func TestOptimizationFrontendFileTypeDetection(t *testing.T) {
	for _, tc := range []struct{ filename, input string }{
		{"problem.opb", "1 x1 >= 1;\n"},
		{"problem.wcnf", "p wcnf 1 1 5\n5 1 0\n"},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.filename)
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := runArguments([]string{path}, strings.NewReader(""), &output); err != nil {
				t.Fatal(err)
			}
			if output.String() != "sat\n" {
				t.Fatalf("unexpected output: %q", output.String())
			}
		})
	}
}
