package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTModelFinderAUFSimpleProjectionGenerationAndAvoidance(t *testing.T) {
	for _, avoid := range []bool{false, true} {
		name := "generation"
		if avoid {
			name = "avoid-prior-fallback"
		}
		t.Run(name, func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			proto, err := protomodel.NewProtoModel(m, config)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, proto.Close()) })
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			one, err := m.ArithmeticIntegerNumeralText("1")
			astTestMust(t, err)
			two, err := m.ArithmeticIntegerNumeralText("2")
			astTestMust(t, err)
			three, err := m.ArithmeticIntegerNumeralText("3")
			astTestMust(t, err)
			hundred, err := m.ArithmeticIntegerNumeralText("100")
			astTestMust(t, err)
			sort, err := m.GetSort(zero)
			astTestMust(t, err)
			graph := newSMTModelFinderAUFGraph(solver.kernel.internalizer.quantifierManager.modelFinder)
			graph.model = proto
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			node, err := newSMTModelFinderAUFNode(0, sort)
			astTestMust(t, err)
			graph.nodes = append(graph.nodes, node)
			astTestMust(t, node.makeInstantiationSet(m))
			for _, entry := range []struct {
				term       ast.ASTHandle
				generation uint32
			}{{zero, 0}, {one, 5}, {two, 1}, {three, 9}} {
				_, err := node.insert(entry.term, entry.generation)
				astTestMust(t, err)
			}
			astTestMust(t, node.insertException(zero))
			expected := two
			if avoid {
				prior, err := newSMTModelFinderAUFNode(1, sort)
				astTestMust(t, err)
				astTestMust(t, prior.setElse(two))
				astTestMust(t, node.insertAvoid(prior))
				expected = one
			}
			astTestMust(t, graph.makeSimpleProjection(node))
			if node.elseValue != expected {
				t.Fatal("generation/avoidance fallback", node.elseValue, expected)
			}
			for _, input := range []ast.ASTHandle{zero, one, two, three, hundred} {
				application, err := m.MkApp(node.projection, []ast.ASTHandle{input})
				astTestMust(t, err)
				value, err := graph.eval(application)
				astTestMust(t, err)
				want := input
				if input == hundred {
					want = expected
				}
				if value != want {
					t.Fatal("simple projection fixed point or fallback", input, value, want)
				}
			}
		})
	}
}

func TestSMTModelFinderAUFWitnessConstraintsWaitForRestart(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	finder := state.quantifierManager.modelFinder
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	factory, err := protomodel.NewArithmeticFactory(m)
	astTestMust(t, err)
	astTestMust(t, proto.RegisterFactory(factory))
	graph := newSMTModelFinderAUFGraph(finder)
	graph.model = proto
	t.Cleanup(func() { astTestMust(t, graph.close()) })
	node, err := newSMTModelFinderAUFNode(0, sort)
	astTestMust(t, err)
	graph.nodes = append(graph.nodes, node)
	astTestMust(t, node.makeInstantiationSet(m))
	_, err = node.insert(zero, 0)
	astTestMust(t, err)
	astTestMust(t, node.insertException(zero))
	// A witness shared by multiple nodes can already have an interpretation
	// that collides with this node's exception; that collision needs an axiom.
	witness, err := graph.witness(sort, nil)
	astTestMust(t, err)
	declaration, err := m.GetDecl(witness)
	astTestMust(t, err)
	astTestMust(t, proto.Core().RegisterConstant(declaration, zero))
	clauses := len(state.Clauses())
	astTestMust(t, graph.makeSimpleProjection(node))
	if node.elseValue.IsNull() || node.elseValue == zero || len(finder.pendingConstraints) != 1 || len(state.Clauses()) != clauses {
		t.Fatal("witness fallback or deferred constraint", node.elseValue, len(finder.pendingConstraints), len(state.Clauses()))
	}
	constraint, err := m.RefGet(finder.pendingConstraints[0])
	astTestMust(t, err)
	constraintRef, err := m.NewRef(constraint)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&constraintRef)) })
	astTestMust(t, state.RestartTheories())
	if len(finder.pendingConstraints) != 0 {
		t.Fatal("restart did not consume witness axiom")
	}
	literal, err := state.Literal(constraint)
	astTestMust(t, err)
	if !state.context.relevant.get(state.boolExpr[literal.Var()]) {
		t.Fatal("restart did not mark witness literal relevant")
	}
	assignment, err := state.Assignment(literal)
	astTestMust(t, err)
	if assignment != lbool.True {
		t.Fatal("restart did not assert witness disequality", assignment)
	}
}

func TestSMTModelFinderAUFArithmeticWitnessIsFreshConstant(t *testing.T) {
	for _, integer := range []bool{true, false} {
		m, config := smtModelGeneratorFixture(t)
		solver, err := NewSMTSolver(m, nil, "AUFLIRA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, solver.Close()) })
		proto, err := protomodel.NewProtoModel(m, config)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, proto.Close()) })
		factory, err := protomodel.NewArithmeticFactory(m)
		astTestMust(t, err)
		astTestMust(t, proto.RegisterFactory(factory))
		zero, err := m.ArithmeticIntegerNumeralText("0")
		astTestMust(t, err)
		if !integer {
			zero, err = m.ArithmeticRealNumeralText("0")
			astTestMust(t, err)
		}
		sort, err := m.GetSort(zero)
		astTestMust(t, err)
		_, err = proto.SomeValue(sort)
		astTestMust(t, err)
		finder := solver.kernel.internalizer.quantifierManager.modelFinder
		graph := newSMTModelFinderAUFGraph(finder)
		graph.model = proto
		t.Cleanup(func() { astTestMust(t, graph.close()) })
		node, err := newSMTModelFinderAUFNode(0, sort)
		astTestMust(t, err)
		graph.nodes = append(graph.nodes, node)
		astTestMust(t, node.makeInstantiationSet(m))
		_, err = node.insert(zero, 0)
		astTestMust(t, err)
		astTestMust(t, node.insertException(zero))
		astTestMust(t, graph.makeSimpleProjection(node))
		// Source set_projection_else: n->insert(k, 0); n->set_else(k), with
		// k's interpretation a fresh value distinct from the exception.
		if _, _, numeral, err := m.ArithmeticNumeralText(node.elseValue); err != nil || numeral {
			t.Fatal("witness else is not the fresh constant k", numeral, err)
		}
		value, err := graph.eval(node.elseValue)
		astTestMust(t, err)
		text, isInt, matched, err := m.ArithmeticNumeralText(value)
		astTestMust(t, err)
		if !matched || isInt != integer || text != "1" || len(finder.pendingConstraints) != 0 {
			t.Fatal("witness interpretation", text, isInt, matched, len(finder.pendingConstraints))
		}
		if _, found := node.set.testElements()[node.elseValue]; !found {
			t.Fatal("witness constant was not added to the instantiation set")
		}
	}
}
