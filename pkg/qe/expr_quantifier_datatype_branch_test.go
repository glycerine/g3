package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExprQEDatatypeSearchEliminatesRecognizedEquality(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	f, m := d.f, d.f.r.manager
	x := f.constant("search-datatype-x", d.list)
	y := f.constant("search-datatype-y", d.list)
	body := f.own(m.MkAnd(d.app(d.consTest, x), f.own(m.MkEq(x, y))))
	bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&bound)) }()
	quantifier, err := m.RefGet(bound)
	fmMust(t, err)
	result, complete, err := eliminateQuantifierDatatypeSearch(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("datatype search did not eliminate the variable")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	quantified, err := m.HasQuantifiers(h)
	fmMust(t, err)
	if quantified {
		t.Fatal("quantifier survived")
	}
	want := d.app(d.consTest, y)
	difference := f.own(m.MkNot(f.own(m.MkEq(h, want))))
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(difference))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("elimination is not equivalent to the constructor recognizer", status)
	}
}

func TestExprQEDatatypeBranchPropagatesAndSubstitutes(t *testing.T) {
	for _, recognized := range []bool{false, true} {
		name := "constructor"
		if recognized {
			name = "equality"
		}
		t.Run(name, func(t *testing.T) {
			d := newDatatypeProjectionFixture(t)
			f, m := d.f, d.f.r.manager
			kernel := qsatKernelOwner(t, f)
			solver, err := kernel.solver()
			fmMust(t, err)
			x := f.constant("branch-datatype-x", d.list)
			y := f.constant("branch-datatype-y", d.list)
			equality := f.own(m.MkEq(x, y))
			formula := equality
			if recognized {
				formula = f.own(m.MkAnd(d.app(d.consTest, x), equality))
			}
			context, err := NewSolverContext(m, []ast.ASTHandle{x})
			fmMust(t, err)
			defer func() { fmMust(t, context.Close()) }()
			plugin, err := NewDatatypeSolverPlugin(context)
			fmMust(t, err)
			fmMust(t, context.AddPlugin(plugin))
			contains, err := ast.NewContainsApp(m, x)
			fmMust(t, err)
			defer func() { fmMust(t, contains.Close()) }()
			if recognized {
				ok, e := plugin.Prepare(contains, formula, []ast.ASTHandle{d.app(d.consTest, x), equality}, nil)
				fmMust(t, e)
				if !ok {
					t.Fatal("equality preparation failed")
				}
			}
			adapter, err := NewSolverNNFAdapter(context)
			fmMust(t, err)
			defer func() { fmMust(t, adapter.Close()) }()
			nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, nnf.Close()) }()
			rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
			fmMust(t, err)
			defer func() { fmMust(t, rw.Close()) }()
			variables, err := newExprQEBranchVariables(m)
			fmMust(t, err)
			defer func() { fmMust(t, variables.Close()) }()
			fmMust(t, variables.add(x))
			truth := f.own(m.MkTrue())
			root, err := newExprQETree(m, f.r.config, nil, truth)
			fmMust(t, err)
			defer func() { fmMust(t, root.Close()) }()
			fmMust(t, root.initialize(formula))
			fmMust(t, variables.consume(root))
			fmMust(t, root.selectVariable(x, big.NewRat(2, 1)))
			branch := big.NewRat(0, 1)
			child, status, err := stepExprQEScalarBranch(root, variables, plugin, branch, f.r.config, rw, nnf, solver, true)
			fmMust(t, err)
			if status != exprQENeedPropagation || child.formula != (ast.ASTRef{}) {
				t.Fatal("branch did not request propagation")
			}
			assertions, err := solver.NumAssertions()
			fmMust(t, err)
			if assertions == 0 {
				t.Fatal("branch constraint was not asserted")
			}
			child, status, err = stepExprQEScalarBranch(root, variables, plugin, branch, f.r.config, rw, nnf, solver, true)
			fmMust(t, err)
			if status != exprQEChooseVariable {
				t.Fatal("branch did not substitute")
			}
			h, err := m.RefGet(child.formula)
			fmMust(t, err)
			escaped, err := contains.Check(h)
			fmMust(t, err)
			if escaped {
				t.Fatal("selected datatype variable survived")
			}
			n, err := child.definitions.Size()
			fmMust(t, err)
			if n != 1 {
				t.Fatal("missing branch definition")
			}
			if !recognized && len(variables.entries) != 2 {
				t.Fatal("fresh datatype variable not registered", len(variables.entries))
			}
		})
	}
}
