package pdr

// Source: src/muz/pdr/pdr_context.cpp context::validate_proof keeps each
// negated side condition in an expr_ref before constructing its fresh kernel.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestPDRProofValidationOwnsNegatedConditionBeforeSolverConstruction(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	var roots []ast.ASTRef
	defer func() {
		m.Budget().FailAfter(-1)
		for i := len(roots) - 1; i >= 0; i-- {
			closureMust(t, m.DelRef(&roots[i]))
		}
		closureMust(t, transformer.Close())
		closureMust(t, owner.Close())
		closeBase()
	}()
	closureMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		closureMust(t, err)
		ref, err := m.NewRef(h)
		closureMust(t, err)
		roots = append(roots, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	name, err := m.Symbols.Intern("pdr-validation-negation-failure")
	closureMust(t, err)
	atom := own(m.MkConst(name, boolean))
	negated := own(m.MkNot(atom))
	doubleNegation := own(m.MkNot(negated))
	proof := own(m.MkProofRewrite(doubleNegation, atom))
	invalid := own(m.MkProofRewrite(atom, negated))
	context := &Context{manager: m, owner: owner}
	// Warm the solver's declaration caches independently of the failure
	// campaign; each completed validation releases its temporary negation.
	if err = context.validateProofHandle(proof); err != nil {
		t.Fatal("valid double-negation proof rejected before failure campaign", err)
	}
	if err = context.validateProofHandle(invalid); !errors.Is(err, errPDRResultValidation) {
		t.Fatal("invalid rewrite passed initial semantic validation", err)
	}

	// The rewrite proof already owns its equality side condition. The Not
	// declaration and AST storage are warm, while the fresh validation kernel
	// must reserve memory after constructing the condition's outer negation.
	before := m.Stats()
	checkOwnership := func(label string) {
		t.Helper()
		after := m.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
			t.Fatal(label, "changed live AST ownership", before, after)
		}
	}
	m.Budget().FailAfter(0)
	err = context.validateProofHandle(proof)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("fresh validation solver did not reach allocation refusal", err)
	}
	checkOwnership("failed validation")
	for retry := 0; retry < 2; retry++ {
		if err = context.validateProofHandle(proof); err != nil {
			t.Fatal("valid double-negation proof did not recover", err)
		}
		checkOwnership("successful validation retry")
	}
	if err = context.validateProofHandle(invalid); !errors.Is(err, errPDRResultValidation) {
		t.Fatal("invalid rewrite passed semantic side-condition validation", err)
	}
	checkOwnership("rejected invalid rewrite")
}
