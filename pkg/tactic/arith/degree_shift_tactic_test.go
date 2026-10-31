package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestDegreeShiftTacticApply(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_proofs", true: "with_proofs"}[proofs], func(t *testing.T) {
			base, m := fixDLFixture(t, proofs)
			x := fixDLConst(t, m, false, "tactic-x")
			power := fixDLApp(t, m, 18, x, fixDLNumber(t, m, false, "2"))
			form, err := m.MkEq(power, fixDLNumber(t, m, false, "4"))
			equationMust(t, err)
			fixDLPin(t, m, form)
			goal := fixDLGoal(t, m, proofs, true, false, form)
			shift, err := NewDegreeShiftTactic(m, base.services)
			equationMust(t, err)
			defer func() { equationMust(t, shift.Close()) }()
			var out tactic.TacticResult
			defer func() { equationMust(t, out.Close()) }()
			equationMust(t, shift.Apply(goal, &out))
			size, err := goal.Size()
			equationMust(t, err)
			if size != 2 || out.ModelConverter == nil {
				t.Fatalf("got %d formulas and model converter %v", size, out.ModelConverter != nil)
			}
			for i := uint32(0); i < size && proofs; i++ {
				f, e := goal.Form(i)
				equationMust(t, e)
				proof, e := goal.Proof(i)
				equationMust(t, e)
				fact, e := m.GetProofFact(proof)
				equationMust(t, e)
				if fact != f {
					t.Fatal("goal proof has wrong fact")
				}
			}
		})
	}
}

func TestDegreeShiftFactorySimplifiesMultiplication(t *testing.T) {
	base, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "factory-x")
	product := fixDLApp(t, m, 9, x, x)
	form, err := m.MkEq(product, fixDLNumber(t, m, false, "4"))
	equationMust(t, err)
	fixDLPin(t, m, form)
	goal := fixDLGoal(t, m, false, true, false, form)
	shift, err := MakeDegreeShiftTactic(m, base.services.Context, base.services.Globals, base.services)
	equationMust(t, err)
	equationMust(t, shift.IncRef())
	defer func() { equationMust(t, shift.DecRef()) }()
	var out tactic.TacticResult
	defer func() { equationMust(t, out.Close()) }()
	equationMust(t, shift.Apply(goal, &out))
	if out.ModelConverter == nil {
		t.Fatal("multiplication was not converted to a power and shifted")
	}
	size, err := goal.Size()
	equationMust(t, err)
	if size != 2 {
		t.Fatalf("got %d goal formulas, want shifted equality and bound", size)
	}
}

func TestDegreeShiftCancellationAndReuse(t *testing.T) {
	base, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "cancel-x")
	power := fixDLApp(t, m, 18, x, fixDLNumber(t, m, false, "2"))
	form, err := m.MkEq(power, fixDLNumber(t, m, false, "4"))
	equationMust(t, err)
	fixDLPin(t, m, form)
	goal := fixDLGoal(t, m, false, false, false, form)
	shift, err := NewDegreeShiftTactic(m, base.services)
	equationMust(t, err)
	defer func() { equationMust(t, shift.Close()) }()
	var out tactic.TacticResult
	defer func() { equationMust(t, out.Close()) }()
	m.ResourceLimit().Cancel()
	err = shift.Apply(goal, &out)
	m.ResourceLimit().ResetCancel()
	if err == nil {
		t.Fatal("canceled degree shift succeeded")
	}
	equationMust(t, shift.Apply(goal, &out))
	size, err := goal.Size()
	equationMust(t, err)
	if size != 2 {
		t.Fatalf("after reset got %d formulas, want 2", size)
	}
}

func TestDegreeShiftContextTranslationUsesTargetServices(t *testing.T) {
	source, sourceManager := fixDLFixture(t, false)
	shift, err := NewDegreeShiftTactic(sourceManager, source.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, shift.Close()) })
	target, targetManager := fixDLFixture(t, false)
	translated, matched, err := TranslateContextLeaf(shift, targetManager, target.context, target.globals, target.services)
	equationMust(t, err)
	if !matched {
		t.Fatal("degree-shift was not translated")
	}
	copy := translated.(*DegreeShiftTactic)
	t.Cleanup(func() { equationMust(t, copy.Close()) })
	if copy.services.Context != target.context || copy.services.Cooperation != target.services.Cooperation {
		t.Fatal("translation kept source evaluator services")
	}
	equationMust(t, shift.Close())
	x := fixDLConst(t, targetManager, false, "translated-degree-x")
	power := fixDLApp(t, targetManager, 18, x, fixDLNumber(t, targetManager, false, "2"))
	form, err := targetManager.MkEq(power, fixDLNumber(t, targetManager, false, "4"))
	equationMust(t, err)
	fixDLPin(t, targetManager, form)
	goal := fixDLGoal(t, targetManager, false, true, false, form)
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, copy.Apply(goal, &result))
	if result.ModelConverter == nil {
		t.Fatal("translated degree shift did not create model converter")
	}
}
