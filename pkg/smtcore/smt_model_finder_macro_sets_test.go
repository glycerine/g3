package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderMacroSetsPreserveOriginalRelevantArguments(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	finder := state.quantifierManager.modelFinder
	seven, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	integer, err := m.GetSort(seven)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "macro-set-f"), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	application, err := m.MkApp(function, []ast.ASTHandle{x})
	astTestMust(t, err)
	body, err := m.MkEq(application, x)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "macro-set-x")}, Body: body})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(q, true))
	astTestMust(t, state.context.SetRelevant(q, true))
	astTestMust(t, state.context.SetAssignment(q, lbool.True))
	sevenNode, err := NewSMTEnode(m, seven, state.context.enodes.get, SMTEnodeOptions{Generation: 4})
	astTestMust(t, err)
	astTestMust(t, state.context.RegisterEnode(sevenNode, true))
	threeNode, err := NewSMTEnode(m, three, state.context.enodes.get, SMTEnodeOptions{Generation: 1})
	astTestMust(t, err)
	astTestMust(t, state.context.RegisterEnode(threeNode, true))
	sevenApplication, _ := smtQuickApplicationNode(t, m, state.context, function, sevenNode, true)
	threeApplication, _ := smtQuickApplicationNode(t, m, state.context, function, threeNode, false)
	sevenNode.root = threeNode
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	astTestMust(t, finder.FixModel(proto))
	if finder.chosen[q] != function {
		t.Fatal("simple macro was not selected")
	}
	set, err := finder.instantiationSet(q, 0, false)
	astTestMust(t, err)
	if set == nil || len(set.elements) != 1 || set.elements[seven] != 4 {
		t.Fatal("macro set lost the original relevant argument", set)
	}
	term, generation, found, err := finder.Inverse(q, 0, seven)
	astTestMust(t, err)
	if !found || term != seven || generation != 4 {
		t.Fatal("macro-based inverse was not built", term, generation, found)
	}
	// With filtering disabled, unmarked quantifiers and applications are
	// relevant too. Both original arguments must enter the macro set.
	state.relevancy.enabled = false
	astTestMust(t, state.context.SetRelevant(q, false))
	unfiltered, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, unfiltered.Close()) })
	astTestMust(t, finder.FixModel(unfiltered))
	all, err := finder.instantiationSet(q, 0, false)
	astTestMust(t, err)
	if all == nil || len(all.elements) != 2 || all.elements[seven] != 4 || all.elements[three] != 1 {
		t.Fatal("disabled filtering omitted a quantifier or macro argument", all)
	}
	state.relevancy.enabled = true
	astTestMust(t, state.context.SetRelevant(q, true))
	astTestMust(t, state.context.SetRelevant(sevenApplication, false))
	astTestMust(t, state.context.SetRelevant(threeApplication, true))
	next, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, next.Close()) })
	astTestMust(t, finder.FixModel(next))
	updated, err := finder.instantiationSet(q, 0, false)
	astTestMust(t, err)
	if !set.closed || !all.closed || updated == set || updated == all || updated == nil || len(updated.elements) != 1 || updated.elements[three] != 1 {
		t.Fatal("new model retained prior terms or ownership", set.closed, updated)
	}
	_, _, found, err = finder.Inverse(q, 0, seven)
	astTestMust(t, err)
	if found {
		t.Fatal("new model retained an obsolete inverse entry")
	}
	term, generation, found, err = finder.Inverse(q, 0, three)
	astTestMust(t, err)
	if !found || term != three || generation != 1 {
		t.Fatal("new model did not rebuild inverse", term, generation, found)
	}
}

func TestSMTModelFinderHintPassRepeatsForDisjointSubsets(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	finder := solver.kernel.internalizer.quantifierManager.modelFinder
	var quantifiers []ast.ASTHandle
	want := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, name := range []string{"hint-subset-f", "hint-subset-g"} {
		function, err := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		application, err := m.MkApp(function, []ast.ASTHandle{x})
		astTestMust(t, err)
		body, err := m.MkEq(application, x)
		astTestMust(t, err)
		for _, suffix := range []string{"a", "b"} {
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, name+suffix)}, Body: body, QID: astTestSymbol(t, m, name+suffix)})
			astTestMust(t, err)
			astTestMust(t, finder.RegisterQuantifier(q))
			quantifiers = append(quantifiers, q)
			want[q] = function
		}
	}
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	remaining, err := finder.processHintMacros(proto, quantifiers)
	astTestMust(t, err)
	if len(remaining) != 0 {
		t.Fatal("hint solver stopped after the first satisfiable subset", remaining)
	}
	for q, function := range want {
		if finder.chosen[q] != function {
			t.Fatal("hint solver omitted a disjoint subset", q, finder.chosen[q])
		}
	}
}
