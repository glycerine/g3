package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderNonAUFMacrosRetainEarlierPassResidue(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	finder := state.quantifierManager.modelFinder
	finder.SetForceTemplate(100)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	function := func(name string) ast.ASTHandle {
		f, err := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		ref, err := m.NewRef(f)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
		return f
	}
	f, g := function("residue-f"), function("residue-g")
	fApp, err := m.MkApp(f, []ast.ASTHandle{x})
	astTestMust(t, err)
	body, err := m.MkEq(fApp, zero)
	astTestMust(t, err)
	condition, err := m.MkArithmeticGE(x, zero)
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	var qs []ast.ASTHandle
	for _, name := range []string{"residue-blocker", "residue-retained", "residue-enabler"} {
		q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")}, Body: body, QID: astTestSymbol(t, m, name), Weight: 1})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(q, true))
		astTestMust(t, finder.candidates[q].Reset())
		qs = append(qs, q)
	}
	// The AUF quantifier's unconditional f macro blocks conditional f in
	// pass one. Selecting g removes that blocker, enabling f in pass two.
	finder.auf[qs[0]], finder.auf[qs[1]], finder.auf[qs[2]] = true, false, false
	astTestMust(t, finder.candidates[qs[0]].Insert(macros.Candidate{Declaration: f, Definition: zero, Condition: yes}))
	astTestMust(t, finder.candidates[qs[0]].Insert(macros.Candidate{Declaration: g, Definition: zero, Condition: condition}))
	astTestMust(t, finder.candidates[qs[1]].Insert(macros.Candidate{Declaration: f, Definition: one, Condition: condition}))
	astTestMust(t, finder.candidates[qs[2]].Insert(macros.Candidate{Declaration: g, Definition: one, Condition: condition}))
	ground, err := m.MkApp(f, []ast.ASTHandle{seven})
	astTestMust(t, err)
	astTestMust(t, state.SetGeneration(4))
	astTestMust(t, state.Internalize(ground, false))
	astTestMust(t, state.context.SetRelevant(ground, true))
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	factory, err := protomodel.NewArithmeticFactory(m)
	astTestMust(t, err)
	astTestMust(t, proto.RegisterFactory(factory))
	residue, err := finder.processNonAUFMacros(proto, qs)
	astTestMust(t, err)
	if finder.chosen[qs[0]] != g || finder.chosen[qs[1]] != f || finder.chosen[qs[2]] != g {
		t.Fatal("fixture did not select macros in two passes", finder.chosen)
	}
	if len(residue) != 1 || residue[0] != qs[1] {
		t.Fatal("later macro selection discarded earlier AUF residue", residue)
	}
	astTestMust(t, finder.populateAUFInstantiationSets(residue, proto))
	set, err := finder.instantiationSet(qs[1], 0, false)
	astTestMust(t, err)
	astTestMust(t, set.MakeInverse(finder.aufSolver.eval))
	term, generation, found, err := finder.Inverse(qs[1], 0, seven)
	astTestMust(t, err)
	if !found || term != seven || generation != 4 {
		t.Fatal("retained residue lost its ground instantiation term", found, term, generation)
	}
}
