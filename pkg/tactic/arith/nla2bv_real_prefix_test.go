package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:265-286.

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func nla2BVRealPrefixInvocation(t *testing.T) *nla2BVInvocation {
	t.Helper()
	r := nla2BVFixture(t, false)
	c, err := r.newInvocation()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, c.close()) })
	c.filter, err = tactic.NewFilterModel(r.manager)
	equationMust(t, err)
	equationMust(t, c.filterOwner.Prepare(r.manager))
	equationMust(t, c.filterOwner.SetModelConverter(c.filter))
	return c
}

func nla2BVFreshCounterMarker(t *testing.T, m *ast.ASTManager) uint64 {
	t.Helper()
	sort, err := m.MkBoolSort()
	equationMust(t, err)
	h, err := m.MkFreshConst("counter-marker", sort)
	equationMust(t, err)
	fixDLPin(t, m, h)
	decl, err := m.GetDecl(h)
	equationMust(t, err)
	name, err := m.GetASTName(decl)
	equationMust(t, err)
	text, err := m.Symbols.String(name)
	equationMust(t, err)
	n, err := strconv.ParseUint(text[strings.LastIndexByte(text, '!')+1:], 10, 64)
	equationMust(t, err)
	return n
}

func TestNLA2BVRealSortFailurePrecedesPrecisionDowngrade(t *testing.T) {
	c := nla2BVRealPrefixInvocation(t)
	c.numBits = 0
	x := fixDLConst(t, c.t.manager, false, "real-invalid-width")
	if err := c.addReal(x); err == nil {
		t.Fatal("zero-width coefficient sort was accepted")
	}
	if !c.satPreserving || len(c.trail) != 0 || len(c.vars) != 0 || len(c.defs) != 0 {
		t.Fatal("failed sort changed precision or published definitions")
	}
	var filter bytes.Buffer
	equationMust(t, c.filter.Display(&filter))
	if filter.String() != "(filter-model-converter)\n" {
		t.Fatal("failed sort published a filter declaration", filter.String())
	}
}

func TestNLA2BVRealCoefficientCreationPrecedesFilterPublication(t *testing.T) {
	succeeded, beforeSecond, filteredFailure := false, false, false
	for warm := 0; warm < 64; warm++ {
		completed := false
		for stop := int64(0); stop < 1024; stop++ {
			t.Run(fmt.Sprintf("warm=%d/%d", warm, stop), func(t *testing.T) {
				c := nla2BVRealPrefixInvocation(t)
				m := c.t.manager
				prefix := "real-prefix"
				x := fixDLConst(t, m, false, prefix)
				// Fill actual AST pages with live constants to reach the reservation
				// between the two factories; no internal counters are modified.
				sort, err := m.GetSort(x)
				equationMust(t, err)
				for i := 0; i < warm; i++ {
					h, err := m.MkFreshConst("warm-real", sort)
					equationMust(t, err)
					fixDLPin(t, m, h)
				}
				before := nla2BVFreshCounterMarker(t, m)
				m.Budget().FailAfter(stop)
				err = c.addReal(x)
				m.Budget().FailAfter(-1)
				if err == nil {
					succeeded, completed = true, true
				} else {
					var failure umemory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != umemory.FailureMemout || !m.NumeralService().RuntimeOwner().State().OutOfMemory) {
						t.Fatal(err)
					}
				}
				// Observe the real manager counter, rather than test-only stage flags.
				after := nla2BVFreshCounterMarker(t, m)
				created := after - before - 1
				var filter bytes.Buffer
				equationMust(t, c.filter.Display(&filter))
				first := strings.Contains(filter.String(), fmt.Sprintf("%s!%d", prefix, before+1))
				second := strings.Contains(filter.String(), fmt.Sprintf("%s_r!%d", prefix, before+2))
				if first && created < 2 || second && !first {
					t.Fatal("filter published before both coefficient factories", created, filter.String())
				}
				if err != nil && created == 1 && !first && !second {
					beforeSecond = true
				}
				if err != nil && first && second {
					filteredFailure = true
				}
				if err == nil && (!first || !second || c.satPreserving || len(c.defs) != 1 || len(c.trail) != 1 || len(c.vars) != 1) {
					t.Fatal("completed real definition handoff", filter.String())
				}
				equationMust(t, c.local.close())
				if len(c.local.refs) != 0 || len(c.local.numbers) != 0 {
					t.Fatal("real construction leaked helper owners")
				}
			})
			if completed {
				break
			}
		}
		if succeeded && beforeSecond && filteredFailure {
			break
		}
	}
	if !succeeded || !beforeSecond || !filteredFailure {
		t.Fatal("source real-construction frontiers were not exercised", succeeded, beforeSecond, filteredFailure)
	}
}
