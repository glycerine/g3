package cmd_context

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestInterruptedNormalizedObjectiveBoundsRestoreSourceOrder(t *testing.T) {
	for _, bitVector := range []bool{false, true} {
		for _, maximize := range []bool{false, true} {
			t.Run(fmt.Sprintf("bitvector=%t/maximize=%t", bitVector, maximize), func(t *testing.T) {
				c := contextFixture(t)
				m, err := c.Manager()
				contextMust(t, err)
				var term ast.ASTHandle
				if bitVector {
					term, err = m.BitVectorNumeralUint64(0, 4)
				} else {
					term, err = m.ArithmeticIntegerNumeralText("0")
				}
				contextMust(t, err)
				contextMust(t, c.AddObjective(term, maximize))
				result := &opt.ArithmeticResult{Status: lbool.Undef, Values: make([]opt.ObjectiveValue, 1)}
				result.Values[0].SoftBoundsValid = true
				result.Values[0].SoftLower.SetInt64(2)
				result.Values[0].SoftUpper.SetInt64(5)
				decomposed := []opt.DecomposedObjective{{Converted: true, Negate: maximize}}
				decomposed[0].Offset.SetInt64(10)
				contextMust(t, restoreNormalizedObjectiveBounds(result, c.objectives, decomposed))
				c.publishOptimizationBounds(result)
				result.Values[0].SourceLower.SetInt64(100)
				result.Values[0].SourceUpper.SetInt64(200)
				var out bytes.Buffer
				contextMust(t, c.WriteOptimizationFrontendValues(&out))
				want := "  [12:15]\n"
				if maximize {
					want = "  [5:8]\n"
				}
				if out.String() != want {
					t.Fatalf("restored bounds = %q, want %q", out.String(), want)
				}
			})
		}
	}
}
