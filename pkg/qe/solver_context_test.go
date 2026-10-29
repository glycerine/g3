package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type solverContextPluginFixture struct {
	family       ast.FamilyID
	answer       ast.ASTHandle
	handle       bool
	calls        *[]ast.FamilyID
	closeCalls   int
	closeFailure error
}

func (p *solverContextPluginFixture) FamilyID() ast.FamilyID { return p.family }
func (p *solverContextPluginFixture) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	*p.calls = append(*p.calls, p.family)
	return p.answer, p.handle, nil
}
func (p *solverContextPluginFixture) IsUninterpreted(ast.ASTHandle) (bool, error) {
	return false, nil
}
func (p *solverContextPluginFixture) Close() error {
	p.closeCalls++
	if p.closeFailure != nil {
		err := p.closeFailure
		p.closeFailure = nil
		return err
	}
	return nil
}

func TestSolverContextSourcePluginDispatchAndLifecycle(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x := f.constant("solver-context-x", f.integer)
	p := f.constant("solver-context-p", f.boolean)
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x, p, x})
	fmMust(t, err)
	calls := []ast.FamilyID{}
	first := &solverContextPluginFixture{family: m.BasicFamilyID(), calls: &calls}
	second := &solverContextPluginFixture{family: f.r.arithmetic, answer: p, handle: true, calls: &calls}
	fmMust(t, ctx.AddPlugin(second))
	fmMust(t, ctx.AddPlugin(first))
	if err := ctx.AddPlugin(&solverContextPluginFixture{family: f.r.arithmetic, calls: &calls}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("duplicate plugin error = %v", err)
	}
	if err := ctx.AddPlugin(&solverContextPluginFixture{family: ast.NullFamilyID, calls: &calls}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("null-family plugin error = %v", err)
	}
	if n, err := ctx.NumVariables(); err != nil || n != 3 {
		t.Fatalf("variables = %d, %v", n, err)
	}
	if index, found, err := ctx.IsVariable(x); err != nil || !found || index != 0 {
		t.Fatalf("first variable = %d, %v, %v", index, found, err)
	}
	if has, err := ctx.HasPlugin(x); err != nil || !has {
		t.Fatalf("arithmetic plugin = %v, %v", has, err)
	}
	if plugin, err := ctx.PluginFor(x); err != nil || plugin != second {
		t.Fatalf("plugin lookup = %T, %v", plugin, err)
	}
	result, err := ctx.MkAtom(x, true)
	fmMust(t, err)
	if result != p || len(calls) != 2 || calls[0] != m.BasicFamilyID() || calls[1] != f.r.arithmetic {
		t.Fatalf("dispatch result/order = %v, %v", result, calls)
	}
	second.handle = false
	calls = calls[:0]
	result, err = ctx.MkAtom(p, false)
	fmMust(t, err)
	matched, child, err := m.IsNot(result)
	fmMust(t, err)
	if !matched || child != p {
		t.Fatal("negative fallback did not preserve source atom")
	}
	failure := errors.New("plugin close refusal")
	first.closeFailure = failure
	if err = ctx.Close(); !errors.Is(err, failure) || first.closeCalls != 1 || second.closeCalls != 0 {
		t.Fatalf("first close = %v, calls %d/%d", err, first.closeCalls, second.closeCalls)
	}
	fmMust(t, ctx.Close())
	if first.closeCalls != 2 || second.closeCalls != 1 {
		t.Fatalf("resumed close calls %d/%d", first.closeCalls, second.closeCalls)
	}
	if _, err = ctx.NumVariables(); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed context error = %v", err)
	}
}

func TestConjunctionEnumSourceOrderAndEqualityExtraction(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x, y := f.constant("conj-x", f.integer), f.constant("conj-y", f.integer)
	p := f.constant("conj-p", f.boolean)
	equality := f.own(m.MkEq(x, y))
	le := f.own(m.MkArithmeticLE(x, y))
	ge := f.own(m.MkArithmeticGE(x, y))
	nested := f.own(m.MkAndArray([]ast.ASTHandle{le, ge}))
	root := f.own(m.MkAndArray([]ast.ASTHandle{p, equality, nested}))
	conjunctions, err := NewConjunctionEnum(m, root)
	fmMust(t, err)
	if n, err := conjunctions.Len(); err != nil || n != 4 {
		t.Fatalf("flattened count = %d, %v", n, err)
	}
	// flatten_and replaces each expanded node with its last appended child;
	// this is the observable source traversal, not left-to-right syntax order.
	for i, want := range []ast.ASTHandle{ge, p, equality, le} {
		got, cause := conjunctions.At(uint32(i))
		fmMust(t, cause)
		if got != want {
			t.Fatalf("conjunct %d = %v, want %v", i, got, want)
		}
	}
	equalities, err := conjunctions.ExtractEqualities()
	fmMust(t, err)
	if len(equalities) != 2 {
		t.Fatalf("equalities = %d, want 2", len(equalities))
	}
	want, err := m.MkArithmeticSub(x, y)
	fmMust(t, err)
	for i := range equalities {
		got, cause := m.RefGet(equalities[i])
		fmMust(t, cause)
		if got != want {
			t.Fatalf("equality %d = %v, want %v", i, got, want)
		}
	}
	fmMust(t, conjunctions.Close())
	for i := len(equalities) - 1; i >= 0; i-- {
		fmMust(t, m.DelRef(&equalities[i]))
	}
	if _, err = conjunctions.At(0); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed conjunction error = %v", err)
	}
}
