package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestCompileExplanationProgramOrdersDependenciesAndFirstProofs(t *testing.T) {
	f := newTableRelationFixture(t)
	rm, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
	declare := func(name string) ast.ASTHandle {
		symbol, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		declaration, e := f.ast.MkFuncDecl(symbol, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, rm.RegisterPredicate(declaration))
		return declaration
	}
	p, r, q := declare("program-seed"), declare("program-middle"), declare("program-output")
	x := own(f.ast.MkVar(0, f.boolean))
	pX, rX, qX := own(f.ast.MkApp(p, []ast.ASTHandle{x})), own(f.ast.MkApp(r, []ast.ASTHandle{x})), own(f.ast.MkApp(q, []ast.ASTHandle{x}))
	pFalse, qTrue := own(f.ast.MkApp(p, []ast.ASTHandle{f.falsehood})), own(f.ast.MkApp(q, []ast.ASTHandle{f.truth}))
	// Consumers deliberately precede producers. Competing producers must keep
	// the first source explanation when they derive the same tuple.
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
		name  string
	}{{qX, []ast.ASTHandle{rX}, "output-step"}, {rX, []ast.ASTHandle{pX}, "middle-step"}, {pFalse, nil, "seed-first"}, {pFalse, nil, "seed-second"}, {qTrue, nil, "alternate"}} {
		rule, e := rm.NewRule(spec.head, spec.tails, nil, spec.name)
		astTestMust(t, e)
		astTestMust(t, set.Add(rule))
		astTestMust(t, rule.Close())
	}
	astTestMust(t, set.SetOutputPredicate(q))
	transformer, err := NewExplanationTransformer(f.ast, rm)
	astTestMust(t, err)
	defer func() { astTestMust(t, transformer.Close()) }()
	transformed, err := transformer.TransformRulesOnly(set)
	astTestMust(t, err)
	defer func() { astTestMust(t, transformed.Close()) }()
	astTestMust(t, transformer.TransferFiniteFacts(set, transformed, f.relations, []ast.ASTHandle{p, r, q}))
	explainedQ, err := transformer.ExplanationDeclaration(q)
	astTestMust(t, err)
	var heads []ast.ASTHandle
	for _, predicate := range []ast.ASTHandle{p, r, q} {
		declaration, e := transformer.ExplanationDeclaration(predicate)
		astTestMust(t, e)
		heads = append(heads, declaration)
	}
	heads = append(heads, q)
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
	wantFalse := proof("output-step", proof("middle-step", proof("seed-first")))
	wantTrue := proof("alternate")
	for repeat := 0; repeat < 2; repeat++ {
		astTestMust(t, f.relations.ResetSaturatedMarks())
		complete, e := execution.Perform(context)
		astTestMust(t, e)
		if !complete {
			t.Fatal("ordered explanation execution incomplete")
		}
		for _, head := range heads {
			marked, e := f.relations.IsSaturated(head)
			astTestMust(t, e)
			if marked {
				t.Fatal("execution marked a head before termination")
			}
		}
		complete, e = termination.Perform(context)
		astTestMust(t, e)
		if !complete {
			t.Fatal("ordered explanation termination incomplete")
		}
		for _, head := range heads {
			marked, e := f.relations.IsSaturated(head)
			astTestMust(t, e)
			if !marked {
				t.Fatal("termination did not mark an explained or finite head")
			}
		}
		output, e := f.relations.GetRelation(q)
		astTestMust(t, e)
		product, e := f.relations.GetExplanationRelation(explainedQ)
		astTestMust(t, e)
		for index, value := range []ast.ASTHandle{f.falsehood, f.truth} {
			found, e := output.Contains(Fact{Element(index)})
			astTestMust(t, e)
			got, explained, e := product.Explanation([]ast.ASTHandle{value})
			astTestMust(t, e)
			want := wantFalse
			if index == 1 {
				want = wantTrue
			}
			if !found || !explained || got != want {
				t.Fatal("DAG schedule lost a tuple or first source proof", index, found, explained, got, want)
			}
		}
		astTestMust(t, context.Reset())
	}
}

func TestCompileExplanationProgramRejectsCyclesBeforePublication(t *testing.T) {
	for _, mutual := range []bool{false, true} {
		f := newTableRelationFixture(t)
		rm, err := base.NewRuleManager(f.ast)
		astTestMust(t, err)
		set, err := base.NewRuleSet(rm)
		astTestMust(t, err)
		declare := func(name string) ast.ASTHandle {
			symbol, e := f.ast.Symbols.Intern(name)
			astTestMust(t, e)
			declaration, e := f.ast.MkFuncDecl(symbol, nil, f.boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, e)
			astTestMust(t, rm.RegisterPredicate(declaration))
			return declaration
		}
		p, q := declare("cycle-P"), declare("cycle-Q")
		pApp, err := f.ast.MkApp(p, nil)
		astTestMust(t, err)
		astTestPin(t, f.ast, pApp)
		qApp, err := f.ast.MkApp(q, nil)
		astTestMust(t, err)
		astTestPin(t, f.ast, qApp)
		body := pApp
		if mutual {
			body = qApp
		}
		rule, err := rm.NewRule(pApp, []ast.ASTHandle{body}, nil, "recursive-P")
		astTestMust(t, err)
		astTestMust(t, set.Add(rule))
		astTestMust(t, rule.Close())
		if mutual {
			rule, err = rm.NewRule(qApp, []ast.ASTHandle{pApp}, nil, "recursive-Q")
			astTestMust(t, err)
			astTestMust(t, set.Add(rule))
			astTestMust(t, rule.Close())
		}
		transformer, err := NewExplanationTransformer(f.ast, rm)
		astTestMust(t, err)
		transformed, err := transformer.TransformRulesOnly(set)
		astTestMust(t, err)
		execution, termination, err := CompileExplanationRulesWithGuard(f.relations, transformed, tableRelationTheoryRewriter(t, f.ast))
		if execution != nil || termination != nil || !errors.Is(err, ErrCompilerUnsupported) {
			t.Fatal("recursive explanation program was published", mutual, execution, termination, err)
		}
		for _, predicate := range []ast.ASTHandle{p, q} {
			declaration, e := transformer.ExplanationDeclaration(predicate)
			astTestMust(t, e)
			_, found, e := f.relations.TryGetExplanationRelation(declaration)
			astTestMust(t, e)
			marked, e := f.relations.IsSaturated(declaration)
			astTestMust(t, e)
			if found || marked {
				t.Fatal("rejected compilation published relation data or marks")
			}
		}
		astTestMust(t, transformed.Close())
		astTestMust(t, transformer.Close())
		astTestMust(t, set.Close())
		astTestMust(t, rm.Close())
	}
}
