package duality

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityFromClausesNativeSafeAndReachable(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		name := "safe"
		if unsafe {
			name = "reachable"
		}
		t.Run(name, func(t *testing.T) {
			ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			wrap := func(h ast.ASTHandle, err error) *Expr {
				t.Helper()
				must(err)
				e, err := NewExpr(ctx, h)
				must(err)
				closeExpr(t, e)
				return e
			}
			raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
			constant := func(name string) *Expr { e, err := ctx.BoolConst(name); must(err); closeExpr(t, e); return e }
			evaluator := derivationEvaluatorFixture(t, ctx, globals)
			quantifiers, err := qe.NewLite(m, nil, true, qe.Config{Context: ctx.config.params, Globals: globals, Cooperation: evaluator.Cooperation})
			must(err)
			defer quantifiers.Close()
			simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer simplifier.Close()
			factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
			must(err)
			defer factory.Close()
			aux, err := factory.New(m, nil, false, true, false, "")
			must(err)
			defer aux.Close()
			p, q, dead := constant("clauses-p"), constant("clauses-q"), constant("clauses-dead")
			no := wrap(m.MkFalse())
			value, err := ctx.BoolVal(unsafe)
			must(err)
			closeExpr(t, value)
			clauses := []*Expr{
				wrap(m.MkImplies(raw(p), raw(q))),
				wrap(m.MkImplies(raw(value), raw(p))),
				wrap(m.MkImplies(raw(q), raw(no))),
				dead,
				wrap(m.MkImplies(raw(no), raw(p))),
			}
			graph, err := NewRPFP(ctx)
			must(err)
			defer graph.Close()
			must(graph.FromClauses(clauses, []uint32{3, 4, 9, 2, 7}, simplifier, quantifiers))
			if len(graph.nodes) != 3 || len(graph.edges) != 4 {
				t.Fatalf("unexpected pruned graph size: %d %d", len(graph.nodes), len(graph.edges))
			}
			pd, err := m.GetDecl(raw(p))
			must(err)
			for _, node := range graph.nodes {
				decl, err := m.RefGet(node.Name)
				must(err)
				if decl == pd && node.RecursionBound != 7 {
					t.Fatal("clause bounds not aggregated by parent")
				}
			}
			for i, edge := range graph.edges {
				if edge.labeled == (ast.ASTRef{}) {
					t.Fatal("source labeled formula not owned")
				}
				if _, err := m.RefGet(edge.labeled); err != nil {
					t.Fatal("labeled formula expired", err)
				}
				original, err := graph.OriginalClause(edge)
				must(err)
				want := []int{0, 1, 2, 4}[i]
				if raw(original) != raw(clauses[want]) {
					t.Fatal("pruning shifted the original proof clause", i, want)
				}
				must(original.Close())
			}
			// Finite recursion bounds are structurally tested above; use unbounded
			// engine mode to test full safety/reachability independently of the bound.
			for _, node := range graph.nodes {
				node.RecursionBound = ^uint32(0)
			}
			engine, err := NewEngine(graph, EngineConfig{Native: native, Auxiliary: aux, Factory: factory, Evaluator: evaluator, Simplifier: simplifier, Quantifiers: quantifiers})
			must(err)
			defer engine.Close()
			solved, err := engine.Solve()
			must(err)
			if solved == unsafe {
				t.Fatalf("safety result %v for reachable=%v", solved, unsafe)
			}
			if unsafe && engine.GetCounterexample().Root() == nil {
				t.Fatal("reachable clauses produced no counterexample")
			}
			rejected, err := NewRPFP(ctx)
			must(err)
			defer rejected.Close()
			if err := rejected.FromClauses(clauses, []uint32{1}, simplifier, quantifiers); err == nil || len(rejected.nodes) != 0 {
				t.Fatal("bound-count error mutated graph")
			}
		})
	}
}

func TestDualityFromClausesQuantifiersAndCumulativeLabelGuard(t *testing.T) {
	for _, labeled := range []bool{false, true} {
		name := "qe-lite"
		if labeled {
			name = "labels-disable-qe"
		}
		t.Run(name, func(t *testing.T) {
			ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			wrap := func(h ast.ASTHandle, err error) *Expr {
				t.Helper()
				must(err)
				e, err := NewExpr(ctx, h)
				must(err)
				closeExpr(t, e)
				return e
			}
			raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
			services := derivationEvaluatorFixture(t, ctx, globals)
			quantifiers, err := qe.NewLite(m, nil, true, qe.Config{Context: ctx.config.params, Globals: globals, Cooperation: services.Cooperation})
			must(err)
			defer quantifiers.Close()
			simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer simplifier.Close()
			boolean, err := m.MkBoolSort()
			must(err)
			x := wrap(m.MkVar(1, boolean))
			y := wrap(m.MkVar(0, boolean))
			xn, err := m.Symbols.Intern("clause-x")
			must(err)
			yn, err := m.Symbols.Intern("clause-y")
			must(err)
			declare := func(text string) *FuncDecl {
				n, err := m.Symbols.Intern(text)
				must(err)
				h, err := m.MkFuncDecl(n, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				d, err := NewFuncDecl(ctx, h)
				must(err)
				closeDecl(t, d)
				return d
			}
			r, s := declare("quantified-R"), declare("quantified-S")
			apply := func(d *FuncDecl, e *Expr) *Expr { v, err := d.Call(e); must(err); closeExpr(t, v); return v }
			equality := wrap(m.MkEq(raw(x), raw(y)))
			firstBody := equality
			if labeled {
				label, err := m.Symbols.Intern("clause-label")
				must(err)
				firstBody = wrap(m.MkLabel(true, []usymbol.Symbol{label}, raw(equality)))
			}
			quantify := func(body *Expr) *Expr {
				return wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean, boolean}, Names: []usymbol.Symbol{xn, yn}, Body: raw(body)}))
			}
			yes := wrap(m.MkTrue())
			no := wrap(m.MkFalse())
			clauses := []*Expr{
				quantify(wrap(m.MkImplies(raw(firstBody), raw(apply(r, x))))),
				quantify(wrap(m.MkImplies(raw(equality), raw(apply(s, x))))),
				wrap(m.MkImplies(raw(wrap(m.MkAnd(raw(apply(r, yes)), raw(apply(s, yes))))), raw(no))),
			}
			graph, err := NewRPFP(ctx)
			must(err)
			defer graph.Close()
			must(graph.FromClauses(clauses, nil, simplifier, quantifiers))
			if len(graph.nodes) != 3 || len(graph.edges) != 3 {
				t.Fatal("quantified graph size mismatch")
			}
			for i, edge := range graph.edges[:2] {
				original, err := graph.OriginalClause(edge)
				must(err)
				if raw(original) != raw(clauses[i]) {
					t.Fatal("preprocessing replaced the original quantified clause")
				}
				must(original.Close())
				formula, err := m.RefGet(edge.F.formula)
				must(err)
				if labeled {
					op, err := m.GetDeclKind(formula)
					must(err)
					if op != ast.ASTOpEq && op != ast.ASTOpIff {
						t.Fatal("label guard did not preserve initialization equality")
					}
				} else if !m.IsTrue(formula) {
					t.Fatal("qe-lite did not eliminate non-head variable")
				}
				term, err := NewExpr(ctx, formula)
				must(err)
				closed, err := IsClosedFormula(term)
				must(err)
				must(term.Close())
				if !closed {
					t.Fatal("delayed binder substitutions left free indices")
				}
				if edge.Parent.RecursionBound != ^uint32(0) {
					t.Fatal("absent bounds did not select unbounded mode")
				}
			}
			// Reach an actual reference-page refusal after one original clause
			// has been acquired, before any transformed graph node is created.
			failed, err := NewRPFP(ctx)
			must(err)
			var held []ast.ASTRef
			defer func() {
				m.Budget().FailAfter(-1)
				must(failed.Close())
				for i := range held {
					must(m.DelRef(&held[i]))
				}
			}()
			m.Budget().FailAfter(0)
			for {
				ref, err := m.NewRef(ast.ASTHandle{})
				if err != nil {
					if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
					break
				}
				held = append(held, ref)
			}
			if len(held) == 0 {
				t.Fatal("fixture needs a reference slot to release")
			}
			must(m.DelRef(&held[len(held)-1]))
			held = held[:len(held)-1]
			before := m.Stats().LiveReferences
			err = failed.FromClauses(clauses, nil, simplifier, quantifiers)
			if !errors.Is(err, arena.ErrInjectedFailure) || len(failed.sourceClauses) != 1 || len(failed.nodes) != 0 {
				t.Fatal("original-clause acquisition frontier", err, len(failed.sourceClauses), len(failed.nodes))
			}
			m.Budget().FailAfter(-1)
			must(failed.Close())
			if m.Stats().LiveReferences != before {
				t.Fatal("partial original-clause owner leaked its reference")
			}
		})
	}
}
