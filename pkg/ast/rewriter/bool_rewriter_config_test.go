package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:23-87;
// src/ast/rewriter/bool_rewriter.h:85-107,113-126,132-156;
// src/ast/rewriter/bool_rewriter_params.hpp:6-30.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func boolRewriterConfigFixture(t testing.TB) (*ast.ASTManager, *uparams.ParamsContext, *uparams.GlobalParamsService, *uparams.ParamsRef) {
	t.Helper()
	c, g, p, b := dynAckSourceFixture(t)
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: b})
	satOK(t, e)
	t.Cleanup(func() { satOK(t, m.Close()) })
	return m, c, g, p
}

func TestBoolRewriterConstructorDefaultsFallbackAndCostRetention(t *testing.T) {
	m, c, g, p := boolRewriterConfigFixture(t)
	rw, e := newBoolRewriter(m, c, g, p)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, rw.Close()) })
	if !rw.flat || rw.elimAnd || rw.localContext || rw.iteExtraRules || rw.blastDistinct || rw.localContextLimit != ^uint32(0) || rw.blastDistinctThreshold != ^uint32(0) || rw.localContextCost != 0 {
		t.Fatal("source constructor defaults", rw)
	}
	satOK(t, g.SourceSetName("rewriter.flat", "false"))
	satOK(t, g.SourceSetName("rewriter.ite_extra_rules", "true"))
	satOK(t, g.SourceSetName("rewriter.blast_distinct_threshold", "11"))
	key := "flat"
	satOK(t, c.SetBoolName(p, &key, true))
	key = "elim_and"
	satOK(t, c.SetBoolName(p, &key, true))
	key = "local_ctx_limit"
	satOK(t, c.SetUIntName(p, &key, 77))
	rw.localContextCost = 123
	satOK(t, rw.UpdtParams(p))
	if !rw.flat || !rw.elimAnd || !rw.iteExtraRules || rw.localContextLimit != 77 || rw.blastDistinctThreshold != 11 || rw.localContextCost != 123 {
		t.Fatal("local/module/default order or cost reset", rw)
	}
	empty, e := c.NewValue()
	satOK(t, e)
	t.Cleanup(func() { satOK(t, c.Del(&empty)) })
	satOK(t, rw.UpdtParams(&empty))
	if rw.flat || rw.elimAnd || !rw.iteExtraRules || rw.localContextLimit != ^uint32(0) || rw.localContextCost != 123 {
		t.Fatal("fresh helper module read", rw)
	}
	rw.ResetLocalContextCost()
	if rw.localContextCost != 0 {
		t.Fatal("source explicit cost reset")
	}
	copyOwner := *rw
	if e = copyOwner.UpdtParams(p); e != uparams.ErrParamsOwner {
		t.Fatal("copied configured owner accepted", e)
	}
	satOK(t, rw.Close())
	if e = rw.UpdtParams(p); e != uparams.ErrParamsOwner {
		t.Fatal("retired configured owner accepted", e)
	}
}

func TestBoolRewriterConjunctionEliminationOwningBufferAndTruthTables(t *testing.T) {
	for _, n := range []int{0, 1, 3, 17, 40} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			m := astTestManager(t)
			rw := boolRewriter{manager: m, flat: true, elimAnd: true}
			var args []ast.ASTHandle
			var refs []uint32
			for i := 0; i < n; i++ {
				h := goalTestAtom(t, m, fmt.Sprintf("elim-%d", i))
				args = append(args, h)
				r, e := m.GetRefCount(h)
				satOK(t, e)
				refs = append(refs, r)
			}
			out := astTestPin(t, m, ast.ASTHandle{})
			status, e := rw.mkAndCore(args, out)
			satOK(t, e)
			got, e := m.RefGet(out)
			satOK(t, e)
			if status != brDone {
				t.Fatal("source elimination status", status)
			}
			if n == 0 {
				truth, e := m.MkTrue()
				satOK(t, e)
				if got != truth {
					t.Fatal("empty elimination")
				}
			}
			if n == 1 {
				if got != args[0] {
					t.Fatal("singleton elimination")
				}
			}
			if n > 1 {
				negative, or, e := m.IsNot(got)
				satOK(t, e)
				if !negative {
					t.Fatal("source negated disjunction")
				}
				count, e := m.GetNumArgs(or)
				satOK(t, e)
				if int(count) != n {
					t.Fatal("spill/order", count)
				}
				for i, a := range args {
					h, e := m.GetArg(or, uint32(i))
					satOK(t, e)
					neg, inner, e := m.IsNot(h)
					satOK(t, e)
					if !neg || inner != a {
						t.Fatal("literal source order", i)
					}
				}
			}
			if n == 3 {
				for bits := 0; bits < 8; bits++ {
					s, _, _ := satSolverCheckFixture(t)
					tr, e := TestNewGoalTranslator(m, s)
					satOK(t, e)
					forms := []ast.ASTHandle{got}
					for i, h := range args {
						if bits&(1<<i) == 0 {
							h, e = m.MkNot(h)
							satOK(t, e)
						}
						forms = append(forms, h)
					}
					satOK(t, tr.Compile(forms))
					r, e := s.Check(nil)
					satOK(t, e)
					if (r == lbool.True) != (bits == 7) || r == lbool.Undef {
						t.Fatal("independent elimination", bits, r)
					}
					satOK(t, tr.Close())
					satOK(t, s.Close())
				}
			}
			satOK(t, m.ResetRef(out))
			for i, h := range args {
				r, e := m.GetRefCount(h)
				satOK(t, e)
				if r != refs[i] {
					t.Fatal("owning intermediate buffer leaked references", i, r, refs[i])
				}
			}
		})
	}
}

func TestBoolRewriterConjunctionEliminationReleasesEarlierTemporaryOnLaterFailure(t *testing.T) {
	m := astTestManager(t)
	rw := boolRewriter{manager: m, flat: true, elimAnd: true}
	first := goalTestAtom(t, m, "first-conjunct")
	before, err := m.GetRefCount(first)
	satOK(t, err)
	out := astTestPin(t, m, ast.ASTHandle{})
	if err := rw.mkAndAsOr([]ast.ASTHandle{first, {}}, out); err == nil {
		t.Fatal("invalid later conjunct was accepted")
	}
	after, err := m.GetRefCount(first)
	satOK(t, err)
	if after != before || len(rw.andArgs) != 0 || rw.andDepth != 0 {
		t.Fatal("earlier ref_buffer entry survived later argument failure", before, after, rw.andArgs, rw.andDepth)
	}
}

func TestBoolRewriterAppDispatcherAndFallbackDepths(t *testing.T) {
	m := astTestManager(t)
	rw := boolRewriter{manager: m, flat: true, elimAnd: true}
	p := goalTestAtom(t, m, "dispatch-p")
	q := goalTestAtom(t, m, "dispatch-q")
	truth, e := m.MkTrue()
	satOK(t, e)
	falsity, e := m.MkFalse()
	satOK(t, e)
	out := astTestPin(t, m, truth)
	for _, tc := range []struct {
		op     int32
		args   []ast.ASTHandle
		status brStatus
	}{
		{ast.ASTOpIff, []ast.ASTHandle{p, p}, brDone}, {ast.ASTOpDistinct, []ast.ASTHandle{p, q}, brRewrite2}, {ast.ASTOpAnd, []ast.ASTHandle{p, q}, brDone}, {ast.ASTOpOr, []ast.ASTHandle{p, q}, brFailed}, {ast.ASTOpNot, []ast.ASTHandle{truth}, brDone}, {ast.ASTOpIte, []ast.ASTHandle{truth, p, q}, brDone}, {ast.ASTOpImplies, []ast.ASTHandle{p, q}, brDone}, {ast.ASTOpXor, []ast.ASTHandle{p, q}, brDone}, {ast.ASTOpTrue, nil, brFailed},
	} {
		app := goalTestApp(t, m, tc.op, tc.args...)
		decl, e := m.GetDecl(app)
		satOK(t, e)
		satOK(t, m.SetRef(out, truth))
		status, e := rw.mkAppCore(decl, tc.args, out)
		satOK(t, e)
		if status != tc.status {
			t.Fatal("source dispatcher", tc.op, status, tc.status)
		}
		satOK(t, rw.mkApp(decl, tc.args, out))
		got, e := m.RefGet(out)
		satOK(t, e)
		if got.IsNull() {
			t.Fatal("raw application fallback failed")
		}
		if tc.op == ast.ASTOpNot && got != falsity {
			t.Fatal("NOT dispatch result")
		}
	}
	xor := goalTestApp(t, m, ast.ASTOpXor, p, q)
	decl, e := m.GetDecl(xor)
	satOK(t, e)
	satOK(t, m.SetRef(out, truth))
	status, e := rw.mkAppCore(decl, nil, out)
	satOK(t, e)
	if status != brFailed {
		t.Fatal("zero XOR shortcut")
	}
	status, e = rw.mkAppCore(decl, []ast.ASTHandle{p}, out)
	satOK(t, e)
	got, e := m.RefGet(out)
	satOK(t, e)
	if status != brDone || got != p {
		t.Fatal("one XOR shortcut")
	}
	_, e = rw.mkAppCore(decl, []ast.ASTHandle{p, q, p}, out)
	if e != ast.ErrASTDomain {
		t.Fatal("source unreachable XOR arity", e)
	}
}

func TestBoolRewriterConstructorHelperRefusalRetiresReachedHeader(t *testing.T) {
	m, c, g, p := boolRewriterConfigFixture(t)
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(1)
	rw, e := newBoolRewriter(m, c, g, p)
	m.BudgetField.FailAfter(-1)
	if !errors.Is(e, arena.ErrInjectedFailure) || rw == nil {
		t.Fatal("reached helper refusal", rw, e)
	}
	satOK(t, rw.Close())
	if m.BudgetField.Used() != before {
		t.Fatal("reached constructor owner leak", m.BudgetField.Used(), before)
	}
}

func TestBoolRewriterDispatcherFamilyAssertionAndNativeFallback(t *testing.T) {
	m := astTestManager(t)
	rw := boolRewriter{manager: m, flat: true}
	p := goalTestAtom(t, m, "dispatch-family-p")
	truth, e := m.MkTrue()
	satOK(t, e)
	out := astTestPin(t, m, truth)
	decl, e := m.GetDecl(p)
	satOK(t, e)
	status, e := rw.mkAppCore(decl, nil, out)
	if ast.ASTSourceDebug {
		if e != ast.ErrASTDomain {
			t.Fatal("source debug family assertion", e)
		}
	} else {
		satOK(t, e)
		if status != brFailed {
			t.Fatal("source native unknown-kind fallback", status)
		}
	}
	got, e := m.RefGet(out)
	satOK(t, e)
	if got != truth {
		t.Fatal("failed dispatch changed output")
	}
	if !ast.ASTSourceDebug {
		satOK(t, rw.mkApp(decl, nil, out))
		got, e = m.RefGet(out)
		satOK(t, e)
		if got != p {
			t.Fatal("native raw application fallback", got, p)
		}
	}
}
