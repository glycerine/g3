package fp

// Source: spacer_context.cpp get_ground_sat_answer local witness construction.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerSymbolicGroundApplicationFailureOwnership(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 2048 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newExpressionFixture(t, "spacer", false)
			e, m := f.engine, f.manager
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.xform.subsumption_checker", "false"))
			zero, err := m.GetArg(f.fact, 0)
			fragmentMust(t, err)
			integer, err := m.GetSort(zero)
			fragmentMust(t, err)
			x, err := m.MkVar(0, integer)
			fragmentMust(t, err)
			two, err := m.ArithmeticIntegerNumeralText("2")
			fragmentMust(t, err)
			three, err := m.ArithmeticIntegerNumeralText("3")
			fragmentMust(t, err)
			lo, err := m.MkArithmeticGE(x, two)
			fragmentMust(t, err)
			hi, err := m.MkArithmeticLE(x, three)
			fragmentMust(t, err)
			bounds, err := m.MkAndArgs([]ast.ASTHandle{lo, hi})
			fragmentMust(t, err)
			forall := func(body ast.ASTHandle) ast.ASTHandle {
				h, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body})
				fragmentMust(t, err)
				return h
			}
			base, err := m.MkImplies(bounds, f.query)
			fragmentMust(t, err)
			fragmentMust(t, e.AddRule(forall(base), "symbolic-range"))
			body, err := m.MkAndArgs([]ast.ASTHandle{f.query, bounds})
			fragmentMust(t, err)
			step, err := m.MkImplies(body, f.applications["u"])
			fragmentMust(t, err)
			fragmentMust(t, e.AddRule(forall(step), "symbolic-goal"))
			before, err := m.GetRefCount(f.declaration)
			fragmentMust(t, err)
			status, err := e.QueryExpression(f.applications["u"])
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("query", status)
			}

			// Leave a small number of slots so reconstruction must also allocate
			// an AST reference page while building numeric witness applications.
			var held []ast.ASTRef
			defer func() {
				for i := range held {
					fragmentMust(t, m.DelRef(&held[i]))
				}
			}()
			frontier := false
			for i := 0; i < 4096; i++ {
				m.Budget().FailAfter(0)
				ref, cause := m.NewRef(ast.ASTHandle{})
				m.Budget().FailAfter(-1)
				if errors.Is(cause, arena.ErrInjectedFailure) {
					frontier = true
					break
				}
				fragmentMust(t, cause)
				held = append(held, ref)
			}
			if !frontier || len(held) < 8 {
				t.Fatal("reference allocation frontier not reached")
			}
			for i := 0; i < 8; i++ {
				last := len(held) - 1
				fragmentMust(t, m.DelRef(&held[last]))
				held = held[:last]
			}
			m.Budget().FailAfter(stop)
			answer, failure := e.ExpressionGroundSatAnswer()
			m.Budget().FailAfter(-1)
			if failure != nil {
				failures++
				if answer != (ast.ASTRef{}) {
					t.Fatal("failed conversion published a partial ground answer")
				}
				var cleanup memory.ConstructionFailure
				if errors.As(failure, &cleanup) {
					fragmentMust(t, cleanup.Cleanup())
				}
				var resource memory.Failure
				if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &resource) || resource.Kind != memory.FailureMemout) {
					t.Fatal("unexpected failure", failure)
				}
			} else {
				success = true
				fragmentMust(t, m.DelRef(&answer))
			}
			answer, err = e.ExpressionGroundSatAnswer()
			fragmentMust(t, err)
			witness, err := m.RefGet(answer)
			fragmentMust(t, err)
			count, err := m.GetNumArgs(witness)
			fragmentMust(t, err)
			if count != 2 {
				t.Fatal("missing goal and symbolic numeric instance", count)
			}
			goal, err := m.GetArg(witness, 0)
			fragmentMust(t, err)
			fact, err := m.GetArg(witness, 1)
			fragmentMust(t, err)
			declaration, err := m.GetDecl(fact)
			fragmentMust(t, err)
			if goal != f.applications["u"] || declaration != f.declaration {
				t.Fatal("witness changed original predicates")
			}
			value, err := m.GetArg(fact, 0)
			fragmentMust(t, err)
			lower, err := m.MkArithmeticGE(value, two)
			fragmentMust(t, err)
			upper, err := m.MkArithmeticLE(value, three)
			fragmentMust(t, err)
			condition, err := m.MkAndArgs([]ast.ASTHandle{lower, upper})
			fragmentMust(t, err)
			negated, err := m.MkNot(condition)
			fragmentMust(t, err)
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
			fragmentMust(t, err)
			fragmentMust(t, solver.Assert(negated))
			status, err = solver.CheckSat(nil)
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("witness violates original numeric bounds", status)
			}
			fragmentMust(t, solver.Close())
			fragmentMust(t, m.DelRef(&answer))
			// The factory owns local solver/proof references until the query
			// context is disposed; compare caller ownership only after disposal.
			fragmentMust(t, e.clearQueryResources())
			if got, err := m.GetRefCount(f.declaration); err != nil || got != before {
				t.Fatal("query disposal lost caller predicate ownership", before, got, err)
			}
			m.Budget().FailAfter(0)
			retired, retiredErr := e.ExpressionGroundSatAnswer()
			m.Budget().FailAfter(-1)
			if retired != (ast.ASTRef{}) || !errors.Is(retiredErr, ErrPropositionalEngine) {
				t.Fatal("retired query reached ground-answer construction", retired, retiredErr)
			}
		})
		if t.Failed() {
			break
		}
	}
	if failures == 0 || !success {
		t.Fatal("incomplete ground-answer construction frontier", failures, success)
	}
}
