package api

import (
	"bytes"
	"reflect"
	"testing"
)

func TestZ3LogWriterRoundTripsThroughReplayer(t *testing.T) {
	var encoded bytes.Buffer
	log, _ := NewZ3LogWriter(&encoded)
	for _, operation := range []func() error{
		log.ResetArguments,
		func() error { return log.Int64(-7) },
		func() error { return log.Uint64(8) },
		func() error { return log.Double(2.5) },
		func() error { return log.String("hello:\n") },
		func() error { return log.Symbol(NamedZ3LogSymbol("named|symbol")) },
		func() error { return log.Uint64(11) },
		func() error { return log.Uint64(12) },
		func() error { return log.UintArray(2) },
		func() error { return log.Call(41) },
	} {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}

	var replayOutput bytes.Buffer
	replayer, err := NewZ3Replayer(bytes.NewReader(encoded.Bytes()), &replayOutput)
	if err != nil {
		t.Fatal(err)
	}
	if err := replayer.RegisterCommand(41, "roundtrip", func(in *Z3Replayer) error {
		i, _ := in.Int64(0)
		u, _ := in.Uint64(1)
		d, _ := in.Double(2)
		s, _ := in.String(3)
		symbol, _ := in.Symbol(4)
		array, _ := in.UintArray(5)
		if i != -7 || u != 8 || d != 2.5 || s != "hello:\n" || symbol != "named|symbol" || !reflect.DeepEqual(array, []uint32{11, 12}) {
			t.Fatalf("unexpected replay values: %d %d %g %q %q %v", i, u, d, s, symbol, array)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := replayer.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestZ3LogResultPointerRoundTrip(t *testing.T) {
	var encoded bytes.Buffer
	log, _ := NewZ3LogWriter(&encoded)
	operations := []func() error{
		log.ResetArguments,
		func() error { return log.Call(1) },
		func() error { return log.Result(0xa0) },
		log.ResetArguments,
		func() error { return log.Pointer(0xa0) },
		func() error { return log.Call(2) },
	}
	for _, operation := range operations {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}
	replayer, _ := NewZ3Replayer(bytes.NewReader(encoded.Bytes()), &bytes.Buffer{})
	_ = replayer.RegisterCommand(1, "make", func(in *Z3Replayer) error { return in.StoreResult(777) })
	_ = replayer.RegisterCommand(2, "use", func(in *Z3Replayer) error {
		object, err := in.Object(0)
		if err != nil {
			return err
		}
		if object != 777 {
			t.Fatalf("got object %d", object)
		}
		return nil
	})
	if err := replayer.Parse(); err != nil {
		t.Fatal(err)
	}
}
