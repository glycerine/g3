package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerExtendedPrettyOwnsRHSOnComparisonConstructionFailure(t *testing.T) {
	succeeded, failures := false, 0
	stops := make([]int64, 64)
	for i := range stops {
		stops[i] = int64(i)
	}
	for stop := int64(64); stop <= 16384; stop *= 2 {
		stops = append(stops, stop)
	}
	for _, stop := range stops {
		if succeeded {
			break
		}
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, _ := newSpacerTestServices(t)
			var pins, outputs []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				depthMust(t, err)
				ref, err := m.NewRef(h)
				depthMust(t, err)
				pins = append(pins, ref)
				return h
			}
			minus := own(m.ArithmeticIntegerNumeralText("-1"))
			sort, err := m.GetSort(minus)
			depthMust(t, err)
			x := own(m.MkFreshConst("rhs-failure-x", sort))
			y := own(m.MkFreshConst("rhs-failure-y", sort))
			negative := own(m.MkArithmeticMul(minus, y))
			lhs := own(m.MkArithmeticAdd(x, negative))
			constants := make([]ast.ASTHandle, 256)
			for i := range constants {
				constants[i] = own(m.ArithmeticIntegerNumeralText(fmt.Sprint(1001 + i)))
			}
			sample := own(m.MkArithmeticLE(lhs, constants[0]))
			declaration, err := m.GetDecl(sample)
			depthMust(t, err)
			family, err := m.ArithmeticFamilyID()
			depthMust(t, err)
			cfg := extendedPrettyConfig{m: m, arithmetic: family}
			before, err := m.GetRefCount(y)
			depthMust(t, err)
			m.Budget().FailAfter(stop)
			for _, constant := range constants {
				var output ast.ASTRef
				output, err = m.NewRef(ast.ASTHandle{})
				if err != nil {
					break
				}
				outputs = append(outputs, output)
				_, err = cfg.ReduceApp(declaration, ast.SliceArguments([]ast.ASTHandle{lhs, constant}), output, ast.ASTRef{})
				if err != nil {
					break
				}
			}
			m.Budget().FailAfter(-1)
			depthMust(t, releaseNormalizeRefs(m, outputs))
			if err != nil {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					depthMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected rewrite failure", err)
				}
				failures++
			} else {
				succeeded = true
			}
			after, err := m.GetRefCount(y)
			depthMust(t, err)
			if after != before {
				t.Fatal("unpublished RHS still references its input", after, before)
			}
			retry, err := m.NewRef(ast.ASTHandle{})
			depthMust(t, err)
			defer func() {
				if retry != (ast.ASTRef{}) {
					depthMust(t, m.DelRef(&retry))
				}
			}()
			_, err = cfg.ReduceApp(declaration, ast.SliceArguments([]ast.ASTHandle{lhs, constants[0]}), retry, ast.ASTRef{})
			depthMust(t, err)
			rewritten, err := m.RefGet(retry)
			depthMust(t, err)
			left, err := m.GetArg(rewritten, 0)
			depthMust(t, err)
			right, err := m.GetArg(rewritten, 1)
			depthMust(t, err)
			first, err := m.GetArg(right, 0)
			depthMust(t, err)
			second, err := m.GetArg(right, 1)
			depthMust(t, err)
			if left != x || first != y || second != constants[0] {
				t.Fatal("retry did not publish x <= y+C")
			}
			depthMust(t, m.DelRef(&retry))
			after, err = m.GetRefCount(y)
			depthMust(t, err)
			if after != before {
				t.Fatal("successful retry leaked RHS ownership", after, before)
			}

		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing construction failure/success frontier", failures, succeeded)
	}
}
