package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderAUFSelectIndexesUseModelFunctionAndRelevantParents(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	finder := state.quantifierManager.modelFinder
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	four, err := m.ArithmeticIntegerNumeralText("4")
	astTestMust(t, err)
	nine, err := m.ArithmeticIntegerNumeralText("9")
	astTestMust(t, err)
	integer, err := m.GetSort(four)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "auf-select-f"), []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	asArray, err := m.MkAsArray(function)
	astTestMust(t, err)
	arraySort, err := m.GetSort(asArray)
	astTestMust(t, err)
	array, err := m.MkConst(astTestSymbol(t, m, "auf-select-array"), arraySort)
	astTestMust(t, err)
	otherArray, err := m.MkConst(astTestSymbol(t, m, "auf-select-other-array"), arraySort)
	astTestMust(t, err)
	x, err := m.MkVar(1, boolean)
	astTestMust(t, err)
	y, err := m.MkVar(0, integer)
	astTestMust(t, err)
	body, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, x, y})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean, integer},
		Names: []symbol.Symbol{astTestSymbol(t, m, "auf-select-x"), astTestSymbol(t, m, "auf-select-y")}, Body: body,
	})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(q, true))
	astTestMust(t, state.context.SetRelevant(q, true))
	astTestMust(t, state.context.SetAssignment(q, lbool.True))
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	makeNode := func(owner ast.ASTHandle, relevant bool, generation uint32) *SMTEnode {
		if smtTestInitEnode(t, state, owner, relevant, generation) {
			return state.context.enodes.get(owner)
		}
		node, err := NewSMTEnode(m, owner, state.context.enodes.get, SMTEnodeOptions{Generation: generation, UpdateChildrenParent: true})
		astTestMust(t, err)
		astTestMust(t, state.context.RegisterEnode(node, relevant))
		return node
	}
	makeNode(array, true, 0)
	makeNode(otherArray, true, 0)
	makeNode(falsehood, true, 2)
	makeNode(truth, true, 3)
	fourNode := makeNode(four, true, 6)
	nineNode := makeNode(nine, true, 7)
	for _, test := range []struct {
		array, first, second ast.ASTHandle
		relevant             bool
	}{{array, falsehood, four, true}, {array, truth, nine, false}, {otherArray, truth, nine, true}} {
		selected, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{test.array, test.first, test.second})
		astTestMust(t, err)
		makeNode(selected, test.relevant, 0)
	}
	fourNode.root = nineNode
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	decl, err := m.GetDecl(array)
	astTestMust(t, err)
	astTestMust(t, proto.Core().RegisterConstant(decl, asArray))
	interpretation, err := model.NewFunctionInterpretation(m, 2)
	astTestMust(t, err)
	// AUF runs before partial functions are completed. Keeping this
	// interpretation partial preserves its as-array value during evaluation.
	astTestMust(t, proto.Core().RegisterFunction(function, interpretation))
	graph := newSMTModelFinderAUFGraph(finder)
	graph.model = proto
	t.Cleanup(func() { astTestMust(t, graph.close()) })
	resolved, err := graph.arrayFunction(array)
	astTestMust(t, err)
	if resolved != function {
		t.Fatal("array model function resolution", resolved, function)
	}
	astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
	if len(graph.selects) != 2 {
		t.Fatal("array index qinfo collection", graph.selects)
	}
	state.relevancy.enabled = false
	astTestMust(t, state.context.SetRelevant(array, false))
	unfiltered := newSMTModelFinderAUFGraph(finder)
	unfiltered.model = proto
	t.Cleanup(func() { astTestMust(t, unfiltered.close()) })
	astTestMust(t, aufTestBuild(t, unfiltered, []ast.ASTHandle{q}))
	for _, test := range []struct {
		variable      uint32
		first, second ast.ASTHandle
	}{{0, falsehood, truth}, {1, four, nine}} {
		node := unfiltered.uvars[smtModelFinderAUFKey{q, 1 - test.variable}].root()
		if len(node.set.testElements()) != 2 {
			t.Fatal("disabled filtering omitted an array or select parent", test.variable, node.set.testElements())
		}
		if _, found := node.set.testElements()[test.first]; !found {
			t.Fatal("missing first select index", test.variable)
		}
		if _, found := node.set.testElements()[test.second]; !found {
			t.Fatal("missing unmarked select index", test.variable)
		}
	}
	state.relevancy.enabled = true
	astTestMust(t, state.context.SetRelevant(array, true))
	astTestMust(t, finder.FixModel(proto))
	for _, test := range []struct {
		variable   uint32
		term       ast.ASTHandle
		generation uint32
	}{{0, falsehood, 2}, {1, four, 6}} {
		set, err := finder.instantiationSet(q, test.variable, false)
		astTestMust(t, err)
		if set == nil || len(set.testElements()) != 1 || set.testElements()[test.term] != test.generation {
			t.Fatal("array-select instantiation set used wrong position, parent or class representative", test.variable, set)
		}
	}
}

func TestSMTModelFinderAUFSelectNestedArrayParents(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	innerFunction, err := m.MkFuncDecl(astTestSymbol(t, m, "auf-nested-inner"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	innerValue, err := m.MkAsArray(innerFunction)
	astTestMust(t, err)
	innerSort, err := m.GetSort(innerValue)
	astTestMust(t, err)
	outerFunction, err := m.MkFuncDecl(astTestSymbol(t, m, "auf-nested-outer"), []ast.ASTHandle{boolean}, innerSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	outerValue, err := m.MkAsArray(outerFunction)
	astTestMust(t, err)
	outerSort, err := m.GetSort(outerValue)
	astTestMust(t, err)
	array, err := m.MkConst(astTestSymbol(t, m, "auf-nested-array"), outerSort)
	astTestMust(t, err)
	selectTerm := func(array, index ast.ASTHandle) ast.ASTHandle {
		term, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, index})
		astTestMust(t, err)
		return term
	}
	x, err := m.MkVar(1, boolean)
	astTestMust(t, err)
	y, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	nestedPattern := selectTerm(array, x)
	body := selectTerm(nestedPattern, y)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean, boolean},
		Names: []symbol.Symbol{astTestSymbol(t, m, "auf-nested-x"), astTestSymbol(t, m, "auf-nested-y")}, Body: body,
	})
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	makeNode := func(term ast.ASTHandle, generation uint32) {
		if smtTestInitEnode(t, state, term, true, generation) {
			return
		}
		node, err := NewSMTEnode(m, term, state.context.enodes.get, SMTEnodeOptions{Generation: generation, UpdateChildrenParent: true})
		astTestMust(t, err)
		astTestMust(t, state.context.RegisterEnode(node, true))
	}
	makeNode(array, 0)
	makeNode(truth, 5)
	makeNode(falsehood, 4)
	groundInner := selectTerm(array, falsehood)
	makeNode(groundInner, 1)
	makeNode(selectTerm(groundInner, truth), 2)
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	decl, err := m.GetDecl(array)
	astTestMust(t, err)
	astTestMust(t, proto.Core().RegisterConstant(decl, outerValue))
	outer, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, outer.InsertEntry([]ast.ASTHandle{falsehood}, innerValue))
	astTestMust(t, proto.Core().RegisterFunction(outerFunction, outer))
	inner, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, proto.Core().RegisterFunction(innerFunction, inner))
	graph := newSMTModelFinderAUFGraph(state.quantifierManager.modelFinder)
	graph.model = proto
	t.Cleanup(func() { astTestMust(t, graph.close()) })
	astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
	// AUDIT.md smtcore_05-F3: is_auf_select checks every argument, the
	// array included, so (select (select A x) y) is not an auf select. Only
	// the inner (select A x) gives a select_var; y is visited as a term and,
	// being Boolean, then as a negative formula only (visit_term marked it
	// in m_pos_cache): the qinfo (x_neq_t y true). y gets no array-parent
	// indexes.
	finder := state.quantifierManager.modelFinder
	info := finder.qinfos[q]
	if info == nil || len(info.selects) != 1 || info.selects[0].selectTerm != nestedPattern ||
		info.selects[0].position != 1 || info.selects[0].variable != 1 {
		t.Fatal("nested select qinfos differ from is_auf_select", info)
	}
	if len(info.literals) != 1 || info.literals[0].kind != aufDisequalGround || info.literals[0].first != 0 || info.literals[0].term != truth {
		t.Fatal("Boolean outer index qinfo", info.literals)
	}
	if !finder.auf[q] {
		t.Fatal("Boolean outer index cleared m_is_auf")
	}
	xNode := graph.uvars[smtModelFinderAUFKey{q, 1}].root()
	if len(xNode.set.testElements()) != 1 || xNode.set.testElements()[falsehood] != 4 {
		t.Fatal("inner select did not follow the ground array's parents", xNode.set.testElements())
	}
	yNode := graph.uvars[smtModelFinderAUFKey{q, 0}].root()
	if len(yNode.set.testElements()) != 1 || yNode.set.testElements()[truth] != 0 {
		t.Fatal("outer select index followed nested array parents", yNode.set.testElements())
	}
	astTestMust(t, state.context.SetRelevant(array, false))
	astTestMust(t, state.context.SetRelevant(groundInner, false))
	matches, err := graph.matchingArrays(nestedPattern)
	astTestMust(t, err)
	if len(matches) != 0 {
		t.Fatal("enabled filtering accepted an unmarked nested array")
	}
	state.relevancy.enabled = false
	matches, err = graph.matchingArrays(nestedPattern)
	astTestMust(t, err)
	if len(matches) != 1 || matches[0].owner != groundInner {
		t.Fatal("disabled filtering skipped nested array parents", matches)
	}
}

// smtTestInitEnode reuses context::init's true or false enode for a fixture
// that wants one with a given relevance and generation.
func smtTestInitEnode(t *testing.T, state *SMTInternalizerState, owner ast.ASTHandle, relevant bool, generation uint32) bool {
	t.Helper()
	node := state.context.enodes.get(owner)
	if node == nil {
		return false
	}
	node.generation = generation
	astTestMust(t, state.context.SetRelevant(owner, relevant))
	return true
}
