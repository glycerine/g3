package api

import (
	"bytes"
	"strings"
	"testing"
)

func TestZ3ReplayerMessagesUseInjectedOutput(t *testing.T) {
	var output bytes.Buffer
	r, _ := NewZ3Replayer(strings.NewReader("M \"hello\\010world\"\n"), &output)
	defer r.Close()
	if err := r.Parse(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "hello\nworld\n" {
		t.Fatalf("message %q", output.String())
	}
}

func TestZ3ReplayerReportsEscapesAndUnknownCommands(t *testing.T) {
	for _, log := range []string{"S \"bad\\999\"", "?"} {
		r, _ := NewZ3Replayer(strings.NewReader(log), new(bytes.Buffer))
		err := r.Parse()
		_ = r.Close()
		if err == nil {
			t.Fatalf("accepted %q", log)
		}
	}
}
