package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSMTLibShellOutputChannelIdentityAndReturnToStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	script := fmt.Sprintf(`(get-option :regular-output-channel)
(get-option :diagnostic-output-channel)
(set-option :regular-output-channel %q)
(echo "file")
(set-option :regular-output-channel "stdout")
(get-option :regular-output-channel)
(echo "back")`, path)
	var output bytes.Buffer
	if err := run(strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if want := "stdout\nstderr\nstdout\nback\n"; output.String() != want {
		t.Fatalf("stdout = %q, want %q", output.String(), want)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "file\n" {
		t.Fatal("redirected output", string(data), err)
	}
}
