package pdr_test

// Source: pdr_context.cpp expand_node l_undef -> unknown_exception and
// context::solve unknown_exception -> l_undef and get_answer's owning true.

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestContextSolvePropagatesNativeConflictLimitUnknown(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	limited, err := globals.Context.NewValue()
	pdrMust(t, err)
	limitName := "max_conflicts"
	pdrMust(t, globals.Context.SetUIntName(&limited, &limitName, 0))
	context, err := pdr.NewContext(manager, &limited, 2, pdr.ReachableConstraintCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	query := pdrFunction(t, manager, "unknown_query", nil, boolean)
	ruleManager, err := base.NewRuleManager(manager)
	pdrMust(t, err)
	pdrMust(t, ruleManager.RegisterPredicate(query))
	rules, err := base.NewRuleSet(ruleManager)
	pdrMust(t, err)

	aName, err := manager.Symbols.Intern("unknown_a")
	pdrMust(t, err)
	a, err := manager.MkConst(aName, boolean)
	pdrMust(t, err)
	bName, err := manager.Symbols.Intern("unknown_b")
	pdrMust(t, err)
	b, err := manager.MkConst(bName, boolean)
	pdrMust(t, err)
	notA, err := manager.MkNot(a)
	pdrMust(t, err)
	notB, err := manager.MkNot(b)
	pdrMust(t, err)
	clauses := make([]ast.ASTHandle, 4)
	clauses[0], err = manager.MkOr(a, b)
	pdrMust(t, err)
	clauses[1], err = manager.MkOr(notA, b)
	pdrMust(t, err)
	clauses[2], err = manager.MkOr(a, notB)
	pdrMust(t, err)
	clauses[3], err = manager.MkOr(notA, notB)
	pdrMust(t, err)
	constraint, err := manager.MkAndArgs(clauses)
	pdrMust(t, err)
	head, err := manager.MkApp(query, nil)
	pdrMust(t, err)
	rule, err := ruleManager.NewRule(head, []ast.ASTHandle{constraint}, nil, "unknown-conflict-limit")
	pdrMust(t, err)
	pdrMust(t, rules.Add(rule))
	pdrMust(t, rule.Close())
	pdrMust(t, context.Initialize(rules))
	// Local reachability checks do not publish a global query result.
	local, err := context.CheckInitial(head, false)
	pdrMust(t, err)
	if local.Status != lbool.Undef {
		t.Fatal("native local conflict-limit result", local.Status)
	}
	if localAnswer, err := context.Answer(query); !errors.Is(err, ast.ErrASTDomain) || localAnswer != (ast.ASTRef{}) {
		t.Fatal("local check published a global answer", localAnswer, err)
	}

	status, err := context.Solve(query)
	pdrMust(t, err)
	if status != lbool.Undef {
		t.Fatal("native SMT unknown was converted to a decision", status)
	}
	last, err := context.LastResult()
	pdrMust(t, err)
	if last != lbool.Undef {
		t.Fatal("unknown solve published a stale result", last)
	}
	before := manager.Stats().LiveReferences
	answer, err := context.Answer(query)
	pdrMust(t, err)
	answerHandle, err := manager.RefGet(answer)
	pdrMust(t, err)
	if !manager.IsTrue(answerHandle) || manager.Stats().LiveReferences != before+1 {
		t.Fatal("unknown solve did not return one owning true answer", answerHandle)
	}
	duplicate, err := context.Answer(query)
	pdrMust(t, err)
	pdrMust(t, manager.DelRef(&duplicate))
	if manager.Stats().LiveReferences != before+1 {
		t.Fatal("repeated unknown answer changed reference ownership")
	}
	if wrong, err := context.Answer(ast.ASTHandle{}); !errors.Is(err, ast.ErrASTDomain) || wrong != (ast.ASTRef{}) {
		t.Fatal("unknown answer accepted another query", wrong, err)
	}
	if model, err := context.Model(query); !errors.Is(err, ast.ErrASTDomain) || model != nil {
		t.Fatal("unknown solve published a model", model, err)
	}
	var certificate bytes.Buffer
	pdrMust(t, context.DisplayCertificate(&certificate, query, false))
	if certificate.String() != "unknown" {
		t.Fatalf("unknown solve published a decided certificate: %q", certificate.String())
	}
	assertUnavailable := func(stage string) {
		t.Helper()
		if stale, err := context.Answer(query); !errors.Is(err, ast.ErrASTDomain) || stale != (ast.ASTRef{}) {
			t.Fatal(stage, "published a stale unknown answer", stale, err)
		}
	}
	solveUnknown := func() {
		t.Helper()
		status, err := context.Solve(query)
		pdrMust(t, err)
		if status != lbool.Undef {
			t.Fatal("repeated native unknown solve", status)
		}
		reference, err := context.Answer(query)
		pdrMust(t, err)
		handle, err := manager.RefGet(reference)
		pdrMust(t, err)
		if !manager.IsTrue(handle) {
			t.Fatal("repeated unknown solve lost source true answer")
		}
		pdrMust(t, manager.DelRef(&reference))
	}
	manager.ResourceLimit().Cancel()
	status, err = context.Solve(query)
	manager.ResourceLimit().ResetCancel()
	if status != lbool.Undef || err == nil {
		t.Fatal("canceled solve did not fail", status, err)
	}
	assertUnavailable("canceled solve")
	solveUnknown()
	pdrMust(t, context.UpdateRules(rules))
	assertUnavailable("rule update")
	solveUnknown()
	pdrMust(t, context.Reset())
	assertUnavailable("reset")
	pdrMust(t, context.Initialize(rules))
	solveUnknown()

	pdrMust(t, context.Close())
	if stale, err := context.Answer(query); !errors.Is(err, pdr.ErrContextOwner) || stale != (ast.ASTRef{}) {
		t.Fatal("closed context published an unknown answer", stale, err)
	}
	if handle, err := manager.RefGet(answer); err != nil || handle != answerHandle {
		t.Fatal("caller-owned unknown answer lost across reset and context close", handle, err)
	}
	pdrMust(t, manager.DelRef(&answer))
	pdrMust(t, rules.Close())
	pdrMust(t, ruleManager.Close())
	pdrMust(t, globals.Context.Del(&limited))
	pdrMust(t, command.Close())
}
