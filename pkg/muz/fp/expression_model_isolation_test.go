package fp

// Sources: pdr_context.cpp get_model and spacer_context.cpp get_model create
// caller-owned interpretations through inductive_property::to_model.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExpressionModelsIsolateCallerInterpretationMutation(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			f := newQueryVariablesOwnershipFixture(t)
			e, m := f.e, f.m
			params, globals := e.params, e.globals
			fragmentMust(t, e.SelectEngine(backend))
			for _, name := range []string{"slice", "inline_eager", "inline_linear"} {
				fragmentMust(t, globals.SourceSetName("fixedpoint.xform."+name, "false"))
			}
			zero := f.own(m.ArithmeticIntegerNumeralText("0"))
			one := f.own(m.ArithmeticIntegerNumeralText("1"))
			declaration, fact := f.predicate("isolated-model-P", zero)
			fragmentMust(t, e.addRuleEager(fact, "zero"))
			query := f.own(m.MkApp(declaration, []ast.ASTHandle{one}))
			status, err := e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("expected unreachable expression", status)
			}
			first, err := e.ExpressionModel()
			fragmentMust(t, err)
			defer func() { fragmentMust(t, first.Close()) }()
			second, err := e.ExpressionModel()
			fragmentMust(t, err)
			defer func() { fragmentMust(t, second.Close()) }()
			firstFunction, err := first.Core().FunctionInterpretation(declaration)
			fragmentMust(t, err)
			secondFunction, err := second.Core().FunctionInterpretation(declaration)
			fragmentMust(t, err)
			if first == second || first.Core() == second.Core() || firstFunction == nil || secondFunction == nil || firstFunction == secondFunction {
				t.Fatal("exported models share mutable interpretation ownership")
			}
			cooperation, err := cooperate.NewCooperation(m.Budget())
			fragmentMust(t, err)
			defer func() { fragmentMust(t, cooperation.Close()) }()
			check := func(md *model.Model, queryTruth bool) {
				t.Helper()
				evaluator, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: params, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
				fragmentMust(t, err)
				defer func() { fragmentMust(t, evaluator.Close()) }()
				fragmentMust(t, evaluator.SetModelCompletion(false))
				for _, obligation := range []struct {
					formula ast.ASTHandle
					truth   bool
				}{{fact, true}, {query, queryTruth}} {
					value, err := evaluator.EvaluateValue(obligation.formula)
					fragmentMust(t, err)
					handle, err := m.RefGet(value)
					fragmentMust(t, err)
					valid := m.IsFalse(handle)
					if obligation.truth {
						valid = m.IsTrue(handle)
					}
					fragmentMust(t, m.DelRef(&value))
					if !valid {
						t.Fatal("exported interpretation changed original fact or query truth", obligation.truth)
					}
				}
			}
			check(first, false)
			check(second, false)
			truth, err := m.MkTrue()
			fragmentMust(t, err)
			fragmentMust(t, firstFunction.SetElse(truth))
			check(first, true)
			check(second, false)
			third, err := e.ExpressionModel()
			fragmentMust(t, err)
			check(third, false)
			fragmentMust(t, third.Close())
			fragmentMust(t, first.Close())
			check(second, false)
			status, err = e.QueryExpression(fact)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("caller model mutation changed replacement query", status)
			}
			check(second, false)
			fragmentMust(t, e.Close())
			check(second, false)
		})
	}
}
