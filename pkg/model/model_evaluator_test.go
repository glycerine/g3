package model_test

// Source: src/model/model_evaluator.cpp:93-203,233-253,282-290,545-579.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"

	"github.com/glycerine/g3/pkg/util/cooperate"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func evaluatorFixture(t *testing.T) (*ast.ASTManager, *model.Model, *model.ModelEvaluator, *uparams.ParamsContext, *uparams.ParamsRef, model.ModelEvaluatorConfig) {
	return evaluatorFixturePlugins(t, true)
}

func evaluatorFixturePlugins(t *testing.T, installPlugins bool) (*ast.ASTManager, *model.Model, *model.ModelEvaluator, *uparams.ParamsContext, *uparams.ParamsRef, model.ModelEvaluatorConfig) {
	c, sy, q := paramsTestOwner(t)
	b := arena.NewBudget(^uint64(0))
	catalog, e := uparams.NewGlobalParamsCatalog(sy, uparams.GlobalParamsCatalogConfig{Budget: b, Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}}, Text: arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}}})
	astTestMust(t, e)
	g, e := uparams.NewGlobalParamsService(sy, c, uparams.GlobalParamsConfig{Budget: b, Catalog: catalog, DescriptorPageSize: 2, DescriptorWordsPageSize: 16})
	astTestMust(t, e)
	astTestMust(t, g.InitializeSource())
	p, e := c.NewValue()
	astTestMust(t, e)
	memory, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b})
	astTestMust(t, e)
	cooperate, e := cooperate.NewCooperation(b)
	astTestMust(t, e)
	t.Cleanup(func() {
		astTestMust(t, cooperate.Close())
		astTestMust(t, memory.Close())
		astTestMust(t, c.Del(&p))
		astTestMust(t, g.Close())
		astTestMust(t, catalog.Close())
		if b.Used() != 0 {
			t.Fatal("evaluator service leak", b.Used())
		}
	})
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	if installPlugins {
		astTestMust(t, m.RegisterDeclarationPlugins())
	}
	md, e := model.NewModel(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, md.Close()) })
	config := model.ModelEvaluatorConfig{Context: c, Globals: g, Params: &p, Memory: memory, Cooperation: cooperate}
	evaluator, e := model.NewModelEvaluator(md.Core(), config)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, evaluator.Close()) })
	return m, md, evaluator, c, &p, config
}
func TestModelEvaluatorConstantCompletionAndBooleanReduction(t *testing.T) {
	m, md, ev, _, _, _ := evaluatorFixture(t)
	a := goalTestAtom(t, m, "eval-a")
	d, e := m.GetDecl(a)
	astTestMust(t, e)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, ev.Evaluate(a, out))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != a {
		t.Fatal("incomplete model should retain symbol", got)
	}
	astTestMust(t, ev.SetModelCompletion(true))
	astTestMust(t, ev.Reset(nil))
	astTestMust(t, ev.SetModelCompletion(true))
	m.Limit.IncCancel()
	e = ev.Evaluate(a, out)
	m.Limit.DecCancel()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if got != no {
		t.Fatal("Boolean completion", got)
	}
	value, e := md.Core().ConstantInterpretation(d)
	astTestMust(t, e)
	if value != no {
		t.Fatal("completion was not registered")
	}
	neg, e := m.MkNot(a)
	astTestMust(t, e)
	astTestPin(t, m, neg)
	astTestMust(t, ev.Evaluate(neg, out))
	yes, e := m.MkTrue()
	astTestMust(t, e)
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if got != yes {
		t.Fatal("Boolean rewriter did not see interpretation", got)
	}
}
func TestModelEvaluatorFunctionEntriesMacrosAndCompletion(t *testing.T) {
	m, md, ev, _, _, _ := evaluatorFixture(t)
	d := modelTestFunctionDecl(t, m, "eval-f")
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	fi := modelTestFunction(t, m, 1)
	astTestMust(t, fi.InsertEntry([]ast.ASTHandle{yes}, no))
	v, e := m.MkVar(0, mustBoolSort(t, m))
	astTestMust(t, e)
	astTestPin(t, m, v)
	astTestMust(t, fi.SetElse(v))
	astTestMust(t, md.Core().RegisterFunction(d, fi))
	out := astTestPin(t, m, ast.ASTHandle{})
	for _, arg := range []ast.ASTHandle{yes, no} {
		term, e := m.MkApp(d, []ast.ASTHandle{arg})
		astTestMust(t, e)
		astTestPin(t, m, term)
		astTestMust(t, ev.Evaluate(term, out))
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if got != no {
			t.Fatal("entry or bound fallback", arg, got)
		}
	}
	unknown := modelTestFunctionDecl(t, m, "eval-new-function")
	term, e := m.MkApp(unknown, []ast.ASTHandle{yes})
	astTestMust(t, e)
	astTestPin(t, m, term)
	astTestMust(t, ev.SetModelCompletion(true))
	astTestMust(t, ev.Evaluate(term, out))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != no {
		t.Fatal("missing function completion", got)
	}
	completed, e := md.Core().FunctionInterpretation(unknown)
	astTestMust(t, e)
	if completed == nil {
		t.Fatal("missing completed graph")
	}
}
func mustBoolSort(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
	s, e := m.MkBoolSort()
	astTestMust(t, e)
	return s
}

func TestModelEvaluatorResourceLimitsAndReset(t *testing.T) {
	m, _, ev, c, p, config := evaluatorFixture(t)
	memory := config.Memory
	a := goalTestAtom(t, m, "limit-eval-a")
	term, e := m.MkNot(a)
	astTestMust(t, e)
	astTestPin(t, m, term)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	out := astTestPin(t, m, yes)
	key := "max_steps"
	astTestMust(t, c.SetUIntName(p, &key, 0))
	astTestMust(t, ev.UpdtParams(p))
	if e = ev.Evaluate(term, out); e == nil || e.Error() != messages.MaxSteps {
		t.Fatal("step limit", e)
	}
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != yes {
		t.Fatal("step failure changed output")
	}
	astTestMust(t, ev.Reset(nil))
	key = "max_memory"
	astTestMust(t, c.SetUIntName(p, &key, 0))
	key = "max_steps"
	astTestMust(t, c.SetUIntName(p, &key, 0))
	astTestMust(t, ev.UpdtParams(p))
	block, e := memory.Allocate(16)
	astTestMust(t, e)
	e = ev.Evaluate(term, out)
	astTestMust(t, memory.Deallocate(&block))
	if e == nil || e.Error() != messages.MaxMemory || !rewriter.IsRewriterException(e) {
		t.Fatal("source memory precedes step limit", e)
	}
	astTestMust(t, ev.Cleanup(nil))
	astTestMust(t, ev.Evaluate(term, out))
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if got != term {
		t.Fatal("cleanup failed to reconstruct evaluator", got)
	}
	copied := *ev
	if e = copied.Evaluate(term, out); e == nil {
		t.Fatal("copied evaluator accepted")
	}
	astTestMust(t, ev.Close())
	if e = ev.Evaluate(term, out); e == nil {
		t.Fatal("closed evaluator accepted")
	}
}

func TestModelEvaluatorPartialFunctionAndUniverseCompletion(t *testing.T) {
	m, md, ev, _, _, _ := evaluatorFixture(t)
	d := modelTestFunctionDecl(t, m, "partial-eval-f")
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	fi := modelTestFunction(t, m, 1)
	astTestMust(t, fi.InsertEntry([]ast.ASTHandle{yes}, no))
	astTestMust(t, md.Core().RegisterFunction(d, fi))
	miss, e := m.MkApp(d, []ast.ASTHandle{no})
	astTestMust(t, e)
	astTestPin(t, m, miss)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, ev.Evaluate(miss, out))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != miss {
		t.Fatal("partial miss unexpectedly completed", got)
	}
	astTestMust(t, ev.Reset(nil))
	astTestMust(t, ev.SetModelCompletion(true))
	astTestMust(t, ev.Evaluate(miss, out))
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if got != no {
		t.Fatal("partial fallback completion", got)
	}
	fallback, e := fi.Else()
	astTestMust(t, e)
	if fallback != no {
		t.Fatal("partial graph not completed")
	}
	s, e := m.MkUninterpretedSort(astTestSymbol(t, m, "eval-universe-sort"), nil)
	astTestMust(t, e)
	astTestPin(t, m, s)
	value, e := m.GetSomeValue(s)
	astTestMust(t, e)
	astTestPin(t, m, value)
	astTestMust(t, md.RegisterUniverse(s, []ast.ASTHandle{value}))
	atom, e := m.MkConst(astTestSymbol(t, m, "eval-universe-constant"), s)
	astTestMust(t, e)
	astTestPin(t, m, atom)
	astTestMust(t, ev.Evaluate(atom, out))
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if got != value {
		t.Fatal("completion ignored model universe", got)
	}
}

func TestModelEvaluatorConstructorRefusalsAndParameterPrecedence(t *testing.T) {
	m, md, ev, c, p, config := evaluatorFixture(t)
	astTestMust(t, config.Globals.SourceSetName("model_evaluator.completion", "true"))
	astTestMust(t, ev.Reset(nil))
	a := goalTestAtom(t, m, "global-completion")
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, ev.Evaluate(a, out))
	no, e := m.MkFalse()
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != no {
		t.Fatal("module completion default ignored")
	}
	key := "completion"
	astTestMust(t, c.SetBoolName(p, &key, false))
	astTestMust(t, ev.Reset(p))
	b := goalTestAtom(t, m, "local-completion")
	astTestMust(t, ev.Evaluate(b, out))
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if got != b {
		t.Fatal("local completion did not override module")
	}
	// Warm the retained reference directory and generated parameter module roots.
	warm, e := model.NewModelEvaluator(md.Core(), config)
	astTestMust(t, e)
	astTestMust(t, warm.Close())
	baseline := m.Budget().Used()
	for fault := int64(0); fault < 256; fault++ {
		m.Budget().FailAfter(fault)
		next, e := model.NewModelEvaluator(md.Core(), config)
		m.Budget().FailAfter(-1)
		if next != nil {
			astTestMust(t, next.Close())
		}
		if m.Budget().Used() != baseline {
			t.Fatal("partial evaluator leak", fault, baseline, m.Budget().Used())
		}
		if e == nil {
			return
		}
	}
	t.Fatal("constructor never completed")
}

func TestModelEvaluatorIndependentBooleanModelsAndDirectMacroOrder(t *testing.T) {
	m, md, ev, _, _, _ := evaluatorFixture(t)
	a, b := goalTestAtom(t, m, "table-a"), goalTestAtom(t, m, "table-b")
	da, e := m.GetDecl(a)
	astTestMust(t, e)
	db, e := m.GetDecl(b)
	astTestMust(t, e)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	neg, e := m.MkNot(b)
	astTestMust(t, e)
	astTestPin(t, m, neg)
	term, e := m.MkAnd(a, neg)
	astTestMust(t, e)
	astTestPin(t, m, term)
	out := astTestPin(t, m, ast.ASTHandle{})
	values := []ast.ASTHandle{no, yes}
	for bits := 0; bits < 4; bits++ {
		astTestMust(t, md.Core().RegisterConstant(da, values[bits&1]))
		astTestMust(t, md.Core().RegisterConstant(db, values[(bits>>1)&1]))
		astTestMust(t, ev.Reset(nil))
		astTestMust(t, ev.Evaluate(term, out))
		got, e := m.RefGet(out)
		astTestMust(t, e)
		want := no
		if bits&1 != 0 && bits&2 == 0 {
			want = yes
		}
		if got != want {
			t.Fatal("independent Boolean valuation", bits, got, want)
		}
	}
	s := mustBoolSort(t, m)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "direct-eval-f"), []ast.ASTHandle{s, s}, s, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	v, e := m.MkVar(0, s)
	astTestMust(t, e)
	astTestPin(t, m, v)
	fi := modelTestFunction(t, m, 2)
	astTestMust(t, fi.SetElse(v))
	astTestMust(t, md.Core().RegisterFunction(d, fi))
	app, e := m.MkApp(d, []ast.ASTHandle{yes, no})
	astTestMust(t, e)
	astTestPin(t, m, app)
	astTestMust(t, ev.Evaluate(app, out))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != yes {
		t.Fatal("macro argument order", got)
	}
}

func TestModelEvaluatorUnportedTheoryDoesNotReturnSuccess(t *testing.T) {
	m, _, ev, _, _, _ := evaluatorFixture(t)
	s := mustBoolSort(t, m)
	info, e := m.NewFuncDeclInfo(42, 0, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "pending-theory"), nil, s, info)
	astTestMust(t, e)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	astTestPin(t, m, d)
	app, e := m.MkApp(d, nil)
	astTestMust(t, e)
	astTestPin(t, m, app)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	out := astTestPin(t, m, yes)
	if e = ev.Evaluate(app, out); e != model.ErrModelEvaluatorTheory {
		t.Fatal("unported theory accepted", e)
	}
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != yes {
		t.Fatal("unsupported reduction changed output")
	}
}
