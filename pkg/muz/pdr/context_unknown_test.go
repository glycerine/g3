package pdr_test

// Source: pdr_context.cpp expand_node l_undef -> unknown_exception and
// context::solve unknown_exception -> l_undef.

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
	if answer, err := context.Answer(query); !errors.Is(err, ast.ErrASTDomain) || answer != (ast.ASTRef{}) {
		t.Fatal("unknown solve published an answer", answer, err)
	}
	if model, err := context.Model(query); !errors.Is(err, ast.ErrASTDomain) || model != nil {
		t.Fatal("unknown solve published a model", model, err)
	}
	var certificate bytes.Buffer
	pdrMust(t, context.DisplayCertificate(&certificate, query, false))
	if certificate.String() != "unknown" {
		t.Fatalf("unknown solve published a decided certificate: %q", certificate.String())
	}

	pdrMust(t, context.Close())
	pdrMust(t, rules.Close())
	pdrMust(t, ruleManager.Close())
	pdrMust(t, globals.Context.Del(&limited))
	pdrMust(t, command.Close())
}
