package api

import (
	"bytes"
	"testing"
)

type flushingLogBuffer struct {
	bytes.Buffer
	flushes int
}

func (b *flushingLogBuffer) Flush() error {
	b.flushes++
	return nil
}

func TestZ3LogScalarAndCommandRecords(t *testing.T) {
	var output flushingLogBuffer
	log, err := NewZ3LogWriter(&output)
	if err != nil {
		t.Fatal(err)
	}
	operations := []func() error{
		log.ResetArguments,
		func() error { return log.Pointer(0) },
		func() error { return log.Pointer(0x1a2b) },
		func() error { return log.Int64(-42) },
		func() error { return log.Uint64(1 << 40) },
		func() error { return log.Double(1.23456789) },
		func() error { return log.String("a:\n") },
		func() error { return log.Call(19) },
		func() error { return log.Result(0xbeef) },
		func() error { return log.Out(0xcafe, 3) },
		func() error { return log.ArrayOut(0xface, 4, 2) },
		func() error { return log.Message("done|") },
	}
	for i, operation := range operations {
		if err := operation(); err != nil {
			t.Fatalf("operation %d: %v", i, err)
		}
	}
	want := "R\nP 0\nP 0x1a2b\nI -42\nU 1099511627776\nD 1.23457\n" +
		"S \"a\\058\\010\"\nC 19\n= 0xbeef\n* 0xcafe 3\n@ 0xface 4 2\nM \"done\\124\"\n"
	if output.String() != want {
		t.Fatalf("records mismatch\n got: %q\nwant: %q", output.String(), want)
	}
	if output.flushes != len(operations) {
		t.Fatalf("got %d flushes, want %d", output.flushes, len(operations))
	}
}
