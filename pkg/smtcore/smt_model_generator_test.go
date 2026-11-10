package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestSMTModelGeneratorHonorsDisabledRelevancyForValuesAndFunctions(t *testing.T) {
	for _, level := range []uint32{0, 2} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			params, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
			key := "relevancy"
			astTestMust(t, cfg.Context.SetUIntName(&params, &key, level))
			key = "auto_config"
			astTestMust(t, cfg.Context.SetBoolName(&params, &key, false))
			s, err := NewSMTSolver(m, &params, "QF_UFLRA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			s.kernel.preprocess = false
			astTestMust(t, s.SetProduceModels(true))
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			sort, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "relevancy-model-x"), sort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "relevancy-model-y"), sort)
			astTestMust(t, err)
			f, err := m.MkFuncDecl(astTestSymbol(t, m, "relevancy-model-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			fx, err := m.MkApp(f, []ast.ASTHandle{x})
			astTestMust(t, err)
			fy, err := m.MkApp(f, []ast.ASTHandle{y})
			astTestMust(t, err)
			equal, err := m.MkEq(fx, fy)
			astTestMust(t, err)
			different, err := m.MkNot(equal)
			astTestMust(t, err)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			unused, err := m.MkConst(astTestSymbol(t, m, "relevancy-model-unused"), boolean)
			astTestMust(t, err)
			astTestMust(t, s.kernel.internalizer.Internalize(unused, true))
			astTestMust(t, s.Assert(different))
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("distinct UF values", status)
			}
			md, err := s.AcquireModel()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, md.DecRef()) })
			var values [2]ast.ASTHandle
			for index, term := range []ast.ASTHandle{x, y} {
				decl, err := m.GetDecl(term)
				astTestMust(t, err)
				values[index], err = md.ConstantInterpretation(decl)
				astTestMust(t, err)
				if values[index].IsNull() {
					t.Fatal("missing arithmetic model constant", index)
				}
			}
			if values[0] == values[1] {
				t.Fatal("UF disequality received equal arithmetic arguments")
			}
			decl, err := m.GetDecl(unused)
			astTestMust(t, err)
			value, err := md.ConstantInterpretation(decl)
			astTestMust(t, err)
			if value.IsNull() != (level != 0) {
				t.Fatal("unused Boolean interpretation ignored relevancy mode", level, value)
			}
			interpretation, err := md.FunctionInterpretation(f)
			astTestMust(t, err)
			if interpretation == nil {
				t.Fatal("missing UF model interpretation")
			}
			evaluator, err := model.NewModelEvaluator(md, cfg)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, evaluator.Close()) })
			ref, err := evaluator.EvaluateValue(equal)
			astTestMust(t, err)
			fact, err := m.RefGet(ref)
			astTestMust(t, err)
			if !m.IsFalse(fact) {
				t.Fatal("generated function interpretation violates the assertion")
			}
			astTestMust(t, m.DelRef(&ref))
		})
	}
}

type smtModelDependencyProcedure struct {
	dependencies []SMTModelValueDependency
}

func (p *smtModelDependencyProcedure) Dependencies() []SMTModelValueDependency {
	return append([]SMTModelValueDependency(nil), p.dependencies...)
}
func (*smtModelDependencyProcedure) MakeValue(*SMTModelGenerator, []ast.ASTHandle) (ast.ASTHandle, error) {
	return ast.ASTHandle{}, nil
}
func (*smtModelDependencyProcedure) IsFresh() bool { return false }

func TestSMTModelGeneratorFreshSourcesAndDependencyOrder(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "model-generator-sort"), nil)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	nodes := make([]*SMTEnode, 3)
	for index, name := range []string{"a", "b", "c"} {
		nodeSort := sort
		if index == 2 {
			nodeSort = boolSort
		}
		expression, makeErr := m.MkConst(astTestSymbol(t, m, "model-generator-"+name), nodeSort)
		astTestMust(t, makeErr)
		nodes[index], makeErr = state.MkEnode(expression, false, false, true)
		astTestMust(t, makeErr)
	}
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	fresh, err := generator.MkExtraFreshValue(sort)
	astTestMust(t, err)
	freshProcedure, err := NewSMTFreshValueProcedure(fresh)
	astTestMust(t, err)
	bDependency, err := NewSMTEnodeValueDependency(nodes[1])
	astTestMust(t, err)
	procedures := map[*SMTEnode]SMTModelValueProcedure{
		nodes[0]: &smtModelDependencyProcedure{},
		nodes[1]: freshProcedure,
		nodes[2]: &smtModelDependencyProcedure{dependencies: []SMTModelValueDependency{bDependency}},
	}
	ordered, err := generator.topSortSources(nodes, procedures)
	astTestMust(t, err)
	if len(ordered) != 4 {
		t.Fatal("source count", len(ordered))
	}
	first, err := ordered[0].Enode()
	astTestMust(t, err)
	second, err := ordered[1].FreshValue()
	astTestMust(t, err)
	third, err := ordered[2].Enode()
	astTestMust(t, err)
	fourth, err := ordered[3].Enode()
	astTestMust(t, err)
	if first != nodes[0] || second != fresh || third != nodes[1] || fourth != nodes[2] {
		t.Fatal("dependency order", first, second, third, fourth)
	}
	if smtcoreSourceDebug {
		debug, _ := newLRASourceAssertionContext(t)
		astTestMust(t, debug.EnableAssertions(true))
	}
	_, err = generator.MkExtraFreshValue(boolSort)
	if smtcoreSourceDebug {
		var exception z3exception.DefaultException
		if !errors.As(err, &exception) || exception.Message() != "assertion violation" {
			t.Fatal("finite fresh source assertion", err)
		}
	} else if err != nil {
		// Release source has no is_infinite check (theory_bv approximate_term
		// leaves large bit-vectors unattached and they take this path).
		t.Fatal("finite fresh release placeholder", err)
	}
}

func TestSMTModelGeneratorRejectsDependencyCycle(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "model-generator-cycle-sort"), nil)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	expressions := make([]ast.ASTHandle, 2)
	nodes := make([]*SMTEnode, 2)
	for index, name := range []string{"left", "right"} {
		expressions[index], err = m.MkConst(astTestSymbol(t, m, "model-cycle-"+name), sort)
		astTestMust(t, err)
		nodes[index], err = state.MkEnode(expressions[index], false, false, true)
		astTestMust(t, err)
	}
	left, err := NewSMTEnodeValueDependency(nodes[0])
	astTestMust(t, err)
	right, err := NewSMTEnodeValueDependency(nodes[1])
	astTestMust(t, err)
	procedures := map[*SMTEnode]SMTModelValueProcedure{
		nodes[0]: &smtModelDependencyProcedure{dependencies: []SMTModelValueDependency{right}},
		nodes[1]: &smtModelDependencyProcedure{dependencies: []SMTModelValueDependency{left}},
	}
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	if _, err = generator.topSortSources(nodes, procedures); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("dependency cycle", err)
	}
}

func smtModelGeneratorFixture(t *testing.T) (*ast.ASTManager, model.ModelEvaluatorConfig) {
	t.Helper()
	symbols, numerals, budget := astTestOwners(t)
	context, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	astTestMust(t, err)
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, globals.InitializeSource())
	params, err := context.NewValue()
	astTestMust(t, err)
	memory, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget})
	astTestMust(t, err)
	cooperation, err := cooperate.NewCooperation(budget)
	astTestMust(t, err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	astTestMust(t, err)
	astTestMust(t, manager.RegisterDeclarationPlugins())
	t.Cleanup(func() {
		astTestMust(t, manager.Close())
		astTestMust(t, cooperation.Close())
		astTestMust(t, memory.Close())
		astTestMust(t, context.Del(&params))
		astTestMust(t, globals.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, context.Close())
	})
	return manager, model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: &params, Memory: memory, Cooperation: cooperation}
}

func TestSMTModelGeneratorBuildsAndPinsRootValues(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "model-build-sort"), nil)
	astTestMust(t, err)
	expression, err := m.MkConst(astTestSymbol(t, m, "model-build-root"), sort)
	astTestMust(t, err)
	value, err := m.MkModelValue(0, sort)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	node, err := state.MkEnode(expression, false, false, true)
	astTestMust(t, err)
	procedure, err := NewSMTExpressionValueProcedure(value)
	astTestMust(t, err)
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generator.Close()) })
	astTestMust(t, generator.SetContext(context))
	astTestMust(t, generator.InitializeModel(config))
	astTestMust(t, generator.BuildValues([]*SMTEnode{node}, map[*SMTEnode]SMTModelValueProcedure{node: procedure}))
	if got, getErr := generator.Value(node); getErr != nil || got != value {
		t.Fatal("built root value", got, getErr)
	}
	if err = generator.BuildValues([]*SMTEnode{node}, map[*SMTEnode]SMTModelValueProcedure{node: procedure}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("duplicate build", err)
	}
}

func TestSMTModelGeneratorUnassignedBooleanCandidateDefaultsToFalse(t *testing.T) {
	for _, enode := range []bool{false, true} {
		t.Run(fmt.Sprint(enode), func(t *testing.T) {
			if enode && smtcoreSourceDebug {
				debug, _ := newLRASourceAssertionContext(t)
				astTestMust(t, debug.EnableAssertions(true))
			}
			m, config := smtModelGeneratorFixture(t)
			expression := smtRelevancyBool(t, m, "model-unassigned-boolean")
			context, err := NewSMTCheckerContext(m)
			astTestMust(t, err)
			state, err := NewSMTInternalizerState(m, context, nil)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, state.Close()) })
			_, err = state.MkBoolVar(expression)
			astTestMust(t, err)
			if enode {
				_, err = state.MkEnode(expression, false, false, true)
				astTestMust(t, err)
			}
			astTestMust(t, context.SetRelevant(expression, true))
			generator, err := NewSMTModelGenerator(m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, generator.Close()) })
			astTestMust(t, generator.SetContext(context))
			astTestMust(t, generator.InitializeModel(config))
			err = generator.BuildStateValues(state)
			if enode && smtcoreSourceDebug {
				var exception z3exception.DefaultException
				if !errors.As(err, &exception) || exception.Message() != "assertion violation" {
					t.Fatal("missing native undefined-enode debug guard", err)
				}
				return
			}
			astTestMust(t, err)
			finalModel, err := generator.FinalModel()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, finalModel.Close()) })
			declaration, err := m.GetDecl(expression)
			astTestMust(t, err)
			value, err := finalModel.Core().ConstantInterpretation(declaration)
			astTestMust(t, err)
			if !m.IsFalse(value) {
				t.Fatal("unassigned Boolean did not get native false candidate value", value)
			}
		})
	}
}

func TestSMTModelGeneratorPublishesBooleanConstant(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	expression, err := m.MkConst(astTestSymbol(t, m, "model-build-boolean"), sort)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	variable, err := state.MkBoolVar(expression)
	astTestMust(t, err)
	_, err = state.MkEnode(expression, false, false, true)
	astTestMust(t, err)
	astTestMust(t, state.SetAssignment(variable, lbool.True))
	astTestMust(t, context.SetRelevant(expression, true))
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generator.Close()) })
	astTestMust(t, generator.SetContext(context))
	astTestMust(t, generator.InitializeModel(config))
	astTestMust(t, generator.BuildStateValues(state))
	finalModel, err := generator.FinalModel()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, finalModel.Close()) })
	declaration, err := m.GetDecl(expression)
	astTestMust(t, err)
	value, err := finalModel.Core().ConstantInterpretation(declaration)
	astTestMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("Boolean model interpretation", value)
	}
}

func TestSMTModelGeneratorPublishesFunctionInterpretation(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "model-function-sort"), nil)
	astTestMust(t, err)
	argument, err := m.MkConst(astTestSymbol(t, m, "model-function-argument"), sort)
	astTestMust(t, err)
	declaration, err := m.MkFuncDecl(astTestSymbol(t, m, "model-function"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	application, err := m.MkApp(declaration, []ast.ASTHandle{argument})
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	_, err = state.MkEnode(argument, false, false, true)
	astTestMust(t, err)
	applicationNode, err := state.MkEnode(application, false, false, true)
	astTestMust(t, err)
	astTestMust(t, context.SetRelevant(argument, true))
	astTestMust(t, context.SetRelevant(application, true))
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generator.Close()) })
	astTestMust(t, generator.SetContext(context))
	astTestMust(t, generator.InitializeModel(config))
	astTestMust(t, generator.BuildStateValues(state))
	want, err := generator.Value(applicationNode)
	astTestMust(t, err)
	finalModel, err := generator.FinalModel()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, finalModel.Close()) })
	interpretation, err := finalModel.Core().FunctionInterpretation(declaration)
	astTestMust(t, err)
	if interpretation == nil || interpretation.NumEntries() != 1 {
		t.Fatal("function interpretation entries", interpretation)
	}
	entry, err := interpretation.Entry(0)
	astTestMust(t, err)
	if got, entryErr := entry.Result(); entryErr != nil || got != want {
		t.Fatal("function interpretation result", got, entryErr)
	}
}

func TestSMTModelGeneratorFinalizesTheories(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	theory := &smtInternalizerTestTheory{SMTTheoryBase: base}
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	astTestMust(t, state.RegisterTheory(theory))
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generator.Close()) })
	astTestMust(t, generator.SetContext(context))
	astTestMust(t, generator.InitializeModel(config))
	astTestMust(t, generator.BuildStateValues(state))
	if theory.finalizeModelCalls != 1 {
		t.Fatal("theory finalization calls", theory.finalizeModelCalls)
	}
}

func TestSMTModelGeneratorRegistersExistingModelValues(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "existing-model-value-sort"), nil)
	astTestMust(t, err)
	value, err := m.MkModelValue(7, sort)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	_, err = state.MkEnode(value, false, false, true)
	astTestMust(t, err)
	astTestMust(t, context.SetRelevant(value, true))
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generator.Close()) })
	astTestMust(t, generator.SetContext(context))
	astTestMust(t, generator.InitializeModel(config))
	astTestMust(t, generator.BuildStateValues(state))
	finalModel, err := generator.FinalModel()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, finalModel.Close()) })
	universe, err := finalModel.Universe(sort)
	astTestMust(t, err)
	if len(universe) != 1 || universe[0] != value {
		t.Fatal("existing model value universe", universe)
	}
}
