package rewriter

// Source: src/ast/rewriter/var_subst.cpp:26-40;
// src/ast/rewriter/var_subst.h:35-53; src/test/var_subst.cpp:60-106.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/messages"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

// Replay the two upstream ENSURE identities through the actual var_subst owner.
func TestVarSubstitutionOriginalRegisteredIdentities(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkUninterpretedSort(astTestSymbol(t, m, "S"), nil)
	satOK(t, e)
	astTestPin(t, m, sort)
	b, e := m.MkBoolSort()
	satOK(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "p"), []ast.ASTHandle{sort, sort}, b, ast.ASTFuncDeclInfo{})
	satOK(t, e)
	astTestPin(t, m, d)
	vars := make([]ast.ASTHandle, 4)
	for i := range vars {
		vars[i], e = m.MkVar(uint32(i), sort)
		satOK(t, e)
		astTestPin(t, m, vars[i])
	}
	app := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(d, []ast.ASTHandle{a, b})
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	and := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkAnd(a, b)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	quant := func(n int, body ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort, sort}[:n], Names: []usymbol.Symbol{astTestSymbol(t, m, "y"), astTestSymbol(t, m, "x")}[:n], Body: body})
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	body := and(app(vars[0], vars[1]), app(vars[2], vars[3]))
	wants := []ast.ASTHandle{quant(1, and(app(vars[0], vars[2]), app(vars[1], vars[3]))), quant(2, and(app(vars[0], vars[1]), app(vars[3], vars[2])))}
	s, e := NewVarSubstitution(m, true)
	satOK(t, e)
	defer func() { satOK(t, s.Close()) }()
	out := astTestPin(t, m, ast.ASTHandle{})
	for i, want := range wants {
		satOK(t, s.Substitute(quant(i+1, body), vars[:2], out))
		got, e := m.RefGet(out)
		satOK(t, e)
		if got != want {
			t.Fatal("source ENSURE identity", i, got, want)
		}
	}
}

func TestVarSubstitutionSharedConditionalMatchesGeneralRewriter(t *testing.T) {
	m := astTestManager(t)
	x, y := shiftTestVar(t, m, 0), shiftTestVar(t, m, 1)
	shared := goalTestApp(t, m, ast.ASTOpOr, x, y)
	body := goalTestApp(t, m, ast.ASTOpIte, x, shared, goalTestApp(t, m, ast.ASTOpNot, shared))
	first := goalTestAtom(t, m, "first")
	truth, err := m.MkTrue()
	satOK(t, err)
	falsity, err := m.MkFalse()
	satOK(t, err)

	specialized, err := NewVarSubstitution(m, true)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, specialized.Close()) })
	general, err := NewVarSubstitution(m, true)
	satOK(t, err)
	general.reducer.engine.substituteVariables = false
	t.Cleanup(func() { satOK(t, general.Close()) })
	got, want := astTestPin(t, m, ast.ASTHandle{}), astTestPin(t, m, ast.ASTHandle{})
	for _, bindings := range [][]ast.ASTHandle{{first, truth}, {y, falsity}, {x, first}, {first}, nil} {
		satOK(t, specialized.Substitute(body, bindings, got))
		satOK(t, general.Substitute(body, bindings, want))
		actual, err := m.RefGet(got)
		satOK(t, err)
		expected, err := m.RefGet(want)
		satOK(t, err)
		if actual != expected {
			t.Fatalf("bindings %v produced %v, general rewriter produced %v", bindings, actual, expected)
		}
	}
}

func TestVarSubstitutionOrderResetNullAndGround(t *testing.T) {
	m := astTestManager(t)
	v := []ast.ASTHandle{shiftTestVar(t, m, 0), shiftTestVar(t, m, 1), shiftTestVar(t, m, 2), shiftTestVar(t, m, 3)}
	expr := goalTestApp(t, m, ast.ASTOpOr, v...)
	out := astTestPin(t, m, ast.ASTHandle{})
	for _, standard := range []bool{false, true} {
		s, e := NewVarSubstitution(m, standard)
		satOK(t, e)
		if s.StandardOrder() != standard {
			t.Fatal("order flag")
		}
		satOK(t, s.Substitute(expr, v[:2], out))
		got, e := m.RefGet(out)
		satOK(t, e)
		expected := []uint32{0, 1, 2, 3}
		if standard {
			expected = []uint32{1, 0, 2, 3}
		}
		shiftTestIndices(t, m, got, expected)
		satOK(t, s.Substitute(expr, nil, out))
		got, e = m.RefGet(out)
		satOK(t, e)
		if got != expr {
			t.Fatal("previous bindings retained")
		}
		satOK(t, s.Substitute(expr, []ast.ASTHandle{{}, v[3]}, out))
		got, e = m.RefGet(out)
		satOK(t, e)
		expected = []uint32{0, 3, 2, 3}
		if standard {
			expected = []uint32{3, 1, 2, 3}
		}
		shiftTestIndices(t, m, got, expected)
		satOK(t, s.Reset())
		if len(s.reducer.engine.bindings) != 0 {
			t.Fatal("Reset retained bindings")
		}
		truth, e := m.MkTrue()
		satOK(t, e)
		satOK(t, s.Substitute(truth, nil, out))
		got, e = m.RefGet(out)
		satOK(t, e)
		if got != truth {
			t.Fatal(got)
		}
		copied := *s
		if e = copied.Substitute(expr, nil, out); !errors.Is(e, ast.ErrASTHandle) {
			t.Fatal("copied owner", e)
		}
		satOK(t, s.Close())
		if e = s.Substitute(expr, nil, out); e == nil {
			t.Fatal("closed owner accepted")
		}
	}
}

func TestVarSubstitutionCancellationPreservesOutputAndReuses(t *testing.T) {
	m := astTestManager(t)
	s, e := NewVarSubstitution(m, false)
	satOK(t, e)
	defer func() { satOK(t, s.Close()) }()
	v := shiftTestVar(t, m, 0)
	a := goalTestAtom(t, m, "subst-cancel")
	out := astTestPin(t, m, a)
	m.Limit.IncCancel()
	e = s.Substitute(v, []ast.ASTHandle{a}, out)
	m.Limit.DecCancel()
	if e == nil || e.Error() != messages.Canceled {
		t.Fatal(e)
	}
	got, e := m.RefGet(out)
	satOK(t, e)
	if got != a {
		t.Fatal("cancellation published output")
	}
	satOK(t, s.Substitute(v, []ast.ASTHandle{a}, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != a {
		t.Fatal(got)
	}
}

// The embedded reducer constructor can stop inside any original member. The
// partial enclosing owner keeps those members available for explicit cleanup.
func TestVarSubstitutionColdConstructionCleanup(t *testing.T) {
	m := astTestManager(t)
	// AST ref pools retain reusable slots; warm their directory before measuring
	// the enclosing constructor, whose members still start empty on every pass.
	warm, e := NewVarSubstitution(m, true)
	satOK(t, e)
	satOK(t, warm.Close())
	baseline := m.Budget().Used()
	completed := false
	for fault := int64(0); fault < 128; fault++ {
		m.Budget().FailAfter(fault)
		s, e := NewVarSubstitution(m, true)
		m.Budget().FailAfter(-1)
		if s != nil {
			satOK(t, s.Close())
		}
		if m.Budget().Used() != baseline {
			t.Fatal("partial constructor leaked", fault, baseline, m.Budget().Used())
		}
		if e == nil {
			completed = true
			break
		}
	}
	if !completed {
		t.Fatal("constructor did not complete")
	}
}

func TestVarSubstitutionFreeReplacementShiftAndDeepTraversal(t *testing.T) {
	m := astTestManager(t)
	s, e := NewVarSubstitution(m, false)
	satOK(t, e)
	defer func() { satOK(t, s.Close()) }()
	b, e := m.MkBoolSort()
	satOK(t, e)
	body := goalTestApp(t, m, ast.ASTOpOr, shiftTestVar(t, m, 0), shiftTestVar(t, m, 1), shiftTestVar(t, m, 2))
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	satOK(t, e)
	astTestPin(t, m, q)
	out := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, s.Substitute(q, []ast.ASTHandle{shiftTestVar(t, m, 3)}, out))
	got, e := m.RefGet(out)
	satOK(t, e)
	gotBody, e := m.GetQuantifierBody(got)
	satOK(t, e)
	shiftTestIndices(t, m, gotBody, []uint32{0, 4, 2})
	term := shiftTestVar(t, m, 0)
	atom := goalTestAtom(t, m, "subst-deep")
	for i := 0; i < 1000; i++ {
		term = goalTestApp(t, m, ast.ASTOpNot, term)
	}
	satOK(t, s.Substitute(term, []ast.ASTHandle{atom}, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	for i := 0; i < 1000; i++ {
		got, e = m.GetArg(got, 0)
		satOK(t, e)
	}
	if got != atom {
		t.Fatal("deep substitution", got, atom)
	}
	if len(s.reducer.engine.core.frames) != 0 {
		t.Fatal("completed substitution retained frames")
	}
}

func TestVarSubstitutionBorrowedAppArgumentsOrderAndReset(t *testing.T) {
	m := astTestManager(t)
	a, b := goalTestAtom(t, m, "borrowed-subst-a"), goalTestAtom(t, m, "borrowed-subst-b")
	app := goalTestApp(t, m, ast.ASTOpOr, a, b)
	args, e := m.AppArgumentView(app)
	satOK(t, e)
	variable := shiftTestVar(t, m, 0)
	out := astTestPin(t, m, ast.ASTHandle{})
	for _, standard := range []bool{false, true} {
		s, e := NewVarSubstitution(m, standard)
		satOK(t, e)
		satOK(t, s.SubstituteArguments(variable, args, out))
		got, e := m.RefGet(out)
		satOK(t, e)
		want := a
		if standard {
			want = b
		}
		if got != want {
			t.Fatal("borrowed binding order", standard, got, want)
		}
		satOK(t, s.SubstituteArguments(variable, ast.SliceArguments(nil), out))
		got, e = m.RefGet(out)
		satOK(t, e)
		if got != variable {
			t.Fatal("borrowed invocation reset")
		}
		satOK(t, s.Close())
	}
}

func TestVarSubstitutionShiftersUseConstructorCaches(t *testing.T) {
	m := astTestManager(t)
	s, err := NewVarSubstitution(m, true)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, s.Close()) })
	r := &s.reducer.engine
	forward, inverse := r.shifter.cache, r.invShifter.cache
	if forward == nil || inverse == nil || forward == inverse || r.shifter.active != nil || r.invShifter.active != nil {
		t.Fatal("expected two distinct root caches at construction")
	}
	satOK(t, r.Reset())
	used := m.Budget().Used()
	m.Budget().FailAfter(0)
	defer m.Budget().FailAfter(-1)
	f := r.shifter.get(m, false)
	inv := r.invShifter.get(m, true)
	if f.core.cache != forward || inv.core.cache != inverse || f.inverse || !inv.inverse {
		t.Fatal("first shift lost constructor cache or direction")
	}
	if r.shifter.get(m, false) != f || r.invShifter.get(m, true) != inv || m.Budget().Used() != used {
		t.Fatal("first shift must use the already reserved capacity")
	}
	satOK(t, r.Reset())
}
