package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:208-263.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestNLA2BVBoundedBitsProjectArithmeticAndFailureFrontiers(t *testing.T) {
	for _, wide := range []bool{false, true} {
		succeeded := false
		for stop := int64(0); stop < 1024; stop++ {
			t.Run(fmt.Sprintf("wide=%v/%d", wide, stop), func(t *testing.T) {
				r := nla2BVFixture(t, false)
				c, err := r.newInvocation()
				equationMust(t, err)
				t.Cleanup(func() { equationMust(t, c.close()) })
				low, up := big.NewInt(7), big.NewInt(-4)
				if wide {
					low = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 65))
					up = new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 65), big.NewInt(17))
				}
				lower, upper := c.local.rational(new(big.Rat).SetInt(low)), c.local.rational(new(big.Rat).SetInt(up))
				equationMust(t, c.local.err)
				s := r.manager.NumeralService()
				before, err := s.Stats()
				equationMust(t, err)
				persistent := bv2RealRecognitionPersistentObjects(t, s)
				r.manager.Budget().FailAfter(stop)
				bits := c.boundedBits(lower, upper)
				r.manager.Budget().FailAfter(-1)
				if c.local.err == nil {
					succeeded = true
					want := new(big.Int).Abs(new(big.Int).Sub(up, low)).BitLen()
					if want == 0 {
						want = 1
					}
					if bits != uint32(want) {
						t.Fatal("independent integer span width", bits, want)
					}
				} else {
					var failure umemory.Failure
					if !errors.Is(c.local.err, arena.ErrInjectedFailure) && (!errors.As(c.local.err, &failure) || failure.Kind != umemory.FailureMemout || !s.RuntimeOwner().State().OutOfMemory) {
						t.Fatal(c.local.err)
					}
				}
				if len(c.local.numbers) != 2 || len(c.local.refs) != 0 || len(c.local.readPending) != 0 || c.numBits != 4 {
					t.Fatal("bounded width helper changed enclosing ownership or default width")
				}
				for i, value := range []*big.Int{low, up} {
					owner := lower
					if i == 1 {
						owner = upper
					}
					actual, err := s.String(owner)
					equationMust(t, err)
					if actual != value.String() {
						t.Fatal("span arithmetic changed a caller bound", i, actual)
					}
				}
				after, err := s.Stats()
				equationMust(t, err)
				if after.CacheLength != before.CacheLength || after.LiveNumericObjects-bv2RealRecognitionPersistentObjects(t, s) != before.LiveNumericObjects-persistent {
					t.Fatal("span arithmetic grew the power cache or leaked temporaries", before, after)
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("bounded span success frontier was not reached", wide)
		}
	}
}

func TestNLA2BVIntegerDefinitionProjectBoundsAndCachedOffset(t *testing.T) {
	for _, mode := range []string{"unbounded", "lower", "upper", "both"} {
		t.Run(mode, func(t *testing.T) {
			r := nla2BVFixture(t, false)
			m := r.manager
			c, err := r.newInvocation()
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, c.close()) })
			c.numBits = 257
			c.filter, err = tactic.NewFilterModel(m)
			equationMust(t, err)
			equationMust(t, c.filterOwner.Prepare(m))
			equationMust(t, c.filterOwner.SetModelConverter(c.filter))
			x := fixDLConst(t, m, true, "integer-offset")
			if mode == "lower" || mode == "both" {
				equationMust(t, c.bounds.Collect(fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "-5")), ast.ASTDependency{}))
			}
			if mode == "upper" || mode == "both" {
				equationMust(t, c.bounds.Collect(fixDLApp(t, m, 2, x, fixDLNumber(t, m, true, "7")), ast.ASTDependency{}))
			}
			equationMust(t, c.addInteger(x))
			if len(c.defs) != 1 || len(c.vars) != 1 || len(c.trail) != 1 || len(c.local.numbers) != 0 || len(c.local.refs) != 0 {
				t.Fatal("integer definition handoff")
			}
			definition := c.defs[0]
			kind, err := m.GetDeclKind(definition)
			equationMust(t, err)
			wantKind, offsetIndex, bvIndex := int32(6), uint32(1), uint32(0)
			wantOffset, wantWidth := "-5", uint32(257)
			if mode == "unbounded" {
				wantKind = 7
				wantOffset = new(big.Int).Lsh(big.NewInt(1), 256).String()
				stats, err := m.NumeralService().Stats()
				equationMust(t, err)
				if stats.CacheLength < 258 {
					t.Fatal("unbounded midpoint bypassed source power_of_two cache", stats.CacheLength)
				}
			} else if mode == "upper" {
				wantKind, offsetIndex, bvIndex, wantOffset = 7, 0, 1, "7"
			} else if mode == "both" {
				wantWidth = 4
			}
			if kind != wantKind {
				t.Fatal("source offset operator", kind, wantKind)
			}
			offset, err := m.GetArg(definition, offsetIndex)
			equationMust(t, err)
			actual, integer, yes, err := m.ArithmeticNumeralText(offset)
			equationMust(t, err)
			if !yes || !integer || actual != wantOffset {
				t.Fatal("source offset numeral", actual, wantOffset)
			}
			conversion, err := m.GetArg(definition, bvIndex)
			equationMust(t, err)
			fresh, err := m.GetArg(conversion, 0)
			equationMust(t, err)
			width, err := m.BitVectorSize(fresh)
			equationMust(t, err)
			if width != wantWidth || c.satPreserving != (mode == "both") {
				t.Fatal("bound width or precision", width, wantWidth, c.satPreserving)
			}
		})
	}
}

func TestNLA2BVUnboundedOffsetCacheFailureFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 1024; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			r := nla2BVFixture(t, false)
			m := r.manager
			c, err := r.newInvocation()
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, c.close()) })
			c.numBits = 65
			c.filter, err = tactic.NewFilterModel(m)
			equationMust(t, err)
			equationMust(t, c.filterOwner.Prepare(m))
			equationMust(t, c.filterOwner.SetModelConverter(c.filter))
			x := fixDLConst(t, m, true, "offset-failure")
			m.Budget().FailAfter(stop)
			err = c.addInteger(x)
			m.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
				if len(c.defs) != 1 {
					t.Fatal("successful unbounded definition was not published")
				}
				offset, err := m.GetArg(c.defs[0], 1)
				equationMust(t, err)
				text, integer, yes, err := m.ArithmeticNumeralText(offset)
				equationMust(t, err)
				if !yes || !integer || text != "18446744073709551616" {
					t.Fatal("source cached midpoint", text)
				}
			} else {
				var failure umemory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != umemory.FailureMemout || !m.NumeralService().RuntimeOwner().State().OutOfMemory) {
					t.Fatal(err)
				}
			}
			if len(c.trail) < len(c.vars) || len(c.vars) < len(c.defs) {
				t.Fatal("definition failure published vectors out of source order")
			}
			// Cache growth commits a valid prefix even if a later allocation fails.
			s := m.NumeralService()
			for i := range s.Cache {
				text, err := s.Manager.String(&s.Cache[i])
				equationMust(t, err)
				if text != new(big.Int).Lsh(big.NewInt(1), uint(i)).String() {
					t.Fatal("failed power cache growth damaged a completed entry", i, text)
				}
			}
			equationMust(t, c.close())
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("cached offset success frontier was not reached")
	}
}
