package shell

import (
	"bytes"
	"strings"
	"testing"
)

func TestOPBFrontendLifecycle(t *testing.T) {
	var output bytes.Buffer
	f, err := NewOPBFrontend(opbFrontendContext(t), &output, OPBFrontendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Status(); err != ErrOPBFrontend {
		t.Fatal("premature status", err)
	}
	if _, err = f.Elapsed(); err != ErrOPBFrontend {
		t.Fatal("premature elapsed", err)
	}
	input := "1 x1 >= 0;"
	if err = f.Run(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if err = f.Run(strings.NewReader(input)); err != ErrOPBFrontend {
		t.Fatal("rerun", err)
	}
	if err = f.WriteResult(); err != nil || output.String() != "sat\nsat\n" {
		t.Fatal(output.String(), err)
	}
}
