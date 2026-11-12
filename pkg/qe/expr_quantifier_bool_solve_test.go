package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// exprQESolveFixture is a search tree over Boolean p, q whose solver
// context carries a Boolean plugin, as quant_elim_plugin's does.
type exprQESolveFixture struct {
	f         *fmFixture
	m         *ast.ASTManager
	p, q      ast.ASTHandle
	ctx       *SolverContext
	plugin    *BoolPlugin
	variables *exprQEBranchVariables
	root      *exprQETree
	tree      *exprQETreeContext
}

func newExprQESolveFixture(t *testing.T, name string, formula func(m *ast.ASTManager, p, q ast.ASTHandle) (ast.ASTHandle, error)) *exprQESolveFixture {
	t.Helper()
	f := qsatKernelFixture(t)
	m := f.r.manager
	s := &exprQESolveFixture{f: f, m: m}
	s.p = f.constant(name+"-p", f.boolean)
	s.q = f.constant(name+"-q", f.boolean)
	truth := f.own(m.MkTrue())
	var err error
	s.ctx, err = NewSolverContext(m, []ast.ASTHandle{s.p, s.q})
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, s.ctx.Close()) })
	s.plugin, err = NewBoolPlugin(s.ctx)
	fmMust(t, err)
	fmMust(t, s.ctx.AddPlugin(s.plugin))
	adapter, err := NewSolverNNFAdapter(s.ctx)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, adapter.Close()) })
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, nnf.Close()) })
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, rw.Close()) })
	s.variables, err = newExprQEBranchVariables(m)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, s.variables.Close()) })
	fmMust(t, s.variables.add(s.q))
	fmMust(t, s.variables.add(s.p))
	s.root, err = newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, s.root.Close()) })
	fml := f.own(formula(m, s.p, s.q))
	fmMust(t, s.root.initialize(fml))
	fmMust(t, s.variables.consume(s.root))
	s.root.atoms, err = NewBoolAtomIndex(m, []ast.ASTHandle{s.p, s.q}, nil)
	fmMust(t, err)
	s.tree = &exprQETreeContext{current: s.root, variables: s.variables, config: f.r.config, rw: rw, nnf: nnf}
	return s
}

// solveOnce runs the Boolean plugin's solve callback once with the context
// bound to the tree, as solve_vars does.
func (s *exprQESolveFixture) solveOnce(t *testing.T) bool {
	t.Helper()
	formula, err := s.m.RefGet(s.tree.current.formula)
	fmMust(t, err)
	conjs, err := NewConjunctionEnum(s.m, formula)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	s.ctx.tree = s.tree
	defer func() { s.ctx.tree = nil }()
	solved, err := s.plugin.SolveContext(conjs, formula, s.tree.current.atoms)
	fmMust(t, err)
	return solved
}

func TestExprQEBoolSolvePrefersUnitConjunctOrder(t *testing.T) {
	s := newExprQESolveFixture(t, "solve", func(m *ast.ASTManager, p, q ast.ASTHandle) (ast.ASTHandle, error) { return m.MkAnd(p, q) })
	m := s.m
	formula, err := m.RefGet(s.root.formula)
	fmMust(t, err)
	conjs, err := NewConjunctionEnum(m, formula)
	fmMust(t, err)
	first, err := conjs.At(0)
	fmMust(t, err)
	fmMust(t, conjs.Close())
	wantIndex, err := s.root.variableIndex(first)
	fmMust(t, err)
	if wantIndex != 1 {
		t.Fatal("test did not distinguish conjunct and variable order")
	}
	if !s.solveOnce(t) || !s.root.isUnit() {
		t.Fatal("unit conjunct was not solved")
	}
	selected, err := m.RefGet(s.root.selected)
	fmMust(t, err)
	if selected != first {
		t.Fatal("first unit conjunct was not selected")
	}
	child := s.tree.current
	definition, err := child.definitions.Definition(0)
	fmMust(t, err)
	if !m.IsTrue(definition) {
		t.Fatal("unit definition")
	}
	rewritten, err := m.RefGet(child.formula)
	fmMust(t, err)
	contains, err := ast.NewContainsApp(m, first)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	yes, err := contains.Check(rewritten)
	fmMust(t, err)
	if yes {
		t.Fatal("eliminated variable remains")
	}
}

func TestExprQEBoolSolvePolarizedVariableOrder(t *testing.T) {
	s := newExprQESolveFixture(t, "solve-polar", func(m *ast.ASTManager, p, q ast.ASTHandle) (ast.ASTHandle, error) { return m.MkOr(p, q) })
	m := s.m
	want, err := m.RefGet(s.root.variables[0])
	fmMust(t, err)
	if !s.solveOnce(t) {
		t.Fatal("polarized variable was not solved")
	}
	selected, err := m.RefGet(s.root.selected)
	fmMust(t, err)
	if selected != want {
		t.Fatal("first polarized variable was not selected")
	}
	definition, err := s.tree.current.definitions.Definition(0)
	fmMust(t, err)
	if !m.IsTrue(definition) {
		t.Fatal("positive polarity definition")
	}
}

func TestExprQEBoolSolveCreatesUnitBranch(t *testing.T) {
	s := newExprQESolveFixture(t, "solve-unit", func(m *ast.ASTManager, p, q ast.ASTHandle) (ast.ASTHandle, error) { return m.MkAnd(p, q) })
	m := s.m
	if !s.solveOnce(t) {
		t.Fatal("Boolean solve did not solve")
	}
	child := s.tree.current
	if !s.root.isUnit() || child == s.root || len(child.variables) != 1 {
		t.Fatal("Boolean solve did not publish unit branch")
	}
	remaining, err := m.RefGet(child.variables[0])
	fmMust(t, err)
	h, err := m.RefGet(child.formula)
	fmMust(t, err)
	if h != remaining {
		t.Fatal("unit substitution did not simplify to remaining variable")
	}
	count, err := child.definitions.Size()
	fmMust(t, err)
	if count != 1 {
		t.Fatal("unit definition missing")
	}
}

func TestExprQESolveVarsRepeatsUntilFixedPoint(t *testing.T) {
	s := newExprQESolveFixture(t, "solve-loop", func(m *ast.ASTManager, p, q ast.ASTHandle) (ast.ASTHandle, error) { return m.MkAnd(p, q) })
	m := s.m
	fmMust(t, solveExprQEVars(s.tree, s.ctx))
	leaf := s.tree.current
	if !s.root.isUnit() || !s.root.children[0].isUnit() || len(leaf.variables) != 0 {
		t.Fatal("expected two successive unit eliminations")
	}
	result, err := m.RefGet(leaf.formula)
	fmMust(t, err)
	if !m.IsTrue(result) {
		t.Fatal("expected the eliminated conjunction to simplify to true")
	}
	if s.ctx.tree != nil {
		t.Fatal("solve_vars left the context bound to the tree")
	}
	fmMust(t, solveExprQEVars(s.tree, s.ctx))
	if s.tree.current != leaf {
		t.Fatal("fixed point produced another branch")
	}
}
