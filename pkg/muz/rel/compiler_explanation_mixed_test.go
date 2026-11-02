package rel

// Source: dl_finite_product_relation.cpp converting_join_fn and
// dl_compiler.cpp recursive delta scheduling for finite magic predicates.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestCompileExplanationProgramJoinsFiniteRecursiveMagicPredicate(t *testing.T) {
	for _, representation := range []string{"hashtable", "sparse"} {
		t.Run(representation, func(t *testing.T) {
			f := newTableRelationFixture(t)
			astTestMust(t, f.relations.SetDefaultFiniteTable(representation))
			rm, err := base.NewRuleManager(f.ast)
			astTestMust(t, err)
			defer func() { astTestMust(t, rm.Close()) }()
			set, err := base.NewRuleSet(rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, set.Close()) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				astTestMust(t, err)
				astTestPin(t, f.ast, h)
				return h
			}
			ruleSort := own(f.ast.MkDatalogRuleSort())
			declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				symbol, e := f.ast.Symbols.Intern(name)
				astTestMust(t, e)
				declaration, e := f.ast.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, e)
				astTestMust(t, rm.RegisterPredicate(declaration))
				return declaration
			}
			p := declare("mixed-proof-input", []ast.ASTHandle{f.bits, ruleSort})
			q := declare("mixed-proof-output", []ast.ASTHandle{f.bits, ruleSort})
			magic := declare("mixed-finite-magic", []ast.ASTHandle{f.bits})
			values := make([]ast.ASTHandle, 4)
			for index := range values {
				values[index] = own(f.ast.BitVectorNumeralRat(new(big.Rat).SetUint64(uint64(index)), 2))
			}
			x := own(f.ast.MkVar(0, f.bits))
			explanation := own(f.ast.MkVar(1, ruleSort))
			pApp := own(f.ast.MkApp(p, []ast.ASTHandle{x, explanation}))
			qApp := own(f.ast.MkApp(q, []ast.ASTHandle{x, explanation}))
			magicApp := own(f.ast.MkApp(magic, []ast.ASTHandle{x}))
			magicZero := own(f.ast.MkApp(magic, []ast.ASTHandle{values[0]}))
			magicOne := own(f.ast.MkApp(magic, []ast.ASTHandle{values[1]}))
			// The consumer appears first; its finite prerequisite reaches a
			// fixpoint before the mixed join copies the existing explanation.
			for _, spec := range []struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
				name  string
			}{{qApp, []ast.ASTHandle{pApp, magicApp}, "mixed-copy"}, {magicOne, []ast.ASTHandle{magicZero}, "magic-forward"}, {magicZero, []ast.ASTHandle{magicOne}, "magic-back"}} {
				rule, e := rm.NewRule(spec.head, spec.tails, nil, spec.name)
				astTestMust(t, e)
				astTestMust(t, set.Add(rule))
				astTestMust(t, rule.Close())
			}
			astTestMust(t, set.SetOutputPredicate(q))
			input, err := f.relations.GetExplanationRelation(p)
			astTestMust(t, err)
			proofs := make([]ast.ASTHandle, 4)
			for index, value := range values {
				symbol, e := f.ast.Symbols.Intern("mixed-origin-" + string(rune('0'+index)))
				astTestMust(t, e)
				proofs[index], e = f.ast.MkDatalogRule(symbol, nil)
				astTestMust(t, e)
				root, e := f.ast.NewRef(proofs[index])
				astTestMust(t, e)
				astTestMust(t, input.AddFact([]ast.ASTHandle{value}, proofs[index]))
				// The source product alone owns the proof until execution.
				astTestMust(t, f.ast.DelRef(&root))
			}
			seed, err := f.relations.GetFiniteRelation(magic)
			astTestMust(t, err)
			if representation == "sparse" {
				if _, ok := seed.(*SparseTable); !ok {
					t.Fatalf("selected magic table is %T", seed)
				}
			}
			_, err = seed.Add(Fact{0})
			astTestMust(t, err)
			execution, termination, err := CompileExplanationRulesWithGuard(f.relations, set, nil)
			astTestMust(t, err)
			defer func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) }()
			astTestMust(t, set.Close())
			context, err := NewExecutionContextWithRelations(f.relations, nil)
			astTestMust(t, err)
			defer func() { astTestMust(t, context.Close()) }()
			for repeat := 0; repeat < 2; repeat++ {
				astTestMust(t, context.SetTimeLimit(1000))
				complete, e := execution.Perform(context)
				astTestMust(t, e)
				if !complete {
					t.Fatal("mixed finite/explanation program did not finish")
				}
				complete, e = termination.Perform(context)
				astTestMust(t, e)
				if !complete {
					t.Fatal("mixed finite/explanation termination did not finish")
				}
				output, e := f.relations.GetExplanationRelation(q)
				astTestMust(t, e)
				finite, e := f.relations.GetFiniteRelation(magic)
				astTestMust(t, e)
				for index, value := range values {
					got, found, e := output.Explanation([]ast.ASTHandle{value})
					astTestMust(t, e)
					want := index < 2
					if found != want || (found && got != proofs[index]) {
						t.Fatal("mixed join lost finite selection or copied proof", index, got, found, want, proofs[index])
					}
					found, e = finite.Contains(Fact{Element(index)})
					astTestMust(t, e)
					if found != want {
						t.Fatal("finite magic SCC differs from independent tuple oracle", index, found, want)
					}
					got, found, e = input.Explanation([]ast.ASTHandle{value})
					astTestMust(t, e)
					if !found || got != proofs[index] {
						t.Fatal("mixed join changed its manager-owned source proof", index, got, found)
					}
				}
				currentInput, e := f.relations.GetExplanationRelation(p)
				astTestMust(t, e)
				if currentInput != input {
					t.Fatal("mixed join replaced its source relation owner")
				}
				count, e := input.Len()
				astTestMust(t, e)
				if count != 4 {
					t.Fatal("mixed join removed source rows", count)
				}
				astTestMust(t, context.Reset())
			}
		})
	}
}
