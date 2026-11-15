package z3exception

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestZ3ExceptionSourceCodesAndLabels(t *testing.T) {
	codes := []ErrorCode{ErrorOK, ErrorMemout, ErrorTimeout, ErrorParser, ErrorUnsoundness, ErrorIncompleteness, ErrorINIFile, ErrorNotImplementedYet, ErrorOpenFile, ErrorCmdLine, ErrorInternalFatal, ErrorTypeCheck, ErrorUnknownResult, ErrorAllocExceeded}
	labels := []string{"out of memory", "timeout", "parser error", "unsoundess", "incompleteness", "invalid INI file", "not implemented yet", "open file", "invalid command line", "internal error", "type error", "unknown error", "number of configured allocations exceeded"}
	if codes[0] != 0 {
		t.Fatal(codes)
	}
	zero, e := NewZ3Error(ErrorOK)
	if z3ErrorSourceDebug {
		if e != ErrExceptionPrecondition {
			t.Fatal(e)
		}
	} else if e != nil || zero.ErrorCode() != ErrorOK || zero.HasErrorCode() || zero.Message() != "unknown error" {
		t.Fatal(zero, e)
	}
	for i, c := range codes[1:] {
		if c != ErrorCode(101+i) {
			t.Fatal(c)
		}
		z, e := NewZ3Error(c)
		if e != nil || z.Message() != labels[i] || z.Error() != labels[i] || z.ErrorCode() != c || !z.HasErrorCode() {
			t.Fatal(c, z, e)
		}
		var base Z3Exception = z
		if !base.HasErrorCode() {
			t.Fatal(base)
		}
	}
	for _, c := range []ErrorCode{1, 100, 114, ^ErrorCode(0)} {
		z, e := NewZ3Error(c)
		if e != nil || z.Message() != "unknown error" || z.ErrorCode() != c {
			t.Fatal(c, z, e)
		}
	}
}
func TestDefaultExceptionSourceStringAndEmbeddedNUL(t *testing.T) {
	for _, message := range []string{"", "plain", "prefix\x00suffix", "\x00hidden", "\xff\xfe", "line\nnext"} {
		x := NewDefaultException(message)
		want := strings.SplitN(message, "\x00", 2)[0]
		if x.Message() != want || x.Error() != want || x.ErrorCode() != ErrorOK || x.HasErrorCode() {
			t.Fatal(message, x)
		}
		if x.MessageField != message {
			t.Fatal("std::string storage must retain bytes")
		}
		var err error = x
		var target DefaultException
		if !errors.As(err, &target) || target.Message() != want {
			t.Fatal(err)
		}
	}
}

func TestDefaultExceptionSourceFormattedConstructor(t *testing.T) {
	args := []any{12, "twelve\x00ignored"}
	x := NewDefaultExceptionf("Format %d %s", args...)
	if x.MessageField != "Format 12 twelve" || x.Message() != "Format 12 twelve" ||
		x.ErrorCode() != ErrorOK || args[1] != "twelve\x00ignored" {
		t.Fatal(x, args)
	}
	x = NewDefaultExceptionf("%s-end", strings.Repeat("x", 1100))
	if x.MessageField != strings.Repeat("x", 1023) || len(x.Message()) != 1023 {
		t.Fatal("source ostream truncation", len(x.MessageField))
	}
	x = NewDefaultExceptionf("%s-tail\x00ignored", "left\x00ignored")
	if x.Message() != "left-tail" {
		t.Fatal("C-string inputs", x.Message())
	}
}

func TestDefaultExceptionRetainedDatalogUnsignedLine(t *testing.T) {
	got := NewDefaultExceptionf("%s at line %u '%s' found '%s'\n", "syntax", uint32(4294967295), "name", "token")
	if want := "syntax at line 4294967295 'name' found 'token'\n"; got.Message() != want {
		t.Fatalf("message = %q, want %q", got.Message(), want)
	}
	got = NewDefaultExceptionf("%%u line %u", uint32(7))
	if want := "%u line 7"; got.Message() != want {
		t.Fatalf("escaped percent = %q, want %q", got.Message(), want)
	}
}

func TestDefaultExceptionRetainedDatalogUnsignedSymbol(t *testing.T) {
	got := NewDefaultExceptionf("unknown symbol number %I64u on line %d in file %s", ^uint64(0), 31, "facts.rel")
	if want := "unknown symbol number 18446744073709551615 on line 31 in file facts.rel"; got.Message() != want {
		t.Fatalf("message = %q, want %q", got.Message(), want)
	}
	got = NewDefaultExceptionf("%%I64u = %I64u", uint64(42))
	if want := "%I64u = 42"; got.Message() != want {
		t.Fatalf("escaped percent = %q, want %q", got.Message(), want)
	}
}

// SourceMessage is the ex.msg() a C++ catch site prints: Go wrapping context
// around a source exception does not reach the user; a plain Go error keeps
// its text.
func TestSourceMessageSkipsGoWrapping(t *testing.T) {
	source := NewDefaultException("max. resource limit exceeded")
	wrapped := fmt.Errorf("smt kernel internalize assertions: %w", errors.Join(fmt.Errorf("asserted preprocessing nnf: %w", source), nil))
	if got := SourceMessage(wrapped); got != "max. resource limit exceeded" {
		t.Fatalf("wrapped source message %q", got)
	}
	plain := fmt.Errorf("context: %w", errors.New("plain"))
	if got := SourceMessage(plain); got != "context: plain" {
		t.Fatalf("plain message %q", got)
	}
	if got := SourceMessage(nil); got != "" {
		t.Fatalf("nil message %q", got)
	}
}
