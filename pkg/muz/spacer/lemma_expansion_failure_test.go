package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerLiteralExpansionColdFailureReleasesPartialResults(t *testing.T) {
	stops := make([]int64, 16)
	for i := range stops {
		stops[i] = int64(i)
	}
	for stop := int64(16); stop <= 16384; stop *= 2 {
		stops = append(stops, stop)
	}
	succeeded, failures := false, 0
	for _, stop := range stops {
		if succeeded {
			break
		}
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, _ := newSpacerTestServices(t)
			p := &PredicateTransformer{manager: m}
			value, err := m.BitVectorNumeralUnsigned(0xaaaaaaaa, 128)
			depthMust(t, err)
			sort, err := m.GetSort(value)
			depthMust(t, err)
			x, err := m.MkFreshConst("partial-expansion", sort)
			depthMust(t, err)
			equality, err := m.MkEq(x, value)
			depthMust(t, err)
			input, err := m.NewRef(equality)
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&input)) }()
			inputCount, err := m.GetRefCount(equality)
			depthMust(t, err)
			xCount, err := m.GetRefCount(x)
			depthMust(t, err)
			beforeRefs := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			result, err := p.expandInductiveLiteral(equality)
			m.Budget().FailAfter(-1)
			if err != nil {
				if result != nil {
					t.Fatal("failed expansion published a result")
				}
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					depthMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected expansion failure", err)
				}
				failures++
			} else {
				succeeded = true
				if len(result.literals) != 128 {
					t.Fatal("expanded width", len(result.literals))
				}
				depthMust(t, result.Close())
			}
			for _, entry := range []struct {
				node  ast.ASTHandle
				count uint32
			}{{equality, inputCount}, {x, xCount}} {
				count, err := m.GetRefCount(entry.node)
				depthMust(t, err)
				if count != entry.count {
					t.Fatal("partial expansion leaked references to input", count, entry.count)
				}
			}
			if m.Stats().LiveReferences != beforeRefs {
				t.Fatal("partial expansion leaked reference slots")
			}
			for attempt := 0; attempt < 2; attempt++ {
				retry, err := p.expandInductiveLiteral(equality)
				depthMust(t, err)
				if len(retry.literals) != 128 {
					t.Fatal("retry width", len(retry.literals))
				}
				depthMust(t, retry.Close())
				count, err := m.GetRefCount(x)
				depthMust(t, err)
				if count != xCount {
					t.Fatal("retry leaked expanded literals", count, xCount)
				}
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing expansion failure/success frontier", failures, succeeded)
	}
}
