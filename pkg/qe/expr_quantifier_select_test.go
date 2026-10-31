package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExprQEVariableSelectionSourceOrderAndUnsupported(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("select-p", f.boolean)
	bvSort := f.own(m.BitVectorSort(4))
	bv := f.constant("select-bv", bvSort)
	truth := f.own(m.MkTrue())
	context, err := NewSolverContext(m, []ast.ASTHandle{p, bv})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	boolPlugin, err := NewBoolPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(boolPlugin))
	bvPlugin, err := NewBitVectorPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(bvPlugin))
	atoms, err := NewBoolAtomIndex(m, []ast.ASTHandle{p}, nil)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(bv))
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, variables.consume(root))
	var free []ast.ASTRef
	defer func() {
		for i := range free {
			fmMust(t, m.DelRef(&free[i]))
		}
	}()
	selected, count, found, err := selectExprQEVariable(root, variables, context, atoms, &free)
	fmMust(t, err)
	if !found || selected != bv || count.RatString() != "16" {
		t.Fatal("source weight scan order", selected, count.RatString())
	}
	sym, err := m.Symbols.Intern("select-unsupported-sort")
	fmMust(t, err)
	sort, err := m.MkUninterpretedSort(sym, nil)
	fmMust(t, err)
	unsupported := f.constant("select-unsupported", sort)
	other, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, other.Close()) }()
	fmMust(t, other.add(p))
	fmMust(t, other.add(unsupported))
	second, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, second.Close()) }()
	fmMust(t, second.initialize(truth))
	fmMust(t, other.consume(second))
	selected, count, found, err = selectExprQEVariable(second, other, context, atoms, &free)
	fmMust(t, err)
	if !found || selected != p || count.RatString() != "2" || len(free) != 1 || second.isPure() {
		t.Fatal("unsupported variable was not released", selected, count.RatString())
	}
	freed, err := m.RefGet(free[0])
	fmMust(t, err)
	if freed != unsupported {
		t.Fatal("wrong free variable")
	}
}
