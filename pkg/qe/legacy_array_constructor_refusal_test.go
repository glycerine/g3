package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyArrayConstructorRollbackPreservesRefusedOwner(t *testing.T) {
	active, callbacks := false, 0
	var rollback error
	var owner *legacyArrayProjection
	f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
		if active {
			callbacks++
			rollback = closeFailedLegacyArrayProjection(owner, failure)
		}
		return failure
	})
	m := f.r.manager
	var err error
	owner, err = newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	md, err := model.NewModelCore(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	root := f.constant("array-constructor-refusal-root", f.integer)
	call, err := owner.syntax.begin(md)
	fmMust(t, err)
	_, err = call.ref(root)
	fmMust(t, err)
	value, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().Del(&value)) }()
	fmMust(t, m.NumeralService().SetString(&value, "18446744073709551617/18446744073709551619"))
	fmMust(t, f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount)+1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	fmMust(t, f.runtime.SetMaxCount(0))
	fmMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
	if callbacks == 0 || body == nil {
		t.Fatal("missing native refusal")
	}
	var construction memory.ConstructionFailure
	if !errors.As(rollback, &construction) || !errors.Is(rollback, ast.ErrASTPending) {
		t.Fatal("rollback did not publish retryable owner", rollback)
	}
	if owner.syntax == nil || len(owner.syntax.pending) != 1 || len(owner.syntax.pending[0].refs) != 1 {
		t.Fatal("rollback lost the concrete pending child")
	}
	actual, err := m.RefGet(owner.syntax.pending[0].refs[0])
	fmMust(t, err)
	if actual != root {
		t.Fatal("rollback changed pending root identity")
	}
	fmMust(t, construction.Cleanup())
	if owner.syntax != nil {
		t.Fatal("rollback retry left children live")
	}
}

func TestLegacyArithmeticConstructorRollbackPreservesRefusedOwner(t *testing.T) {
	active, callbacks := false, 0
	var rollback error
	var owner *legacyArithmeticProjection
	f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
		if active {
			callbacks++
			rollback = closeFailedLegacyArithmeticProjection(owner, failure)
		}
		return failure
	})
	m := f.r.manager
	var err error
	owner, err = newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	md, err := model.NewModelCore(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	root := f.constant("arithmetic-constructor-refusal-root", f.integer)
	call, err := owner.syntax.begin(md)
	fmMust(t, err)
	_, err = call.hold(root, nil)
	fmMust(t, err)
	value, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().Del(&value)) }()
	fmMust(t, m.NumeralService().SetString(&value, "18446744073709551617/18446744073709551619"))
	fmMust(t, f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount)+1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	fmMust(t, f.runtime.SetMaxCount(0))
	fmMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
	if callbacks == 0 || body == nil {
		t.Fatal("missing native refusal")
	}
	var construction memory.ConstructionFailure
	if !errors.As(rollback, &construction) || !errors.Is(rollback, ast.ErrASTPending) {
		t.Fatal("rollback did not publish retryable owner", rollback)
	}
	if owner.syntax == nil || len(owner.syntax.pending) != 1 || len(owner.syntax.pending[0].refs) != 1 {
		t.Fatal("rollback lost the concrete pending child")
	}
	actual, err := m.RefGet(owner.syntax.pending[0].refs[0])
	fmMust(t, err)
	if actual != root {
		t.Fatal("rollback changed pending root identity")
	}
	fmMust(t, construction.Cleanup())
	fmMust(t, construction.Cleanup())
	if owner.syntax != nil {
		t.Fatal("rollback retry left children live")
	}
}

func TestLegacyProjectionConstructorRollbackPreservesRefusedOwner(t *testing.T) {
	active, callbacks := false, 0
	var rollback error
	var owner *LegacyProjection
	f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
		if active {
			callbacks++
			rollback = closeFailedLegacyProjection(owner, failure)
		}
		return failure
	})
	m := f.r.manager
	var err error
	owner, err = NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	md, err := model.NewModelCore(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	root := f.constant("combined-constructor-refusal-root", f.integer)
	call, err := owner.arithmetic.syntax.begin(md)
	fmMust(t, err)
	_, err = call.hold(root, nil)
	fmMust(t, err)
	value, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().Del(&value)) }()
	fmMust(t, m.NumeralService().SetString(&value, "18446744073709551617/18446744073709551619"))
	fmMust(t, f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount)+1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	fmMust(t, f.runtime.SetMaxCount(0))
	fmMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
	if callbacks == 0 || body == nil {
		t.Fatal("missing native refusal")
	}
	var construction memory.ConstructionFailure
	if !errors.As(rollback, &construction) || !errors.Is(rollback, ast.ErrASTPending) {
		t.Fatal("rollback did not publish retryable owner", rollback)
	}
	// The array child holds no AST roots, so only the arithmetic child with
	// the pending call is retained for the retry.
	if owner.arithmetic == nil || len(owner.arithmetic.syntax.pending) != 1 {
		t.Fatal("rollback lost the pending parent child")
	}
	actual, err := m.RefGet(owner.arithmetic.syntax.pending[0].refs[0])
	fmMust(t, err)
	if actual != root {
		t.Fatal("rollback changed pending root identity")
	}
	fmMust(t, construction.Cleanup())
	fmMust(t, construction.Cleanup())
	if owner.manager != nil || owner.arithmetic != nil || owner.array != nil {
		t.Fatal("rollback retry left parent children live")
	}
}
