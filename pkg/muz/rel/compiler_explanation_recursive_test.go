package rel

// Source: dl_compiler.cpp compile_preds_init/compile_rule_evaluation/compile_loop.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestCompileExplanationProgramSpecializesEveryRecursiveTail(t *testing.T) {
	f := newTableRelationFixture(t)
	rm, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		symbol, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		declaration, e := f.ast.MkFuncDecl(symbol, sorts, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, rm.RegisterPredicate(declaration))
		return declaration
	}
	p, q := declare("nonlinear-P", []ast.ASTHandle{f.boolean, f.boolean}), declare("nonlinear-Q", nil)
	p00 := own(f.ast.MkApp(p, []ast.ASTHandle{f.falsehood, f.falsehood}))
	p01 := own(f.ast.MkApp(p, []ast.ASTHandle{f.falsehood, f.truth}))
	p11 := own(f.ast.MkApp(p, []ast.ASTHandle{f.truth, f.truth}))
	qApp := own(f.ast.MkApp(q, nil))
	// The nonlinear rule runs before its P11 producer. Once P11 is new, the
	// first recursive occurrence matches no old-delta row: only substituting
	// the second occurrence can combine new P11 with the already-known P00.
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
		name  string
	}{{p01, []ast.ASTHandle{p00, p11}, "combine"}, {p11, []ast.ASTHandle{qApp}, "produce"}, {p00, nil, "base"}, {qApp, []ast.ASTHandle{p11}, "back"}, {qApp, nil, "trigger"}} {
		rule, e := rm.NewRule(spec.head, spec.tails, nil, spec.name)
		astTestMust(t, e)
		astTestMust(t, set.Add(rule))
		astTestMust(t, rule.Close())
	}
	astTestMust(t, set.SetOutputPredicate(p))
	transformer, err := NewExplanationTransformer(f.ast, rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, transformer.Close()) }()
	transformed, err := transformer.TransformRulesOnly(set)
	astTestMust(t, err)
	defer func() { astTestMust(t, transformed.Close()) }()
	astTestMust(t, transformer.TransferFiniteFacts(set, transformed, f.relations, []ast.ASTHandle{p, q}))
	explainedP, err := transformer.ExplanationDeclaration(p)
	astTestMust(t, err)
	explainedQ, err := transformer.ExplanationDeclaration(q)
	astTestMust(t, err)
	execution, termination, err := CompileExplanationRulesWithGuard(f.relations, transformed, tableRelationTheoryRewriter(t, f.ast))
	astTestMust(t, err)
	defer func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) }()
	astTestMust(t, transformer.Close())
	astTestMust(t, transformed.Close())
	astTestMust(t, set.Close())
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	proof := func(name string, arguments ...ast.ASTHandle) ast.ASTHandle {
		symbol, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		return own(f.ast.MkDatalogRule(symbol, arguments))
	}
	baseProof, trigger := proof("base"), proof("trigger")
	produced := proof("produce", trigger)
	combined := proof("combine", baseProof, produced)
	for repeat := 0; repeat < 2; repeat++ {
		astTestMust(t, context.SetTimeLimit(1000))
		complete, e := execution.Perform(context)
		astTestMust(t, e)
		if !complete {
			t.Fatal("nonlinear explanation SCC failed to reach a fixpoint")
		}
		complete, e = termination.Perform(context)
		astTestMust(t, e)
		if !complete {
			t.Fatal("nonlinear explanation termination incomplete")
		}
		output, e := f.relations.GetRelation(p)
		astTestMust(t, e)
		product, e := f.relations.GetExplanationRelation(explainedP)
		astTestMust(t, e)
		for a := 0; a < 2; a++ {
			for b := 0; b < 2; b++ {
				found, e := output.Contains(Fact{Element(a), Element(b)})
				astTestMust(t, e)
				want := a == 0 || b == 1
				values := []ast.ASTHandle{f.falsehood, f.falsehood}
				if a == 1 {
					values[0] = f.truth
				}
				if b == 1 {
					values[1] = f.truth
				}
				got, explained, e := product.Explanation(values)
				astTestMust(t, e)
				wantProof := baseProof
				if a == 1 {
					wantProof = produced
				} else if b == 1 {
					wantProof = combined
				}
				if found != want || explained != want || (want && got != wantProof) {
					t.Fatal("recursive tail specialization lost tuple or ordered proof", a, b, found, explained, got, want, wantProof)
				}
			}
		}
		qProduct, e := f.relations.GetExplanationRelation(explainedQ)
		astTestMust(t, e)
		got, found, e := qProduct.Explanation(nil)
		astTestMust(t, e)
		if !found || got != trigger {
			t.Fatal("recursive backedge replaced Q's first proof", got, found, trigger)
		}
		astTestMust(t, context.Reset())
	}
}
