package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderFlattenOwnsScopedQuantifierAndMapsOriginalBinders(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	finder := state.quantifierManager.modelFinder
	seven, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	integer, err := m.GetSort(seven)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "flatten-p"), []ast.ASTHandle{integer, boolean}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	x, err := m.MkVar(1, integer)
	astTestMust(t, err)
	y, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	body, err := m.MkApp(function, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	inner, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "flatten-y")}, Body: body, Weight: 2,
	})
	astTestMust(t, err)
	qid := astTestSymbol(t, m, "flatten-outer")
	outer, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "flatten-x")}, Body: inner, Weight: 5, QID: qid,
	})
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(outer, true))
	flat := finder.flat[outer]
	count, err := m.GetNumDeclarations(flat.quantifier)
	astTestMust(t, err)
	weight, err := m.GetQuantifierWeight(flat.quantifier)
	astTestMust(t, err)
	flatQID, _, err := m.GetQuantifierIDs(flat.quantifier)
	astTestMust(t, err)
	flatBody, err := m.GetQuantifierBody(flat.quantifier)
	astTestMust(t, err)
	if count != 2 || weight != 2 || flatQID != qid || flatBody != body || flat.quantifier == outer {
		t.Fatal("source flattened binder metadata", count, weight, flatQID, flatBody)
	}
	truth, err := m.MkTrue()
	astTestMust(t, err)
	for _, term := range []ast.ASTHandle{seven, truth} {
		if smtTestInitEnode(t, state, term, true, 4) {
			continue
		}
		node, err := NewSMTEnode(m, term, state.context.enodes.get, SMTEnodeOptions{Generation: 4})
		astTestMust(t, err)
		astTestMust(t, state.context.RegisterEnode(node, true))
	}
	ground, err := m.MkApp(function, []ast.ASTHandle{seven, truth})
	astTestMust(t, err)
	node, err := NewSMTEnode(m, ground, state.context.enodes.get, SMTEnodeOptions{})
	astTestMust(t, err)
	astTestMust(t, state.context.RegisterEnode(node, true))
	astTestMust(t, state.context.SetRelevant(outer, true))
	astTestMust(t, state.context.SetAssignment(outer, lbool.True))
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	astTestMust(t, finder.FixModel(proto))
	set, err := finder.instantiationSet(outer, 0, false)
	astTestMust(t, err)
	if set == nil || len(set.elements) != 1 || set.elements[seven] != 4 {
		t.Fatal("flat graph failed to map original outer binder", set)
	}
	if len(finder.sets[outer]) != 1 {
		t.Fatal("inner binder leaked into original instance bindings")
	}
	astTestMust(t, state.PopScopes(1))
	if _, found := finder.flat[outer]; found {
		t.Fatal("popped quantifier retained flattened analysis")
	}
	if _, err := m.RefGet(flat.reference); err == nil {
		t.Fatal("popped flattened AST reference was not released")
	}
}

func TestSMTModelCheckerFlattenChecksAllBindersAndInstantiatesOriginal(t *testing.T) {
	for _, innerInteger := range []bool{false, true} {
		name := "finite-inner"
		if innerInteger {
			name = "infinite-inner"
		}
		t.Run(name, func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			state := solver.kernel.internalizer
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			innerSort := boolean
			if innerInteger {
				zero, err := m.ArithmeticIntegerNumeralText("0")
				astTestMust(t, err)
				innerSort, err = m.GetSort(zero)
				astTestMust(t, err)
			}
			function, err := m.MkFuncDecl(astTestSymbol(t, m, "flatten-check-p"), []ast.ASTHandle{boolean, innerSort}, boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			x, err := m.MkVar(1, boolean)
			astTestMust(t, err)
			y, err := m.MkVar(0, innerSort)
			astTestMust(t, err)
			body, err := m.MkApp(function, []ast.ASTHandle{x, y})
			astTestMust(t, err)
			inner, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{innerSort}, Names: []symbol.Symbol{astTestSymbol(t, m, "flatten-check-y")}, Body: body})
			astTestMust(t, err)
			outer, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "flatten-check-x")}, Body: inner})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(outer, true))
			proto, err := protomodel.NewProtoModel(m, config)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, proto.Close()) })
			falsehood, err := m.MkFalse()
			astTestMust(t, err)
			interpretation, err := model.NewFunctionInterpretation(m, 2)
			astTestMust(t, err)
			astTestMust(t, interpretation.SetElse(falsehood))
			astTestMust(t, proto.Core().RegisterFunction(function, interpretation))
			checker := state.quantifierManager.modelChecker
			satisfied, err := checker.Check(proto, []ast.ASTHandle{outer})
			astTestMust(t, err)
			astTestMust(t, checker.Restart())
			if satisfied || len(state.quantifiers.schedule.newEntries) != 1 {
				t.Fatal("flattened counterexample was not queued", satisfied)
			}
			entries, err := state.quantifiers.schedule.ScheduleNew(map[*SMTFingerprint]bool{})
			astTestMust(t, err)
			if len(entries) != 1 {
				t.Fatal("flattened counterexample queue", len(entries))
			}
			// Materialization validates the original quantifier's binding
			// count and yields a residual inner quantifier for later work.
			astTestMust(t, state.quantifierManager.instantiate(entries))
		})
	}
}
