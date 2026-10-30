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
	for _, tc := range []struct {
		name                                  string
		withArgument, branches, multipleRules bool
	}{
		{"nullary-leaf", false, false, false},
		{"unconstrained-leaf-argument", true, false, false},
		{"opposite-sibling-values", true, true, false},
		{"multiple-leaf-rule-tags", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			if tc.withArgument {
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
			items := []struct {
				head ast.ASTHandle
				tail []ast.ASTHandle
				name string
			}{
				{query, []ast.ASTHandle{root}, "query"},
				{root, []ast.ASTHandle{leaf}, "root"},
				{leaf, nil, "leaf"},
			}
			var expectedBranch []ast.ASTHandle
			if tc.branches {
				left := own(m.MkApp(decl("ground-answer-left", nil), nil))
				right := own(m.MkApp(decl("ground-answer-right", nil), nil))
				leafTrue := own(m.MkApp(leafDecl, []ast.ASTHandle{own(m.MkTrue())}))
				leafFalse := own(m.MkApp(leafDecl, []ast.ASTHandle{own(m.MkFalse())}))
				items[1].tail = []ast.ASTHandle{left, right}
				items = append(items, struct {
					head ast.ASTHandle
					tail []ast.ASTHandle
					name string
				}{left, []ast.ASTHandle{leafTrue}, "left"})
				items = append(items, struct {
					head ast.ASTHandle
					tail []ast.ASTHandle
					name string
				}{right, []ast.ASTHandle{leafFalse}, "right"})
				expectedBranch = []ast.ASTHandle{root, left, right, leafTrue, leafFalse}
			}
			if tc.multipleRules {
				alternative := items[2]
				alternative.name = "leaf-alternative"
				items = append(items, alternative)
			}
			for _, item := range items {
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
			if !tc.branches {
				// query -> root -> leaf needs two child searches (root at
				// levels zero and one), three lemma additions/promotions and two reach facts.
				for key, expected := range map[string]uint32{
					"SPACER num queries":           2,
					"SPACER num reach queries":     2,
					"SPACER num reuse reach facts": 1,
					"SPACER num lemmas":            3,
					"SPACER max query lvl":         2,
					"SPACER max depth":             2,
				} {
					if got := spacerContextStatistic(t, ctx, key); got != expected {
						t.Fatalf("%s = %d, want %d", key, got, expected)
					}
				}
			}
			answer, err := ctx.GroundSatAnswer(queryDecl)
			must(err)
			defer func() { must(m.DelRef(&answer)) }()
			h, err := m.RefGet(answer)
			must(err)
			count, err := m.GetNumArgs(h)
			must(err)
			if tc.branches {
				if int(count) != len(expectedBranch) {
					t.Fatal("branching answer length", count)
				}
				for i, expected := range expectedBranch {
					actual, err := m.GetArg(h, uint32(i))
					must(err)
					if actual != expected {
						t.Fatal("branching answer order or sibling model scope", i, actual, expected)
					}
				}
				return
			}
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
			if tc.withArgument {
				value, err := m.GetArg(second, 0)
				must(err)
				if !m.IsTrue(value) && !m.IsFalse(value) {
					t.Fatal("model completion did not ground the unconstrained Boolean argument")
				}
			}
		})
	}
}
