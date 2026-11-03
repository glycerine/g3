package ast

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// A cold, wide substitution crosses both node and reference pages. Include
// nested patterns so the source application and quantifier reconstruction
// branches own their fresh nodes before the next allocation can fail.
func TestExprAbstractorAllocationFailureOwnsEveryConstructedNode(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested=%t", nested), func(t *testing.T) {
			failed, completed := 0, false
			for allowance := int64(0); allowance < 1024 && !completed; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					m := astTestManager(t)
					boolean, err := m.MkBoolSort()
					astTestMust(t, err)
					const width = 512
					bound := make([]ASTHandle, width)
					domain := make([]ASTHandle, width)
					for i := range bound {
						bound[i], err = m.MkFreshConst(fmt.Sprintf("abstract-allocation-%d", i), boolean)
						astTestMust(t, err)
						domain[i] = boolean
					}
					decl, err := m.MkFuncDecl(astTestSymbol(t, m, "abstract-allocation-body"), domain, boolean, ASTFuncDeclInfo{})
					astTestMust(t, err)
					body, err := m.MkApp(decl, bound)
					astTestMust(t, err)
					input := body
					if nested {
						pattern, err := m.MkFamilyApp(2, 0, nil, []ASTHandle{body})
						astTestMust(t, err)
						input, err = m.MkQuantifier(ASTQuantifierSpec{
							Forall: true, Sorts: []ASTHandle{boolean},
							Names: []usymbol.Symbol{astTestSymbol(t, m, "inner-allocation")},
							Body:  body, Patterns: []ASTHandle{pattern},
						})
						astTestMust(t, err)
					}
					astTestPin(t, m, input)
					out := astTestPin(t, m, input)
					counts := func() [4]uint32 {
						s := m.Stats()
						return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
					}
					before := counts()
					abstractor, err := NewExprAbstractor(m)
					astTestMust(t, err)
					m.Budget().FailAfter(allowance)
					cause := abstractor.Abstract(3, bound, input, out)
					m.Budget().FailAfter(-1)
					astTestMust(t, abstractor.Close())
					astTestMust(t, abstractor.Close())
					if cause != nil {
						failed++
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal("unexpected abstraction failure", cause)
						}
						if result, err := m.RefGet(out); err != nil || result != input {
							t.Fatal("failed abstraction published a partial output", result, err)
						}
						if after := counts(); after != before {
							t.Fatal("failed abstraction lost AST ownership", before, after)
						}
						abstractor, err = NewExprAbstractor(m)
						astTestMust(t, err)
						astTestMust(t, abstractor.Abstract(3, bound, input, out))
						astTestMust(t, abstractor.Close())
					} else {
						completed = true
					}
					result, err := m.RefGet(out)
					astTestMust(t, err)
					if result == input {
						t.Fatal("successful abstraction did not rewrite input")
					}
					base := uint32(3)
					if nested {
						base++
						rewritten, err := m.GetQuantifierBody(result)
						astTestMust(t, err)
						pattern, err := m.GetPattern(result, 0)
						astTestMust(t, err)
						patternBody, err := m.GetArg(pattern, 0)
						astTestMust(t, err)
						if patternBody != rewritten {
							t.Fatal("rewritten body and pattern lost source sharing")
						}
						result = rewritten
					}
					for i := range bound {
						argument, err := m.GetArg(result, uint32(i))
						astTestMust(t, err)
						index, err := m.GetVarIndex(argument)
						astTestMust(t, err)
						if want := base + uint32(width-i-1); index != want {
							t.Fatal("abstraction changed source variable order", i, index, want)
						}
						original, err := m.GetArg(body, uint32(i))
						astTestMust(t, err)
						if original != bound[i] {
							t.Fatal("abstraction mutated original input", i)
						}
					}
					astTestMust(t, m.SetRef(out, input))
					if after := counts(); after != before {
						t.Fatal("successful abstraction or retry lost AST ownership", before, after)
					}
				}) {
					return
				}
			}
			if failed == 0 || !completed {
				t.Fatal("allocation campaign missed failure or completion", failed, completed)
			}
		})
	}
}
