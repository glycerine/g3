package rel

// Source: dl_compiler.cpp:1191,1243,1266 marks only completed components and
// skips them on restart; explanation rules store their results during execution.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExplanationCompilerRestartPreservesCompletedStrataAndProofs(t *testing.T) {
	for _, relationLevel := range []bool{false, true} {
		name := "fact"
		if relationLevel {
			name = "relation"
		}
		t.Run(name, func(t *testing.T) {
			f := newExplanationNegativeFixture(t, "hashtable")
			must := func(err error) { t.Helper(); astTestMust(t, err) }
			declare := func(name string, sorts ...ast.ASTHandle) ast.ASTHandle {
				declaration := finiteInterpretedProjectDeclaration(t, f.ast, name, sorts, f.boolean)
				must(f.rules.RegisterPredicate(declaration))
				return declaration
			}
			edge := declare("explanation-restart-edge", f.bits, f.bits)
			reach := declare("explanation-restart-reach", f.bits)
			app := func(predicate ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
				return f.own(f.ast.MkApp(predicate, args))
			}
			values := make([]ast.ASTHandle, 4)
			for i := range values {
				values[i] = f.bv(uint64(i))
				astTestPin(t, f.ast, values[i])
			}
			for i := 0; i < 3; i++ {
				f.add(app(edge, values[i], values[i+1]), nil, nil, fmt.Sprintf("restart-edge-%d", i))
			}
			f.add(app(reach, values[0]), nil, nil, "restart-seed")
			x, y := f.own(f.ast.MkVar(0, f.bits)), f.own(f.ast.MkVar(1, f.bits))
			f.add(app(reach, y), []ast.ASTHandle{app(reach, x), app(edge, x, y)}, nil, "restart-step")
			must(f.set.SetOutputPredicate(reach))
			transformer, err := NewExplanationTransformer(f.ast, f.rules)
			must(err)
			t.Cleanup(func() { must(transformer.Close()) })
			transformed, err := transformer.TransformRulesOnly(f.set)
			must(err)
			t.Cleanup(func() { must(transformed.Close()) })
			if relationLevel {
				must(transformer.TransferRelationFacts(f.set, transformed, f.relations, []ast.ASTHandle{edge, reach}))
			} else {
				must(transformer.TransferFiniteFacts(f.set, transformed, f.relations, []ast.ASTHandle{edge, reach}))
			}
			explainedEdge, err := transformer.ExplanationDeclaration(edge)
			must(err)
			explainedReach, err := transformer.ExplanationDeclaration(reach)
			must(err)
			guard := tableRelationTheoryRewriter(t, f.ast)
			compile := func() (*InstructionBlock, *InstructionBlock) {
				t.Helper()
				execution, termination, err := compileExplanationRulesWithGuard(f.relations, transformed, guard, relationLevel)
				must(err)
				t.Cleanup(func() { must(execution.Close()); must(termination.Close()) })
				return execution, termination
			}
			count := func(predicate ast.ASTHandle) int {
				t.Helper()
				if relationLevel {
					product, found, err := f.relations.TryGetRelationExplanation(predicate)
					must(err)
					if !found {
						return 0
					}
					rows, err := product.Len()
					must(err)
					return rows
				}
				product, found, err := f.relations.TryGetExplanationRelation(predicate)
				must(err)
				if !found {
					return 0
				}
				rows, err := product.Len()
				must(err)
				return rows
			}
			explanation := func(predicate ast.ASTHandle, args ...ast.ASTHandle) (ast.ASTHandle, bool) {
				t.Helper()
				if relationLevel {
					product, err := f.relations.GetRelationExplanation(predicate)
					must(err)
					proof, found, err := product.Explanation(args)
					must(err)
					return proof, found
				}
				product, err := f.relations.GetExplanationRelation(predicate)
				must(err)
				proof, found, err := product.Explanation(args)
				must(err)
				return proof, found
			}
			checkMark := func(predicate ast.ASTHandle, want bool) {
				t.Helper()
				marked, err := f.relations.IsSaturated(predicate)
				must(err)
				if marked != want {
					t.Fatal("wrong explanation component saturation", predicate, marked, want)
				}
			}
			wanted := []ast.ASTHandle{f.proof("restart-seed")}
			for i := 0; i < 3; i++ {
				wanted = append(wanted, f.proof("restart-step", wanted[i], f.proof(fmt.Sprintf("restart-edge-%d", i))))
			}
			allowed := map[string]bool{"restart-seed": true, "restart-step": true, "restart-edge-0": true, "restart-edge-1": true, "restart-edge-2": true, "e_union": true}
			checkReach := func(rows int) {
				t.Helper()
				if got := count(explainedReach); got != rows {
					t.Fatal("restart lost explained reachability", got, rows)
				}
				for i, value := range values {
					proof, found := explanation(explainedReach, value)
					if found != (i < rows) {
						t.Fatal("restart changed explained tuple membership", i, found, rows)
					}
					if !found {
						continue
					}
					if relationLevel {
						names := relationExplanationProofNames(t, f.ast, proof, allowed)
						if names["restart-seed"] == 0 || names["restart-step"] == 0 || names["restart-edge-0"] == 0 {
							t.Fatal("whole-relation restart lost source proof provenance", names)
						}
					} else if proof != wanted[i] {
						t.Fatal("fact restart changed ordered derivation", i, proof, wanted[i])
					}
				}
			}
			execution, termination := compile()
			interrupt, interrupted := true, false
			context, err := NewExecutionContextWithRelations(f.relations, func() bool {
				if interrupt && count(explainedReach) == 2 {
					interrupted = true
					return true
				}
				return false
			})
			must(err)
			t.Cleanup(func() { must(context.Close()) })
			complete, err := execution.Perform(context)
			must(err)
			if complete || !interrupted {
				t.Fatal("did not interrupt after committed recursive progress", complete, interrupted)
			}
			checkMark(explainedEdge, true)
			checkMark(explainedReach, false)
			checkReach(2)
			partialProof, found := explanation(explainedReach, values[1])
			if !found {
				t.Fatal("missing partial proof")
			}
			astTestPin(t, f.ast, partialProof)
			interrupt = false
			complete, err = termination.Perform(context)
			must(err)
			if !complete {
				t.Fatal("explanation termination incomplete")
			}
			checkMark(explainedEdge, true)
			checkMark(explainedReach, false)
			checkReach(2)
			if proof, _ := explanation(explainedReach, values[1]); proof != partialProof {
				t.Fatal("termination replaced committed partial proof")
			}
			must(context.Close())
			must(execution.Close())
			must(termination.Close())

			// A completed component must not execute again: its actual stored
			// product and proof remain identical, even in whole-relation mode.
			factEdge, _, err := f.relations.TryGetExplanationRelation(explainedEdge)
			must(err)
			wholeEdge, _, err := f.relations.TryGetRelationExplanation(explainedEdge)
			must(err)
			edgeProof, found := explanation(explainedEdge, values[0], values[1])
			if !found {
				t.Fatal("completed lower component lost its tuple")
			}
			astTestPin(t, f.ast, edgeProof)
			checkSkipped := func() {
				t.Helper()
				if relationLevel {
					owner, _, err := f.relations.TryGetRelationExplanation(explainedEdge)
					must(err)
					if owner != wholeEdge {
						t.Fatal("restart re-executed completed whole-relation component")
					}
				} else {
					owner, _, err := f.relations.TryGetExplanationRelation(explainedEdge)
					must(err)
					if owner != factEdge {
						t.Fatal("restart re-executed completed per-fact component")
					}
				}
				if proof, _ := explanation(explainedEdge, values[0], values[1]); proof != edgeProof {
					t.Fatal("restart grew a completed component's proof history")
				}
			}
			retry, finish := compile()
			checkMark(explainedEdge, true)
			checkMark(explainedReach, false)
			checkSkipped()
			healthy, err := NewExecutionContextWithRelations(f.relations, func() bool { checkSkipped(); return false })
			must(err)
			t.Cleanup(func() { must(healthy.Close()) })
			for _, block := range []*InstructionBlock{retry, finish} {
				complete, err = block.Perform(healthy)
				must(err)
				if !complete {
					t.Fatal("healthy explanation restart incomplete")
				}
			}
			checkSkipped()
			checkReach(4)
			output, err := f.relations.GetFiniteRelation(reach)
			must(err)
			rows, err := output.Facts()
			must(err)
			instructionWantFacts(t, rows, Fact{0}, Fact{1}, Fact{2}, Fact{3})
			for _, predicate := range []ast.ASTHandle{explainedEdge, explainedReach, reach} {
				checkMark(predicate, true)
			}
			done, _ := compile()
			if length, err := done.Len(); err != nil || length != 0 {
				t.Fatal("fully saturated program was compiled for execution again", length, err)
			}
		})
	}
}
