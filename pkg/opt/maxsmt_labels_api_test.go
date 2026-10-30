package opt

// Source: src/opt/maxsmt.h get_model label copy semantics.

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestMaxSMTResultLabelsReturnsDefensiveCopy(t *testing.T) {
	fixture := newWeightedSoftFixture(t)
	manager := fixture.manager
	md, err := model.NewModel(manager)
	if err != nil {
		t.Fatal(err)
	}
	defer md.Close()
	if err := md.Core().IncRef(); err != nil {
		t.Fatal(err)
	}
	result := &ArithmeticResult{Model: md.Core()}
	defer result.Close()
	first, err := manager.Symbols.Intern("first-label")
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Symbols.Intern("second-label")
	if err != nil {
		t.Fatal(err)
	}
	result.Labels = []symbol.Symbol{first, second}
	solver := &ArithmeticSolver{}
	labels, err := solver.ResultLabels(result)
	if err != nil || len(labels) != 2 {
		t.Fatal(labels, err)
	}
	labels[0] = second
	again, err := solver.ResultLabels(result)
	if err != nil || again[0] != first {
		t.Fatal("labels alias", again, err)
	}
}
