package clp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestCLPSearchVerboseLevels(t *testing.T) {
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	if err != nil {
		t.Fatal(err)
	}
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := diagnostic.SetProcessVerbosityLevel(oldLevel); err != nil {
			t.Error(err)
		}
		if err := diagnostic.SetProcessVerboseStream(oldStream); err != nil {
			t.Error(err)
		}
	})
	for _, tc := range []struct {
		level uint32
		want  string
	}{
		{0, ""},
		{1, "search 20 0\nsearch 19 1\n"},
		{2, "search 20 0\n0 clp-Q\nsearch 19 1\n1 clp-P\n"},
	} {
		stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
		if err != nil {
			t.Fatal(err)
		}
		if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
			t.Fatal(err)
		}
		if err = diagnostic.SetProcessVerbosityLevel(tc.level); err != nil {
			t.Fatal(err)
		}
		f := newFixture(t)
		engine, err := clp.NewEngine(f.m, f.set, f.solver, 20)
		if err != nil {
			t.Fatal(err)
		}
		status, err := engine.Query(f.app["Q"])
		if err != nil || status != lbool.True {
			t.Fatalf("level %d: %v, %v", tc.level, status, err)
		}
		if err = engine.Close(); err != nil {
			t.Fatal(err)
		}
		got, err := stream.String()
		if err != nil || got != tc.want {
			t.Fatalf("level %d: got %q, want %q, error %v", tc.level, got, tc.want, err)
		}
		if err = diagnostic.SetProcessVerboseStream(oldStream); err != nil {
			t.Fatal(err)
		}
		if err = stream.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
