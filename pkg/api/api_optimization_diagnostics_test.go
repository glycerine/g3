package api

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextOptimizationDiagnostics(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	truth, err := manager.MkTrue()
	astTestMust(t, err)
	astTestMust(t, context.Assert(truth))
	status, err := context.CheckSat()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	var output bytes.Buffer
	violations, err := context.WriteOptimizationModelValidation(&output)
	astTestMust(t, err)
	valid, err := context.OptimizationModelValid()
	astTestMust(t, err)
	if violations != 0 || output.Len() != 0 || !valid {
		t.Fatal(violations, output.String(), valid)
	}
	reason, err := context.OptimizationReasonUnknown()
	astTestMust(t, err)
	if reason != "" {
		t.Fatal(reason)
	}
}

func TestAPIOptimizeDiagnosticsAndClosedGuards(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	truth, err := manager.MkTrue()
	astTestMust(t, err)
	optimizer, err := context.NewOptimize()
	astTestMust(t, err)
	astTestMust(t, optimizer.Assert(truth))
	status, err := optimizer.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	valid, err := optimizer.OptimizationModelValid()
	astTestMust(t, err)
	if !valid {
		t.Fatal("optimizer model rejected")
	}
	var output bytes.Buffer
	violations, err := optimizer.WriteOptimizationModelValidation(&output)
	astTestMust(t, err)
	if violations != 0 || output.Len() != 0 {
		t.Fatal(violations, output.String())
	}
	astTestMust(t, optimizer.Close())
	if _, err = optimizer.OptimizationModelValid(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
	if _, err = optimizer.WriteOptimizationModelValidation(&output); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}

func TestSMTContextOptimizationDiagnosticsClosed(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, context.Close())
	if _, err = context.OptimizationReasonUnknown(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
	if _, err = context.OptimizationModelValid(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
	if _, err = context.WriteOptimizationModelValidation(&bytes.Buffer{}); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}
