package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderAUFGraphSharesPatternlessArgumentSets(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	finder := state.quantifierManager.modelFinder
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "auf-graph-sort"), nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "auf-graph-p", sort, boolean)
	r := smtQuickPredicate(t, m, "auf-graph-r", sort, boolean)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	astTestMust(t, err)
	rx, err := m.MkApp(r, []ast.ASTHandle{x})
	astTestMust(t, err)
	body, err := m.MkAnd(px, rx)
	astTestMust(t, err)
	makeQuantifier := func(name string, body ast.ASTHandle) ast.ASTHandle {
		q, e := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, name)}, Body: body,
		})
		astTestMust(t, e)
		astTestMust(t, state.Internalize(q, true))
		astTestMust(t, state.context.SetRelevant(q, true))
		astTestMust(t, state.context.SetAssignment(q, lbool.True))
		return q
	}
	first := makeQuantifier("auf-graph-x", body)
	second := makeQuantifier("auf-graph-y", rx)
	a, aNode := smtEnodeConstant(t, m, 9300, sort, 3)
	b, bNode := smtEnodeConstant(t, m, 9301, sort, 7)
	ignored, ignoredNode := smtEnodeConstant(t, m, 9302, sort, 1)
	for _, node := range []*SMTEnode{aNode, bNode, ignoredNode} {
		astTestMust(t, state.context.RegisterEnode(node, true))
	}
	smtQuickApplicationNode(t, m, state.context, p, aNode, true)
	smtQuickApplicationNode(t, m, state.context, r, bNode, true)
	smtQuickApplicationNode(t, m, state.context, p, ignoredNode, false)
	// Mimic an accidental model-based equivalence merge. The original b
	// argument must survive even when its root points at a.
	bNode.root = aNode
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	astTestMust(t, finder.FixModel(proto))
	for _, q := range []ast.ASTHandle{first, second} {
		set, err := finder.instantiationSet(q, 0, false)
		astTestMust(t, err)
		if set == nil || len(set.testElements()) != 2 || set.testElements()[a] != 3 || set.testElements()[b] != 7 {
			t.Fatalf("q %v did not inherit shared argument terms: %+v", q, set)
		}
		if _, found := set.testElements()[ignored]; found {
			t.Fatal("irrelevant ground application contributed an argument")
		}
	}
	state.relevancy.enabled = false
	unfiltered := newSMTModelFinderAUFGraph(finder)
	t.Cleanup(func() { astTestMust(t, unfiltered.close()) })
	astTestMust(t, aufTestBuild(t, unfiltered, []ast.ASTHandle{first, second}))
	for _, q := range []ast.ASTHandle{first, second} {
		set := unfiltered.uvars[smtModelFinderAUFKey{q, 0}].root().set
		if len(set.testElements()) != 3 || set.testElements()[a] != 3 || set.testElements()[b] != 7 || set.testElements()[ignored] != 1 {
			t.Fatal("disabled filtering omitted a shared function argument", set.testElements())
		}
	}
}

func TestSMTModelFinderAUFGraphEqualityAndDisequalityConstraints(t *testing.T) {
	m, state, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	relevancy, err := NewSMTRelevancyPropagator(state.context, true)
	astTestMust(t, err)
	astTestMust(t, state.SetRelevancy(relevancy))
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "auf-literal-sort"), nil)
	astTestMust(t, err)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	y, err := m.MkVar(1, sort)
	astTestMust(t, err)
	a, aNode := smtEnodeConstant(t, m, 9400, sort, 4)
	b, bNode := smtEnodeConstant(t, m, 9401, sort, 8)
	astTestMust(t, state.context.RegisterEnode(aNode, true))
	astTestMust(t, state.context.RegisterEnode(bNode, true))
	unmarked, unmarkedNode := smtEnodeConstant(t, m, 9402, sort, 9)
	astTestMust(t, state.context.RegisterEnode(unmarkedNode, false))
	equalVariables, err := m.MkEq(x, y)
	astTestMust(t, err)
	equalGround, err := m.MkEq(x, a)
	astTestMust(t, err)
	disequalGround, err := m.MkNot(equalGround)
	astTestMust(t, err)
	disequalVariables, err := m.MkNot(equalVariables)
	astTestMust(t, err)
	for _, test := range []struct {
		name      string
		body      ast.ASTHandle
		merged    bool
		avoid     bool
		exception bool
		terms     int
	}{
		{"equal-variables", equalVariables, false, true, false, 0},
		{"disequal-variables", disequalVariables, true, false, false, 0},
		{"equal-ground", equalGround, false, false, true, 2},
		{"disequal-ground", disequalGround, false, false, false, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{sort, sort},
				Names: []symbol.Symbol{astTestSymbol(t, m, "auf-literal-y"), astTestSymbol(t, m, "auf-literal-x")}, Body: test.body,
			})
			astTestMust(t, err)
			graph := newSMTModelFinderAUFGraph(manager.modelFinder)
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
			first := graph.uvars[smtModelFinderAUFKey{q, 0}].root()
			second := graph.uvars[smtModelFinderAUFKey{q, 1}].root()
			if (first == second) != test.merged || (len(first.avoid) != 0) != test.avoid || (len(first.exceptions) != 0) != test.exception {
				t.Fatal("literal graph constraints", first, second)
			}
			if len(first.set.testElements()) != test.terms {
				t.Fatal("literal instantiation terms", first.set.testElements())
			}
			if test.name == "equal-ground" {
				state.relevancy.enabled = false
				unfiltered := newSMTModelFinderAUFGraph(manager.modelFinder)
				t.Cleanup(func() { astTestMust(t, unfiltered.close()) })
				astTestMust(t, aufTestBuild(t, unfiltered, []ast.ASTHandle{q}))
				set := unfiltered.uvars[smtModelFinderAUFKey{q, 0}].root().set
				if len(set.testElements()) != 3 || set.testElements()[unmarked] != 9 {
					t.Fatal("disabled filtering omitted an uninterpreted equality term", set.testElements())
				}
				state.relevancy.enabled = true
			}
			if test.exception && (first.exceptions[0] != a || first.set.testElements()[b] != 8) {
				t.Fatal("uninterpreted equality omitted context term or exception", first)
			}
			if test.name == "disequal-ground" {
				if generation, found := first.set.testElements()[a]; !found || generation != 0 {
					t.Fatal("disequal ground term generation", first.set.testElements())
				}
			}
		})
	}
}

func TestSMTModelFinderEqualityUsesEnodeRegistrationOrder(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := "internalized"
		if mixed {
			name = "borrowed after scoped enode"
		}
		t.Run(name, func(t *testing.T) {
			m, state, _, qm := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
			sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "registration-order"), nil)
			astTestMust(t, err)
			// With a collision, the order of insertion is observable in the inverse map.
			var buckets [8]ast.ASTHandle
			var older, newer ast.ASTHandle
			for i := uint32(0); newer.IsNull(); i++ {
				term, err := m.MkConst(symbol.Numerical(16600+i), sort)
				astTestMust(t, err)
				h, err := m.GetHash(term)
				astTestMust(t, err)
				at := h % uint32(len(buckets))
				if !buckets[at].IsNull() {
					older, newer = buckets[at], term
				} else {
					buckets[at] = term
				}
			}
			if mixed {
				astTestMust(t, state.PushScope())
			}
			astTestMust(t, state.Internalize(newer, false))
			if mixed {
				node, err := NewSMTEnode(m, older, nil, SMTEnodeOptions{CongruenceEnabled: true})
				astTestMust(t, err)
				astTestMust(t, state.context.RegisterEnode(node, true))
				astTestMust(t, state.context.RegisterEnode(node, true))
			} else {
				astTestMust(t, state.Internalize(older, false))
			}
			for _, term := range []ast.ASTHandle{older, newer} {
				astTestMust(t, state.context.SetRelevant(term, true))
			}
			x, err := m.MkVar(0, sort)
			astTestMust(t, err)
			body, err := m.MkEq(x, older)
			astTestMust(t, err)
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "x")}, Body: body})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(q, true))
			graph := newSMTModelFinderAUFGraph(qm.modelFinder)
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
			expected := ast.NewObjMap(m)
			astTestMust(t, expected.InsertNum(newer, 0))
			astTestMust(t, expected.InsertNum(older, 0))
			got := graph.uvars[smtModelFinderAUFKey{q, 0}].root().set.entries()
			want := expected.Entries()
			if len(got) != 2 || got[0].Key != want[0].Key || got[1].Key != want[1].Key {
				t.Fatalf("equality set order %v, want %v", got, want)
			}
			if mixed {
				astTestMust(t, state.PopScopes(1))
				nodes := state.context.registeredEnodes
				if len(nodes) != state.initEnodes+1 || nodes[len(nodes)-1].owner != older {
					t.Fatal("scope pop lost or duplicated a borrowed enode")
				}
			}
		})
	}
}
