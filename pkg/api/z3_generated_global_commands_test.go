package api

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestGeneratedGlobalReplayCommands(t *testing.T) {
	log := "R\nS \"timeout\"\nS \"19\"\nC 0\nR\nS \"timeout\"\nS \"\"\nC 2\nC 900\nR\nC 1\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = RegisterZ3GeneratedGlobalCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check", func(r *Z3Replayer) error {
		value, e := r.String(1)
		if e != nil {
			return e
		}
		if value != "19" {
			t.Fatalf("replayed global value %q", value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := GlobalParamResetAll(); e != nil {
			t.Error(e)
		}
	})
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	value, ok, err := GlobalParamGet("timeout")
	if err != nil || !ok || value == "19" {
		t.Fatal("global reset", value, ok, err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedGlobalCommands(r); err != ErrZ3ReplayState {
		t.Fatal("closed replay registration", err)
	}
}

func TestGeneratedGlobalLogAndReplay(t *testing.T) {
	var records bytes.Buffer
	writer, err := NewZ3LogWriter(&records)
	if err != nil {
		t.Fatal(err)
	}
	previous := processZ3Log
	processZ3Log = writer
	defer func() { processZ3Log = previous; _ = writer.Close() }()
	t.Cleanup(func() {
		if e := GlobalParamResetAll(); e != nil {
			t.Error(e)
		}
	})
	if err = GlobalParamSet("timeout", "21"); err != nil {
		t.Fatal(err)
	}
	value, ok, err := GlobalParamGet("timeout")
	if err != nil || !ok || value != "21" {
		t.Fatal("recorded get", value, ok, err)
	}
	if err = writer.Call(900); err != nil {
		t.Fatal(err)
	}
	if err = GlobalParamResetAll(); err != nil {
		t.Fatal(err)
	}
	processZ3Log = previous
	want := "R\nS \"timeout\"\nS \"21\"\nC 0\nR\nS \"timeout\"\nS \"\"\nC 2\nC 900\nR\nC 1\n"
	if records.String() != want {
		t.Fatalf("generated log\n got %q\nwant %q", records.String(), want)
	}
	r, err := NewZ3Replayer(bytes.NewReader(records.Bytes()), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = RegisterZ3GeneratedGlobalCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-output", func(r *Z3Replayer) error {
		got, e := r.String(1)
		if e != nil {
			return e
		}
		if got != "21" {
			t.Fatalf("generated replay output %q", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedGlobalReplayMissingValueIsNull(t *testing.T) {
	warnings := diagnostic.BorrowProcessWarningService()
	enabled := warnings.Enabled()
	if err := warnings.Enable(false); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := warnings.Enable(enabled); err != nil {
			t.Error(err)
		}
	}()
	r, err := NewZ3Replayer(strings.NewReader("R\nS \"no_such_g3_parameter\"\nS \"\"\nC 2\nC 901\n"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = RegisterZ3GeneratedGlobalCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-null", func(r *Z3Replayer) error {
		isNull, e := r.StringIsNull(1)
		if e != nil {
			return e
		}
		if !isNull {
			t.Fatal("missing output was not null")
		}
		if e = r.SetString(1, ""); e != nil {
			return e
		}
		isNull, e = r.StringIsNull(1)
		if e != nil {
			return e
		}
		if isNull {
			t.Fatal("empty string remained null")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedGlobalLogFailureDoesNotSkipUpdate(t *testing.T) {
	writer, err := NewZ3LogWriter(closedReplayWriter{})
	if err != nil {
		t.Fatal(err)
	}
	previous := processZ3Log
	processZ3Log = writer
	defer func() {
		processZ3Log = previous
		_ = writer.Close()
		if e := GlobalParamResetAll(); e != nil {
			t.Error(e)
		}
	}()
	if err = GlobalParamSet("timeout", "24"); err != nil {
		t.Fatal("source API failed", err)
	}
	if !errors.Is(writer.Err(), io.ErrClosedPipe) {
		t.Fatal("log failure was not recorded", writer.Err())
	}
	processZ3Log = previous
	value, ok, err := GlobalParamGet("timeout")
	if err != nil || !ok || value != "24" {
		t.Fatal("source update skipped", value, ok, err)
	}
}

type reentrantGlobalLogWriter struct {
	bytes.Buffer
	action  func()
	entered bool
}

func (w *reentrantGlobalLogWriter) Write(data []byte) (int, error) {
	if !w.entered {
		w.entered = true
		w.action()
	}
	return w.Buffer.Write(data)
}

func TestGeneratedGlobalLogSuppressesNestedAPICalls(t *testing.T) {
	var nestedErr error
	out := &reentrantGlobalLogWriter{action: func() { _, _, nestedErr = GlobalParamGet("timeout") }}
	writer, err := NewZ3LogWriter(out)
	if err != nil {
		t.Fatal(err)
	}
	previous := processZ3Log
	processZ3Log = writer
	defer func() {
		processZ3Log = previous
		_ = writer.Close()
		if e := GlobalParamResetAll(); e != nil {
			t.Error(e)
		}
	}()
	if err = GlobalParamSet("timeout", "31"); err != nil {
		t.Fatal(err)
	}
	if nestedErr != nil {
		t.Fatal(nestedErr)
	}
	want := "R\nS \"timeout\"\nS \"31\"\nC 0\n"
	if out.String() != want {
		t.Fatalf("nested records %q", out.String())
	}
}
