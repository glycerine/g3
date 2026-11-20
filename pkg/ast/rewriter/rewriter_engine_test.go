package rewriter

// Source: src/ast/rewriter/rewriter_def.h:23-180,182-562,562-750;
// src/ast/rewriter/rewriter.h:365-408; src/ast/rewriter/bool_rewriter.h:181-200;
// src/test/var_subst.cpp:60-106.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestBoolRewriterStarOriginalTraversalAndIndependentTruthTables(t *testing.T) {
	m, c, g, p := boolRewriterConfigFixture(t)
	rw, e := newBoolRewriterStar(m, c, g, p)
	satOK(t, e)
	defer func() { satOK(t, rw.Close()) }()
	a, b, d := goalTestAtom(t, m, "star-a"), goalTestAtom(t, m, "star-b"), goalTestAtom(t, m, "star-d")
	mk := func(op int32, args ...ast.ASTHandle) ast.ASTHandle { return goalTestApp(t, m, op, args...) }
	truth, e := m.MkTrue()
	satOK(t, e)
	falsity, e := m.MkFalse()
	satOK(t, e)
	choose := func(c, a, b bool) bool {
		if c {
			return a
		}
		return b
	}
	cases := []struct {
		term, want ast.ASTHandle
		eval       func(bool, bool, bool) bool
	}{
		{mk(ast.ASTOpNot, mk(ast.ASTOpNot, a)), a, func(a, b, d bool) bool { return a }},
		{mk(ast.ASTOpAnd, truth, a, mk(ast.ASTOpOr, falsity, b), a), mk(ast.ASTOpAnd, a, b), func(a, b, d bool) bool { return a && b }},
		{mk(ast.ASTOpImplies, a, mk(ast.ASTOpAnd, b, truth)), mk(ast.ASTOpOr, mk(ast.ASTOpNot, a), b), func(a, b, d bool) bool { return !a || b }},
		{mk(ast.ASTOpXor, a, a), falsity, func(a, b, d bool) bool { return false }},
		{mk(ast.ASTOpDistinct, a, a), falsity, func(a, b, d bool) bool { return false }},
		{mk(ast.ASTOpIte, truth, a, mk(ast.ASTOpAnd, b, d)), a, func(a, b, d bool) bool { return a }},
		{mk(ast.ASTOpIte, a, truth, falsity), a, func(a, b, d bool) bool { return a }},
		{mk(ast.ASTOpOr, mk(ast.ASTOpAnd, a, falsity), mk(ast.ASTOpAnd, b, truth)), b, func(a, b, d bool) bool { return b }},
		{mk(ast.ASTOpIte, mk(ast.ASTOpNot, truth), a, mk(ast.ASTOpNot, b)), mk(ast.ASTOpNot, b), func(a, b, d bool) bool { return !b }},
		{mk(ast.ASTOpIte, a, b, d), mk(ast.ASTOpIte, a, b, d), func(a, b, d bool) bool { return choose(a, b, d) }},
	}
	out := astTestPin(t, m, ast.ASTHandle{})
	for _, tc := range cases {
		satOK(t, rw.Rewrite(tc.term, out))
		got, e := m.RefGet(out)
		satOK(t, e)
		if got != tc.want {
			t.Fatal("original recursive Boolean shape", got, tc.want)
		}
		for bits := 0; bits < 8; bits++ {
			want := tc.eval(bits&1 != 0, bits&2 != 0, bits&4 != 0)
			env := map[ast.ASTHandle]bool{a: bits&1 != 0, b: bits&2 != 0, d: bits&4 != 0}
			for _, term := range []ast.ASTHandle{tc.term, got} {
				if rewriterOracleEval(t, m, term, env) != want {
					t.Fatal("independent original/result truth table", bits, term)
				}
			}
			// The original goal translator requires XOR/IMPLIES/DISTINCT to
			// be simplified first. Check the emitted formula with actual CDCL.
			for _, term := range []ast.ASTHandle{got} {
				solver, _, _ := satSolverCheckFixture(t)
				tr, e := TestNewGoalTranslator(m, solver)
				satOK(t, e)
				forms := []ast.ASTHandle{term}
				for i, v := range []ast.ASTHandle{a, b, d} {
					if bits&(1<<i) == 0 {
						v, e = m.MkNot(v)
						satOK(t, e)
					}
					forms = append(forms, v)
				}
				satOK(t, tr.Compile(forms))
				answer, e := solver.Check(nil)
				satOK(t, e)
				if answer == lbool.Undef || (answer == lbool.True) != want {
					t.Fatal("independent recursive rewrite meaning", bits, answer, want)
				}
				satOK(t, tr.Close())
				satOK(t, solver.Close())
			}
		}
	}
}

// Finite test oracle only; this supplies no production rewriting or solver
// behavior, and evaluates each actual original/result AST independently.
func rewriterOracleEval(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, env map[ast.ASTHandle]bool) bool {
	t.Helper()
	if value, ok := env[h]; ok {
		return value
	}
	op, e := m.GetDeclKind(h)
	satOK(t, e)
	n, e := m.GetNumArgs(h)
	satOK(t, e)
	child := func(i uint32) bool { v, e := m.GetArg(h, i); satOK(t, e); return rewriterOracleEval(t, m, v, env) }
	switch op {
	case ast.ASTOpTrue:
		return true
	case ast.ASTOpFalse:
		return false
	case ast.ASTOpNot:
		return !child(0)
	case ast.ASTOpIff, ast.ASTOpEq:
		return child(0) == child(1)
	case ast.ASTOpXor, ast.ASTOpDistinct:
		return child(0) != child(1)
	case ast.ASTOpImplies:
		return !child(0) || child(1)
	case ast.ASTOpIte:
		if child(0) {
			return child(1)
		}
		return child(2)
	case ast.ASTOpAnd:
		for i := uint32(0); i < n; i++ {
			if !child(i) {
				return false
			}
		}
		return true
	case ast.ASTOpOr:
		for i := uint32(0); i < n; i++ {
			if child(i) {
				return true
			}
		}
		return false
	}
	t.Fatal("unsupported finite oracle term", h, op)
	return false
}

func TestBoolRewriterStarITEConstantFoldAndCancelReset(t *testing.T) {
	m, c, g, p := boolRewriterConfigFixture(t)
	rw, e := newBoolRewriterStar(m, c, g, p)
	satOK(t, e)
	defer func() { satOK(t, rw.Close()) }()
	atom := goalTestAtom(t, m, "star-lazy")
	truth, e := m.MkTrue()
	satOK(t, e)
	deep := atom
	for i := 0; i < 50; i++ {
		deep = goalTestApp(t, m, ast.ASTOpNot, deep)
	}
	ite := goalTestApp(t, m, ast.ASTOpIte, truth, atom, deep)
	out := astTestPin(t, m, atom)
	satOK(t, rw.Rewrite(ite, out))
	got, e := m.RefGet(out)
	satOK(t, e)
	if got != atom || rw.engine.numSteps != 1 {
		t.Fatal("source lazy ITE branch traversal", got, rw.engine.numSteps)
	}
	m.Limit.IncCancel()
	e = rw.Rewrite(deep, out)
	m.Limit.DecCancel()
	if e == nil || e.Error() != messages.Canceled {
		t.Fatal("source cancellation exception", e)
	}
	if len(rw.engine.core.frames) != 0 || len(rw.engine.core.scopes) != 0 {
		t.Fatal("cancel did not reset source stacks")
	}
	satOK(t, rw.Rewrite(deep, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != atom {
		t.Fatal("source reuse after cancellation reset")
	}
}

func TestBetaReducerOriginalBindingOrderAndBinderShift(t *testing.T) {
	m := astTestManager(t)
	rw, e := newBetaReducer(m)
	satOK(t, e)
	defer func() { satOK(t, rw.Close()) }()
	v := []ast.ASTHandle{shiftTestVar(t, m, 0), shiftTestVar(t, m, 1), shiftTestVar(t, m, 2), shiftTestVar(t, m, 3)}
	expr := goalTestApp(t, m, ast.ASTOpOr, v[0], v[1], v[2], v[3])
	out := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, rw.engine.SetBindings(v[:2]))
	satOK(t, rw.Rewrite(expr, out))
	got, e := m.RefGet(out)
	satOK(t, e)
	shiftTestIndices(t, m, got, []uint32{0, 1, 2, 3})
	satOK(t, rw.engine.SetInvBindings(v[:2]))
	satOK(t, rw.Rewrite(expr, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	shiftTestIndices(t, m, got, []uint32{1, 0, 2, 3})
	b, e := m.MkBoolSort()
	satOK(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: expr})
	satOK(t, e)
	astTestPin(t, m, q)
	satOK(t, rw.engine.Reset())
	satOK(t, rw.engine.SetInvBindings(v[:2]))
	satOK(t, rw.Rewrite(q, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	body, e := m.GetQuantifierBody(got)
	satOK(t, e)
	shiftTestIndices(t, m, body, []uint32{0, 2, 1, 3})
	// Null binding slots preserve the corresponding source variable.
	satOK(t, rw.engine.Reset())
	satOK(t, rw.engine.SetBindings([]ast.ASTHandle{{}, v[3]}))
	satOK(t, rw.Rewrite(expr, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	shiftTestIndices(t, m, got, []uint32{0, 3, 2, 3})
}

// Source configuration callbacks exercise traversal state rather than adding
// a second production simplifier.
type rewriterTestConfig struct {
	DefaultConfiguration
	replaceDecl, result      ast.ASTHandle
	status                   brStatus
	macroDecl, macroBody     ast.ASTHandle
	substFrom, substTo, skip ast.ASTHandle
	calls                    map[ast.ASTHandle]uint32
	stepLimit                uint32
	frameLimit               uint32
	rejectScopes             bool
	resets, cleanups         uint32
}

func (c *rewriterTestConfig) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, pr ast.ASTRef) (brStatus, error) {
	c.calls[d]++
	if d == c.replaceDecl {
		return c.status, c.manager.SetRef(out, c.result)
	}
	return brFailed, nil
}
func (c *rewriterTestConfig) PreVisit(t ast.ASTHandle) (bool, error) { return t != c.skip, nil }
func (c *rewriterTestConfig) GetSubst(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	return c.substTo, ast.ASTHandle{}, t == c.substFrom, nil
}
func (c *rewriterTestConfig) GetMacro(d ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	return c.macroBody, ast.ASTHandle{}, ast.ASTHandle{}, d == c.macroDecl, nil
}
func (c *rewriterTestConfig) MaxStepsExceeded(n uint32) (bool, error) {
	return c.stepLimit != 0 && n > c.stepLimit, nil
}
func (c *rewriterTestConfig) MaxFramesExceeded(n uint32) bool {
	return c.frameLimit != 0 && n >= c.frameLimit
}
func (c *rewriterTestConfig) MaxScopesExceeded(uint32) bool { return c.rejectScopes }
func (c *rewriterTestConfig) Reset()                        { c.resets++ }
func (c *rewriterTestConfig) Cleanup()                      { c.cleanups++ }
func rewriterTestDecl(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	b, e := m.MkBoolSort()
	satOK(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	satOK(t, e)
	astTestPin(t, m, d)
	return d
}
func rewriterTestApp(t *testing.T, m *ast.ASTManager, d, arg ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, e := m.MkApp(d, []ast.ASTHandle{arg})
	satOK(t, e)
	astTestPin(t, m, h)
	return h
}
func TestRewriterEngineOriginalBoundedRevisitDepthsAndSubstitution(t *testing.T) {
	m := astTestManager(t)
	a := goalTestAtom(t, m, "depth-a")
	b := goalTestAtom(t, m, "depth-b")
	f, g, h, k := rewriterTestDecl(t, m, "depth-f"), rewriterTestDecl(t, m, "depth-g"), rewriterTestDecl(t, m, "depth-h"), rewriterTestDecl(t, m, "depth-k")
	chain := rewriterTestApp(t, m, g, rewriterTestApp(t, m, h, rewriterTestApp(t, m, k, a)))
	term := rewriterTestApp(t, m, f, a)
	out := astTestPin(t, m, ast.ASTHandle{})
	for _, st := range []brStatus{brRewrite1, brRewrite2, brRewrite3, brRewriteFull} {
		cfg := rewriterTestConfig{DefaultConfiguration: DefaultConfiguration{m}, replaceDecl: f, result: chain, status: st, calls: map[ast.ASTHandle]uint32{}}
		rw, e := newRewriterEngine(m, &cfg)
		satOK(t, e)
		satOK(t, rw.Rewrite(term, out))
		got, e := m.RefGet(out)
		satOK(t, e)
		if got != chain || cfg.calls[f] != 1 || cfg.calls[g] != 1 {
			t.Fatal("source bounded rewrite root", st, got, cfg.calls)
		}
		wantH, wantK := uint32(0), uint32(0)
		if st >= brRewrite2 {
			wantH = 1
		}
		if st >= brRewrite3 {
			wantK = 1
		}
		if cfg.calls[h] != wantH || cfg.calls[k] != wantK {
			t.Fatal("original depth/status mapping", st, cfg.calls, wantH, wantK)
		}
		satOK(t, rw.Close())
	}
	// get_subst runs before the depth-zero guard and pre_visit.
	child := rewriterTestApp(t, m, h, a)
	replacement := rewriterTestApp(t, m, g, child)
	cfg := rewriterTestConfig{DefaultConfiguration: DefaultConfiguration{m}, replaceDecl: f, result: replacement, status: brRewrite1, substFrom: child, substTo: b, calls: map[ast.ASTHandle]uint32{}}
	rw, e := newRewriterEngine(m, &cfg)
	satOK(t, e)
	defer func() { satOK(t, rw.Close()) }()
	satOK(t, rw.Rewrite(term, out))
	got, e := m.RefGet(out)
	satOK(t, e)
	expected := rewriterTestApp(t, m, g, b)
	if got != expected || cfg.calls[h] != 0 {
		t.Fatal("substitution before depth guard", got, expected, cfg.calls)
	}
	satOK(t, rw.Reset())
	cfg.skip = term
	cfg.calls = map[ast.ASTHandle]uint32{}
	satOK(t, rw.Rewrite(term, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != term || len(cfg.calls) != 0 {
		t.Fatal("source pre_visit skip", got, cfg.calls)
	}
}
func TestRewriterEngineOriginalMaxStepsResumeAndCleanup(t *testing.T) {
	m := astTestManager(t)
	a := goalTestAtom(t, m, "resume-a")
	f := rewriterTestDecl(t, m, "resume-f")
	term := a
	for i := 0; i < 1000; i++ {
		term = rewriterTestApp(t, m, f, term)
	}
	cfg := rewriterTestConfig{DefaultConfiguration: DefaultConfiguration{m}, stepLimit: 3, calls: map[ast.ASTHandle]uint32{}}
	rw, e := newRewriterEngine(m, &cfg)
	satOK(t, e)
	defer func() { satOK(t, rw.Close()) }()
	out := astTestPin(t, m, a)
	e = rw.Rewrite(term, out)
	if e == nil || e.Error() != messages.MaxSteps || rw.numSteps != 4 || len(rw.core.frames) != 4 {
		t.Fatal("source interrupted frame prefix", e, rw.numSteps, len(rw.core.frames))
	}
	got, e := m.RefGet(out)
	satOK(t, e)
	if got != a {
		t.Fatal("interruption published output")
	}
	cfg.stepLimit = 0
	satOK(t, rw.Resume(out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != term || cfg.calls[f] != 1000 || len(rw.core.frames) != 0 || rw.numSteps <= 1000 {
		t.Fatal("source resume and iterative depth", got, cfg.calls[f], rw.numSteps)
	}
	satOK(t, rw.Reset())
	if cfg.resets != 1 {
		t.Fatal("configuration reset")
	}
	satOK(t, rw.Cleanup())
	if cfg.cleanups != 1 || cap(rw.bindings) != 0 || cap(rw.shifts) != 0 {
		t.Fatal("configuration cleanup")
	}
	satOK(t, rw.Rewrite(term, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != term {
		t.Fatal("reuse after cleanup")
	}
}
func TestRewriterEngineOriginalMacroExpansionUnderBinder(t *testing.T) {
	m := astTestManager(t)
	f := rewriterTestDecl(t, m, "macro-f")
	v0, v1, v2 := shiftTestVar(t, m, 0), shiftTestVar(t, m, 1), shiftTestVar(t, m, 2)
	body := goalTestApp(t, m, ast.ASTOpOr, v0, v2)
	arg := rewriterTestApp(t, m, f, v0)
	b, e := m.MkBoolSort()
	satOK(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: arg})
	satOK(t, e)
	astTestPin(t, m, q)
	cfg := rewriterTestConfig{DefaultConfiguration: DefaultConfiguration{m}, macroDecl: f, macroBody: body, calls: map[ast.ASTHandle]uint32{}}
	rw, e := newRewriterEngine(m, &cfg)
	satOK(t, e)
	defer func() { satOK(t, rw.Close()) }()
	out := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, rw.Rewrite(q, out))
	got, e := m.RefGet(out)
	satOK(t, e)
	actual, e := m.GetQuantifierBody(got)
	satOK(t, e)
	want := goalTestApp(t, m, ast.ASTOpOr, v0, v1)
	if actual != want || len(rw.bindings) != 0 || len(rw.shifts) != 0 || len(rw.core.scopes) != 0 || rw.core.numQVars != 0 {
		t.Fatal("source macro forward/inverse shifts", actual, want, rw.bindings, rw.core.numQVars)
	}
	// Ground macro definitions bypass recursive rewriting; variable definitions
	// still pass through the original explicit AST_VAR frame state.
	satOK(t, rw.Reset())
	cfg.macroBody = v0
	satOK(t, rw.Rewrite(arg, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != v0 {
		t.Fatal("variable macro definition", got)
	}
	satOK(t, rw.Reset())
	cfg.macroBody = goalTestAtom(t, m, "macro-ground")
	satOK(t, rw.Rewrite(arg, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != cfg.macroBody {
		t.Fatal("ground macro definition", got)
	}
}
func TestBetaReducerOriginalPatternAndNoPatternReplacement(t *testing.T) {
	m := astTestManager(t)
	rw, e := newBetaReducer(m)
	satOK(t, e)
	defer func() { satOK(t, rw.Close()) }()
	b, e := m.MkBoolSort()
	satOK(t, e)
	v0, v1 := shiftTestVar(t, m, 0), shiftTestVar(t, m, 1)
	a := goalTestAtom(t, m, "annotation-binding")
	body := goalTestApp(t, m, ast.ASTOpOr, v0, v1)
	pat, e := m.MkFamilyApp(2, 0, nil, []ast.ASTHandle{body})
	satOK(t, e)
	astTestPin(t, m, pat)
	out := astTestPin(t, m, ast.ASTHandle{})
	for _, pattern := range []bool{true, false} {
		satOK(t, rw.engine.Reset())
		satOK(t, rw.engine.SetInvBindings([]ast.ASTHandle{a}))
		spec := ast.ASTQuantifierSpec{Forall: false, Weight: 7, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body}
		if pattern {
			spec.Patterns = []ast.ASTHandle{pat}
		} else {
			spec.NoPatterns = []ast.ASTHandle{body}
		}
		q, e := m.MkQuantifier(spec)
		satOK(t, e)
		astTestPin(t, m, q)
		satOK(t, rw.Rewrite(q, out))
		got, e := m.RefGet(out)
		satOK(t, e)
		rewritten, e := m.GetQuantifierBody(got)
		satOK(t, e)
		want := goalTestApp(t, m, ast.ASTOpOr, v0, a)
		if rewritten != want {
			t.Fatal("body bindings", rewritten, want)
		}
		if pattern {
			p, e := m.GetPattern(got, 0)
			satOK(t, e)
			child, e := m.GetArg(p, 0)
			satOK(t, e)
			if child != want {
				t.Fatal("rewritten pattern", child, want)
			}
		} else {
			p, e := m.GetNoPattern(got, 0)
			satOK(t, e)
			if p != body {
				t.Fatal("original no-pattern is_pattern replacement guard", p, body)
			}
		}
		forall, e := m.IsForall(got)
		satOK(t, e)
		weight, e := m.GetQuantifierWeight(got)
		satOK(t, e)
		if forall || weight != 7 {
			t.Fatal("preserved quantifier metadata", forall, weight)
		}
	}
}

func TestRewriterEngineOriginalFrameScopeLimitsAndDisabledCancel(t *testing.T) {
	m := astTestManager(t)
	a := goalTestAtom(t, m, "limits-a")
	f := rewriterTestDecl(t, m, "limits-f")
	term := rewriterTestApp(t, m, f, rewriterTestApp(t, m, f, rewriterTestApp(t, m, f, a)))
	cfg := rewriterTestConfig{DefaultConfiguration: DefaultConfiguration{m}, frameLimit: 2, calls: map[ast.ASTHandle]uint32{}}
	rw, e := newRewriterEngine(m, &cfg)
	satOK(t, e)
	defer func() { satOK(t, rw.Close()) }()
	out := astTestPin(t, m, a)
	e = rw.Rewrite(term, out)
	if e == nil || e.Error() != messages.MaxFrames || len(rw.core.frames) != 2 {
		t.Fatal("source frame-limit exception", e, len(rw.core.frames))
	}
	cfg.frameLimit = 0
	satOK(t, rw.Reset())
	cfg.rejectScopes = true
	b, e := m.MkBoolSort()
	satOK(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: term})
	satOK(t, e)
	astTestPin(t, m, q)
	e = rw.Rewrite(q, out)
	if e == nil || e.Error() != messages.MaxScopes || len(rw.core.scopes) != 0 || len(rw.bindings) != 0 {
		t.Fatal("scope check before entering", e, rw.core.scopes, rw.bindings)
	}
	cfg.rejectScopes = false
	satOK(t, rw.Reset())
	satOK(t, rw.core.SetCancelCheck(false))
	m.Limit.IncCancel()
	e = rw.Rewrite(q, out)
	m.Limit.DecCancel()
	satOK(t, e)
	got, e := m.RefGet(out)
	satOK(t, e)
	if got != q {
		t.Fatal("source disabled cancellation", got, q)
	}
	copied := *rw
	if e = copied.Rewrite(term, out); e != ast.ErrASTHandle {
		t.Fatal("copied engine", e)
	}
	satOK(t, rw.Close())
	if e = rw.Rewrite(term, out); e != ast.ErrASTHandle {
		t.Fatal("closed engine", e)
	}
}
