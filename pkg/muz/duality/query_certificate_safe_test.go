package duality_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type dualityCertificateRefusingWriter struct {
	remaining int
}

func (w *dualityCertificateRefusingWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, io.ErrClosedPipe
	}
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestDualitySafeCertificateModelAndWriterRetry(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(owner.Close()) }()
	m, err := owner.Manager()
	must(err)
	config, err := owner.SolverTranslationConfig()
	must(err)
	backend, err := duality.NewBackend(m, config, true)
	must(err)
	defer func() { must(backend.Close()) }()

	var refs []ast.ASTRef
	defer func() {
		for i := range refs {
			must(m.DelRef(&refs[i]))
		}
	}()
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		ref, err := m.NewRef(h)
		must(err)
		refs = append(refs, ref)
		return h
	}
	boolean := pin(m.MkBoolSort())
	name, err := m.Symbols.Intern("safe-certificate-P")
	must(err)
	predicate := pin(m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	truth, falsity := pin(m.MkTrue()), pin(m.MkFalse())
	fact := pin(m.MkApp(predicate, []ast.ASTHandle{truth}))
	query := pin(m.MkApp(predicate, []ast.ASTHandle{falsity}))
	prepared, err := duality.PrepareQuery(m, []ast.ASTHandle{fact}, []uint32{math.MaxUint32}, query, []ast.ASTHandle{predicate}, math.MaxUint32)
	must(err)
	status, err := backend.Query(prepared)
	must(err)
	must(prepared.Close())
	if status != lbool.False {
		t.Fatal("safe query", status)
	}

	writeExpressionCalled := false
	var output bytes.Buffer
	must(backend.WriteCertificate(&output, func(io.Writer, ast.ASTHandle) error {
		writeExpressionCalled = true
		return io.ErrClosedPipe
	}))
	if writeExpressionCalled {
		t.Fatal("safe certificate entered counterexample derivation")
	}
	text := output.String()
	if !strings.HasPrefix(text, "(fixedpoint \n") || !strings.Contains(text, "safe-certificate-P") || !strings.HasSuffix(text, ")\n") {
		t.Fatal("safe certificate", text)
	}

	before := m.Stats().LiveReferences
	err = backend.WriteCertificate(&dualityCertificateRefusingWriter{remaining: 17}, owner.WriteExpression)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("safe certificate writer error", err)
	}
	if after := m.Stats().LiveReferences; after != before {
		t.Fatal("failed safe certificate leaked model references", before, after)
	}
	output.Reset()
	must(backend.WriteCertificate(&output, owner.WriteExpression))
	if !strings.Contains(output.String(), "safe-certificate-P") {
		t.Fatal("safe certificate retry", output.String())
	}
}
