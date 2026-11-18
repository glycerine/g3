package qe

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExprQEBoolPopSolvesAndBlocksPath(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	p := f.constant("pop-p", f.boolean)
	q := f.constant("pop-q", f.boolean)
	truth := f.own(m.MkTrue())
	formula := f.own(m.MkAnd(p, q))
	fmMust(t, solver.AssertFormula(formula))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("pop fixture is not satisfiable")
	}
	context, err := NewSolverContext(m, []ast.ASTHandle{p, q})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	plugin, err := NewBoolPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(plugin))
	adapter, err := NewSolverNNFAdapter(context)
	fmMust(t, err)
	defer func() { fmMust(t, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, nnf.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	partitioner, err := NewConjunctionPartitioner(m)
	fmMust(t, err)
	defer func() { fmMust(t, partitioner.Close()) }()
	fmMust(t, partitioner.AddPlugin(plugin))
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(q))
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	root.atoms, err = NewBoolAtomIndex(m, []ast.ASTHandle{p, q}, nil)
	fmMust(t, err)
	var free []ast.ASTRef
	leaf, _, outcome, err := popExprQEBoolTree(root, variables, context, plugin, partitioner, solver, nil, nil, f.r.config, rw, nnf, &free, false)
	fmMust(t, err)
	if outcome != exprQEPopCompleted || len(leaf.variables) != 0 || len(free) != 0 {
		t.Fatal("Boolean pop did not complete both units")
	}
	h, err := m.RefGet(leaf.formula)
	fmMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("eliminated leaf is not true")
	}
	status, err = solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("completed pop did not block the explored path")
	}
}

// AUDIT.md qe_02-F8: pop computes the partition once and hands it to
// process_partition; z3 traces get_partition three times here (the block,
// then each independent group).
func TestPopComputesPartitionOnce(t *testing.T) {
	f := qsatKernelFixture(t)
	trace := qeTraceCapture(t, f)
	m := f.r.manager
	a := f.constant("partition-a", f.integer)
	b := f.constant("partition-b", f.integer)
	x := f.own(m.MkVar(1, f.integer))
	y := f.own(m.MkVar(0, f.integer))
	times := func(k string, v ast.ASTHandle) ast.ASTHandle { return f.own(m.MkArithmeticMul(f.number(k), v)) }
	body := f.own(m.MkAndArgs([]ast.ASTHandle{
		f.own(m.MkArithmeticGT(times("2", x), a)),
		f.own(m.MkArithmeticLT(times("3", x), f.own(m.MkArithmeticAdd(a, f.number("7"))))),
		f.own(m.MkArithmeticLT(times("3", y), b)),
		f.own(m.MkArithmeticGT(times("5", y), f.own(m.MkArithmeticSub(b, f.number("4"))))),
	}))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer, f.integer}, Names: []symbol.Symbol{symbol.Numerical(1), symbol.Numerical(0)}, Body: body}))
	e := newExprQuantElim(m, f.r.config)
	defer func() { fmMust(t, e.Close()) }()
	result, err := e.run(f.own(m.MkTrue()), q)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	if n := strings.Count(trace.String(), "[qe] get_partition src/qe/qe.cpp"); n != 3 {
		t.Fatalf("get_partition traced %d times, z3 traces it 3 times:\n%s", n, trace.String())
	}
}
