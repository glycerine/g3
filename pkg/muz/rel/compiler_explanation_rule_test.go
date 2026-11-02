package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestCompileTransformedExplanationRuleBuildsHeadAndOutputBridge(t *testing.T) {
	for _, mode := range []string{"permuted", "nullary", "empty"} {
		t.Run(mode, func(t *testing.T) {
			f := newTableRelationFixture(t)
			rm, err := base.NewRuleManager(f.ast)
			astTestMust(t, err)
			defer func() { astTestMust(t, rm.Close()) }()
			set, err := base.NewRuleSet(rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, set.Close()) }()
			declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				sym, e := f.ast.Symbols.Intern(name)
				astTestMust(t, e)
				d, e := f.ast.MkFuncDecl(sym, domain, f.boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, e)
				astTestMust(t, rm.RegisterPredicate(d))
				return d
			}
			p := declare("compiled-explanation-P", []ast.ASTHandle{f.boolean, f.boolean, f.boolean})
			var qSorts []ast.ASTHandle
			if mode == "permuted" {
				qSorts = []ast.ASTHandle{f.boolean, f.boolean}
			}
			q := declare("compiled-explanation-Q", qSorts)
			own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
			x, y, z := own(f.ast.MkVar(0, f.boolean)), own(f.ast.MkVar(1, f.boolean)), own(f.ast.MkVar(2, f.boolean))
			tail := own(f.ast.MkApp(p, []ast.ASTHandle{x, y, z}))
			var headArgs []ast.ASTHandle
			if mode == "permuted" {
				headArgs = []ast.ASTHandle{y, x}
			}
			head := own(f.ast.MkApp(q, headArgs))
			guard := own(f.ast.MkEq(z, f.truth))
			rule, err := rm.NewRule(head, []ast.ASTHandle{tail, guard}, nil, "compiled-step")
			astTestMust(t, err)
			astTestMust(t, set.Add(rule))
			astTestMust(t, rule.Close())
			astTestMust(t, set.SetOutputPredicate(q))
			original, err := f.relations.GetRelation(p)
			astTestMust(t, err)
			if mode != "empty" {
				for _, row := range []Fact{{1, 0, 1}, {0, 1, 1}, {1, 1, 0}} {
					_, err = original.Add(row)
					astTestMust(t, err)
				}
			}
			transformer, err := NewExplanationTransformer(f.ast, rm)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformer.Close()) }()
			transformed, err := transformer.TransformRulesOnly(set)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformed.Close()) }()
			astTestMust(t, transformer.TransferFiniteFacts(set, transformed, f.relations, []ast.ASTHandle{p, q}))
			explainedQ, err := transformer.ExplanationDeclaration(q)
			astTestMust(t, err)
			var blocks []*InstructionBlock
			defer func() {
				for _, block := range blocks {
					astTestMust(t, block.Close())
				}
			}()
			count, err := transformed.Len()
			astTestMust(t, err)
			if count != 2 {
				t.Fatal("expected explained rule and finite bridge", count)
			}
			theory := tableRelationTheoryRewriter(t, f.ast)
			for index := 0; index < count; index++ {
				generated, e := transformed.Rule(index)
				astTestMust(t, e)
				block, e := CompileExplanationRuleWithGuard(f.relations, generated, theory)
				astTestMust(t, e)
				astTestMust(t, generated.Close())
				blocks = append(blocks, block)
			}
			// The compiled program owns its rule/condition references. Transformer and
			// input rule-set disposal cannot retire manager facts or pending bytecode.
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
						t.Fatal("explanation rule program incomplete")
					}
					astTestMust(t, context.Reset())
				}
				finite, e := f.relations.GetRelation(q)
				astTestMust(t, e)
				if mode == "permuted" {
					for a := 0; a < 2; a++ {
						for b := 0; b < 2; b++ {
							found, e := finite.Contains(Fact{Element(a), Element(b)})
							astTestMust(t, e)
							want := a != b
							if found != want {
								t.Fatal("compiled permuted head", a, b, found, want)
							}
						}
					}
				} else {
					found, e := finite.Contains(Fact{})
					astTestMust(t, e)
					if found != (mode != "empty") {
						t.Fatal("compiled nullary head", found, mode)
					}
				}
				if mode != "empty" {
					product, e := f.relations.GetExplanationRelation(explainedQ)
					astTestMust(t, e)
					var values []ast.ASTHandle
					if mode == "permuted" {
						values = []ast.ASTHandle{f.falsehood, f.truth}
					}
					got, found, e := product.Explanation(values)
					astTestMust(t, e)
					if !found {
						t.Fatal("compiled head lost explanation")
					}
					factSymbol, e := f.ast.Symbols.Intern("fact")
					astTestMust(t, e)
					fact := own(f.ast.MkDatalogRule(factSymbol, nil))
					stepSymbol, e := f.ast.Symbols.Intern("compiled-step")
					astTestMust(t, e)
					want := own(f.ast.MkDatalogRule(stepSymbol, []ast.ASTHandle{fact}))
					if got != want {
						t.Fatal("compiled explanation differs from original derivation", got, want)
					}
				}
				if n, e := original.Len(); e != nil || n != map[bool]int{true: 0, false: 3}[mode == "empty"] {
					t.Fatal("compiled program changed source table", n, e)
				}
			}
		})
	}
}

func TestCompileExplanationRuleRejectsUnsupportedShapesBeforePublication(t *testing.T) {
	f := newTableRelationFixture(t)
	rm, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rm.Close()) }()
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		d, e := f.ast.MkFuncDecl(sym, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, rm.RegisterPredicate(d))
		return d
	}
	p := declare("unsupported-explanation-P", []ast.ASTHandle{f.boolean})
	x, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, x)
	pX, err := f.ast.MkApp(p, []ast.ASTHandle{x})
	astTestMust(t, err)
	astTestPin(t, f.ast, pX)
	y, err := f.ast.MkVar(1, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, y)
	unboundCondition, err := f.ast.MkEq(y, f.truth)
	astTestMust(t, err)
	astTestPin(t, f.ast, unboundCondition)
	transformer, err := NewExplanationTransformer(f.ast, rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, transformer.Close()) }()
	for _, spec := range []struct {
		head     ast.ASTHandle
		tails    []ast.ASTHandle
		negative []bool
	}{{pX, []ast.ASTHandle{pX}, []bool{true}}, {pX, []ast.ASTHandle{pX, unboundCondition}, nil}, {pX, []ast.ASTHandle{pX, pX}, nil}} {
		rule, e := rm.NewRule(spec.head, spec.tails, spec.negative, "unsupported")
		astTestMust(t, e)
		generated, e := transformer.ExplanationRule(rule)
		astTestMust(t, e)
		astTestMust(t, rule.Close())
		block, e := CompileExplanationRuleWithGuard(f.relations, generated, tableRelationTheoryRewriter(t, f.ast))
		if block != nil || !errors.Is(e, ErrCompilerUnsupported) {
			t.Fatal("unsupported explanation shape published a program", block, e)
		}
		astTestMust(t, generated.Close())
	}
}
