package shell

import (
	"bytes"
	"strconv"
	"testing"
)

func TestShellVersionSourceTupleAndHostWidth(t *testing.T) {
	var out bytes.Buffer
	if err := WriteVersion(&out); err != nil {
		t.Fatal(err)
	}
	want := "Z3 version 4.7.1 - " + strconv.Itoa(strconv.IntSize) + " bit\n"
	if out.String() != want {
		t.Fatalf("version %q want %q", out.String(), want)
	}
}
