package smt2

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2OutputChannelsRouteCommandsAndAppend(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(t.TempDir(), "regular.out")
	if err := os.WriteFile(path, []byte("existing\n"), 0600); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`(echo "before")
(set-option :regular-output-channel %q)
(echo "after")
(get-option :regular-output-channel)
(simplify (+ 1 2))`, path)
	var output bytes.Buffer
	if err := ParseCommands(c, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "before\n" {
		t.Fatal("original stream", output.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "existing\nafter\n" + path + "\n3\n"; string(content) != want {
		t.Fatalf("file = %q, want %q", content, want)
	}
	var next bytes.Buffer
	if err := ParseCommands(c, strings.NewReader(`(get-option :regular-output-channel)`), &next); err != nil {
		t.Fatal(err)
	}
	if next.String() != "caller-owned\n" {
		t.Fatal("caller stream name", next.String())
	}
}

func TestSMT2OutputChannelFailedOpenResetsStream(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(t.TempDir(), "missing", "out")
	var output bytes.Buffer
	err = ParseCommands(c, strings.NewReader(fmt.Sprintf(`(set-option :regular-output-channel %q)`, path)), &output)
	if err == nil || !strings.Contains(err.Error(), "failed to set output stream") {
		t.Fatal("open failure", err)
	}
	name, ok, err := c.GetOption("regular-output-channel")
	if err != nil || !ok || name != "stdout" {
		t.Fatal("default stream after failure", name, ok, err)
	}
}

func TestSMT2OutputChannelSwitchRoutesSuccess(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first"), filepath.Join(dir, "second")
	script := fmt.Sprintf(`(set-option :print-success true)
(set-option :regular-output-channel %q)
(echo "first")
(set-option :regular-output-channel %q)
(echo "second")`, first, second)
	var output bytes.Buffer
	if err := ParseCommands(c, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ path, want string }{
		{first, "success\nfirst\n"},
		{second, "success\nsecond\n"},
	} {
		data, err := os.ReadFile(check.path)
		if err != nil || string(data) != check.want {
			t.Fatalf("%s: %q, %v; want %q", check.path, data, err, check.want)
		}
	}
	if output.String() != "success\n" {
		t.Fatal("original stream", output.String())
	}
}
