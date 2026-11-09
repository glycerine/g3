package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderRegistrationRelevanceAndScopeLifecycle(t *testing.T) {
	m, state, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	finder := manager.modelFinder
	if finder == nil {
		t.Fatal("quantifier manager omitted model finder")
	}
	first, _, _ := smtManagedQuantifier(t, m, "model-finder-first")
	second, _, _ := smtManagedQuantifier(t, m, "model-finder-second")
	astTestMust(t, state.Internalize(first, true))
	if len(finder.quantifiers) != 1 || finder.quantifiers[0] != first {
		t.Fatal("base quantifier was not registered", finder.quantifiers)
	}
	astTestMust(t, state.context.SetRelevant(first, true))
	astTestMust(t, state.context.SetAssignment(first, lbool.True))
	relevant, err := finder.relevantQuantifiers()
	astTestMust(t, err)
	if len(relevant) != 1 || relevant[0] != first {
		t.Fatal("active quantifier collection", relevant)
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(second, true))
	if len(finder.quantifiers) != 2 {
		t.Fatal("scoped quantifier was not registered", finder.quantifiers)
	}
	astTestMust(t, state.PopScopes(1))
	if len(finder.quantifiers) != 1 || finder.quantifiers[0] != first {
		t.Fatal("scoped quantifier was not restored", finder.quantifiers)
	}
	if _, found := finder.registered[second]; found {
		t.Fatal("popped quantifier remained registered")
	}
	astTestMust(t, manager.InitSearch())
}

func TestSMTModelFinderIgnoresTriggerOnlyDeclarations(t *testing.T) {
	m, state, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	sort, err := m.MkSort(astTestSymbol(t, m, "model-finder-pattern-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "model-finder-pattern-p", sort, boolean)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{body})
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "model-finder-pattern-x")},
		Body: truth, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(quantifier, true))
	owner, binding := smtEnodeConstant(t, m, 9100, sort, 7)
	astTestMust(t, state.context.RegisterEnode(binding, true))
	_, _ = smtQuickApplicationNode(t, m, state.context, predicate, binding, true)
	graph := newSMTModelFinderAUFGraph(manager.modelFinder)
	t.Cleanup(func() { astTestMust(t, graph.close()) })
	astTestMust(t, graph.build([]ast.ASTHandle{quantifier}))
	set := graph.uvars[smtModelFinderAUFKey{quantifier, 0}].root().set
	if len(graph.fvars) != 0 || len(set.testElements()) != 0 {
		t.Fatal("trigger-only occurrence added a body constraint", owner, set.testElements())
	}
}

func TestSMTModelFinderPublishesIsolatedSimpleMacro(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "model-finder-simple-f"), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	application, err := m.MkApp(function, []ast.ASTHandle{variable})
	astTestMust(t, err)
	body, err := m.MkEq(application, variable)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "model-finder-simple-x")}, Body: body,
	})
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.SetProduceModels(true))
	astTestMust(t, solver.Assert(quantifier))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		reason, _ := solver.ReasonUnknown()
		t.Fatal("simple macro quantifier", status, reason)
	}
	generated, err := solver.AcquireModel()
	astTestMust(t, err)
	if generated == nil {
		t.Fatal("simple macro omitted generated model")
	}
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	interpretation, err := generated.FunctionInterpretation(function)
	astTestMust(t, err)
	if interpretation == nil {
		t.Fatal("simple macro function interpretation was not published")
	}
	fallback, err := interpretation.Else()
	astTestMust(t, err)
	index, err := m.GetVarIndex(fallback)
	astTestMust(t, err)
	if index != 0 {
		t.Fatal("simple macro fallback", fallback, index)
	}
}

func TestSMTModelFinderPublishesSharedHintMacro(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "model-finder-hint-f"), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	application, err := m.MkApp(function, []ast.ASTHandle{variable})
	astTestMust(t, err)
	body, err := m.MkEq(application, variable)
	astTestMust(t, err)
	makeQuantifier := func(name string) ast.ASTHandle {
		q, e := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")},
			Body: body, QID: astTestSymbol(t, m, name),
		})
		astTestMust(t, e)
		return q
	}
	first, second := makeQuantifier("model-finder-hint-q1"), makeQuantifier("model-finder-hint-q2")
	if first == second {
		t.Fatal("shared-macro quantifiers were hash-consed")
	}
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.SetProduceModels(true))
	astTestMust(t, solver.Assert(first))
	astTestMust(t, solver.Assert(second))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		reason, _ := solver.ReasonUnknown()
		t.Fatal("shared hint macro quantifiers", status, reason)
	}
	finder := solver.kernel.internalizer.quantifierManager.modelFinder
	// Pattern inference rebuilds the quantified assertions before the model
	// finder sees them. Verify both retained QIDs and their selected definition.
	seen := make(map[symbol.Symbol]bool)
	for q, selected := range finder.chosen {
		qid, _, err := m.GetQuantifierIDs(q)
		astTestMust(t, err)
		seen[qid] = true
		if selected != function {
			t.Fatal("hint search selected the wrong shared definition", finder.chosen)
		}
	}
	if len(seen) != 2 || !seen[astTestSymbol(t, m, "model-finder-hint-q1")] || !seen[astTestSymbol(t, m, "model-finder-hint-q2")] {
		t.Fatal("hint search omitted a shared definition quantifier", seen)
	}
	generated, err := solver.AcquireModel()
	astTestMust(t, err)
	if generated == nil {
		t.Fatal("shared hint macro omitted generated model")
	}
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	interpretation, err := generated.FunctionInterpretation(function)
	astTestMust(t, err)
	if interpretation == nil {
		t.Fatal("shared hint macro function interpretation was not published")
	}
	fallback, err := interpretation.Else()
	astTestMust(t, err)
	if index, e := m.GetVarIndex(fallback); e != nil || index != 0 {
		t.Fatal("shared hint macro fallback", fallback, index, e)
	}
}

func TestSMTModelFinderForcedFullMacrosRejectDependencyCycle(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	makeFunction := func(name string) ast.ASTHandle {
		declaration, e := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		return declaration
	}
	f, g := makeFunction("model-finder-full-f"), makeFunction("model-finder-full-g")
	fApp, err := m.MkApp(f, []ast.ASTHandle{variable})
	astTestMust(t, err)
	gApp, err := m.MkApp(g, []ast.ASTHandle{variable})
	astTestMust(t, err)
	body, err := m.MkEq(fApp, gApp)
	astTestMust(t, err)
	makeQuantifier := func(name string) ast.ASTHandle {
		quantifier, e := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")},
			Body: body, Weight: 10, QID: astTestSymbol(t, m, name),
		})
		astTestMust(t, e)
		return quantifier
	}
	first, second := makeQuantifier("model-finder-full-q1"), makeQuantifier("model-finder-full-q2")
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	astTestMust(t, state.Internalize(first, true))
	astTestMust(t, state.Internalize(second, true))
	finder := state.quantifierManager.modelFinder
	if len(finder.quantifiers) != 2 {
		t.Fatal("forced full macro quantifiers were not registered", finder.quantifiers)
	}
	first, second = finder.quantifiers[0], finder.quantifiers[1]
	truth, err := m.MkTrue()
	astTestMust(t, err)
	for quantifier, candidate := range map[ast.ASTHandle]macros.Candidate{
		first:  {Declaration: f, Definition: gApp, Condition: truth, SatisfyAtom: true},
		second: {Declaration: g, Definition: fApp, Condition: truth, SatisfyAtom: true},
	} {
		astTestMust(t, finder.candidates[quantifier].Reset())
		astTestMust(t, finder.candidates[quantifier].Insert(candidate))
		finder.auf[quantifier] = true
	}
	finder.hint.infos[first].macros = []hintConditionalMacro{{function: f, definition: gApp, satisfyAtom: true}}
	finder.hint.infos[second].macros = []hintConditionalMacro{{function: g, definition: fApp, satisfyAtom: true}}
	astTestMust(t, state.context.SetRelevant(first, true))
	astTestMust(t, state.context.SetAssignment(first, lbool.True))
	astTestMust(t, state.context.SetRelevant(second, true))
	astTestMust(t, state.context.SetAssignment(second, lbool.True))
	proto, err := protomodel.NewProtoModel(m, modelConfig)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	if finder.forceTemplate != 10 {
		t.Fatal("mbqi.force_template was not propagated", finder.forceTemplate)
	}
	astTestMust(t, finder.FixModel(proto))
	if finder.chosen[first] != f || !finder.chosen[second].IsNull() {
		t.Fatal("forced full macro cycle selection", finder.chosen)
	}
	if dependencies := finder.dependencies.GetDependencies(f); dependencies.Len() != 1 {
		t.Fatal("full macro dependencies", dependencies)
	} else if !dependencies.Contains(g) {
		t.Fatal("full macro omitted dependency", dependencies)
	}
	if finder.dependencies.Contains(g) {
		t.Fatal("cyclic full macro dependency was retained")
	}
	interpretation, err := proto.Core().FunctionInterpretation(f)
	astTestMust(t, err)
	if interpretation == nil {
		t.Fatal("accepted full macro was not published")
	}
	definition, err := interpretation.Else()
	astTestMust(t, err)
	if definition != gApp {
		t.Fatal("full macro definition", definition, gApp)
	}
	interpretation, err = proto.Core().FunctionInterpretation(g)
	astTestMust(t, err)
	if interpretation != nil {
		// AUF may complete the remaining ground function; it must still
		// reject the cyclic full-macro definition g(x)=f(x).
		fallback, err := interpretation.Else()
		astTestMust(t, err)
		if fallback == fApp {
			t.Fatal("cyclic full macro was published")
		}
	}
}

func TestSMTModelFinderAUFClassification(t *testing.T) {
	m, _, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	predicate, err := m.MkFuncDecl(astTestSymbol(t, m, "model-finder-auf-p"), []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	predicateApp, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	bound, err := m.MkArithmeticLE(variable, zero)
	astTestMust(t, err)
	product, err := m.MkArithmeticMul(variable, variable)
	astTestMust(t, err)
	nonlinear, err := m.MkArithmeticLE(product, zero)
	astTestMust(t, err)
	makeQuantifier := func(name string, body ast.ASTHandle) ast.ASTHandle {
		quantifier, e := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")}, Body: body,
		})
		astTestMust(t, e)
		return quantifier
	}
	for _, test := range []struct {
		name string
		body ast.ASTHandle
		want bool
	}{{"uninterpreted-variable-argument", predicateApp, true}, {"variable-ground-bound", bound, true}, {"interpreted-nonlinear-term", nonlinear, false}} {
		quantifier := makeQuantifier("model-finder-auf-"+test.name, test.body)
		got, err := manager.modelFinder.classifyAUF(quantifier)
		astTestMust(t, err)
		if got != test.want {
			t.Fatalf("%s classification %v want %v", test.name, got, test.want)
		}
	}
}

func TestSMTModelFinderCombinesConditionalNonAUFMacros(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "model-finder-conditional-f"), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	application, err := m.MkApp(function, []ast.ASTHandle{variable})
	astTestMust(t, err)
	equality, err := m.MkEq(application, zero)
	astTestMust(t, err)
	product, err := m.MkArithmeticMul(variable, variable)
	astTestMust(t, err)
	nonlinear, err := m.MkArithmeticLE(product, zero)
	astTestMust(t, err)
	body, err := m.MkAndArray([]ast.ASTHandle{equality, nonlinear})
	astTestMust(t, err)
	condition1, err := m.MkArithmeticLE(variable, zero)
	astTestMust(t, err)
	condition2, err := m.MkArithmeticGE(variable, zero)
	astTestMust(t, err)
	makeQuantifier := func(name string) ast.ASTHandle {
		quantifier, e := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")},
			Body: body, QID: astTestSymbol(t, m, name),
		})
		astTestMust(t, e)
		return quantifier
	}
	first, second := makeQuantifier("model-finder-conditional-q1"), makeQuantifier("model-finder-conditional-q2")
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	astTestMust(t, state.Internalize(first, true))
	astTestMust(t, state.Internalize(second, true))
	finder := state.quantifierManager.modelFinder
	for quantifier, candidate := range map[ast.ASTHandle]macros.Candidate{
		first:  {Declaration: function, Definition: zero, Condition: condition1, SatisfyAtom: true},
		second: {Declaration: function, Definition: one, Condition: condition2, SatisfyAtom: true},
	} {
		astTestMust(t, finder.candidates[quantifier].Reset())
		astTestMust(t, finder.candidates[quantifier].Insert(candidate))
		finder.auf[quantifier] = false
		finder.hint.infos[quantifier].macros = []hintConditionalMacro{{function: function, definition: candidate.Definition, satisfyAtom: true}}
		astTestMust(t, state.context.SetRelevant(quantifier, true))
		astTestMust(t, state.context.SetAssignment(quantifier, lbool.True))
	}
	proto, err := protomodel.NewProtoModel(m, modelConfig)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	astTestMust(t, finder.FixModel(proto))
	if finder.chosen[first] != function || finder.chosen[second] != function {
		t.Fatal("conditional non-AUF macros were not selected", finder.chosen)
	}
	interpretation, err := proto.Core().FunctionInterpretation(function)
	astTestMust(t, err)
	if interpretation == nil {
		t.Fatal("conditional non-AUF definition was not published")
	}
	definition, err := interpretation.Else()
	astTestMust(t, err)
	want, err := m.MkIte(condition2, one, zero)
	astTestMust(t, err)
	if definition != want {
		t.Fatal("conditional non-AUF definition", definition, want)
	}
}
