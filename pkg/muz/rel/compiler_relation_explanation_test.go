package rel

// Source: dl_mk_explanations.cpp relation-level product/sieve registration,
// get_e_rule and explanation_relation_plugin::union_fn.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCompileRelationExplanationRulesAggregateWholeProofHistory(t *testing.T) {
	for _, nullary := range []bool{false, true} {
		name := "finite-projection"
		if nullary {
			name = "nullary-projection"
		}
		t.Run(name, func(t *testing.T) {
			f := newExplanationNegativeFixture(t, "hashtable")
			p, r := f.declare("whole-input-P", 2), f.declare("whole-input-R", 2)
			qArity := 1
			if nullary {
				qArity = 0
			}
			q := f.declare("whole-output-Q", qArity)
			x, y := f.own(f.ast.MkVar(0, f.boolean)), f.own(f.ast.MkVar(1, f.boolean))
			var headArgs []ast.ASTHandle
			if !nullary {
				headArgs = []ast.ASTHandle{x}
			}
			head := f.own(f.ast.MkApp(q, headArgs))
			pApp := f.own(f.ast.MkApp(p, []ast.ASTHandle{x, y}))
			rApp := f.own(f.ast.MkApp(r, []ast.ASTHandle{x, y}))
			f.add(head, []ast.ASTHandle{pApp}, nil, "whole-step-P")
			f.add(head, []ast.ASTHandle{rApp}, nil, "whole-step-R")
			transformer, err := NewExplanationTransformer(f.ast, f.rules)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, transformer.Close()) })
			transformed, err := transformer.TransformRulesOnly(f.set)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, transformed.Close()) })
			explainedP, err := transformer.ExplanationDeclaration(p)
			astTestMust(t, err)
			explainedR, err := transformer.ExplanationDeclaration(r)
			astTestMust(t, err)
			explainedQ, err := transformer.ExplanationDeclaration(q)
			astTestMust(t, err)
			pProof, rProof := f.proof("whole-origin-P"), f.proof("whole-origin-R")
			seed := func(declaration, proof ast.ASTHandle, rows []Fact) *RelationExplanationProduct {
				product, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
				astTestMust(t, err)
				for _, row := range rows {
					_, err = product.table.Add(row)
					astTestMust(t, err)
				}
				astTestMust(t, product.explanation.AddFact([]ast.ASTHandle{proof}))
				astTestMust(t, f.relations.StoreRelationExplanation(declaration, product))
				return product
			}
			pRows, rRows := []Fact{{0, 0}, {1, 0}}, []Fact{{0, 1}, {1, 1}}
			pSource, rSource := seed(explainedP, pProof, pRows), seed(explainedR, rProof, rRows)
			// transform_facts gives even a rule-defined predicate with no
			// original tuples a fact proof in its independent proof sieve.
			// Source instruction I/O drops this finite-empty owner on load.
			headSorts := make([]ast.ASTHandle, qArity)
			for index := range headSorts {
				headSorts[index] = f.boolean
			}
			emptyHead, err := NewRelationExplanationProduct(f.relations, headSorts)
			astTestMust(t, err)
			astTestMust(t, emptyHead.explanation.AddFact([]ast.ASTHandle{f.proof("fact")}))
			astTestMust(t, f.relations.StoreRelationExplanation(explainedQ, emptyHead))
			guard := tableRelationTheoryRewriter(t, f.ast)
			var blocks []*InstructionBlock
			t.Cleanup(func() {
				for _, block := range blocks {
					astTestMust(t, block.Close())
				}
			})
			count, err := transformed.Len()
			astTestMust(t, err)
			if count != 2 {
				t.Fatal("relation-level fixture needs exactly two explained unary rules", count)
			}
			for index := 0; index < count; index++ {
				rule, err := transformed.Rule(index)
				astTestMust(t, err)
				block, err := CompileRelationExplanationRuleWithGuard(f.relations, rule, guard)
				astTestMust(t, err)
				astTestMust(t, rule.Close())
				blocks = append(blocks, block)
			}
			astTestMust(t, transformer.Close())
			astTestMust(t, transformed.Close())
			astTestMust(t, f.set.Close())
			context, err := NewExecutionContextWithRelations(f.relations, nil)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, context.Close()) })
			steps := []ast.ASTHandle{f.proof("whole-step-P", pProof), f.proof("whole-step-R", rProof)}
			var history ast.ASTHandle
			for repeat := 0; repeat < 2; repeat++ {
				for index, block := range blocks {
					complete, err := block.Perform(context)
					astTestMust(t, err)
					if !complete {
						t.Fatal("relation-level compiled rule did not complete")
					}
					astTestMust(t, context.Reset())
					output, err := f.relations.GetRelationExplanation(explainedQ)
					astTestMust(t, err)
					rows, err := output.table.Facts()
					astTestMust(t, err)
					if nullary {
						instructionWantFacts(t, rows, Fact{})
					} else {
						instructionWantFacts(t, rows, Fact{0}, Fact{1})
					}
					values := [][]ast.ASTHandle{nil}
					if !nullary {
						values = [][]ast.ASTHandle{{f.falsehood}, {f.truth}}
					}
					var current ast.ASTHandle
					for _, key := range values {
						proof, found, err := output.Explanation(key)
						astTestMust(t, err)
						if !found {
							t.Fatal("finite projection lost whole-relation explanation", key)
						}
						if history.IsNull() {
							if proof != steps[index] {
								t.Fatal("first rule did not assign its exact child proof", proof, steps[index])
							}
						} else {
							requireRelationUnionProof(t, f.ast, proof, history, steps[index])
						}
						if !current.IsNull() && current != proof {
							t.Fatal("tuples did not share their whole-relation proof", current, proof)
						}
						current = proof
					}
					// Reexecution adds another source proof contribution even when
					// every finite tuple already exists, as relation-level union does.
					history = current
				}
				for _, source := range []struct {
					declaration ast.ASTHandle
					product     *RelationExplanationProduct
					proof       ast.ASTHandle
					rows        []Fact
				}{{explainedP, pSource, pProof, pRows}, {explainedR, rSource, rProof, rRows}} {
					product, err := f.relations.GetRelationExplanation(source.declaration)
					astTestMust(t, err)
					if product != source.product {
						t.Fatal("compiled relation rule replaced its source product")
					}
					rows, err := product.table.Facts()
					astTestMust(t, err)
					instructionWantFacts(t, rows, source.rows...)
					proofs, err := product.explanation.Data()
					astTestMust(t, err)
					if len(proofs) != 1 || proofs[0] != source.proof {
						t.Fatal("compiled relation rule changed a source whole proof", proofs, source.proof)
					}
				}
			}
		})
	}
}
