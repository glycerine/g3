package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestPDRContextPropertiesBeforeRuleInitialization(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
	must := func(err error) { t.Helper(); closureMust(t, err) }
	context, err := NewContext(m, nil, 2, ReachableNoCache, owner.contexts.config)
	must(err)
	defer func() {
		must(context.Close())
	}()
	declaration, err := transformer.Head()
	must(err)
	levels, err := context.NumLevels(declaration)
	must(err)
	if levels != 0 {
		t.Fatal("unknown predicate has levels", levels)
	}
	delta, err := context.CoverDelta(-1, declaration)
	must(err)
	h, err := m.RefGet(delta)
	must(err)
	if !m.IsTrue(h) {
		t.Fatal("unknown predicate cover is not true")
	}
	must(m.DelRef(&delta))
	falsity, err := m.MkFalse()
	must(err)
	must(context.AddCover(2, declaration, falsity))
	levels, err = context.NumLevels(declaration)
	must(err)
	if levels != 3 {
		t.Fatal("pre-initialization cover frame was lost", levels)
	}
	delta, err = context.CoverDelta(2, declaration)
	must(err)
	defer func() { must(m.DelRef(&delta)) }()
	h, err = m.RefGet(delta)
	must(err)
	if !m.IsFalse(h) {
		t.Fatal("pre-initialization cover changed")
	}
}

func TestPDRInitializeInheritsPrequeryArithmeticCover(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
	must := func(err error) { t.Helper(); closureMust(t, err) }
	context, err := NewContext(m, nil, 2, ReachableConstraintCache, owner.contexts.config)
	must(err)
	defer func() { must(context.Close()) }()
	declaration, err := transformer.Head()
	must(err)
	falsity, err := m.MkFalse()
	must(err)
	must(context.AddCover(2, declaration, falsity))
	prior := context.transformers[declaration]
	priorSolver := prior.solver.context.solver
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	must(rules.RegisterPredicate(declaration))
	set, err := base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	sort, err := m.GetDomain(declaration, 0)
	must(err)
	variable, err := m.MkVar(0, sort)
	must(err)
	head, err := m.MkApp(declaration, []ast.ASTHandle{variable})
	must(err)
	rule, err := rules.NewRule(head, nil, nil, "prequery-arithmetic-fact")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	must(context.Initialize(set))
	if context.transformers[declaration] == prior || !prior.closed {
		t.Fatal("initial rule installation did not replace and close the pre-query transformer")
	}
	if context.transformers[declaration].solver.context.solver == priorSolver {
		t.Fatal("initial rule installation reused a solver created in the old proof mode")
	}
	levels, err := context.NumLevels(declaration)
	must(err)
	if levels != 3 {
		t.Fatal("initial rule installation lost pre-query levels", levels)
	}
	cover, err := context.CoverDelta(2, declaration)
	must(err)
	defer func() { must(m.DelRef(&cover)) }()
	handle, err := m.RefGet(cover)
	must(err)
	if !m.IsFalse(handle) {
		t.Fatal("initial rule installation lost pre-query cover")
	}
	if context.owner.contexts.arithmeticMode() != smtcore.SMTArithmeticDifferenceLogic {
		t.Fatal("pre-query solvers prevented difference-logic selection", context.owner.contexts.arithmeticMode())
	}
	if err = context.owner.contexts.configureArithmetic(true, smtcore.SMTArithmeticSimplex, false); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("arithmetic configuration changed after rule solvers were allocated", err)
	}
}
