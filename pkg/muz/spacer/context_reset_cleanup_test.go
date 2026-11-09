//go:build g3debug

package spacer

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Solver retirement allocates nothing: ~itp_solver does not pop, and the
// virtual solver factory releases its solvers without validating
// assertions. A reset under a zero allocation allowance therefore completes,
// and covers can be installed again afterwards.
func TestSpacerResetPreservesAllCoversDuringSolverRetirementFailure(t *testing.T) {
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("reset cleanup leaked bytes", budget.Used())
		}
	})
	f := newContextAnswerFixture(t, true)
	c, m := f.context, f.context.manager
	budget = m.Budget()
	status, err := c.Solve(0, 3)
	depthMust(t, err)
	if status != lbool.False {
		t.Fatal("recursive program was not proved unreachable", status)
	}
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	falsity, err := m.MkFalse()
	depthMust(t, err)
	var declarations []ast.ASTHandle
	for i := 0; i < 128; i++ {
		name, err := m.Symbols.Intern(fmt.Sprintf("reset-cover-%d", i))
		depthMust(t, err)
		declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		depthMust(t, err)
		root, err := m.NewRef(declaration)
		depthMust(t, err)
		t.Cleanup(func() { depthMust(t, m.DelRef(&root)) })
		depthMust(t, c.AddCover(2, declaration, falsity))
		declarations = append(declarations, declaration)
	}
	// One solver already retired before the reset.
	last := c.transformers[declarations[len(declarations)-1]]
	depthMust(t, last.solver.Close())
	depthMust(t, last.reachSolver.Close())
	budget.FailAfter(0)
	err = c.Reset()
	budget.FailAfter(-1)
	if err != nil || c.resetting {
		t.Fatal("solver retirement reserved storage during reset", err, c.resetting)
	}
	if len(c.transformers) != 0 || c.initialized {
		t.Fatal("reset did not finish teardown")
	}
	if result, err := c.LastResult(); err != nil || result != lbool.Undef {
		t.Fatal("reset kept its previous solve result", result, err)
	}
	depthMust(t, c.AddCover(2, declarations[0], falsity))
	cover, err := c.CoverDelta(2, declarations[0])
	depthMust(t, err)
	body, err := m.RefGet(cover)
	depthMust(t, err)
	depthMust(t, m.DelRef(&cover))
	if !m.IsFalse(body) {
		t.Fatal("reset did not restore cover installation")
	}
}
