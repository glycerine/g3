package rel

// Source: dl_mk_explanations.cpp get_e_rule and dl_compiler.cpp
// add_unbound_columns_for_negation/compile_rule_evaluation_run.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type explanationNegativeFixture struct {
	*tableRelationFixture
	rules *base.RuleManager
	set   *base.RuleSet
}

func newExplanationNegativeFixture(t *testing.T, representation string) *explanationNegativeFixture {
	t.Helper()
	f := &explanationNegativeFixture{tableRelationFixture: newTableRelationFixture(t)}
	astTestMust(t, f.relations.SetDefaultFiniteTable(representation))
	var err error
	f.rules, err = base.NewRuleManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.rules.Close()) })
	f.set, err = base.NewRuleSet(f.rules)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.set.Close()) })
	return f
}

func (f *explanationNegativeFixture) own(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	astTestMust(f.t, err)
	astTestPin(f.t, f.ast, h)
	return h
}

func (f *explanationNegativeFixture) declare(name string, arity int) ast.ASTHandle {
	f.t.Helper()
	symbol, err := f.ast.Symbols.Intern(name)
	astTestMust(f.t, err)
	domain := make([]ast.ASTHandle, arity)
	for i := range domain {
		domain[i] = f.boolean
	}
	declaration, err := f.ast.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(f.t, err)
	astTestMust(f.t, f.rules.RegisterPredicate(declaration))
	return declaration
}

func (f *explanationNegativeFixture) add(head ast.ASTHandle, tails []ast.ASTHandle, negative []bool, name string) {
	f.t.Helper()
	rule, err := f.rules.NewRule(head, tails, negative, name)
	astTestMust(f.t, err)
	astTestMust(f.t, f.set.Add(rule))
	astTestMust(f.t, rule.Close())
}

func (f *explanationNegativeFixture) proof(name string, args ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	symbol, err := f.ast.Symbols.Intern(name)
	astTestMust(f.t, err)
	return f.own(f.ast.MkDatalogRule(symbol, args))
}

func (f *explanationNegativeFixture) transform(predicates ...ast.ASTHandle) (*ExplanationTransformer, *base.RuleSet) {
	f.t.Helper()
	transformer, err := NewExplanationTransformer(f.ast, f.rules)
	astTestMust(f.t, err)
	f.t.Cleanup(func() { astTestMust(f.t, transformer.Close()) })
	transformed, err := transformer.TransformRulesOnly(f.set)
	astTestMust(f.t, err)
	f.t.Cleanup(func() { astTestMust(f.t, transformed.Close()) })
	astTestMust(f.t, transformer.TransferFiniteFacts(f.set, transformed, f.relations, predicates))
	return transformer, transformed
}

func TestCompileExplanationFiniteNegationAgainstTupleOracle(t *testing.T) {
	for _, representation := range []string{"hashtable", "sparse"} {
		for _, mode := range []string{"positive-constant", "negative-only-head", "negative-only-project", "nullary-empty", "nullary-full"} {
			t.Run(representation+"/"+mode, func(t *testing.T) {
				f := newExplanationNegativeFixture(t, representation)
				qArity, nArity := 1, 2
				if mode == "negative-only-project" {
					qArity = 0
				}
				if mode == "nullary-empty" || mode == "nullary-full" {
					qArity, nArity = 0, 0
				}
				p, q, n := f.declare("negative-input", 1), f.declare("negative-output", qArity), f.declare("negative-blocker", nArity)
				x := f.own(f.ast.MkVar(0, f.boolean))
				var headArgs []ast.ASTHandle
				if qArity != 0 {
					headArgs = []ast.ASTHandle{x}
				}
				head := f.own(f.ast.MkApp(q, headArgs))
				var negativeArgs []ast.ASTHandle
				if nArity != 0 {
					negativeArgs = []ast.ASTHandle{x, x}
					if mode == "positive-constant" {
						negativeArgs[1] = f.truth
					}
				}
				negative := f.own(f.ast.MkApp(n, negativeArgs))
				tails, flags := []ast.ASTHandle{negative}, []bool{true}
				if mode == "positive-constant" {
					positive := f.own(f.ast.MkApp(p, []ast.ASTHandle{x}))
					tails, flags = []ast.ASTHandle{positive, negative}, []bool{false, true}
				}
				f.add(head, tails, flags, "negative-step")
				astTestMust(t, f.set.SetOutputPredicate(q))
				input, err := f.relations.GetFiniteRelation(p)
				astTestMust(t, err)
				_, err = input.Add(Fact{0})
				astTestMust(t, err)
				_, err = input.Add(Fact{1})
				astTestMust(t, err)
				blocker, err := f.relations.GetFiniteRelation(n)
				astTestMust(t, err)
				var originalRows []Fact
				switch mode {
				case "positive-constant":
					originalRows = []Fact{{0, 1}, {1, 0}}
				case "negative-only-head", "negative-only-project":
					// Only the diagonal (0,0) blocks x. Off-diagonal (1,0)
					// must not satisfy both occurrences in not N(x,x).
					originalRows = []Fact{{0, 0}, {1, 0}}
				case "nullary-full":
					originalRows = []Fact{{}}
				}
				for _, row := range originalRows {
					_, err = blocker.Add(row)
					astTestMust(t, err)
				}
				transformer, transformed := f.transform(p, q, n)
				explainedQ, err := transformer.ExplanationDeclaration(q)
				astTestMust(t, err)
				explainedP, err := transformer.ExplanationDeclaration(p)
				astTestMust(t, err)
				positiveSource, err := f.relations.GetExplanationRelation(explainedP)
				astTestMust(t, err)
				execution, termination, err := CompileExplanationRulesWithGuard(f.relations, transformed, tableRelationTheoryRewriter(t, f.ast))
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) })
				astTestMust(t, transformer.Close())
				astTestMust(t, transformed.Close())
				astTestMust(t, f.set.Close())
				context, err := NewExecutionContextWithRelations(f.relations, nil)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, context.Close()) })
				factProof := f.proof("fact")
				wantProof := f.proof("negative-step")
				if mode == "positive-constant" {
					wantProof = f.proof("negative-step", factProof)
				}
				for repeat := 0; repeat < 2; repeat++ {
					complete, err := execution.Perform(context)
					astTestMust(t, err)
					if !complete {
						t.Fatal("negative explanation execution incomplete")
					}
					complete, err = termination.Perform(context)
					astTestMust(t, err)
					if !complete {
						t.Fatal("negative explanation termination incomplete")
					}
					output, err := f.relations.GetFiniteRelation(q)
					astTestMust(t, err)
					product, err := f.relations.GetExplanationRelation(explainedQ)
					astTestMust(t, err)
					for index := 0; index < 1+qArity; index++ {
						var row Fact
						var values []ast.ASTHandle
						want := mode != "nullary-full"
						if qArity != 0 {
							row = Fact{Element(index)}
							value := f.falsehood
							if index == 1 {
								value = f.truth
							}
							values, want = []ast.ASTHandle{value}, index == 1
						}
						found, err := output.Contains(row)
						astTestMust(t, err)
						got, explained, err := product.Explanation(values)
						astTestMust(t, err)
						if found != want || explained != want || (want && got != wantProof) {
							t.Fatal("negative tuple or positive-only explanation differs", mode, row, found, explained, got, want, wantProof)
						}
					}
					current, err := f.relations.GetFiniteRelation(n)
					astTestMust(t, err)
					if current != blocker {
						t.Fatal("negation replaced its source relation owner")
					}
					count, err := blocker.Len()
					astTestMust(t, err)
					if count != len(originalRows) {
						t.Fatal("negation changed source row count", count, len(originalRows))
					}
					for _, row := range originalRows {
						found, err := blocker.Contains(row)
						astTestMust(t, err)
						if !found {
							t.Fatal("negation consumed a source tuple", row)
						}
					}
					for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
						proof, found, err := positiveSource.Explanation([]ast.ASTHandle{value})
						astTestMust(t, err)
						if !found || proof != factProof {
							t.Fatal("negative filtering changed a positive source proof", value, proof, found)
						}
					}
					astTestMust(t, context.Reset())
				}
			})
		}
	}
}

func TestCompileExplanationDerivedNegativeRequiresFiniteBridge(t *testing.T) {
	for _, mode := range []string{"missing-bridge", "bridge", "negative-cycle"} {
		t.Run(mode, func(t *testing.T) {
			f := newExplanationNegativeFixture(t, "sparse")
			p, q, n := f.declare("bridge-input", 1), f.declare("bridge-output", 1), f.declare("bridge-blocker", 1)
			x := f.own(f.ast.MkVar(0, f.boolean))
			pApp, qApp, nApp := f.own(f.ast.MkApp(p, []ast.ASTHandle{x})), f.own(f.ast.MkApp(q, []ast.ASTHandle{x})), f.own(f.ast.MkApp(n, []ast.ASTHandle{x}))
			f.add(qApp, []ast.ASTHandle{pApp, nApp}, []bool{false, true}, "bridge-negative")
			if mode == "negative-cycle" {
				f.add(nApp, []ast.ASTHandle{qApp}, nil, "negative-backedge")
			} else {
				f.add(f.own(f.ast.MkApp(n, []ast.ASTHandle{f.falsehood})), nil, nil, "derive-blocker")
			}
			astTestMust(t, f.set.SetOutputPredicate(q))
			if mode != "missing-bridge" {
				astTestMust(t, f.set.SetOutputPredicate(n))
			}
			input, err := f.relations.GetFiniteRelation(p)
			astTestMust(t, err)
			_, err = input.Add(Fact{0})
			astTestMust(t, err)
			_, err = input.Add(Fact{1})
			astTestMust(t, err)
			transformer, transformed := f.transform(p, q, n)
			explainedQ, err := transformer.ExplanationDeclaration(q)
			astTestMust(t, err)
			execution, termination, err := CompileExplanationRulesWithGuard(f.relations, transformed, tableRelationTheoryRewriter(t, f.ast))
			if mode != "bridge" {
				if execution != nil || termination != nil || !errors.Is(err, ErrCompilerUnsupported) {
					t.Fatal("unsupported negative program was published", mode, execution, termination, err)
				}
				product, e := f.relations.GetExplanationRelation(explainedQ)
				astTestMust(t, e)
				count, e := product.Len()
				astTestMust(t, e)
				marked, e := f.relations.IsSaturated(explainedQ)
				astTestMust(t, e)
				if count != 0 || marked {
					t.Fatal("rejected negative program published facts or saturation", count, marked)
				}
				return
			}
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) })
			astTestMust(t, transformer.Close())
			astTestMust(t, transformed.Close())
			astTestMust(t, f.set.Close())
			context, err := NewExecutionContextWithRelations(f.relations, nil)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, context.Close()) })
			wantProof := f.proof("bridge-negative", f.proof("fact"))
			for repeat := 0; repeat < 2; repeat++ {
				complete, err := execution.Perform(context)
				astTestMust(t, err)
				if !complete {
					t.Fatal("stratified negative execution incomplete")
				}
				complete, err = termination.Perform(context)
				astTestMust(t, err)
				if !complete {
					t.Fatal("stratified negative termination incomplete")
				}
				product, err := f.relations.GetExplanationRelation(explainedQ)
				astTestMust(t, err)
				for index, value := range []ast.ASTHandle{f.falsehood, f.truth} {
					got, found, err := product.Explanation([]ast.ASTHandle{value})
					astTestMust(t, err)
					if found != (index == 1) || (found && got != wantProof) {
						t.Fatal("finite bridge did not precede negative consumer", index, got, found, wantProof)
					}
				}
				astTestMust(t, context.Reset())
			}
		})
	}
}
