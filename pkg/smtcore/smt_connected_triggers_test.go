package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// There are no ground S terms before matching. The connected trigger cannot
// produce the first S fact, but the ordinary delayed trigger can. The second
// quantifier then contradicts it without model-based quantifier instantiation.
func TestSMTConnectedMultiPatternFallback(t *testing.T) {
	for _, mode := range []string{"verification", "ordinary", "no-delayed-round", "extra-eager", "explicit"} {
		t.Run(mode, func(t *testing.T) {
			m, services := smtModelGeneratorFixture(t)
			local, err := services.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, services.Context.Del(&local)) })
			mbqi, macros := "mbqi", "macro_finder"
			astTestMust(t, services.Context.SetBoolName(&local, &mbqi, false))
			astTestMust(t, services.Context.SetBoolName(&local, &macros, false))
			if mode == "extra-eager" {
				param := "qi.max_multi_patterns"
				astTestMust(t, services.Context.SetUIntName(&local, &param, 1))
			}
			solver, err := NewSMTSolver(m, &local, "", SMTSolverConfig{Params: services.Context, ModelEvaluator: services, PreferConnectedMultiPatterns: mode != "ordinary"})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			if mode == "no-delayed-round" {
				// The number of delayed rematches is an internal context parameter.
				solver.kernel.smtParams.qi.state.MaxLazyMultipatternMatching = 0
				astTestMust(t, solver.UpdateParams(&local))
			}
			u, err := m.MkUninterpretedSort(astTestSymbol(t, m, "trigger-u"), nil)
			astTestMust(t, err)
			v, err := m.MkUninterpretedSort(astTestSymbol(t, m, "trigger-v"), nil)
			astTestMust(t, err)
			truth, err := m.MkBoolSort()
			astTestMust(t, err)
			r, err := m.MkFuncDecl(astTestSymbol(t, m, "R"), []ast.ASTHandle{u, v}, truth, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			s, err := m.MkFuncDecl(astTestSymbol(t, m, "S"), []ast.ASTHandle{v, v}, truth, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			sorts := []ast.ASTHandle{u, u, v, v, v}
			variables := make([]ast.ASTHandle, len(sorts))
			names := make([]symbol.Symbol, len(sorts))
			for i, sort := range sorts {
				variables[i], err = m.MkVar(uint32(len(sorts)-1-i), sort)
				astTestMust(t, err)
				names[i] = symbol.Numerical(uint32(i))
			}
			app := func(f, a, b ast.ASTHandle) ast.ASTHandle {
				t.Helper()
				h, e := m.MkApp(f, []ast.ASTHandle{a, b})
				astTestMust(t, e)
				return h
			}
			x, y, a, b, c := variables[0], variables[1], variables[2], variables[3], variables[4]
			guards := []ast.ASTHandle{app(r, x, a), app(r, y, b), app(r, x, c)}
			conclusion := app(s, b, a)
			guard, err := m.MkAndArgs(guards)
			astTestMust(t, err)
			body, err := m.MkImplies(guard, conclusion)
			astTestMust(t, err)
			spec := ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body, Weight: 1}
			if mode == "explicit" {
				original, e := m.MkPattern(guards)
				astTestMust(t, e)
				spec.Patterns = []ast.ASTHandle{original}
			}
			q, err := m.MkQuantifier(spec)
			astTestMust(t, err)
			astTestMust(t, solver.Assert(q))
			k := solver.kernel
			astTestMust(t, k.inferPendingPatterns())
			inferred := k.assertions[0].Formula
			count, err := m.GetNumPatterns(inferred)
			astTestMust(t, err)
			want := uint32(1)
			if mode == "verification" && !ast.ASTSourceDebug && !diagnostic.TraceCompiled() {
				want = 2
			}
			if count != want {
				t.Fatalf("got %d patterns, want %d", count, want)
			}
			terms := func(i uint32) []ast.ASTHandle {
				t.Helper()
				p, e := m.GetPattern(inferred, i)
				astTestMust(t, e)
				n, e := m.GetNumArgs(p)
				astTestMust(t, e)
				out := make([]ast.ASTHandle, n)
				for j := range out {
					out[j], e = m.GetArg(p, uint32(j))
					astTestMust(t, e)
				}
				return out
			}
			ordinary := terms(count - 1)
			if len(ordinary) != 3 {
				t.Fatalf("ordinary pattern has %d terms", len(ordinary))
			}
			for i, h := range ordinary {
				if h != guards[i] {
					t.Fatalf("ordinary term %d changed", i)
				}
			}
			if count == 2 {
				joined := terms(0)
				if len(joined) != 4 || joined[3] != conclusion {
					t.Fatalf("connected trigger %v", joined)
				}
				for i, h := range ordinary {
					if joined[i] != h {
						t.Fatalf("missing ordinary term %d", i)
					}
				}
			}
			absent := app(s, b, c)
			noS, err := m.MkNot(absent)
			astTestMust(t, err)
			explicit, err := m.MkPattern([]ast.ASTHandle{absent})
			astTestMust(t, err)
			none, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{v, v}, Names: names[:2], Body: noS, Patterns: []ast.ASTHandle{explicit}, Weight: 1})
			astTestMust(t, err)
			astTestMust(t, solver.Assert(none))
			astTestMust(t, solver.Push())
			t0, err := m.MkConst(astTestSymbol(t, m, "t"), u)
			astTestMust(t, err)
			v0, err := m.MkConst(astTestSymbol(t, m, "v"), v)
			astTestMust(t, err)
			astTestMust(t, solver.Assert(app(r, t0, v0)))
			verdict, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if verdict != lbool.False {
				t.Fatalf("expected refutation, got %v", verdict)
			}
			astTestMust(t, solver.Pop(1))
		})
	}
}
