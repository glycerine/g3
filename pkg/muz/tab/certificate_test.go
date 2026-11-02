package tab

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type tabCertificateFailWriter struct{}

func (tabCertificateFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDisplayCertificateAnswersAndOwnership(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	truth, err := m.MkTrue()
	must(err)
	truthRef, err := m.NewRef(truth)
	must(err)
	defer func() { must(m.DelRef(&truthRef)) }()
	before, err := m.GetRefCount(truth)
	must(err)
	var output bytes.Buffer
	must(DisplayCertificate(m, &output, truthRef))
	if output.String() != "true\n" {
		t.Fatal("unreachable certificate", output.String())
	}
	after, err := m.GetRefCount(truth)
	must(err)
	if after != before {
		t.Fatal("certificate display changed answer ownership", before, after)
	}

	scope, err := ast.NewScopedProof(m)
	must(err)
	proof, err := m.MkProofAsserted(truth)
	must(err)
	proofRef, err := m.NewRef(proof)
	must(err)
	must(scope.Close())
	defer func() { must(m.DelRef(&proofRef)) }()
	output.Reset()
	must(DisplayCertificate(m, &output, proofRef))
	if !strings.Contains(output.String(), "asserted") || !strings.HasSuffix(output.String(), "\n") {
		t.Fatal("reachable proof certificate", output.String())
	}
	if err = DisplayCertificate(m, tabCertificateFailWriter{}, proofRef); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("writer refusal", err)
	}
	if err = DisplayCertificate(m, nil, proofRef); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("nil writer", err)
	}
	if err = DisplayCertificate(nil, io.Discard, proofRef); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil manager", err)
	}
	if err = DisplayCertificate(m, io.Discard, ast.ASTRef{}); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("missing answer", err)
	}
}
