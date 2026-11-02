package rel

// Source: dl_compiler.cpp make_full_relation/ground head assembly and
// dl_mk_explanations.cpp get_e_rule's zero-child explanation record.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestCompileGroundExplanationRulesAgainstTupleOracle(t *testing.T) {
	for _, mode := range []string{"constants", "nullary", "true-guard", "false-guard", "unbound-shared"} {
		t.Run(mode, func(t *testing.T) {
			f := newTableRelationFixture(t)
			rm, err := base.NewRuleManager(f.ast)
			astTestMust(t, err)
			defer func() { astTestMust(t, rm.Close()) }()
			set, err := base.NewRuleSet(rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, set.Close()) }()
			own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
			var sorts, arguments, conditions []ast.ASTHandle
			if mode != "nullary" {
				sorts = []ast.ASTHandle{f.boolean, f.boolean, f.boolean}
				arguments = []ast.ASTHandle{f.truth, f.falsehood, f.truth}
			}
			switch mode {
			case "true-guard":
				conditions = []ast.ASTHandle{f.truth}
			case "false-guard":
				conditions = []ast.ASTHandle{f.falsehood}
			case "unbound-shared":
				x, y := own(f.ast.MkVar(0, f.boolean)), own(f.ast.MkVar(1, f.boolean))
				arguments = []ast.ASTHandle{x, y, x}
				conditions = []ast.ASTHandle{own(f.ast.MkEq(y, f.truth))}
			}
			symbol, err := f.ast.Symbols.Intern("ground-explanation-output")
			astTestMust(t, err)
			q, err := f.ast.MkFuncDecl(symbol, sorts, f.boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			astTestMust(t, rm.RegisterPredicate(q))
			head := own(f.ast.MkApp(q, arguments))
			rule, err := rm.NewRule(head, conditions, nil, "ground-step")
			astTestMust(t, err)
			astTestMust(t, set.Add(rule))
			astTestMust(t, rule.Close())
			astTestMust(t, set.SetOutputPredicate(q))
			transformer, err := NewExplanationTransformer(f.ast, rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformer.Close()) }()
			transformed, err := transformer.TransformRulesOnly(set)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformed.Close()) }()
			astTestMust(t, transformer.TransferFiniteFacts(set, transformed, f.relations, []ast.ASTHandle{q}))
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
			if count != 2 {
				t.Fatal("ground program needs its explained rule and finite bridge", count)
			}
			for index := 0; index < count; index++ {
				generated, e := transformed.Rule(index)
				astTestMust(t, e)
				block, e := CompileExplanationRuleWithGuard(f.relations, generated, theory)
				astTestMust(t, e)
				astTestMust(t, generated.Close())
				blocks = append(blocks, block)
			}
			// Bytecode owns the rules and condition references needed by the new
			// identity/full-column instructions after the transform is discarded.
			astTestMust(t, transformer.Close())
			astTestMust(t, transformed.Close())
			astTestMust(t, set.Close())
			context, err := NewExecutionContextWithRelations(f.relations, nil)
			astTestMust(t, err)
			defer func() { astTestMust(t, context.Close()) }()
			step, err := f.ast.Symbols.Intern("ground-step")
			astTestMust(t, err)
			wantExplanation := own(f.ast.MkDatalogRule(step, nil))
			for repeat := 0; repeat < 2; repeat++ {
				for _, block := range blocks {
					complete, e := block.Perform(context)
					astTestMust(t, e)
					if !complete {
						t.Fatal("ground explanation program incomplete")
					}
					astTestMust(t, context.Reset())
				}
				output, e := f.relations.GetRelation(q)
				astTestMust(t, e)
				product, e := f.relations.GetExplanationRelation(explained)
				astTestMust(t, e)
				verify := func(row Fact, values []ast.ASTHandle, want bool) {
					found, e := output.Contains(row)
					astTestMust(t, e)
					if found != want {
						t.Fatal("ground tuple differs from rule semantics", row, found, want)
					}
					got, found, e := product.Explanation(values)
					astTestMust(t, e)
					if found != want || (want && got != wantExplanation) {
						t.Fatal("ground rule explanation must have zero child derivations", row, got, found, wantExplanation, want)
					}
				}
				if mode == "nullary" {
					verify(Fact{}, nil, true)
				} else {
					for a := 0; a < 2; a++ {
						for b := 0; b < 2; b++ {
							for c := 0; c < 2; c++ {
								want := a == 1 && b == 0 && c == 1 && mode != "false-guard"
								if mode == "unbound-shared" {
									want = a == c && b == 1
								}
								values := []ast.ASTHandle{f.falsehood, f.falsehood, f.falsehood}
								for column, value := range []int{a, b, c} {
									if value == 1 {
										values[column] = f.truth
									}
								}
								verify(Fact{Element(a), Element(b), Element(c)}, values, want)
							}
						}
					}
				}
			}
		})
	}
}
