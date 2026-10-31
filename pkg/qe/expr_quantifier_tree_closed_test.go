package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExprQETreeClosedPartitionCompletesModelAndBlocks(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	p := f.constant("closed-p", f.boolean)
	r := f.constant("closed-free", f.boolean)
	truth := f.own(m.MkTrue())
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	rDecl, err := m.GetDecl(r)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(rDecl, truth))
	eval, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
	fmMust(t, err)
	defer func() { fmMust(t, eval.Close()) }()
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	notP := f.own(m.MkNot(p))
	tautology := f.own(m.MkOr(p, notP))
	formula := f.own(m.MkAnd(tautology, r))
	fmMust(t, solver.AssertFormula(formula))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("closed-partition fixture is not satisfiable")
	}
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	context, err := NewSolverContext(m, []ast.ASTHandle{p})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	plugin, err := NewBoolPlugin(context)
	fmMust(t, err)
	defer func() { fmMust(t, plugin.Close()) }()
	partitioner, err := NewConjunctionPartitioner(m)
	fmMust(t, err)
	defer func() { fmMust(t, partitioner.Close()) }()
	fmMust(t, partitioner.AddPlugin(plugin))
	part, err := partitionExprQETree(root, variables, partitioner)
	fmMust(t, err)
	defer func() { fmMust(t, part.Close()) }()
	child, handled, err := closeExprQETreePartition(root, part, solver, eval, md.Core(), f.r.config, true)
	fmMust(t, err)
	if !handled || !root.isUnit() || len(child.variables) != 0 {
		t.Fatal("closed partition was not published")
	}
	closed, err := m.RefGet(child.formula)
	fmMust(t, err)
	if closed != r {
		t.Fatal("wrong closed formula")
	}
	definition, err := child.definitions.Definition(0)
	fmMust(t, err)
	if !m.IsTrue(definition) && !m.IsFalse(definition) {
		t.Fatal("missing Boolean model completion")
	}
	status, err = solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("completed assignment was not blocked")
	}
}
