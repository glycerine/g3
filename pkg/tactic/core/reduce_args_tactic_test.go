package core

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type reduceArgsTestFixture struct {
	m           *ast.ASTManager
	c           *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	services    model.ModelEvaluatorConfig
	r           *ReduceArgsTactic
	goal        *tactic.Goal
	declaration ast.ASTHandle
	terms       []ast.ASTHandle
}

func newReduceArgsTestFixture(t *testing.T, models bool) *reduceArgsTestFixture {
	t.Helper()
	m, c, g, services := solveEqFixture(t)
	simplifyMust(t, m.RegisterDeclarationPlugins())
	r, err := NewReduceArgsTactic(m, c, g, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, r.Close()) })
	goal, err := tactic.NewGoal(m, false, models, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	boolean, err := m.MkBoolSort()
	simplifyMust(t, err)
	name, err := m.Symbols.Intern("reduce-model-f")
	simplifyMust(t, err)
	d, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	p, q, _ := propagateAtoms(t, m)
	var terms []ast.ASTHandle
	for i, arg := range []ast.ASTHandle{p, q} {
		value, err := m.MkBoolVal(i == 0)
		simplifyMust(t, err)
		term, err := m.MkApp(d, []ast.ASTHandle{value, arg})
		simplifyMust(t, err)
		simplifyPin(t, m, term)
		terms = append(terms, term)
		form := term
		if i != 0 {
			form, err = m.MkNot(term)
			simplifyMust(t, err)
		}
		simplifyMust(t, goal.Assert(form))
	}
	return &reduceArgsTestFixture{m: m, c: c, globals: g, services: services, r: r, goal: goal, declaration: d, terms: terms}
}
func TestReduceArgsModelLifetimeFilteringAndVariables(t *testing.T) {
	f := newReduceArgsTestFixture(t, true)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.r.Apply(f.goal, &result))
	if result.ModelConverter == nil {
		t.Fatal("missing model converter")
	}
	md, err := model.NewModel(f.m)
	simplifyMust(t, err)
	var reference tactic.ModelRef
	simplifyMust(t, reference.Set(md.Core()))
	t.Cleanup(func() { simplifyMust(t, reference.Close()) })
	var helpers []ast.ASTHandle
	for i := uint32(0); i < 2; i++ {
		term, err := f.goal.Form(i)
		simplifyMust(t, err)
		if i != 0 {
			_, term, err = f.m.IsNot(term)
			simplifyMust(t, err)
		}
		d, err := f.m.GetDecl(term)
		simplifyMust(t, err)
		helpers = append(helpers, d)
		arity, err := f.m.GetArity(d)
		simplifyMust(t, err)
		if arity != 1 {
			t.Fatal("wrong reduced arity", arity)
		}
		fi, err := model.NewFunctionInterpretation(f.m, arity)
		simplifyMust(t, err)
		value, err := f.m.MkBoolVal(i == 0)
		simplifyMust(t, err)
		simplifyMust(t, fi.SetElse(value))
		simplifyMust(t, md.Core().RegisterFunction(d, fi))
	}
	simplifyMust(t, f.r.Cleanup())
	simplifyMust(t, f.r.Close())
	simplifyMust(t, result.ModelConverter.ApplyModel(&reference, 0))
	converted, err := reference.Get()
	simplifyMust(t, err)
	if converted.NumFunctionDeclarations() != 1 {
		t.Fatal("fresh functions survived filtering", converted.NumFunctionDeclarations())
	}
	fi, err := converted.FunctionInterpretation(f.declaration)
	simplifyMust(t, err)
	if fi == nil || fi.Arity() != 2 {
		t.Fatal("original function signature not reconstructed")
	}
	evaluator, err := model.NewModelEvaluator(converted, f.services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, evaluator.Close()) })
	simplifyMust(t, evaluator.SetModelCompletion(true))
	for i, term := range f.terms {
		value, err := evaluator.EvaluateValue(term)
		simplifyMust(t, err)
		h, err := f.m.RefGet(value)
		simplifyMust(t, err)
		correct := f.m.IsTrue(h)
		if i != 0 {
			correct = f.m.IsFalse(h)
		}
		simplifyMust(t, f.m.DelRef(&value))
		if !correct {
			t.Fatal("reconstructed function violates original application", i)
		}
	}
	simplifyMust(t, evaluator.Close())
	simplifyMust(t, result.Close())
	for _, d := range helpers {
		refs, err := f.m.GetRefCount(d)
		simplifyMust(t, err)
		if refs != 1 {
			t.Fatal("unexpected helper reference beyond its goal application", refs)
		}
	}
	simplifyMust(t, f.goal.Close())
	for _, d := range helpers {
		if f.m.IsLiveHandle(d) {
			t.Fatal("helper declaration survived its last owner")
		}
	}
}
func TestReduceArgsGuardsCancellationParametersTranslationAndTrace(t *testing.T) {
	f := newReduceArgsTestFixture(t, false)
	params, err := f.c.NewValue()
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, f.c.Del(&params)) })
	key := "max_steps"
	simplifyMust(t, f.c.SetUIntName(&params, &key, 0))
	simplifyMust(t, f.r.UpdateParams(&params))
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("reduce_args")
	simplifyMust(t, f.r.BindTrace(tags))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.r.Apply(f.goal, &result))
	if result.ModelConverter != nil {
		t.Fatal("disabled models produced converter")
	}
	if tags.ShouldTrace("reduce_args") {
		for _, s := range []string{"find_non_candidates src/tactic/core/reduce_args_tactic.cpp:169", "non_candidates:\n", "decl2args:\n", "reduce-model-f: 10"} {
			if !strings.Contains(trace.String(), s) {
				t.Fatal("missing source trace", s, trace.String())
			}
		}
	}
	copy := *f.r
	if err = copy.Apply(f.goal, &result); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owner accepted", err)
	}
	simplifyMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	guarded, err := tactic.NewGoal(f.m, true, false, true)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, guarded.Close()) })
	if err = f.r.Apply(guarded, &result); err == nil || err.Error() != "reduce-args does not support proof production" {
		t.Fatal("proof/core refusal order", err)
	}
	if len(result.Goals) != 1 || result.Goals[0] != f.goal {
		t.Fatal("refusal reset previous result")
	}
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: f.m.Symbols, Numerals: f.m.NumeralService(), Budget: f.m.Budget()})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, target.Close()) })
	simplifyMust(t, target.RegisterDeclarationPlugins())
	translated, err := f.r.Translate(target)
	simplifyMust(t, err)
	simplifyMust(t, translated.IncRef())
	t.Cleanup(func() { simplifyMust(t, translated.DecRef()) })
	tr, err := ast.NewASTTranslation(f.m, target, true)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, tr.Close()) })
	goal, err := f.goal.Translate(tr)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	var translatedResult tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, translatedResult.Close()) })
	simplifyMust(t, translated.Apply(goal, &translatedResult))
	target.ResourceLimit().IncCancel()
	canceled := true
	t.Cleanup(func() {
		if canceled {
			target.ResourceLimit().DecCancel()
		}
	})
	empty, err := tactic.NewGoal(target, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, empty.Close()) })
	simplifyMust(t, translated.Apply(empty, &translatedResult))
	err = translated.Apply(goal, &translatedResult)
	var exception tactic.TacticException
	if !errors.As(err, &exception) {
		t.Fatal("nonempty goal did not reach source cancellation", err)
	}
	target.ResourceLimit().DecCancel()
	canceled = false
	closedTags := diagnostic.NewTraceTagsWithWriter(ctxClosedWriter{})
	closedTags.EnableAll(true)
	simplifyMust(t, f.r.BindTrace(closedTags))
	simplifyMust(t, f.r.Cleanup())
	simplifyMust(t, f.r.Apply(f.goal, &result))
}
func TestReduceArgsConstructorAndApplicationAllocationFrontiers(t *testing.T) {
	for _, models := range []bool{false, true} {
		success := false
		for stop := int64(0); stop < 2048 && !success; stop++ {
			t.Run(fmt.Sprintf("models=%v/%d", models, stop), func(t *testing.T) {
				f := newReduceArgsTestFixture(t, models)
				if models {
					zero, err := f.m.ArithmeticIntegerNumeralText("0")
					simplifyMust(t, err)
					integer, err := f.m.GetSort(zero)
					simplifyMust(t, err)
					boolean, err := f.m.MkBoolSort()
					simplifyMust(t, err)
					name, err := f.m.Symbols.Intern("reduce-wide-f")
					simplifyMust(t, err)
					d, err := f.m.MkFuncDecl(name, []ast.ASTHandle{integer, boolean}, boolean, ast.ASTFuncDeclInfo{})
					simplifyMust(t, err)
					p, q, _ := propagateAtoms(t, f.m)
					for i := 0; i < 64; i++ {
						numeral, err := f.m.ArithmeticIntegerNumeralText(fmt.Sprint(i))
						simplifyMust(t, err)
						arg := p
						if i%2 != 0 {
							arg = q
						}
						term, err := f.m.MkApp(d, []ast.ASTHandle{numeral, arg})
						simplifyMust(t, err)
						simplifyMust(t, f.goal.Assert(term))
					}
				}
				var result tactic.TacticResult
				t.Cleanup(func() { simplifyMust(t, result.Close()) })
				f.m.Budget().FailAfter(stop)
				err := f.r.Apply(f.goal, &result)
				f.m.Budget().FailAfter(-1)
				if err == nil {
					success = true
				} else {
					var failure memory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == memory.FailureMemout) {
						t.Fatal(err)
					}
				}
				simplifyMust(t, f.r.Cleanup())
				if len(f.r.pending) != 0 {
					t.Fatal("application cleanup left pending owners")
				}
			})
		}
		if !success {
			t.Fatal("no successful application allocation frontier")
		}
	}
	success := false
	for stop := int64(0); stop < 32 && !success; stop++ {
		t.Run(fmt.Sprintf("constructor/%d", stop), func(t *testing.T) {
			m, c, g, services := solveEqFixture(t)
			simplifyMust(t, m.RegisterDeclarationPlugins())
			m.Budget().FailAfter(stop)
			r, err := NewReduceArgsTactic(m, c, g, services)
			m.Budget().FailAfter(-1)
			if r != nil {
				simplifyMust(t, r.Close())
			}
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				simplifyMust(t, failure.Cleanup())
			}
			if err == nil {
				success = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
		})
	}
	if !success {
		t.Fatal("no successful constructor allocation frontier")
	}
}
