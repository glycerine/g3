//go:build g3debug

package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Debug solver-assertion validation allocates while disabling virtual guards;
// the equivalent native reset can complete without any budget reservation.
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
	// Enough independent virtual guards to cross a real allocator boundary
	// when Reset disables them, without depending on a fixed allocation count.
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
	// Advance one solver retirement before refusing subsequent guards, so
	// the reset must preserve covers on both sides of its cleanup frontier.
	last := c.transformers[declarations[len(declarations)-1]]
	depthMust(t, last.solver.Close())
	depthMust(t, last.reachSolver.Close())
	budget.FailAfter(0)
	err = c.Reset()
	budget.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || !c.resetting {
		t.Fatal("reset did not reach a solver retirement refusal", err, c.resetting)
	}
	closed, pending := 0, 0
	for _, declaration := range declarations {
		transformer := c.transformers[declaration]
		if transformer == nil {
			t.Fatal("reset discarded a predicate before solver retirement completed")
		}
		if transformer.solver.closed {
			closed++
		} else if transformer.solver.closing {
			pending++
			if _, err := transformer.solver.LevelCount(); !errors.Is(err, ErrPropSolverOwner) {
				t.Fatal("partly closed solver accepted ordinary work", err)
			}
		}
		cover, err := c.CoverDelta(2, declaration)
		depthMust(t, err)
		body, err := m.RefGet(cover)
		depthMust(t, err)
		depthMust(t, m.DelRef(&cover))
		if !m.IsFalse(body) {
			t.Fatal("reset lost a preexisting cover", declaration, body)
		}
	}
	if closed == 0 || pending == 0 {
		t.Fatal("fixture missed partially completed solver retirement", closed, pending)
	}
	if status, err := c.Solve(0, 1); status != lbool.Undef || !errors.Is(err, ErrContextOwner) {
		t.Fatal("resetting context accepted a solve", status, err)
	}
	if err := c.AddCover(3, declarations[0], falsity); !errors.Is(err, ErrContextOwner) {
		t.Fatal("resetting context accepted a cover mutation", err)
	}
	if result, err := c.LastResult(); err != nil || result != lbool.Undef {
		t.Fatal("failed reset published its previous solve result", result, err)
	}
	if _, err := c.Model(f.query); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("failed reset exported its previous model", err)
	}
	if _, err := c.Answer(f.query); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("failed reset exported its previous answer", err)
	}
	depthMust(t, c.Reset())
	if c.resetting || len(c.transformers) != 0 || c.initialized {
		t.Fatal("reset retry did not finish teardown")
	}
	depthMust(t, c.AddCover(2, declarations[0], falsity))
	cover, err := c.CoverDelta(2, declarations[0])
	depthMust(t, err)
	body, err := m.RefGet(cover)
	depthMust(t, err)
	depthMust(t, m.DelRef(&cover))
	if !m.IsFalse(body) {
		t.Fatal("reset retry did not restore cover installation")
	}
}
