package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderAUFProductionExtensionPreservesGroundFunction(t *testing.T) {
	for _, mode := range []string{"simple", "monotone", "already-complete"} {
		t.Run(mode, func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			state := solver.kernel.internalizer
			finder := state.quantifierManager.modelFinder
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			five, err := m.ArithmeticIntegerNumeralText("5")
			astTestMust(t, err)
			ten, err := m.ArithmeticIntegerNumeralText("10")
			astTestMust(t, err)
			twenty, err := m.ArithmeticIntegerNumeralText("20")
			astTestMust(t, err)
			integer, err := m.GetSort(zero)
			astTestMust(t, err)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			falsehood, err := m.MkFalse()
			astTestMust(t, err)
			function := smtQuickPredicate(t, m, "auf-extension-p", integer, boolean)
			x, err := m.MkVar(0, integer)
			astTestMust(t, err)
			body, err := m.MkApp(function, []ast.ASTHandle{x})
			astTestMust(t, err)
			if mode == "monotone" {
				comparison, err := m.MkArithmeticLE(x, ten)
				astTestMust(t, err)
				negative, err := m.MkNot(comparison)
				astTestMust(t, err)
				body, err = m.MkAnd(body, negative)
				astTestMust(t, err)
			}
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "auf-extension-x")}, Body: body,
			})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(q, true))
			astTestMust(t, state.context.SetRelevant(q, true))
			astTestMust(t, state.context.SetAssignment(q, lbool.True))
			for _, term := range []ast.ASTHandle{zero, ten} {
				node, err := NewSMTEnode(m, term, state.context.enodes.get, SMTEnodeOptions{})
				astTestMust(t, err)
				astTestMust(t, state.context.RegisterEnode(node, true))
				smtQuickApplicationNode(t, m, state.context, function, node, true)
			}
			proto, err := protomodel.NewProtoModel(m, config)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, proto.Close()) })
			original, err := model.NewFunctionInterpretation(m, 1)
			astTestMust(t, err)
			astTestMust(t, original.InsertEntry([]ast.ASTHandle{zero}, falsehood))
			astTestMust(t, original.InsertEntry([]ast.ASTHandle{ten}, truth))
			if mode == "already-complete" {
				astTestMust(t, original.SetElse(falsehood))
			}
			astTestMust(t, proto.Core().RegisterFunction(function, original))
			astTestMust(t, finder.FixModel(proto))
			extended, err := proto.Core().FunctionInterpretation(function)
			astTestMust(t, err)
			if mode == "already-complete" {
				if extended != original {
					t.Fatal("complete interpretation was replaced")
				}
			} else {
				fallback, err := extended.Else()
				astTestMust(t, err)
				auxiliary, err := m.GetDecl(fallback)
				astTestMust(t, err)
				if auxiliary == function {
					t.Fatal("function extension recursed into itself")
				}
				preserved, err := proto.Core().FunctionInterpretation(auxiliary)
				astTestMust(t, err)
				if preserved == nil || preserved.NumEntries() != 2 {
					t.Fatal("old ground graph was not preserved under auxiliary function")
				}
				projectedArgument, err := m.GetArg(fallback, 0)
				astTestMust(t, err)
				if kind, err := m.GetKind(projectedArgument); err != nil || kind != ast.ASTApp {
					t.Fatal("extended function omitted argument projection", kind, err)
				}
			}
			graph := newSMTModelFinderAUFGraph(finder)
			graph.model = proto
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			for _, entry := range []struct{ input, want ast.ASTHandle }{{zero, falsehood}, {ten, truth}} {
				application, err := m.MkApp(function, []ast.ASTHandle{entry.input})
				astTestMust(t, err)
				value, err := graph.eval(application)
				astTestMust(t, err)
				if value != entry.want {
					t.Fatal("projection changed an existing ground value", entry.input, value, entry.want)
				}
			}
			if mode == "monotone" {
				for _, entry := range []struct{ input, want ast.ASTHandle }{{five, falsehood}, {twenty, truth}} {
					application, err := m.MkApp(function, []ast.ASTHandle{entry.input})
					astTestMust(t, err)
					value, err := graph.eval(application)
					astTestMust(t, err)
					if value != entry.want {
						t.Fatal("production monotone extension chose wrong interval", entry.input, value, entry.want)
					}
				}
			}
		})
	}
}
