package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// arithmeticSolveFixture runs arith_qe_util::solve over a search tree whose
// root binds the given variables, so elim_var records definitions.
type arithmeticSolveFixture struct {
	f      *fmFixture
	m      *ast.ASTManager
	ctx    *SolverContext
	plugin *ArithmeticPlugin
	vars   *exprQEBranchVariables
	root   *exprQETree
	tree   *exprQETreeContext
}

func newArithmeticSolveFixture(t *testing.T, f *fmFixture, variables ...ast.ASTHandle) *arithmeticSolveFixture {
	t.Helper()
	m := f.r.manager
	s := &arithmeticSolveFixture{f: f, m: m}
	var err error
	s.ctx, err = NewSolverContext(m, variables)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, s.ctx.Close()) })
	s.plugin, err = NewArithmeticPlugin(s.ctx, f.r.config)
	fmMust(t, err)
	fmMust(t, s.ctx.AddPlugin(s.plugin))
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, rw.Close()) })
	adapter, err := NewSolverNNFAdapter(s.ctx)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, adapter.Close()) })
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, nnf.Close()) })
	s.vars, err = newExprQEBranchVariables(m)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, s.vars.Close()) })
	for i := len(variables) - 1; i >= 0; i-- {
		fmMust(t, s.vars.add(variables[i]))
	}
	s.root, err = newExprQETree(m, f.r.config, nil, f.own(m.MkTrue()))
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, s.root.Close()) })
	s.tree = &exprQETreeContext{current: s.root, variables: s.vars, config: f.r.config, rw: rw, nnf: nnf}
	return s
}

// solve runs one solve callback on formula and returns the eliminated
// variable, the unit child and its definition.
func (s *arithmeticSolveFixture) solve(t *testing.T, formula ast.ASTHandle) (ast.ASTHandle, *exprQETree, ast.ASTHandle) {
	t.Helper()
	fmMust(t, s.root.initialize(formula))
	fmMust(t, s.vars.consume(s.root))
	conjs, err := NewConjunctionEnum(s.m, formula)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	s.ctx.tree = s.tree
	solved, err := s.plugin.Solve(conjs, formula)
	s.ctx.tree = nil
	fmMust(t, err)
	if !solved || !s.root.isUnit() {
		t.Fatal("arithmetic equality was not solved")
	}
	x, err := s.m.RefGet(s.root.selected)
	fmMust(t, err)
	child := s.tree.current
	definition, err := child.definitions.Definition(0)
	fmMust(t, err)
	return x, child, definition
}

func arithmeticSolveContains(t *testing.T, m *ast.ASTManager, x, e ast.ASTHandle) bool {
	t.Helper()
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	found, err := contains.Check(e)
	fmMust(t, err)
	return found
}

func TestArithmeticEquationSingularInteger(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("solve-x", f.integer)
	y := f.constant("solve-y", f.integer)
	s := newArithmeticSolveFixture(t, f, x)
	left := f.own(m.MkArithmeticAdd(x, y))
	eliminated, child, definition := s.solve(t, f.own(m.MkEq(left, f.number("0"))))
	if eliminated != x || len(child.variables) != 0 {
		t.Fatal("integer unit equation metadata")
	}
	rewritten, err := m.RefGet(child.formula)
	fmMust(t, err)
	// solve_singular's definition is the unsimplified x - (x + y), as in
	// the source; only the substituted formula is simplified.
	if arithmeticSolveContains(t, m, x, rewritten) || arithmeticSolveContains(t, m, y, definition) == false {
		t.Fatal("integer result retained eliminated variable")
	}
}

func TestArithmeticEquationSingularNonUnitReal(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("solve-rx", f.real)
	y := f.constant("solve-ry", f.real)
	s := newArithmeticSolveFixture(t, f, x)
	two := f.own(m.ArithmeticRealNumeralText("2"))
	zero := f.own(m.ArithmeticRealNumeralText("0"))
	twoX := f.own(m.MkArithmeticMul(two, x))
	left := f.own(m.MkArithmeticAdd(twoX, y))
	eliminated, child, definition := s.solve(t, f.own(m.MkEq(left, zero)))
	if eliminated != x {
		t.Fatal("real equation eliminated the wrong variable")
	}
	rewritten, err := m.RefGet(child.formula)
	fmMust(t, err)
	if arithmeticSolveContains(t, m, x, rewritten) {
		t.Fatal("real result retained eliminated variable")
	}
	if sort, err := m.GetSort(definition); err != nil || sort != f.real {
		t.Fatal("real definition sort", sort, err)
	}
}

// An unused real variable does not prevent solving an integer row; the
// Omega step introduces an auxiliary integer.
func TestArithmeticEquationIntegerOmegaStep(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("omega-x", f.integer)
	y := f.constant("omega-y", f.integer)
	r := f.constant("omega-r", f.real)
	s := newArithmeticSolveFixture(t, f, x, y, r)
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	threeY := f.own(m.MkArithmeticMul(f.number("3"), y))
	linear := f.own(m.MkArithmeticAdd(twoX, threeY))
	left := f.own(m.MkArithmeticAdd(linear, f.number("1")))
	eliminated, child, definition := s.solve(t, f.own(m.MkEq(left, f.number("0"))))
	if eliminated != x || len(child.variables) != 3 {
		t.Fatal("integer Omega metadata")
	}
	rewritten, err := m.RefGet(child.formula)
	fmMust(t, err)
	if arithmeticSolveContains(t, m, x, rewritten) || arithmeticSolveContains(t, m, x, definition) {
		t.Fatal("Omega result retained eliminated variable")
	}
	fresh, err := m.RefGet(child.variables[2])
	fmMust(t, err)
	if fresh == y || fresh == r || !arithmeticSolveContains(t, m, fresh, definition) {
		t.Fatal("Omega definition lost auxiliary integer")
	}
}

func TestArithmeticEquationIntegerUnitAvoidsAuxiliary(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("unit-x", f.integer)
	s := newArithmeticSolveFixture(t, f, x)
	_, child, _ := s.solve(t, f.own(m.MkEq(x, f.number("0"))))
	if len(child.variables) != 0 || len(s.vars.entries) != 1 {
		t.Fatal("unit integer equation introduced an auxiliary")
	}
}
