package spacer

// Source: src/muz/spacer/spacer_context.cpp context::get_answer and
// context::get_ground_sat_answer result dispatch.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type contextAnswerFixture struct {
	context *Context
	rules   *base.RuleSet
	rule    *base.Rule
	manager *base.RuleManager
	query   ast.ASTHandle
	other   ast.ASTHandle
}

func newContextAnswerFixture(t *testing.T, recursive bool) *contextAnswerFixture {
	t.Helper()
	manager, globals := newSpacerTestServices(t)
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.Symbols.Intern(map[bool]string{false: "answer-fact", true: "answer-recursive"}[recursive])
	if err != nil {
		t.Fatal(err)
	}
	query, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	otherName, err := manager.Symbols.Intern("answer-other")
	if err != nil {
		t.Fatal(err)
	}
	other, err := manager.MkFuncDecl(otherName, nil, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err = rules.RegisterPredicate(query); err != nil {
		t.Fatal(err)
	}
	head, err := manager.MkApp(query, nil)
	if err != nil {
		t.Fatal(err)
	}
	var body []ast.ASTHandle
	if recursive {
		body = []ast.ASTHandle{head}
	}
	rule, err := rules.NewRule(head, body, make([]bool, len(body)), "answer-rule")
	if err != nil {
		t.Fatal(err)
	}
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(rule); err != nil {
		t.Fatal(err)
	}
	context, err := NewContext(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Initialize(set); err != nil {
		t.Fatal(err)
	}
	if err = context.SetQuery(query); err != nil {
		t.Fatal(err)
	}
	fixture := &contextAnswerFixture{context: context, rules: set, rule: rule, manager: rules, query: query, other: other}
	t.Cleanup(func() {
		if fixture.context != nil && !fixture.context.closed {
			if err := fixture.context.Close(); err != nil {
				t.Fatal(err)
			}
		}
		if err := errors.Join(fixture.rules.Close(), fixture.rule.Close(), fixture.manager.Close()); err != nil {
			t.Fatal(err)
		}
	})
	return fixture
}

func TestContextAnswerStatusDispatchAndOwnership(t *testing.T) {
	t.Run("unsolved-and-undef", func(t *testing.T) {
		fixture := newContextAnswerFixture(t, true)
		answer, err := fixture.context.Answer(fixture.query)
		if err != nil {
			t.Fatal(err)
		}
		handle, err := fixture.context.manager.RefGet(answer)
		if err != nil || !fixture.context.manager.IsTrue(handle) {
			t.Fatal("unsolved answer", handle, err)
		}
		if err = fixture.context.manager.DelRef(&answer); err != nil {
			t.Fatal(err)
		}
		ground, err := fixture.context.GroundSatAnswer(fixture.query)
		if err != nil || ground != (ast.ASTRef{}) {
			t.Fatal("unsolved ground answer", ground, err)
		}
		if _, err = fixture.context.Answer(fixture.other); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("answer accepted another declaration", err)
		}
		if _, err = fixture.context.GroundSatAnswer(fixture.other); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("ground answer accepted another declaration", err)
		}
		status, err := fixture.context.Solve(0, 0)
		if err != nil || status != lbool.Undef {
			t.Fatal("bounded undefined solve", status, err)
		}
		answer, err = fixture.context.Answer(fixture.query)
		if err != nil {
			t.Fatal(err)
		}
		handle, err = fixture.context.manager.RefGet(answer)
		if err != nil || !fixture.context.manager.IsTrue(handle) {
			t.Fatal("undefined answer", handle, err)
		}
		if err = fixture.context.manager.DelRef(&answer); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("sat", func(t *testing.T) {
		fixture := newContextAnswerFixture(t, false)
		status, err := fixture.context.Solve(0, 1)
		if err != nil || status != lbool.True {
			t.Fatal("fact solve", status, err)
		}
		answer, err := fixture.context.Answer(fixture.query)
		if err != nil || answer == (ast.ASTRef{}) {
			t.Fatal("SAT answer", answer, err)
		}
		ground, err := fixture.context.GroundSatAnswer(fixture.query)
		if err != nil || ground == (ast.ASTRef{}) {
			t.Fatal("SAT ground answer", ground, err)
		}
		if err = errors.Join(fixture.context.manager.DelRef(&ground), fixture.context.manager.DelRef(&answer)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unsat", func(t *testing.T) {
		fixture := newContextAnswerFixture(t, true)
		status, err := fixture.context.Solve(0, 3)
		if err != nil || status != lbool.False {
			t.Fatal("recursive solve", status, err)
		}
		answer, err := fixture.context.Answer(fixture.query)
		if err != nil || answer == (ast.ASTRef{}) {
			t.Fatal("UNSAT answer", answer, err)
		}
		if err = fixture.context.manager.DelRef(&answer); err != nil {
			t.Fatal(err)
		}
		ground, err := fixture.context.GroundSatAnswer(fixture.query)
		if err != nil || ground != (ast.ASTRef{}) {
			t.Fatal("UNSAT ground answer", ground, err)
		}
		if err = fixture.context.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err = fixture.context.Answer(fixture.query); !errors.Is(err, ErrContextOwner) {
			t.Fatal("closed answer", err)
		}
		if _, err = fixture.context.GroundSatAnswer(fixture.query); !errors.Is(err, ErrContextOwner) {
			t.Fatal("closed ground answer", err)
		}
	})
}
