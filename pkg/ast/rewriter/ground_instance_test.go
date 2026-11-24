package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Compare the complete var_subst calls, including the polls on a shared DAG
// and its short-circuited branches. Keeping the outputs live in this manager
// makes equality of handles an equality check on canonical expressions.
func TestGroundInstancePlansConditionalSharing(t *testing.T) {
	m := astTestManager(t)
	s, err := m.MkBoolSort()
	satOK(t, err)
	x, y := shiftTestVar(t, m, 0), shiftTestVar(t, m, 1)
	left, right := goalTestAtom(t, m, "left"), goalTestAtom(t, m, "right")
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "f"), []ast.ASTHandle{s, s}, s, ast.ASTFuncDeclInfo{})
	satOK(t, err)
	predicate := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(f, []ast.ASTHandle{a, b})
		satOK(t, err)
		astTestPin(t, m, h)
		return h
	}
	ite := func(cond, a, b ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkIte(cond, a, b)
		satOK(t, err)
		astTestPin(t, m, h)
		return h
	}
	xy, yx := predicate(x, y), predicate(y, x)
	shared := ite(y, xy, yx)
	root := ite(ite(xy, shared, x), predicate(shared, xy), predicate(yx, shared))
	truth, err := m.MkTrue()
	satOK(t, err)
	falsity, err := m.MkFalse()
	satOK(t, err)
	reference, err := NewVarSubstitution(m, true)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, reference.Close()) })
	sub, err := NewVarSubstitution(m, true)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, sub.Close()) })
	var plans GroundInstancePlans
	t.Cleanup(func() { satOK(t, plans.Clear()) })
	want := astTestPin(t, m, ast.ASTHandle{})
	got := astTestPin(t, m, ast.ASTHandle{})
	limit := m.ResourceLimit()

	compare := func(term ast.ASTHandle, args []ast.ASTHandle, budget uint32) {
		t.Helper()
		run := func(planned bool, out ast.ASTRef) (ast.ASTHandle, uint64, string) {
			t.Helper()
			satOK(t, m.SetRef(out, right))
			if budget != 0 {
				satOK(t, limit.Push(budget))
				defer limit.Pop()
			}
			start := limit.Count()
			var err error
			if planned {
				err = plans.Substitute(sub, term, args, out)
			} else {
				err = reference.Substitute(term, args, out)
			}
			h, readErr := m.RefGet(out)
			satOK(t, readErr)
			if err != nil {
				if h != right {
					t.Fatal("interrupted substitution published a partial result")
				}
				return h, limit.Count() - start, err.Error()
			}
			return h, limit.Count() - start, ""
		}
		a, stepsA, errA := run(false, want)
		b, stepsB, errB := run(true, got)
		if a != b || stepsA != stepsB || errA != errB {
			t.Fatalf("bindings %v, budget %d: ordinary (%v,%d,%q), planned (%v,%d,%q)",
				args, budget, a, stepsA, errA, b, stepsB, errB)
		}
	}

	for i := 0; i < 5; i++ {
		compare(root, []ast.ASTHandle{left, right}, 2*maxGroundInstanceNodes-1)
	}
	if len(plans.bodies) != 0 {
		t.Fatal("a substitution near its resource limit started compiling a plan")
	}
	for i := 0; i < 8; i++ {
		compare(root, []ast.ASTHandle{left, right}, 2*maxGroundInstanceNodes)
	}
	if !ast.ASTValidateHandles && !diagnostic.TraceCompiled() && plans.bodies[root].plan == nil {
		t.Fatal("hot conditional body never compiled")
	}
	for _, args := range [][]ast.ASTHandle{
		{right, left}, {truth, left}, {falsity, right}, {left}, {{}, right}, {x, right},
	} {
		compare(root, args, 0)
		compare(root, args, 2*maxGroundInstanceNodes)
	}
	for budget := uint32(1); budget <= 20; budget++ {
		compare(root, []ast.ASTHandle{right, left}, budget)
	}
	limit.IncCancel()
	compare(root, []ast.ASTHandle{left, right}, 0)
	limit.DecCancel()
	compare(root, []ast.ASTHandle{left, right}, 0)
	always := ite(truth, xy, shared)
	for i := 0; i < 5; i++ {
		compare(always, []ast.ASTHandle{left, right}, 0)
	}
	if plans.bodies[always].plan != nil {
		t.Fatal("literal condition must take the short-circuiting traversal")
	}
	satOK(t, plans.Clear())
	if plans.manager != nil || plans.bodies != nil || cap(plans.held) != 0 {
		t.Fatal("clearing the solver retained traversal storage")
	}
}

func TestGroundInstancePlansDoNotPinRetiredTerms(t *testing.T) {
	m := astTestManager(t)
	x := shiftTestVar(t, m, 0)
	a := goalTestAtom(t, m, "instance-argument")
	body, err := m.MkNot(x)
	satOK(t, err)
	satOK(t, m.IncRef(body))
	s, err := NewVarSubstitution(m, true)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, s.Close()) })
	var plans GroundInstancePlans
	t.Cleanup(func() { satOK(t, plans.Clear()) })
	out := astTestPin(t, m, ast.ASTHandle{})
	for i := 0; i < 5; i++ {
		satOK(t, plans.Substitute(s, body, []ast.ASTHandle{a}, out))
	}
	result, err := m.RefGet(out)
	satOK(t, err)
	satOK(t, s.Reset())
	satOK(t, m.ResetRef(out))
	satOK(t, m.DecRef(body))
	if m.IsLiveHandle(body) || m.IsLiveHandle(result) {
		t.Fatal("traversal metadata kept a retired input or output alive")
	}
	replacement, err := m.MkNot(x)
	satOK(t, err)
	astTestPin(t, m, replacement)
	if replacement == body {
		t.Fatal("reconstructed formula reused a retired handle")
	}
	satOK(t, plans.Substitute(s, replacement, []ast.ASTHandle{a}, out))
	if plans.bodies[replacement].plan != nil {
		t.Fatal("new generation reused the retired formula's plan")
	}
}
