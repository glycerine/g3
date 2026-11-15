package pdr_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
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

// AUDIT.md muz_pdr_02-F4: inherit_properties walks m_prop2level, an
// obj_map<expr, unsigned>, in table order; every lemma ever inserted stays
// at its strongest level.
func TestPredicatePropertiesFollowPropToLevelTableOrder(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 4)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "property_table_order", nil, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableHashCache)
	pdrMust(t, err)
	var lemmas []ast.ASTHandle
	levels := []uint32{2, pdr.InfinityLevel, 0, 1, pdr.InfinityLevel, 0, 3, 1, 2, 0, 1}
	table := ast.NewObjMap(manager)
	for i, level := range levels {
		lemma := pdrBool(t, manager, fmt.Sprintf("property_table_order_%d", i))
		lemmas = append(lemmas, lemma)
		_, err = transformer.AddProperty(lemma, level)
		pdrMust(t, err)
		_, err = table.InsertIfNotThere(lemma)
		pdrMust(t, err)
	}
	// Promote one lemma: the table keeps its slot.
	_, err = transformer.AddProperty(lemmas[2], 3)
	pdrMust(t, err)
	properties, err := transformer.Properties()
	pdrMust(t, err)
	entries := table.Entries()
	if len(properties) != len(entries) {
		t.Fatal("property count", len(properties), len(entries))
	}
	for i, entry := range entries {
		if properties[i].Formula != entry.Key {
			t.Fatal("properties not in m_prop2level table order at", i)
		}
		want := levels[0]
		for j, lemma := range lemmas {
			if lemma == entry.Key {
				want = levels[j]
			}
		}
		if entry.Key == lemmas[2] {
			want = 3
		}
		if properties[i].Level != want {
			t.Fatal("property level", i, properties[i].Level, want)
		}
	}
	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
