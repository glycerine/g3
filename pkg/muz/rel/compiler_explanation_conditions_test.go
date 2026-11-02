package rel

// Source: dl_compiler.cpp interpreted-tail free-variable enumeration,
// filtering, head assembly and projection.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCompileExplanationConditionWitnessesAgainstTupleOracle(t *testing.T) {
	for _, representation := range []string{"hashtable", "sparse"} {
		for _, mode := range []string{"project-witness", "head-reuse", "ground-nonempty", "ground-contradiction", "negative-reuse", "two-sorts"} {
			t.Run(representation+"/"+mode, func(t *testing.T) {
				f := newExplanationNegativeFixture(t, representation)
				arity := 1
				if mode == "head-reuse" {
					arity = 2
				} else if mode == "ground-nonempty" || mode == "ground-contradiction" {
					arity = 0
				}
				p, q, n := f.declare("condition-input", 1), f.declare("condition-output", arity), f.declare("condition-blocker", 1)
				x, y := f.own(f.ast.MkVar(0, f.boolean)), f.own(f.ast.MkVar(1, f.boolean))
				headArgs := []ast.ASTHandle{x}
				if arity == 0 {
					headArgs = nil
				} else if arity == 2 {
					headArgs = []ast.ASTHandle{y, y}
				}
				head := f.own(f.ast.MkApp(q, headArgs))
				positive := f.own(f.ast.MkApp(p, []ast.ASTHandle{x}))
				tails, flags := []ast.ASTHandle{positive}, []bool{false}
				switch mode {
				case "ground-nonempty", "ground-contradiction":
					tails, flags = nil, nil
					tails = append(tails, f.own(f.ast.MkEq(y, f.truth)))
					if mode == "ground-contradiction" {
						tails = append(tails, f.own(f.ast.MkEq(y, f.falsehood)))
					}
				case "negative-reuse":
					negative := f.own(f.ast.MkApp(n, []ast.ASTHandle{y}))
					tails, flags = append(tails, negative), append(flags, true)
					tails = append(tails, f.own(f.ast.MkEq(y, x)))
				case "two-sorts":
					bits := f.own(f.ast.MkVar(1, f.bits))
					boolean := f.own(f.ast.MkVar(2, f.boolean))
					two := f.own(f.ast.BitVectorNumeralUint64(2, 2))
					tails = append(tails, f.own(f.ast.MkEq(bits, two)), f.own(f.ast.MkEq(boolean, f.truth)), f.own(f.ast.MkEq(x, boolean)))
				default:
					tails = append(tails, f.own(f.ast.MkEq(y, x)))
				}
				// Negation flags cover every tail; interpreted tails remain positive.
				for len(flags) < len(tails) {
					flags = append(flags, false)
				}
				f.add(head, tails, flags, "condition-step")
				astTestMust(t, f.set.SetOutputPredicate(q))
				input, err := f.relations.GetFiniteRelation(p)
				astTestMust(t, err)
				_, err = input.Add(Fact{0})
				astTestMust(t, err)
				_, err = input.Add(Fact{1})
				astTestMust(t, err)
				negative, err := f.relations.GetFiniteRelation(n)
				astTestMust(t, err)
				_, err = negative.Add(Fact{1})
				astTestMust(t, err)
				transformer, transformed := f.transform(p, q, n)
				explainedQ, err := transformer.ExplanationDeclaration(q)
				astTestMust(t, err)
				explainedP, err := transformer.ExplanationDeclaration(p)
				astTestMust(t, err)
				positiveSource, err := f.relations.GetExplanationRelation(explainedP)
				astTestMust(t, err)
				execution, termination, err := CompileExplanationRulesWithGuard(f.relations, transformed, tableRelationTheoryRewriter(t, f.ast))
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) })
				astTestMust(t, transformer.Close())
				astTestMust(t, transformed.Close())
				astTestMust(t, f.set.Close())
				context, err := NewExecutionContextWithRelations(f.relations, nil)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, context.Close()) })
				fact := f.proof("fact")
				wantProof := f.proof("condition-step", fact)
				if arity == 0 {
					wantProof = f.proof("condition-step")
				}
				for repeat := 0; repeat < 2; repeat++ {
					complete, err := execution.Perform(context)
					astTestMust(t, err)
					if !complete {
						t.Fatal("condition witness execution incomplete")
					}
					complete, err = termination.Perform(context)
					astTestMust(t, err)
					if !complete {
						t.Fatal("condition witness termination incomplete")
					}
					output, err := f.relations.GetFiniteRelation(q)
					astTestMust(t, err)
					product, err := f.relations.GetExplanationRelation(explainedQ)
					astTestMust(t, err)
					verify := func(row Fact, values []ast.ASTHandle, want bool) {
						found, err := output.Contains(row)
						astTestMust(t, err)
						got, explained, err := product.Explanation(values)
						astTestMust(t, err)
						if found != want || explained != want || (want && got != wantProof) {
							t.Fatal("condition witness tuple or original proof differs", mode, row, found, explained, got, want, wantProof)
						}
					}
					if arity == 0 {
						verify(Fact{}, nil, mode == "ground-nonempty")
					} else {
						for a, av := range []ast.ASTHandle{f.falsehood, f.truth} {
							if arity == 1 {
								want := mode != "negative-reuse" || a == 0
								if mode == "two-sorts" {
									want = a == 1
								}
								verify(Fact{Element(a)}, []ast.ASTHandle{av}, want)
							} else {
								for b, bv := range []ast.ASTHandle{f.falsehood, f.truth} {
									verify(Fact{Element(a), Element(b)}, []ast.ASTHandle{av, bv}, a == b)
								}
							}
						}
					}
					for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
						proof, found, err := positiveSource.Explanation([]ast.ASTHandle{value})
						astTestMust(t, err)
						if !found || proof != fact {
							t.Fatal("condition witness filtering changed a source explanation", value, proof, found)
						}
					}
					astTestMust(t, context.Reset())
				}
			})
		}
	}
}

func TestCompileExplanationRejectsUnsupportedConditionDomainsBeforePublication(t *testing.T) {
	for _, mode := range []string{"infinite-integer", "unbound-rule-sort"} {
		t.Run(mode, func(t *testing.T) {
			f := newExplanationNegativeFixture(t, "hashtable")
			p, q := f.declare("unsupported-condition-input", 1), f.declare("unsupported-condition-output", 1)
			x := f.own(f.ast.MkVar(0, f.boolean))
			positive := f.own(f.ast.MkApp(p, []ast.ASTHandle{x}))
			head := f.own(f.ast.MkApp(q, []ast.ASTHandle{x}))
			var value ast.ASTHandle
			if mode == "infinite-integer" {
				_, err := f.ast.RegisterArithmeticPlugin()
				astTestMust(t, err)
				value = f.own(f.ast.ArithmeticIntegerNumeralText("0"))
			} else {
				value = f.proof("unbound-condition-value")
			}
			sort, err := f.ast.GetSort(value)
			astTestMust(t, err)
			unbound := f.own(f.ast.MkVar(1, sort))
			condition := f.own(f.ast.MkEq(unbound, value))
			f.add(head, []ast.ASTHandle{positive, condition}, nil, "unsupported-condition")
			astTestMust(t, f.set.SetOutputPredicate(q))
			transformer, transformed := f.transform(p, q)
			explainedQ, err := transformer.ExplanationDeclaration(q)
			astTestMust(t, err)
			execution, termination, err := CompileExplanationRulesWithGuard(f.relations, transformed, tableRelationTheoryRewriter(t, f.ast))
			if execution != nil || termination != nil || !errors.Is(err, ErrCompilerUnsupported) {
				t.Fatal("unsupported condition variable published bytecode", mode, execution, termination, err)
			}
			product, err := f.relations.GetExplanationRelation(explainedQ)
			astTestMust(t, err)
			count, err := product.Len()
			astTestMust(t, err)
			marked, err := f.relations.IsSaturated(explainedQ)
			astTestMust(t, err)
			if count != 0 || marked {
				t.Fatal("unsupported condition published a result", count, marked)
			}
		})
	}
}

func TestCompileExplanationConditionSparseIndicesAndMixedHeadPermutation(t *testing.T) {
	for _, representation := range []string{"hashtable", "sparse"} {
		t.Run(representation, func(t *testing.T) {
			f := newExplanationNegativeFixture(t, representation)
			p := f.declare("sparse-condition-input", 1)
			symbol, err := f.ast.Symbols.Intern("sparse-condition-output")
			astTestMust(t, err)
			q, err := f.ast.MkFuncDecl(symbol, []ast.ASTHandle{f.boolean, f.bits, f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			astTestMust(t, f.rules.RegisterPredicate(q))
			x := f.own(f.ast.MkVar(0, f.boolean))
			z := f.own(f.ast.MkVar(3, f.bits))
			y := f.own(f.ast.MkVar(7, f.boolean))
			two := f.own(f.ast.BitVectorNumeralUint64(2, 2))
			head := f.own(f.ast.MkApp(q, []ast.ASTHandle{y, z, x}))
			body := f.own(f.ast.MkApp(p, []ast.ASTHandle{x}))
			// Witness order follows variable indices; head order deliberately
			// requests the Boolean witness before the bit-vector witness.
			f.add(head, []ast.ASTHandle{body, f.own(f.ast.MkEq(y, x)), f.own(f.ast.MkEq(z, two))}, nil, "sparse-condition-step")
			astTestMust(t, f.set.SetOutputPredicate(q))
			input, err := f.relations.GetFiniteRelation(p)
			astTestMust(t, err)
			_, err = input.Add(Fact{0})
			astTestMust(t, err)
			_, err = input.Add(Fact{1})
			astTestMust(t, err)
			transformer, transformed := f.transform(p, q)
			explained, err := transformer.ExplanationDeclaration(q)
			astTestMust(t, err)
			execution, termination, err := CompileExplanationRulesWithGuard(f.relations, transformed, tableRelationTheoryRewriter(t, f.ast))
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) })
			astTestMust(t, transformer.Close())
			astTestMust(t, transformed.Close())
			astTestMust(t, f.set.Close())
			context, err := NewExecutionContextWithRelations(f.relations, nil)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, context.Close()) })
			wantProof := f.proof("sparse-condition-step", f.proof("fact"))
			bits := make([]ast.ASTHandle, 4)
			for index := range bits {
				bits[index] = f.own(f.ast.BitVectorNumeralUint64(uint64(index), 2))
			}
			for repeat := 0; repeat < 2; repeat++ {
				complete, err := execution.Perform(context)
				astTestMust(t, err)
				if !complete {
					t.Fatal("sparse-index condition execution incomplete")
				}
				complete, err = termination.Perform(context)
				astTestMust(t, err)
				if !complete {
					t.Fatal("sparse-index condition termination incomplete")
				}
				output, err := f.relations.GetFiniteRelation(q)
				astTestMust(t, err)
				product, err := f.relations.GetExplanationRelation(explained)
				astTestMust(t, err)
				for a, av := range []ast.ASTHandle{f.falsehood, f.truth} {
					for b, bv := range bits {
						for c, cv := range []ast.ASTHandle{f.falsehood, f.truth} {
							row := Fact{Element(a), Element(b), Element(c)}
							want := a == c && b == 2
							found, err := output.Contains(row)
							astTestMust(t, err)
							proof, explained, err := product.Explanation([]ast.ASTHandle{av, bv, cv})
							astTestMust(t, err)
							if found != want || explained != want || (want && proof != wantProof) {
								t.Fatal("sparse variable/head permutation changed tuple or proof", row, found, explained, proof, want, wantProof)
							}
						}
					}
				}
				astTestMust(t, context.Reset())
			}
		})
	}
}
