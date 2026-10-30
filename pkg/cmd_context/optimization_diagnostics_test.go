package cmd_context

// Source: src/shell/opt_frontend.cpp model validation branches.

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func optimizationDiagnosticsContext(t *testing.T) *Context {
	t.Helper()
	context, err := NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	return context
}

func TestOptimizationModelValidationFailuresAndSuccess(t *testing.T) {
	context := optimizationDiagnosticsContext(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsity, err := manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Assert(truth, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = context.CheckSat(nil); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	count, err := context.writeOptimizationModelValidation(&output, []ast.ASTHandle{truth, falsity})
	if err != nil || count != 1 || strings.TrimSpace(output.String()) != "false evaluates to: false" {
		t.Fatal(count, output.String(), err)
	}
	output.Reset()
	count, err = context.WriteOptimizationModelValidation(&output)
	if err != nil || count != 0 || output.Len() != 0 {
		t.Fatal(count, output.String(), err)
	}
	valid, err := context.OptimizationModelValid()
	if err != nil || !valid {
		t.Fatal(valid, err)
	}
}

func TestOptimizationModelValidationAvailabilityAndWriter(t *testing.T) {
	context := optimizationDiagnosticsContext(t)
	if _, err := context.WriteOptimizationModelValidation(&bytes.Buffer{}); err == nil {
		t.Fatal("validation accepted missing model")
	}
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Assert(truth, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = context.CheckSat(nil); err != nil {
		t.Fatal(err)
	}
	if _, err = context.WriteOptimizationModelValidation(nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if reason, err := context.OptimizationReasonUnknown(); err != nil || reason != "" {
		t.Fatal(reason, err)
	}
}
