package rel

// Source: dl_compiler.cpp compile_preds_init/compile_rule_evaluation/compile_loop,
// using the relation-level product/sieve installed by dl_mk_explanations.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Whole proofs form a shared DAG. Count each node once so this oracle also
// remains bounded when repeated execution adds overlapping derivations.
func relationExplanationProofNames(t *testing.T, manager *ast.ASTManager, proof ast.ASTHandle, allowed map[string]bool) map[string]int {
	t.Helper()
	names := make(map[string]int)
	seen := make(map[ast.ASTHandle]bool)
	var visit func(ast.ASTHandle)
	visit = func(term ast.ASTHandle) {
		if seen[term] {
			return
		}
		seen[term] = true
		declaration, err := manager.GetDecl(term)
		astTestMust(t, err)
		symbol, err := manager.GetASTName(declaration)
		astTestMust(t, err)
		name, err := manager.Symbols.Display(symbol)
		astTestMust(t, err)
		if !allowed[name] {
			t.Fatal("whole proof contains an unrelated derivation", name)
		}
		names[name]++
		count, err := manager.GetNumArgs(term)
		astTestMust(t, err)
		for index := uint32(0); index < count; index++ {
			argument, err := manager.GetArg(term, index)
			astTestMust(t, err)
			visit(argument)
		}
	}
	visit(proof)
	return names
}

func TestCompileRelationExplanationProgramRecursiveSeedsAndFiniteTermination(t *testing.T) {
	for _, representation := range []string{"hashtable", "sparse"} {
		for _, mutual := range []bool{false, true} {
			for _, seeded := range []bool{false, true} {
				name := representation + "/self"
				if mutual {
					name = representation + "/mutual"
				}
				if seeded {
					name += "/seeded"
				} else {
					name += "/unseeded"
				}
				t.Run(name, func(t *testing.T) {
					f := newExplanationNegativeFixture(t, representation)
					p, q := f.declare("whole-cycle-P", 0), f.declare("whole-cycle-Q", 0)
					pApp, qApp := f.own(f.ast.MkApp(p, nil)), f.own(f.ast.MkApp(q, nil))
					if mutual {
						f.add(pApp, []ast.ASTHandle{qApp}, nil, "whole-to-P")
						f.add(qApp, []ast.ASTHandle{pApp}, nil, "whole-to-Q")
					} else {
						f.add(pApp, []ast.ASTHandle{pApp}, nil, "whole-self")
					}
					astTestMust(t, f.set.SetOutputPredicate(p))
					if seeded {
						seed := p
						if mutual {
							seed = q
						}
						finite, err := f.relations.GetFiniteRelation(seed)
						astTestMust(t, err)
						_, err = finite.Add(Fact{})
						astTestMust(t, err)
					}
					transformer, err := NewExplanationTransformer(f.ast, f.rules)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, transformer.Close()) })
					transformed, err := transformer.TransformRulesOnly(f.set)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, transformed.Close()) })
					astTestMust(t, transformer.TransferRelationFacts(f.set, transformed, f.relations, []ast.ASTHandle{p, q}))
					explainedP, err := transformer.ExplanationDeclaration(p)
					astTestMust(t, err)
					guard := tableRelationTheoryRewriter(t, f.ast)
					execution, termination, err := CompileRelationExplanationRulesWithGuard(f.relations, transformed, guard)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) })
					astTestMust(t, transformer.Close())
					astTestMust(t, transformed.Close())
					astTestMust(t, f.set.Close())
					context, err := NewExecutionContextWithRelations(f.relations, nil)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, context.Close()) })
					history := f.proof("fact")
					for repeat := 0; repeat < 2; repeat++ {
						astTestMust(t, context.SetTimeLimit(1000))
						complete, err := execution.Perform(context)
						astTestMust(t, err)
						if !complete {
							t.Fatal("whole-proof growth prevented the finite fixpoint")
						}
						for _, predicate := range []ast.ASTHandle{p, explainedP} {
							marked, err := f.relations.IsSaturated(predicate)
							astTestMust(t, err)
							if !marked {
								t.Fatal("completed execution did not mark output", predicate)
							}
						}
						complete, err = termination.Perform(context)
						astTestMust(t, err)
						if !complete {
							t.Fatal("whole-relation saturation did not complete")
						}
						output, err := f.relations.GetFiniteRelation(p)
						astTestMust(t, err)
						rows, err := output.Facts()
						astTestMust(t, err)
						if seeded {
							instructionWantFacts(t, rows, Fact{})
						} else {
							instructionWantFacts(t, rows)
						}
						product, err := f.relations.GetRelationExplanation(explainedP)
						astTestMust(t, err)
						proof, found, err := product.Explanation(nil)
						astTestMust(t, err)
						if found != seeded {
							t.Fatal("empty fact proof invented a tuple, or seed was lost", found, seeded)
						}
						if seeded {
							if !mutual {
								requireRelationUnionProof(t, f.ast, proof, history, f.proof("whole-self", history))
							} else {
								names := relationExplanationProofNames(t, f.ast, proof, map[string]bool{"fact": true, "e_union": true, "whole-to-P": true, "whole-to-Q": true})
								if names["fact"] == 0 || names["whole-to-P"] == 0 {
									t.Fatal("mutual proof lost seed or source rule provenance", names)
								}
							}
							history = proof
							astTestPin(t, f.ast, history)
						}
						for _, predicate := range []ast.ASTHandle{p, explainedP} {
							marked, err := f.relations.IsSaturated(predicate)
							astTestMust(t, err)
							if !marked {
								t.Fatal("termination changed completed output mark", predicate)
							}
						}
						astTestMust(t, context.Reset())
					}
				})
			}
		}
	}
}

func TestCompileRelationExplanationProgramSpecializesEveryRecursiveTail(t *testing.T) {
	f := newExplanationNegativeFixture(t, "hashtable")
	p, q := f.declare("whole-nonlinear-P", 2), f.declare("whole-nonlinear-Q", 0)
	p00 := f.own(f.ast.MkApp(p, []ast.ASTHandle{f.falsehood, f.falsehood}))
	p01 := f.own(f.ast.MkApp(p, []ast.ASTHandle{f.falsehood, f.truth}))
	p11 := f.own(f.ast.MkApp(p, []ast.ASTHandle{f.truth, f.truth}))
	qApp := f.own(f.ast.MkApp(q, nil))
	// Only the second P occurrence can combine newly produced P11 with old
	// P00; specializing just the first recursive occurrence loses P01.
	f.add(p01, []ast.ASTHandle{p00, p11}, nil, "whole-combine")
	f.add(p11, []ast.ASTHandle{qApp}, nil, "whole-produce")
	f.add(p00, nil, nil, "whole-base")
	f.add(qApp, []ast.ASTHandle{p11}, nil, "whole-back")
	f.add(qApp, nil, nil, "whole-trigger")
	astTestMust(t, f.set.SetOutputPredicate(p))
	transformer, err := NewExplanationTransformer(f.ast, f.rules)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, transformer.Close()) })
	transformed, err := transformer.TransformRulesOnly(f.set)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, transformed.Close()) })
	astTestMust(t, transformer.TransferRelationFacts(f.set, transformed, f.relations, []ast.ASTHandle{p, q}))
	explainedP, err := transformer.ExplanationDeclaration(p)
	astTestMust(t, err)
	guard := tableRelationTheoryRewriter(t, f.ast)
	execution, termination, err := CompileRelationExplanationRulesWithGuard(f.relations, transformed, guard)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) })
	astTestMust(t, transformer.Close())
	astTestMust(t, transformed.Close())
	astTestMust(t, f.set.Close())
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	for repeat := 0; repeat < 2; repeat++ {
		astTestMust(t, context.SetTimeLimit(1000))
		complete, err := execution.Perform(context)
		astTestMust(t, err)
		if !complete {
			t.Fatal("nonlinear whole-relation SCC did not reach a finite fixpoint")
		}
		complete, err = termination.Perform(context)
		astTestMust(t, err)
		if !complete {
			t.Fatal("nonlinear whole-relation termination incomplete")
		}
		output, err := f.relations.GetFiniteRelation(p)
		astTestMust(t, err)
		rows, err := output.Facts()
		astTestMust(t, err)
		if len(rows) != 3 {
			t.Fatal("nonlinear finite output has unexpected cardinality", rows)
		}
		for _, row := range []Fact{{0, 0}, {0, 1}, {1, 1}} {
			found, err := output.Contains(row)
			astTestMust(t, err)
			if !found {
				t.Fatal("recursive occurrence specialization lost a finite tuple", row, rows)
			}
		}
		product, err := f.relations.GetRelationExplanation(explainedP)
		astTestMust(t, err)
		var whole ast.ASTHandle
		for _, values := range [][]ast.ASTHandle{{f.falsehood, f.falsehood}, {f.falsehood, f.truth}, {f.truth, f.truth}} {
			proof, found, err := product.Explanation(values)
			astTestMust(t, err)
			if !found || (!whole.IsNull() && proof != whole) {
				t.Fatal("nonlinear tuples lost their shared whole-relation proof", found, proof, whole)
			}
			whole = proof
		}
		_, found, err := product.Explanation([]ast.ASTHandle{f.truth, f.falsehood})
		astTestMust(t, err)
		if found {
			t.Fatal("whole proof invented the unreachable P10 tuple")
		}
		names := relationExplanationProofNames(t, f.ast, whole, map[string]bool{"fact": true, "e_union": true, "whole-combine": true, "whole-produce": true, "whole-base": true, "whole-back": true, "whole-trigger": true})
		for _, name := range []string{"whole-combine", "whole-produce", "whole-base", "whole-trigger", "e_union"} {
			if names[name] == 0 {
				t.Fatal("nonlinear whole proof lost a derivation", name, names)
			}
		}
		astTestMust(t, context.Reset())
	}
}
