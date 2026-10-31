package ast

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func legacyBenchmarkStreamFixture(t *testing.T) (*ASTManager, *LegacySMTPrinter, ASTHandle, LegacySMTBenchmarkConfig) {
	t.Helper()
	m := astTestManager(t)
	p, err := NewLegacySMTPrinter(m)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	root, err := m.MkConst(astTestSymbol(t, m, "streamed-benchmark-p"), boolean)
	astTestMust(t, err)
	astTestPin(t, m, root)
	return m, p, root, LegacySMTBenchmarkConfig{BenchmarkName: astTestSymbol(t, m, "streamed")}
}

func TestLegacySMTBenchmarkPublishesPrefixBeforeDeclarationFailure(t *testing.T) {
	_, p, root, config := legacyBenchmarkStreamFixture(t)
	var out strings.Builder
	cause := errors.New("declaration policy refused")
	const prefix = "; streamed\n(set-info :status unknown)\n"
	calls := 0
	config.IsDeclaredFunc = func(ASTHandle) (bool, error) {
		calls++
		if out.String() != prefix {
			t.Fatal("declaration operand ran before metadata publication", out.String())
		}
		return false, cause
	}
	err := p.DisplayBenchmark(&out, root, config)
	if !errors.Is(err, cause) || calls != 1 || out.String() != prefix {
		t.Fatal("source failure prefix", err, calls, out.String())
	}
}

type legacyBenchmarkRefusingWriter struct {
	writes int
	short  bool
}

func (w *legacyBenchmarkRefusingWriter) Write([]byte) (int, error) {
	w.writes++
	if w.short {
		return 0, nil
	}
	return 0, io.ErrClosedPipe
}

func TestLegacySMTBenchmarkStreamFailureStillEvaluatesOperands(t *testing.T) {
	for _, short := range []bool{false, true} {
		for _, operandFails := range []bool{false, true} {
			_, p, root, config := legacyBenchmarkStreamFixture(t)
			w := &legacyBenchmarkRefusingWriter{short: short}
			cause := errors.New("later declaration operand refused")
			calls := 0
			config.IsDeclaredFunc = func(ASTHandle) (bool, error) {
				calls++
				if w.writes != 1 {
					t.Fatal("source ostream did not fail before operand", w.writes)
				}
				if operandFails {
					return false, cause
				}
				return false, nil
			}
			err := p.DisplayBenchmark(w, root, config)
			want := io.ErrClosedPipe
			if short {
				want = io.ErrShortWrite
			}
			if !errors.Is(err, want) || errors.Is(err, cause) != operandFails || calls != 1 || w.writes != 1 {
				t.Fatal("stream/operand evaluation", err, calls, w.writes)
			}
		}
	}
}

func TestLegacySMTBenchmarkNilWriterPrecedesOperandReads(t *testing.T) {
	_, p, root, config := legacyBenchmarkStreamFixture(t)
	called := false
	config.IsDeclaredFunc = func(ASTHandle) (bool, error) { called = true; return false, nil }
	if err := p.DisplayBenchmark(nil, root, config); !errors.Is(err, ErrASTHandle) || called {
		t.Fatal("nil output guard", err, called)
	}
}
