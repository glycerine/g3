package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
	"strings"
	"testing"
)

func TestSpacerGeneralizerStatisticsSurviveFailedExpressionPreparation(t *testing.T) {
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("generalizer expression fixture leaked bytes", budget.Used())
		}
	})
	f := newPDRLifecycleFixture(t)
	budget = f.manager.Budget()
	e := f.engine
	fragmentMust(t, e.SelectEngine("spacer"))
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	parameters, err := e.params.NewValue()
	must(err)
	defer func() { must(e.params.Del(&parameters)) }()
	for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
		must(e.params.SetBoolName(&parameters, &name, false))
	}
	must(e.SetQueryParameters(&parameters))
	m := f.manager
	arithmetic, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("statistics-integer-chain")
	must(err)
	declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(declaration))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("-1")
	must(err)
	fact, err := m.MkApp(declaration, []ast.ASTHandle{zero})
	must(err)
	must(e.AddRule(fact, "statistics-zero"))
	variable, err := m.MkVar(0, integer)
	must(err)
	current, err := m.MkApp(declaration, []ast.ASTHandle{variable})
	must(err)
	nextValue, err := m.MkArithmeticAdd(variable, one)
	must(err)
	next, err := m.MkApp(declaration, []ast.ASTHandle{nextValue})
	must(err)
	edge, err := m.MkImplies(current, next)
	must(err)
	must(e.AddRule(edge, "statistics-successor"))
	query, err := m.MkApp(declaration, []ast.ASTHandle{three})
	must(err)
	queryRef, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&queryRef)) }()
	value := func(key statistics.Key) float64 {
		t.Helper()
		stats := statistics.New()
		must(e.CollectStatistics(stats))
		var result float64
		for i := 0; i < stats.Size(); i++ {
			if stats.Key(i) == key {
				if stats.IsUint(i) {
					result += float64(stats.UintValue(i))
				} else {
					result += stats.DoubleValue(i)
				}
			}
		}
		return result
	}
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.False || e.queryClone == nil {
		t.Fatal("expected an unreachable expression query owner", status)
	}
	count := value("bool inductive gen")
	elapsed := value("time.spacer.solve.reach.gen.bool_ind")
	if count == 0 {
		t.Fatal("fixture did not invoke Boolean inductive generalization")
	}
	condition, err := m.MkArithmeticLT(variable, zero)
	must(err)
	body, err := m.MkAnd(current, condition)
	must(err)
	variableName, err := m.Symbols.Intern("failed-preparation-variable")
	must(err)
	quantified, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{variableName}, Body: body})
	must(err)
	quantifiedRef, err := m.NewRef(quantified)
	must(err)
	defer func() { must(m.DelRef(&quantifiedRef)) }()
	limit := m.ResourceLimit()
	limit.IncCancel()
	_, err = e.QueryExpression(quantified)
	limit.DecCancel()
	if err == nil || !strings.Contains(err.Error(), "canceled") || e.queryClone != nil || e.spacer != nil {
		t.Fatal("expected failed preparation before program installation", err)
	}
	if _, err = e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("failed preparation published a result", err)
	}
	if got := value("bool inductive gen"); got != count {
		t.Fatal("failed preparation lost generalizer counter", got, count)
	}
	if got := value("time.spacer.solve.reach.gen.bool_ind"); got != elapsed {
		t.Fatal("failed preparation lost generalizer time", got, elapsed)
	}
	status, err = e.QueryExpression(query)
	must(err)
	if status != lbool.False {
		t.Fatal("retry result", status)
	}
	collected := statistics.New()
	must(e.queryClone.CollectStatistics(collected))
	var currentCount float64
	for i := 0; i < collected.Size(); i++ {
		if collected.IsUint(i) && collected.Key(i) == "bool inductive gen" {
			currentCount += float64(collected.UintValue(i))
		}
	}
	if currentCount == 0 || value("bool inductive gen") != currentCount {
		t.Fatal("new installation accumulated obsolete generalizer counts", value("bool inductive gen"), currentCount)
	}
	must(e.ResetStatistics())
	if value("bool inductive gen") != 0 || value("time.spacer.solve.reach.gen.bool_ind") != 0 {
		t.Fatal("statistics reset left generalizer history")
	}
	prior := e.queryClone
	context := prior.spacer
	m.Budget().FailAfter(0)
	_, err = e.QueryExpression(query)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected refused previous-query disposal", err)
	}
	if e.queryClone != prior || prior.closed || prior.spacer != context {
		t.Fatal("refused disposal discarded its context owner")
	}
	if _, err = e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("refused disposal exposed an obsolete result", err)
	}
	status, err = e.QueryExpression(query)
	must(err)
	if status != lbool.False {
		t.Fatal("retry after previous-query disposal refusal", status)
	}
	if !prior.closed {
		t.Fatal("retry did not finish previous-query disposal")
	}
	must(context.Close())
}
