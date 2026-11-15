package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderCancellationAndResourceCheckpoint(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	finder := solver.kernel.internalizer.quantifierManager.modelFinder
	q, _, _ := smtManagedQuantifier(t, m, "finder-cancel")
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	m.Limit.Cancel()
	// register_quantifier has no checkpoint of its own, and the analyzer
	// of the clause body (p x) never reaches one (smtcore_05-F8): the
	// first insert_qinfo compares against nothing and no formula is
	// pushed on m_ftodo.
	astTestMust(t, finder.RegisterQuantifier(q))
	// model_finder::fix_model returns before any checkpoint when no
	// quantifier is relevant (math_misc_01-F1 follow-up).
	if err = finder.FixModel(proto); err != nil {
		t.Fatal("model fixing without quantifiers checked cancellation", err)
	}
	m.Limit.ResetCancel()
	scope, err := control.NewLimitScope(&m.Limit, 1)
	astTestMust(t, err)
	astTestMust(t, finder.checkpoint())
	err = finder.checkpoint()
	if err == nil || !strings.Contains(err.Error(), "resource limit") {
		t.Fatal("model finder ignored resource exhaustion", err)
	}
	scope.Close()
	astTestMust(t, finder.checkpoint())
	astTestMust(t, finder.FixModel(proto))
	if _, err = m.GetKind(finder.flatQuantifier(q)); err != nil {
		t.Fatal("cancellation recovery lost registered quantifier", err)
	}
}

// smtcore_05-F8: model_finder checkpoints sit in insert_qinfo (one per
// comparison), process_formulas_on_stack and the second population pass,
// not in register_quantifier or fix_model's entry.
func TestSMTModelFinderCheckpointPlacement(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	finder := solver.kernel.internalizer.quantifierManager.modelFinder
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	before := m.Limit.Count()
	astTestMust(t, finder.FixModel(proto))
	if delta := m.Limit.Count() - before; delta != 0 {
		t.Fatal("fix_model without relevant quantifiers counted steps", delta)
	}
	single, predicate, sort := smtManagedQuantifier(t, m, "finder-placement")
	before = m.Limit.Count()
	astTestMust(t, finder.RegisterQuantifier(single))
	first := m.Limit.Count() - before
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	var literals []ast.ASTHandle
	for _, name := range []string{"", "-q", "-r"} {
		decl := predicate
		if name != "" {
			decl = smtQuickPredicate(t, m, "finder-placement"+name, sort, boolean)
		}
		literal, err := m.MkApp(decl, []ast.ASTHandle{variable})
		astTestMust(t, err)
		literals = append(literals, literal)
	}
	body, err := m.MkOr3(literals[0], literals[1], literals[2])
	astTestMust(t, err)
	clause, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "finder-placement-y")}, Body: body,
	})
	astTestMust(t, err)
	astTestPin(t, m, clause)
	before = m.Limit.Count()
	astTestMust(t, finder.RegisterQuantifier(clause))
	if second := m.Limit.Count() - before; second != first+3 {
		t.Fatal("insert_qinfo checkpoints: zero, one and two comparisons", first, second)
	}
}
