package rel

// Source: dl_compiler.cpp prepare_tail/make_duplicate_column/head assembly.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestCompileExplanationConstantsAndRepeatedColumnsAgainstTupleOracle(t *testing.T) {
	for _, mode := range []string{"repeated-tail-head", "permuted-repeat", "absent-constant"} {
		t.Run(mode, func(t *testing.T) {
			f := newTableRelationFixture(t)
			rm, err := base.NewRuleManager(f.ast)
			astTestMust(t, err)
			defer func() { astTestMust(t, rm.Close()) }()
			set, err := base.NewRuleSet(rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, set.Close()) }()
			declare := func(name string) ast.ASTHandle {
				symbol, e := f.ast.Symbols.Intern(name)
				astTestMust(t, e)
				d, e := f.ast.MkFuncDecl(symbol, []ast.ASTHandle{f.boolean, f.boolean, f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, e)
				astTestMust(t, rm.RegisterPredicate(d))
				return d
			}
			p, q := declare("column-input"), declare("column-output")
			own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
			x, y := own(f.ast.MkVar(0, f.boolean)), own(f.ast.MkVar(1, f.boolean))
			constant := f.truth
			if mode == "absent-constant" {
				constant = f.falsehood
			}
			tailArgs, headArgs := []ast.ASTHandle{constant, x, x}, []ast.ASTHandle{x, x, x}
			var guards []ast.ASTHandle
			if mode == "permuted-repeat" {
				tailArgs = []ast.ASTHandle{constant, y, x}
				headArgs = []ast.ASTHandle{x, y, x}
				guards = []ast.ASTHandle{own(f.ast.MkEq(y, f.truth))}
			}
			tail, head := own(f.ast.MkApp(p, tailArgs)), own(f.ast.MkApp(q, headArgs))
			rule, err := rm.NewRule(head, append([]ast.ASTHandle{tail}, guards...), nil, "column-step")
			astTestMust(t, err)
			astTestMust(t, set.Add(rule))
			astTestMust(t, rule.Close())
			astTestMust(t, set.SetOutputPredicate(q))
			original, err := f.relations.GetRelation(p)
			astTestMust(t, err)
			inserted := 0
			for a := 0; a < 2; a++ {
				for b := 0; b < 2; b++ {
					for c := 0; c < 2; c++ {
						if mode == "absent-constant" && a == 0 {
							continue
						}
						_, err = original.Add(Fact{Element(a), Element(b), Element(c)})
						astTestMust(t, err)
						inserted++
					}
				}
			}
			transformer, err := NewExplanationTransformer(f.ast, rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformer.Close()) }()
			transformed, err := transformer.TransformRulesOnly(set)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformed.Close()) }()
			astTestMust(t, transformer.TransferFiniteFacts(set, transformed, f.relations, []ast.ASTHandle{p, q}))
			explained, err := transformer.ExplanationDeclaration(q)
			astTestMust(t, err)
			theory := tableRelationTheoryRewriter(t, f.ast)
			var blocks []*InstructionBlock
			defer func() {
				for _, block := range blocks {
					astTestMust(t, block.Close())
				}
			}()
			count, err := transformed.Len()
			astTestMust(t, err)
			for index := 0; index < count; index++ {
				generated, e := transformed.Rule(index)
				astTestMust(t, e)
				block, e := CompileExplanationRuleWithGuard(f.relations, generated, theory)
				astTestMust(t, e)
				astTestMust(t, generated.Close())
				blocks = append(blocks, block)
			}
			astTestMust(t, transformer.Close())
			astTestMust(t, transformed.Close())
			astTestMust(t, set.Close())
			context, err := NewExecutionContextWithRelations(f.relations, nil)
			astTestMust(t, err)
			defer func() { astTestMust(t, context.Close()) }()
			for repeat := 0; repeat < 2; repeat++ {
				for _, block := range blocks {
					complete, e := block.Perform(context)
					astTestMust(t, e)
					if !complete {
						t.Fatal("constant/repeated-column program incomplete")
					}
					astTestMust(t, context.Reset())
				}
				output, e := f.relations.GetRelation(q)
				astTestMust(t, e)
				for a := 0; a < 2; a++ {
					for b := 0; b < 2; b++ {
						for c := 0; c < 2; c++ {
							found, e := output.Contains(Fact{Element(a), Element(b), Element(c)})
							astTestMust(t, e)
							want := a == b && b == c
							if mode == "permuted-repeat" {
								want = b == 1 && a == c
							}
							if mode == "absent-constant" {
								want = false
							}
							if found != want {
								t.Fatal("compiled tuple differs from original rule", mode, a, b, c, found, want)
							}
						}
					}
				}
				product, e := f.relations.GetExplanationRelation(explained)
				astTestMust(t, e)
				if mode != "absent-constant" {
					value := f.falsehood
					middle := value
					if mode == "permuted-repeat" {
						middle = f.truth
					}
					got, found, e := product.Explanation([]ast.ASTHandle{value, middle, value})
					astTestMust(t, e)
					factName, e := f.ast.Symbols.Intern("fact")
					astTestMust(t, e)
					fact := own(f.ast.MkDatalogRule(factName, nil))
					stepName, e := f.ast.Symbols.Intern("column-step")
					astTestMust(t, e)
					want := own(f.ast.MkDatalogRule(stepName, []ast.ASTHandle{fact}))
					if !found || got != want {
						t.Fatal("column duplication corrupted explanation suffix", got, found, want)
					}
				}
				if n, e := original.Len(); e != nil || n != inserted {
					t.Fatal("compiler filtering changed source-owned rows", n, inserted, e)
				}
			}
		})
	}
}
