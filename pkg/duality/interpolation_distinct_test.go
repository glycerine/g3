package duality

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

func TestInterpolationAxiomLargeDistinctLegacySolverWorkaround(t *testing.T) {
	ctx, _, native := dualityContextFixture(t)
	manager, err := interp.NewManager(native)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
	}()
	graph, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := graph.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, count := range []int{10, 11} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			var refs []ast.ASTRef
			defer func() {
				for i := len(refs) - 1; i >= 0; i-- {
					if err := native.DelRef(&refs[i]); err != nil {
						t.Error(err)
					}
				}
			}()
			args := make([]ast.ASTHandle, count)
			for i := range args {
				args[i], err = native.ArithmeticIntegerNumeralText(strconv.Itoa(i))
				if err != nil {
					t.Fatal(err)
				}
				ref, e := native.NewRef(args[i])
				if e != nil {
					t.Fatal(e)
				}
				refs = append(refs, ref)
			}
			distinct, e := native.MkDistinct(args)
			if e != nil {
				t.Fatal(e)
			}
			input, e := NewExpr(ctx, distinct)
			if e != nil {
				t.Fatal(e)
			}
			defer func() {
				if e := input.Close(); e != nil {
					t.Error(e)
				}
			}()
			out, changed, e := graph.interpolationAxiom(manager, input)
			if e != nil {
				t.Fatal(e)
			}
			if count == 10 {
				if changed || out != input {
					t.Fatal("source threshold changed at ten arguments")
				}
				return
			}
			if !changed || out == input {
				t.Fatal("eleven-argument distinct was not rewritten")
			}
			defer func() {
				if e := out.Close(); e != nil {
					t.Error(e)
				}
			}()
			h, e := out.Raw()
			if e != nil {
				t.Fatal(e)
			}
			if op, e := manager.Operator(h); e != nil || op != interp.OperatorAnd {
				t.Fatal("rewritten axiom root", op, e)
			}
			parts, e := manager.Args(h)
			if e != nil || len(parts) != count {
				t.Fatal("rewritten pair count", len(parts), e)
			}
			var declaration ast.ASTHandle
			for i, part := range parts {
				op, e := manager.Operator(part)
				if e != nil || op != interp.OperatorEqual {
					t.Fatal("rewritten constraint", i, op, e)
				}
				pair, e := manager.Args(part)
				if e != nil || len(pair) != 2 {
					t.Fatal("equality arity", i, e)
				}
				decl, e := native.GetDecl(pair[0])
				if e != nil {
					t.Fatal(e)
				}
				if i == 0 {
					declaration = decl
				} else if declaration != decl {
					t.Fatal("fresh function differs between arguments")
				}
			}
		})
	}
}
