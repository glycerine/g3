package smtcore

import (
	"strings"
	"testing"

	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/control"
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
	err = finder.RegisterQuantifier(q)
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatal("registration ignored cancellation", err)
	}
	if len(finder.quantifiers) != 0 || len(finder.flat) != 0 || len(finder.sets) != 0 {
		t.Fatal("canceled registration retained analysis state")
	}
	// model_finder::fix_model returns before any checkpoint when no
	// quantifier is relevant (math_misc_01-F1 follow-up).
	if err = finder.FixModel(proto); err != nil {
		t.Fatal("model fixing without quantifiers checked cancellation", err)
	}
	m.Limit.ResetCancel()
	astTestMust(t, finder.RegisterQuantifier(q))
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
