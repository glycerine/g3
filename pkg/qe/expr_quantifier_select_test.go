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

func TestExprQEDatatypeBranchesPrepareSolverAtoms(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	f, m := d.f, d.f.r.manager
	x := f.constant("qe-selected-list", d.list)
	y := f.constant("qe-selected-other", d.list)
	recognizer := d.app(d.consTest, x)
	equality := f.own(m.MkEq(x, y))
	formula := f.own(m.MkAnd(recognizer, equality))
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	context, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	plugin, err := NewDatatypeSolverPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(plugin))
	defer func() { fmMust(t, context.Close()) }()
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(x))
	root, err := newExprQETree(m, f.r.config, nil, formula)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	atoms, err := NewBoolAtomIndex(m, []ast.ASTHandle{recognizer, equality}, nil)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	var free []ast.ASTRef
	defer func() {
		for i := range free {
			fmMust(t, m.DelRef(&free[i]))
		}
	}()
	selected, branchCount, found, err := selectExprQEVariable(root, variables, context, atoms, &free)
	fmMust(t, err)
	if !found || selected != x || branchCount.RatString() != "2" || len(free) != 0 {
		t.Fatal("recursive variable was not selected", found, branchCount.RatString(), len(free))
	}
	count, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().CleanupValue(&count)) }()
	ok, err := exprQEPluginBranches(plugin, contains, formula, []ast.ASTHandle{recognizer, equality}, nil, &count)
	fmMust(t, err)
	n, err := m.NumeralService().Unsigned(&count)
	fmMust(t, err)
	if !ok || n != 2 {
		t.Fatal("recursive equality branches", ok, n)
	}
	branch, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().CleanupValue(&branch)) }()
	fmMust(t, plugin.Assign(contains, formula, &branch))
	constraints, err := plugin.TakeConstraints()
	fmMust(t, err)
	if len(constraints) != 1 {
		t.Fatal("equality constraint count", len(constraints))
	}
	h, err := m.RefGet(constraints[0].Formula)
	fmMust(t, err)
	if h != equality {
		t.Fatal("wrong equality constraint")
	}
	fmMust(t, m.DelRef(&constraints[0].Formula))
	formulaRef, err := m.NewRef(formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&formulaRef)) }()
	definition, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&definition)) }()
	ok, err = plugin.SubstituteRecursive(contains, &branch, formulaRef, &definition)
	fmMust(t, err)
	if !ok {
		t.Fatal("prepared equality branch could not substitute")
	}
	h, err = m.RefGet(definition)
	fmMust(t, err)
	if h != y {
		t.Fatal("wrong branch definition")
	}
	unsupported := f.predicate("qe-opaque-datatype", x)
	unsupportedFormula := f.own(m.MkAnd(recognizer, unsupported))
	ok, err = exprQEPluginBranches(plugin, contains, unsupportedFormula, []ast.ASTHandle{recognizer, unsupported}, nil, &count)
	fmMust(t, err)
	if ok {
		t.Fatal("opaque occurrence was accepted for equality branching")
	}
}
