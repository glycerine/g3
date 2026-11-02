package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math/big"
	"testing"
)

func TestDualityEngineNativeRecursiveIntegerReachability(t *testing.T) {
	for _, nonlinear := range []bool{false, true} {
		for _, unsafe := range []bool{false, true} {
			name := "linear-safe"
			if nonlinear {
				name = "nonlinear-safe"
			}
			if unsafe {
				name = "linear-reachable"
				if nonlinear {
					name = "nonlinear-reachable"
				}
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
					term, err := NewExpr(ctx, h)
					must(err)
					closeExpr(t, term)
					return term
				}
				raw := func(term *Expr) ast.ASTHandle { t.Helper(); h, err := term.Raw(); must(err); return h }
				factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
				must(err)
				defer factory.Close()
				nativeParams, err := native.Params()
				must(err)
				proofParams := ctx.config.params.EmptyValue()
				must(ctx.config.params.Copy(&proofParams, nativeParams))
				defer ctx.config.params.Del(&proofParams)
				eq2ineq := "eq2ineq"
				must(ctx.config.params.SetBoolName(&proofParams, &eq2ineq, true))
				must(native.UpdateParams(&proofParams))
				aux, err := factory.New(m, nil, false, true, false, "")
				must(err)
				defer aux.Close()
				manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
				must(err)
				defer manager.Close()
				zero := wrap(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
				one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
				two := wrap(m.ArithmeticNumeralRat(big.NewRat(2, 1), true))
				integer, err := zero.Sort()
				must(err)
				defer integer.Close()
				yes, err := ctx.BoolVal(true)
				must(err)
				closeExpr(t, yes)
				boolean, err := yes.Sort()
				must(err)
				defer boolean.Close()
				intRaw, err := m.GetSort(raw(zero))
				must(err)
				nameSymbol, err := m.Symbols.Intern("recursive-state-x")
				must(err)
				x := wrap(m.MkConst(nameSymbol, intRaw))
				declaration := func(name string) *FuncDecl {
					t.Helper()
					symbol, err := ctx.StringSymbol(name)
					must(err)
					decl, err := ctx.Function(symbol, []*Sort{integer}, boolean)
					must(err)
					closeDecl(t, decl)
					return decl
				}
				reachDecl := declaration("recursive-Reach")
				stepDecl := declaration("recursive-step-position0")
				queryDecl := declaration("recursive-query-position0")
				reachTerm, err := reachDecl.Call(x)
				must(err)
				closeExpr(t, reachTerm)
				prior := wrap(m.MkArithmeticSub(raw(x), raw(one)))
				step, err := stepDecl.Call(prior)
				must(err)
				closeExpr(t, step)
				source, err := NewRPFP(ctx)
				must(err)
				defer source.Close()
				reach, err := source.CreateNode(reachTerm)
				must(err)
				relations := []*FuncDecl{stepDecl}
				children := []*Node{reach}
				if nonlinear {
					secondDecl := declaration("recursive-step-position1")
					second, err := secondDecl.Call(prior)
					must(err)
					closeExpr(t, second)
					step, err = step.And(second)
					must(err)
					closeExpr(t, step)
					relations = append(relations, secondDecl)
					children = append(children, reach)
				}
				positive := wrap(m.MkArithmeticGT(raw(x), raw(zero)))
				recursive, err := positive.And(step)
				must(err)
				closeExpr(t, recursive)
				initial := wrap(m.MkEq(raw(x), raw(zero)))
				body, err := initial.Or(recursive)
				must(err)
				closeExpr(t, body)
				rule, err := NewTransformer(ctx, relations, []*Expr{x}, body)
				must(err)
				defer rule.Close()
				_, err = source.CreateEdge(reach, rule, children)
				must(err)
				goalTerm, err := ctx.BoolConst("recursive-Goal")
				must(err)
				closeExpr(t, goalTerm)
				goal, err := source.CreateNode(goalTerm)
				must(err)
				must(goal.Bound.SetEmpty())
				queried, err := queryDecl.Call(x)
				must(err)
				closeExpr(t, queried)
				condition := wrap(m.MkArithmeticLT(raw(x), raw(zero)))
				if unsafe {
					condition = wrap(m.MkEq(raw(x), raw(two)))
				}
				goalBody, err := queried.And(condition)
				must(err)
				closeExpr(t, goalBody)
				goalRule, err := NewTransformer(ctx, []*FuncDecl{queryDecl}, nil, goalBody)
				must(err)
				defer goalRule.Close()
				_, err = source.CreateEdge(goal, goalRule, []*Node{reach})
				must(err)
				engine, err := NewEngine(source, EngineConfig{Native: native, Auxiliary: aux, Factory: factory, Evaluator: derivationEvaluatorFixture(t, ctx, globals), Simplifier: manager})
				must(err)
				defer engine.Close()
				if !engine.SetOption("stratified_inlining", "1") {
					t.Fatal("stratified option rejected")
				}
				must(native.Push())
				caller, err := ctx.BoolConst("recursive-caller")
				must(err)
				closeExpr(t, caller)
				must(native.Assert(raw(caller)))
				_, err = native.CheckSat(nil)
				must(err)
				baseline, err := native.NumAssertions()
				must(err)
				solved, err := engine.Solve()
				must(err)
				if solved == unsafe {
					t.Fatal("recursive integer reachability", solved, unsafe)
				}
				if unsafe {
					cex := engine.GetCounterexample()
					if cex.Tree() == nil || cex.Root() == nil {
						t.Fatal("missing recursive counterexample")
					}
					// x=2 needs three Reach rule applications (2,1,0), plus Goal.
					activeReach := 0
					values := make(map[ast.ASTHandle]bool)
					for _, node := range cex.Tree().nodes {
						if node.Map != nil && node.Map.Map == reach {
							empty, err := cex.Tree().Empty(node)
							must(err)
							if !empty && node.Outgoing != nil {
								activeReach++
								evaluated, err := cex.Tree().EvalNode(node)
								must(err)
								defer evaluated.Close()
								arg, err := m.GetArg(raw(evaluated), 0)
								must(err)
								values[arg] = true
							}
						}
					}
					if activeReach < 3 || !values[raw(zero)] || !values[raw(one)] || !values[raw(two)] {
						t.Fatal("counterexample did not unfold to reachable depth", activeReach)
					}
				} else {
					invariant, err := reach.Annotation.Formula()
					must(err)
					defer invariant.Close()
					expected := wrap(m.MkArithmeticGE(raw(x), raw(zero)))
					fact, err := NewTransformer(ctx, nil, []*Expr{x}, expected)
					must(err)
					defer fact.Close()
					implies, err := reach.Annotation.SubsetEq(fact, aux)
					must(err)
					// Independently validate that the learned relation includes
					// the base and is closed under the recursive step.
					invariantRaw, err := invariant.Raw()
					must(err)
					priorRef, err := substituteExpression(ctx, invariantRaw, map[ast.ASTHandle]ast.ASTHandle{raw(x): raw(prior)})
					must(err)
					defer m.DelRef(&priorRef)
					priorRaw, err := m.RefGet(priorRef)
					must(err)
					priorInvariant, err := NewExpr(ctx, priorRaw)
					must(err)
					defer priorInvariant.Close()
					stepInvariant, err := positive.And(priorInvariant)
					must(err)
					defer stepInvariant.Close()
					closure, err := initial.Or(stepInvariant)
					must(err)
					defer closure.Close()
					negativeInvariant, err := invariant.Not()
					must(err)
					defer negativeInvariant.Close()
					violation, err := closure.And(negativeInvariant)
					must(err)
					defer violation.Close()
					must(aux.Push())
					must(aux.Assert(raw(violation)))
					status, err := aux.CheckSat(nil)
					must(err)
					must(aux.Pop(1))
					if status != lbool.False {
						t.Fatal("learned arithmetic relation is not an inductive invariant", status)
					}
					if !implies || engine.GetCounterexample().Tree() != nil {
						t.Fatal("recursive safety invariant/counterexample status")
					}
				}
				depth, err := native.ScopeLevel()
				must(err)
				assertions, err := native.NumAssertions()
				must(err)
				if depth != 1 || assertions != baseline {
					t.Fatal("recursive solve leaked caller scopes/assertions")
				}
				must(native.Pop(1))
			})
		}
	}
}
