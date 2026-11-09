package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

func TestSMTModelFinderAUFPreparationSharesSafeTermsAndFreezesUniverse(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "auf-prepare-sort"), nil)
	astTestMust(t, err)
	graph := newSMTModelFinderAUFGraph(solver.kernel.internalizer.quantifierManager.modelFinder)
	graph.model = proto
	t.Cleanup(func() { astTestMust(t, graph.close()) })
	for i := uint32(0); i < 2; i++ {
		node, err := newSMTModelFinderAUFNode(i, sort)
		astTestMust(t, err)
		astTestMust(t, node.makeInstantiationSet(m))
		graph.nodes = append(graph.nodes, node)
	}
	astTestMust(t, graph.prepareInstantiationSets())
	var term ast.ASTHandle
	for _, node := range graph.nodes {
		if len(node.set.testElements()) != 1 {
			t.Fatal("empty uninterpreted set was not populated", node.set.testElements())
		}
		for candidate := range node.set.testElements() {
			if !term.IsNull() && candidate != term {
				t.Fatal("same-sort sets did not share a fresh context term")
			}
			term = candidate
			private, err := m.IsModelValue(term)
			astTestMust(t, err)
			if private {
				t.Fatal("private universe value leaked into instantiation terms")
			}
		}
	}
	finite, err := proto.IsFinite(sort)
	astTestMust(t, err)
	if !finite {
		t.Fatal("uninterpreted node sort was not frozen")
	}
	value, err := graph.eval(term)
	astTestMust(t, err)
	private, err := m.IsModelValue(value)
	astTestMust(t, err)
	if !private || value == term {
		t.Fatal("safe context term did not evaluate inside the frozen universe", term, value)
	}
}

func TestSMTModelFinderAUFMonotoneExceptionNeighbors(t *testing.T) {
	for _, domain := range []string{"integer", "real", "bit-vector"} {
		t.Run(domain, func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			proto, err := protomodel.NewProtoModel(m, config)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, proto.Close()) })
			var zero, before, after ast.ASTHandle
			if domain == "bit-vector" {
				_, err = m.RegisterBitVectorPlugin()
				astTestMust(t, err)
				zero, err = m.BitVectorNumeralUnsigned(0, 4)
				astTestMust(t, err)
				before, err = m.BitVectorNumeralUnsigned(15, 4)
				astTestMust(t, err)
				after, err = m.BitVectorNumeralUnsigned(1, 4)
				astTestMust(t, err)
			} else if domain == "integer" {
				zero, err = m.ArithmeticIntegerNumeralText("0")
				astTestMust(t, err)
				before, err = m.ArithmeticIntegerNumeralText("-1")
				astTestMust(t, err)
				after, err = m.ArithmeticIntegerNumeralText("1")
				astTestMust(t, err)
			} else {
				zero, err = m.ArithmeticRealNumeralText("0")
				astTestMust(t, err)
			}
			sort, err := m.GetSort(zero)
			astTestMust(t, err)
			graph := newSMTModelFinderAUFGraph(solver.kernel.internalizer.quantifierManager.modelFinder)
			graph.model = proto
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			node, err := newSMTModelFinderAUFNode(0, sort)
			astTestMust(t, err)
			graph.nodes = append(graph.nodes, node)
			astTestMust(t, node.makeInstantiationSet(m))
			_, err = node.insert(zero, 7)
			astTestMust(t, err)
			astTestMust(t, node.setMono())
			astTestMust(t, node.insertException(zero))
			astTestMust(t, graph.addMonotoneExceptions())
			if domain == "real" {
				if len(node.set.testElements()) != 1 {
					t.Fatal("real exception received discrete neighbors", node.set.testElements())
				}
			} else {
				if len(node.set.testElements()) != 3 || node.set.testElements()[before] != 0 || node.set.testElements()[after] != 0 || node.set.testElements()[zero] != 7 {
					t.Fatal("monotone neighbors or generations", node.set.testElements())
				}
			}
		})
	}
}
