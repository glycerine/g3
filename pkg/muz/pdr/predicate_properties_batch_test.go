package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/pdr"
)

func TestPredicatePropertiesExposeOnlyEffectivePromotedFrame(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 2)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "property_snapshot_batch", nil, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableHashCache)
	pdrMust(t, err)
	property := pdrBool(t, manager, "promoted_property_batch")
	_, err = transformer.AddProperty(property, 0)
	pdrMust(t, err)
	_, err = transformer.AddProperty(property, 2)
	pdrMust(t, err)
	properties, err := transformer.Properties()
	pdrMust(t, err)
	if len(properties) != 1 || properties[0].Formula != property || properties[0].Level != 2 {
		t.Fatal("effective property snapshot", properties)
	}
	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
