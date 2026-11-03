package spacer

// Source: src/muz/spacer/spacer_context.h context::set_unsat.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestContextSetEmptyProgramUnsatSkipsSolveWork(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	rules, err := base.NewRuleManager(manager)
	depthMust(t, err)
	t.Cleanup(func() { depthMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	depthMust(t, err)
	t.Cleanup(func() { depthMust(t, set.Close()) })
	boolean, err := manager.MkBoolSort()
	depthMust(t, err)
	name, err := manager.Symbols.Intern("spacer-empty-program-query")
	depthMust(t, err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	depthMust(t, err)
	depthMust(t, rules.RegisterPredicate(declaration))
	depthMust(t, set.SetOutputPredicate(declaration))
	valid, err := set.Finalize()
	depthMust(t, err)
	if !valid {
		t.Fatal("empty output program is not stratified")
	}
	context, err := NewContext(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	depthMust(t, err)
	t.Cleanup(func() { depthMust(t, context.Close()) })
	depthMust(t, context.InitializePredicates(set, nil))
	depthMust(t, context.SetQuery(declaration))
	context.validateResult = true
	context.printStatistics = true

	manager.Budget().FailAfter(0)
	err = context.SetEmptyProgramUnsat()
	manager.Budget().FailAfter(-1)
	depthMust(t, err)
	status, err := context.LastResult()
	depthMust(t, err)
	if status != lbool.False || !context.solved || context.bounded || context.queue.root != nil {
		t.Fatal("empty-program result state", status, context.solved, context.bounded, context.queue.root)
	}
	seconds, err := context.timers.solve.Seconds()
	depthMust(t, err)
	if seconds != 0 {
		t.Fatal("empty-program result entered solve timer", seconds)
	}
	result, err := context.Model(declaration)
	depthMust(t, err)
	count, err := result.Core().NumDeclarations()
	depthMust(t, err)
	if count != 0 {
		t.Fatal("empty-program model declarations", count)
	}
	depthMust(t, result.Close())
	answer, err := context.Answer(declaration)
	depthMust(t, err)
	answerHandle, err := manager.RefGet(answer)
	depthMust(t, err)
	if !manager.IsTrue(answerHandle) {
		t.Fatal("empty-program answer", answerHandle)
	}
	depthMust(t, manager.DelRef(&answer))
}

func TestContextSetEmptyProgramUnsatRejectsLiveRules(t *testing.T) {
	fixture := newContextAnswerFixture(t, false)
	if err := fixture.context.SetEmptyProgramUnsat(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("nonempty program accepted trivial unsat result", err)
	}
}
