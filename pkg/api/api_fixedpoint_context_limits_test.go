package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextFixedpointResourceLimitAndRetry(t *testing.T) {
	for _, withOptions := range []bool{false, true} {
		t.Run(map[bool]string{false: "query", true: "query with options"}[withOptions], func(t *testing.T) {
			c, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, c.Close()) }()
			astTestMust(t, c.SetFixedpointEngine("pdr"))
			predicate, err := c.DeclareRelation("context-limit-p", nil)
			astTestMust(t, err)
			astTestMust(t, c.AddFixedpointFact(predicate, nil))
			m, err := c.Manager()
			astTestMust(t, err)
			limit := m.ResourceLimit()
			depth, previous := len(limit.Limits), limit.Limit
			query := func() (lbool.LBool, error) {
				if withOptions {
					return c.QueryFixedpointWithOptions(predicate, &FixedpointQueryOptions{})
				}
				return c.QueryFixedpoint(predicate)
			}
			// Flush the input before limiting the query: source HNF may return an
			// empty prefix when interrupted, permanently advancing that input.
			status, err := query()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("initial fixedpoint query", status)
			}
			astTestMust(t, c.SetUIntOption("rlimit", 1))
			status, _ = query()
			if status != lbool.Undef {
				t.Fatal("fixedpoint query exceeded resource limit", status)
			}
			if len(limit.Limits) != depth || limit.Limit != previous {
				t.Fatal("fixedpoint resource scope was not restored")
			}
			astTestMust(t, c.SetUIntOption("rlimit", 0))
			status, err = query()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("fixedpoint retry", status)
			}
		})
	}
}

func TestSMTContextFixedpointLevelResourceLimitAndRetry(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, c.Close()) }()
	astTestMust(t, c.SetFixedpointEngine("spacer"))
	predicate, err := c.DeclareRelation("context-level-limit-p", nil)
	astTestMust(t, err)
	astTestMust(t, c.AddFixedpointFact(predicate, nil))
	// Publish the pending fact before testing cancellation of the level query.
	status, err := c.QueryFixedpointFromLevel(predicate, 0)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("initial fixedpoint level query", status)
	}
	astTestMust(t, c.SetUIntOption("rlimit", 1))
	status, _ = c.QueryFixedpointFromLevel(predicate, 0)
	if status != lbool.Undef {
		t.Fatal("fixedpoint level query exceeded resource limit", status)
	}
	astTestMust(t, c.SetUIntOption("rlimit", 0))
	status, err = c.QueryFixedpointFromLevel(predicate, 0)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("fixedpoint level query retry", status)
	}
}
