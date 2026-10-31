package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestDT2BVContextTranslationPreservesParametersAndModel(t *testing.T) {
	source := dt2bvFixture(t, true)
	p, err := source.context.NewValue()
	sharingMust(t, err)
	key := "random_seed"
	sharingMust(t, source.context.SetUIntName(&p, &key, 17))
	original, err := NewDT2BVTactic(source.manager, source.context, &p, source.services)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, original.Close()) })
	sharingMust(t, source.context.SetUIntName(&p, &key, 29))
	sharingMust(t, original.UpdateParams(&p))
	sharingMust(t, source.context.Del(&p))
	target := dt2bvFixture(t, true)
	translated, err := original.TranslateWithServices(target.manager, target.context, target.services)
	sharingMust(t, err)
	sharingMust(t, translated.IncRef())
	t.Cleanup(func() { sharingMust(t, translated.DecRef()) })
	copy, ok := translated.(*DT2BVTactic)
	if !ok || copy.context != target.context || copy.services.Cooperation != target.services.Cooperation || copy.utility == original.utility {
		t.Fatal("translated datatype tactic lost target ownership")
	}
	seed, err := target.context.GetUIntName(&copy.parameters, &key, 0)
	sharingMust(t, err)
	if seed != 17 {
		t.Fatalf("source no-op update changed translated constructor policy: %d", seed)
	}
	sharingMust(t, original.Close())
	sort, constructors := dt2bvEnum(t, copy)
	goal, decl := dt2bvGoal(t, copy, sort, constructors, "translated-datatype", true)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, copy.Apply(goal, &result))
	if result.ModelConverter == nil {
		t.Fatal("missing translated datatype converter")
	}
	formula, err := goal.Form(0)
	sharingMust(t, err)
	bv, err := copy.manager.GetArg(formula, 0)
	sharingMust(t, err)
	bvDecl, err := copy.manager.GetDecl(bv)
	sharingMust(t, err)
	md, err := model.NewModel(copy.manager)
	sharingMust(t, err)
	var output tactic.ModelRef
	sharingMust(t, output.Set(md.Core()))
	t.Cleanup(func() { sharingMust(t, output.Close()) })
	value, err := copy.manager.BitVectorNumeralUnsigned(1, 2)
	sharingMust(t, err)
	sharingMust(t, md.Core().RegisterConstant(bvDecl, value))
	sharingMust(t, result.ModelConverter.ApplyModel(&output, 0))
	reconstructed, err := output.Get()
	sharingMust(t, err)
	value, err = reconstructed.ConstantInterpretation(decl)
	sharingMust(t, err)
	valueDecl, err := copy.manager.GetDecl(value)
	sharingMust(t, err)
	if valueDecl != constructors[1] {
		t.Fatal("wrong reconstructed datatype value")
	}
}
