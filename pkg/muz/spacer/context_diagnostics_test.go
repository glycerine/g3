package spacer

// Source: src/muz/spacer/spacer_context.h's empty display_certificate and
// src/muz/spacer/spacer_context.cpp's empty get_proof result.

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
)

func TestContextCertificateAndProofPresentationSurface(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	context, err := NewContext(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	output.WriteString("prefix")
	if err = context.DisplayCertificate(&output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "prefix" {
		t.Fatal("source empty certificate hook wrote output", output.String())
	}
	// The native hook ignores the stream entirely.
	if err = context.DisplayCertificate(nil); err != nil {
		t.Fatal(err)
	}
	proof, err := context.Proof()
	if err != nil || !proof.IsNull() {
		t.Fatal("source Spacer context exposed an overall proof", proof, err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if err = context.DisplayCertificate(&output); !errors.Is(err, ErrContextOwner) {
		t.Fatal("closed context accepted certificate display", err)
	}
	if _, err = context.Proof(); !errors.Is(err, ErrContextOwner) {
		t.Fatal("closed context exposed a proof", err)
	}
}
