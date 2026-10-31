package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:60-62,259-262;
// src/util/ref_vector.h:39-59,90-94.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestNLA2BVDefinitionVectorAllocationPublicationPrefixes(t *testing.T) {
	seen := [4]bool{}
	succeeded := false
	for stop := int64(0); stop < 128; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			r := nla2BVFixture(t, false)
			m := r.manager
			c, err := r.newInvocation()
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, c.close()) })
			x := fixDLConst(t, m, true, "vector-variable")
			definition := fixDLNumber(t, m, true, "42")
			inputRefs, err := m.GetRefCount(x)
			equationMust(t, err)
			definitionRefs, err := m.GetRefCount(definition)
			equationMust(t, err)
			m.Budget().FailAfter(stop)
			err = c.addDefinition(x, definition)
			m.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
			if len(c.trail) < len(c.vars) || len(c.vars) < len(c.defs) || len(c.trail) > 1 {
				t.Fatal("vectors published out of source order")
			}
			seen[len(c.trail)+len(c.vars)+len(c.defs)] = true
			contains, lookupErr := m.ExprSubstitutionContains(c.sub, x)
			if len(c.trail) == 0 && (lookupErr != nil || contains) {
				t.Fatal("substitution preceded trail publication", contains, lookupErr)
			}
			if len(c.vars) != 0 && (lookupErr != nil || !contains) {
				t.Fatal("variable publication preceded substitution", contains, lookupErr)
			}
			equationMust(t, c.close())
			for _, bytes := range []uint64{c.trailBytes, c.defsBytes, c.varsBytes} {
				if bytes != 0 {
					t.Fatal("vector allocation survived invocation destruction")
				}
			}
			xRefs, err := m.GetRefCount(x)
			equationMust(t, err)
			valueRefs, err := m.GetRefCount(definition)
			equationMust(t, err)
			if xRefs != inputRefs || valueRefs != definitionRefs {
				t.Fatal("definition prefix leaked references", inputRefs, xRefs, definitionRefs, valueRefs)
			}
		})
		if succeeded {
			break
		}
	}
	if !succeeded || seen != [4]bool{true, true, true, true} {
		t.Fatal("definition publication frontiers were not exercised", succeeded, seen)
	}
}

func TestNLA2BVVectorWarmAppendGrowthFailureAndClose(t *testing.T) {
	for _, name := range []string{"trail", "vars", "defs"} {
		t.Run(name, func(t *testing.T) {
			r := nla2BVFixture(t, false)
			m := r.manager
			c, err := r.newInvocation()
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, c.close()) })
			h := fixDLConst(t, m, true, "vector-owner")
			values, charged := &c.trail, &c.trailBytes
			if name == "vars" {
				values, charged = &c.vars, &c.varsBytes
				h, err = m.GetDecl(h)
				equationMust(t, err)
			} else if name == "defs" {
				values, charged = &c.defs, &c.defsBytes
			}
			refs, err := m.GetRefCount(h)
			equationMust(t, err)
			equationMust(t, c.appendOwned(values, charged, h))
			m.Budget().FailAfter(0)
			err = c.appendOwned(values, charged, h)
			m.Budget().FailAfter(-1)
			equationMust(t, err)
			if len(*values) != 2 || cap(*values) != 2 || *charged == 0 {
				t.Fatal("source first allocation or warm append")
			}
			before := m.Budget().Used()
			m.Budget().FailAfter(0)
			err = c.appendOwned(values, charged, h)
			m.Budget().FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) || len(*values) != 2 || m.Budget().Used() != before {
				t.Fatal("failed vector growth changed the completed prefix or charge", err)
			}
			for _, entry := range *values {
				if entry != h {
					t.Fatal("failed growth changed an existing vector entry")
				}
			}
			after, err := m.GetRefCount(h)
			equationMust(t, err)
			if after != refs+2 {
				t.Fatal("failed append leaked its unpublished reference", refs, after)
			}
			equationMust(t, c.appendOwned(values, charged, h))
			if cap(*values) != 3 || len(*values) != 3 {
				t.Fatal("source growth retry")
			}
			equationMust(t, c.close())
			if *values != nil || *charged != 0 {
				t.Fatal("vector storage survived close")
			}
			after, err = m.GetRefCount(h)
			equationMust(t, err)
			if after != refs {
				t.Fatal("vector destruction lost reference accounting", refs, after)
			}
		})
	}
}
