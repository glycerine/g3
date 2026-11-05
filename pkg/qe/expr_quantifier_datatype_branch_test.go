package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
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

func TestExprQEDatatypeSearchContinuesAfterFreshVariable(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	f, m := d.f, d.f.r.manager
	x := f.constant("search-fresh-datatype-x", d.list)
	y := f.constant("search-fresh-datatype-y", d.list)
	body := f.own(m.MkEq(x, y))
	bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&bound)) }()
	quantifier, err := m.RefGet(bound)
	fmMust(t, err)
	result, complete, err := eliminateQuantifierDatatypeSearch(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("search did not finish after introducing a datatype variable")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	negated := f.own(m.MkNot(h))
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(negated))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("projection should be valid", status)
	}
}

func TestExprQEDatatypeSearchWithBothEqualityPolarities(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	f, m := d.f, d.f.r.manager
	x := f.constant("search-both-datatype-x", d.list)
	y := f.constant("search-both-datatype-y", d.list)
	b := f.constant("search-both-datatype-b", f.boolean)
	eq := f.own(m.MkEq(x, y))
	positive := f.own(m.MkAnd(b, eq))
	negative := f.own(m.MkAnd(f.own(m.MkNot(b)), f.own(m.MkNot(eq))))
	body := f.own(m.MkAnd(d.app(d.consTest, x), f.own(m.MkOr(positive, negative))))
	bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&bound)) }()
	quantifier, err := m.RefGet(bound)
	fmMust(t, err)
	result, complete, err := eliminateQuantifierDatatypeSearch(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("both-polarity datatype search did not finish")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	want := f.own(m.MkOr(f.own(m.MkNot(b)), d.app(d.consTest, y)))
	different := f.own(m.MkNot(f.own(m.MkEq(h, want))))
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("datatype projection differs from expected result", status)
	}
}

func TestQuantifierDatatypeBlocks(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := "datatype"
		if mixed {
			name = "datatype-boolean"
		}
		t.Run(name, func(t *testing.T) {
			d := newDatatypeProjectionFixture(t)
			f, m := d.f, d.f.r.manager
			x := f.constant("block-datatype-x", d.list)
			y := f.constant("block-datatype-y", d.list)
			z := f.constant("block-datatype-z", d.list)
			body := f.own(m.MkAndArray([]ast.ASTHandle{d.app(d.consTest, x), f.own(m.MkEq(x, y)), f.own(m.MkEq(y, z))}))
			variables := []ast.ASTHandle{x, y}
			if mixed {
				b := f.constant("block-datatype-b", f.boolean)
				body = f.own(m.MkAnd(b, body))
				variables = append(variables, b)
			}
			bound, err := bindExprQuantifierVariables(m, variables, body)
			fmMust(t, err)
			defer func() { fmMust(t, m.DelRef(&bound)) }()
			q, err := m.RefGet(bound)
			fmMust(t, err)
			var result ast.ASTRef
			var complete bool
			if mixed {
				result, complete, err = eliminateQuantifierMixed(m, q, f.r.config)
			} else {
				result, complete, err = eliminateQuantifierDatatypeBlock(m, q, f.r.config)
			}
			fmMust(t, err)
			if !complete {
				t.Fatal("datatype block was not eliminated")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			want := d.app(d.consTest, z)
			difference := f.own(m.MkNot(f.own(m.MkEq(value, want))))
			kernel := qsatKernelOwner(t, f)
			solver, err := kernel.solver()
			fmMust(t, err)
			fmMust(t, solver.AssertFormula(difference))
			status, err := solver.CheckSat(nil)
			fmMust(t, err)
			if status != lbool.False {
				t.Fatal("datatype block projection differs from expected", status)
			}
		})
	}
}

func TestQuantifierDatatypeBlockUniversal(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	f, m := d.f, d.f.r.manager
	x := f.own(m.MkVar(1, d.list))
	y := f.own(m.MkVar(0, d.list))
	body := f.own(m.MkEq(x, y))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{d.list, d.list}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
	result, complete, err := eliminateQuantifierDatatypeBlock(m, q, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("universal datatype block was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(value))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("all lists cannot be equal", status)
	}
}

func TestExprQEFreshVariablePolicy(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	f, m := d.f, d.f.r.manager
	x := f.constant("policy-datatype", d.list)
	y := f.constant("policy-arithmetic", f.integer)
	_, err := m.RegisterBitVectorPlugin()
	fmMust(t, err)
	bvSort := f.own(m.BitVectorSort(3))
	z := f.constant("policy-bit-vector", bvSort)
	context, err := NewSolverContext(m, []ast.ASTHandle{x, y, z})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	datatypes, err := NewDatatypeSolverPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(datatypes))
	arithmetic, err := NewArithmeticPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(arithmetic))
	bitVectors, err := NewBitVectorPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(bitVectors))
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(x))
	ok, err := exprQENewVariablesSupported(variables, context, 0)
	fmMust(t, err)
	if !ok {
		t.Fatal("datatype variable was rejected")
	}
	fmMust(t, variables.add(y))
	ok, err = exprQENewVariablesSupported(variables, context, 1)
	fmMust(t, err)
	if !ok {
		t.Fatal("source arithmetic branch variable was rejected")
	}
	fmMust(t, variables.add(z))
	ok, err = exprQENewVariablesSupported(variables, context, 2)
	fmMust(t, err)
	if !ok {
		t.Fatal("bit-vector variable was rejected")
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
