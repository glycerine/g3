package spacer

// Source: src/muz/spacer/spacer_context.cpp context::get_ground_sat_answer.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerGroundAnswerIncludesNullaryRootAndCompletesArguments(t *testing.T) {
	for _, withArgument := range []bool{false, true} {
		t.Run(map[bool]string{false: "nullary-leaf", true: "unconstrained-leaf-argument"}[withArgument], func(t *testing.T) {
			m, globals := newSpacerTestServices(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			var pins []ast.ASTRef
			defer func() { must(releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				must(err)
				ref, err := m.NewRef(h)
				must(err)
				pins = append(pins, ref)
				return h
			}
			boolean := own(m.MkBoolSort())
			rules, err := base.NewRuleManager(m)
			must(err)
			defer func() { must(rules.Close()) }()
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				t.Helper()
				symbol, err := m.Symbols.Intern(name)
				must(err)
				h := own(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
				must(rules.RegisterPredicate(h))
				return h
			}
			queryDecl := decl("ground-answer-query", nil)
			rootDecl := decl("ground-answer-root", nil)
			var domain, args []ast.ASTHandle
			if withArgument {
				domain = []ast.ASTHandle{boolean}
				args = []ast.ASTHandle{own(m.MkVar(0, boolean))}
			}
			leafDecl := decl("ground-answer-leaf", domain)
			query := own(m.MkApp(queryDecl, nil))
			root := own(m.MkApp(rootDecl, nil))
			leaf := own(m.MkApp(leafDecl, args))
			set, err := base.NewRuleSet(rules)
			must(err)
			defer func() { must(set.Close()) }()
			for _, item := range []struct {
				head ast.ASTHandle
				tail []ast.ASTHandle
				name string
			}{
				{query, []ast.ASTHandle{root}, "query"},
				{root, []ast.ASTHandle{leaf}, "root"},
				{leaf, nil, "leaf"},
			} {
				rule, err := rules.NewRule(item.head, item.tail, nil, item.name)
				must(err)
				must(set.Add(rule))
				must(rule.Close())
			}
			ctx, err := NewContext(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
			must(err)
			defer func() { must(ctx.Close()) }()
			must(ctx.Initialize(set))
			must(ctx.SetQuery(queryDecl))
			status, err := ctx.Solve(0, 5)
			must(err)
			if status != lbool.True {
				t.Fatal("expected satisfiable derivation", status)
			}
			answer, err := ctx.GroundSatAnswer(queryDecl)
			must(err)
			defer func() { must(m.DelRef(&answer)) }()
			h, err := m.RefGet(answer)
			must(err)
			count, err := m.GetNumArgs(h)
			must(err)
			if count != 2 {
				t.Fatalf("answer must contain the nullary root and leaf, got %d arguments", count)
			}
			first, err := m.GetArg(h, 0)
			must(err)
			if first != root {
				t.Fatal("answer omitted the nullary root predicate")
			}
			second, err := m.GetArg(h, 1)
			must(err)
			actualDecl, err := m.GetDecl(second)
			must(err)
			if actualDecl != leafDecl {
				t.Fatal("answer did not contain the leaf predicate")
			}
			if withArgument {
				value, err := m.GetArg(second, 0)
				must(err)
				if !m.IsTrue(value) && !m.IsFalse(value) {
					t.Fatal("model completion did not ground the unconstrained Boolean argument")
				}
			}
		})
	}
}
