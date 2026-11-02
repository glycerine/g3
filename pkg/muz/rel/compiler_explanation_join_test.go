package rel

// Source: dl_compiler.cpp compile_rule_evaluation_run's two-tail join and
// dl_mk_explanations.cpp get_e_rule's ordered child derivations.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestCompileTwoTailExplanationRulesAgainstTupleOracle(t *testing.T) {
	for _, mode := range []string{"shared-guard", "cartesian", "right-repeat", "self-join", "empty-right", "nullary-left", "nullary-both"} {
		t.Run(mode, func(t *testing.T) {
			f := newTableRelationFixture(t)
			rm, err := base.NewRuleManager(f.ast)
			astTestMust(t, err)
			defer func() { astTestMust(t, rm.Close()) }()
			set, err := base.NewRuleSet(rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, set.Close()) }()
			own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
			declare := func(name string, arity int) ast.ASTHandle {
				symbol, e := f.ast.Symbols.Intern(name)
				astTestMust(t, e)
				sorts := make([]ast.ASTHandle, arity)
				for index := range sorts {
					sorts[index] = f.boolean
				}
				declaration, e := f.ast.MkFuncDecl(symbol, sorts, f.boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, e)
				astTestMust(t, rm.RegisterPredicate(declaration))
				return declaration
			}
			leftArity, rightArity, headArity := 3, 3, 3
			if mode == "nullary-left" || mode == "nullary-both" {
				leftArity = 0
			}
			if mode == "nullary-both" {
				rightArity, headArity = 0, 0
			}
			p, r, q := declare("join-left", leftArity), declare("join-right", rightArity), declare("join-output", headArity)
			if mode == "self-join" {
				r = p
			}
			x, key, y := own(f.ast.MkVar(0, f.boolean)), own(f.ast.MkVar(1, f.boolean)), own(f.ast.MkVar(2, f.boolean))
			leftArgs, rightArgs, headArgs := []ast.ASTHandle{x, key, key}, []ast.ASTHandle{key, y, f.truth}, []ast.ASTHandle{y, x, y}
			var guards []ast.ASTHandle
			switch mode {
			case "shared-guard":
				guards = []ast.ASTHandle{own(f.ast.MkEq(y, f.truth))}
			case "cartesian":
				leftArgs, rightArgs, headArgs = []ast.ASTHandle{x, x, x}, []ast.ASTHandle{y, y, y}, []ast.ASTHandle{x, y, x}
			case "right-repeat":
				leftArgs, rightArgs, headArgs = []ast.ASTHandle{f.truth, key, x}, []ast.ASTHandle{key, y, y}, []ast.ASTHandle{x, y, f.truth}
			case "nullary-left":
				leftArgs, rightArgs, headArgs = nil, []ast.ASTHandle{x, key, y}, []ast.ASTHandle{x, y, x}
			case "nullary-both":
				leftArgs, rightArgs, headArgs = nil, nil, nil
			}
			tailLeft, tailRight, head := own(f.ast.MkApp(p, leftArgs)), own(f.ast.MkApp(r, rightArgs)), own(f.ast.MkApp(q, headArgs))
			rule, err := rm.NewRule(head, append([]ast.ASTHandle{tailLeft, tailRight}, guards...), nil, "join-step")
			astTestMust(t, err)
			astTestMust(t, set.Add(rule))
			astTestMust(t, rule.Close())
			astTestMust(t, set.SetOutputPredicate(q))
			fill := func(predicate ast.ASTHandle, arity int, empty bool) *HashTable {
				table, e := f.relations.GetRelation(predicate)
				astTestMust(t, e)
				if !empty {
					if arity == 0 {
						_, e = table.Add(Fact{})
						astTestMust(t, e)
					} else {
						for a := 0; a < 2; a++ {
							for b := 0; b < 2; b++ {
								for c := 0; c < 2; c++ {
									_, e = table.Add(Fact{Element(a), Element(b), Element(c)})
									astTestMust(t, e)
								}
							}
						}
					}
				}
				return table
			}
			left := fill(p, leftArity, false)
			right := left
			if r != p {
				right = fill(r, rightArity, mode == "empty-right")
			}
			transformer, err := NewExplanationTransformer(f.ast, rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformer.Close()) }()
			transformed, err := transformer.TransformRulesOnly(set)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformed.Close()) }()
			predicates := []ast.ASTHandle{p, q}
			if r != p {
				predicates = append(predicates, r)
			}
			astTestMust(t, transformer.TransferFiniteFacts(set, transformed, f.relations, predicates))
			proof := func(name string) ast.ASTHandle {
				symbol, e := f.ast.Symbols.Intern(name)
				astTestMust(t, e)
				return own(f.ast.MkDatalogRule(symbol, nil))
			}
			leftProof, rightProof := proof("left-origin"), proof("right-origin")
			install := func(predicate ast.ASTHandle, table *HashTable, arity int, explanation ast.ASTHandle) {
				declaration, e := transformer.ExplanationDeclaration(predicate)
				astTestMust(t, e)
				sorts := make([]ast.ASTHandle, arity)
				for index := range sorts {
					sorts[index] = f.boolean
				}
				product, e := NewFiniteExplanationProduct(f.relations, sorts)
				astTestMust(t, e)
				rows, e := table.Facts()
				astTestMust(t, e)
				for _, row := range rows {
					values := make([]ast.ASTHandle, len(row))
					for column, value := range row {
						values[column] = f.falsehood
						if value == 1 {
							values[column] = f.truth
						}
					}
					astTestMust(t, product.AddFact(values, explanation))
				}
				astTestMust(t, f.relations.StoreExplanationRelation(declaration, product))
			}
			install(p, left, leftArity, leftProof)
			if r != p {
				install(r, right, rightArity, rightProof)
			} else {
				rightProof = leftProof
			}
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
			symbol, err := f.ast.Symbols.Intern("join-step")
			astTestMust(t, err)
			wantProof := own(f.ast.MkDatalogRule(symbol, []ast.ASTHandle{leftProof, rightProof}))
			for repeat := 0; repeat < 2; repeat++ {
				for _, block := range blocks {
					complete, e := block.Perform(context)
					astTestMust(t, e)
					if !complete {
						t.Fatal("two-tail explanation program incomplete")
					}
					astTestMust(t, context.Reset())
				}
				output, e := f.relations.GetRelation(q)
				astTestMust(t, e)
				product, e := f.relations.GetExplanationRelation(explained)
				astTestMust(t, e)
				verify := func(row Fact, want bool) {
					found, e := output.Contains(row)
					astTestMust(t, e)
					values := make([]ast.ASTHandle, len(row))
					for column, value := range row {
						values[column] = f.falsehood
						if value == 1 {
							values[column] = f.truth
						}
					}
					got, explained, e := product.Explanation(values)
					astTestMust(t, e)
					if found != want || explained != want || (want && got != wantProof) {
						t.Fatal("two-tail tuple or ordered child explanations differ", row, found, explained, got, want, wantProof)
					}
				}
				if mode == "nullary-both" {
					verify(Fact{}, true)
				} else {
					for a := 0; a < 2; a++ {
						for b := 0; b < 2; b++ {
							for c := 0; c < 2; c++ {
								want := a == c
								switch mode {
								case "shared-guard":
									want = a == 1 && c == 1
								case "right-repeat":
									want = c == 1
								case "empty-right":
									want = false
								}
								verify(Fact{Element(a), Element(b), Element(c)}, want)
							}
						}
					}
				}
				for _, input := range []*HashTable{left, right} {
					signature, e := input.Signature()
					astTestMust(t, e)
					want := 8
					if signature.Len() == 0 {
						want = 1
					}
					if mode == "empty-right" && input == right {
						want = 0
					}
					if n, e := input.Len(); e != nil || n != want {
						t.Fatal("two-tail compiler changed source-owned input", n, want, e)
					}
				}
			}
		})
	}
}
