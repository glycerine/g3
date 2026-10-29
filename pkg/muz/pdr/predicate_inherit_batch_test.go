package pdr_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
)

func TestPredicateInheritanceCopiesFiniteAndInvariantProperties(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 4)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "inherit_properties_batch", nil, boolean)
	source, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableHashCache)
	pdrMust(t, err)
	target, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableHashCache)
	pdrMust(t, err)
	finite := pdrBool(t, manager, "inherit_finite_batch")
	invariant := pdrBool(t, manager, "inherit_invariant_batch")
	_, err = source.AddProperty(finite, 1)
	pdrMust(t, err)
	_, err = source.AddProperty(invariant, pdr.InfinityLevel)
	pdrMust(t, err)
	pdrMust(t, target.InheritProperties(source))
	properties, err := target.Properties()
	pdrMust(t, err)
	if len(properties) != 2 {
		t.Fatal("inherited property count", properties)
	}
	otherHead := pdrFunction(t, manager, "inherit_wrong_head_batch", nil, boolean)
	other, err := pdr.NewPredicateTransformer(owner, otherHead, pdr.ReachableHashCache)
	pdrMust(t, err)
	if err = other.InheritProperties(source); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("head mismatch guard", err)
	}
	pdrMust(t, other.Close())
	pdrMust(t, target.Close())
	pdrMust(t, source.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
