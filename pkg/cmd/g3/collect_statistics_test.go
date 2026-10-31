package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerCollectStatistics(t *testing.T) {
	script := "(declare-const p Bool)\n(assert p)\n(apply collect-statistics)\n"
	var output bytes.Buffer
	if err := run(strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if !strings.Contains(output.String(), ":uninterpreted-constants    1") || !strings.Contains(output.String(), "(goals") {
		t.Fatal(output.String())
	}
}
