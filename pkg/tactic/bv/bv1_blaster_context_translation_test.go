package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestBV1BlasterContextTranslationCopiesPolicyAndServices(t *testing.T) {
	source := sharingTacticFixture(t, true)
	sourceGlobals := uparams.BorrowProcessGlobalParamsService()
	p, err := source.context.NewValue()
	sharingMust(t, err)
	steps := "max_steps"
	models := "produce_models"
	sharingMust(t, source.context.SetUIntName(&p, &steps, 1234))
	sharingMust(t, source.context.SetBoolName(&p, &models, true))
	original, err := NewBV1BlasterTactic(source.manager, source.context, sourceGlobals, &p, nil)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, original.Close()) })
	sharingMust(t, source.context.Del(&p))
	tags := diagnostic.NewTraceTags()
	sharingMust(t, original.BindTrace(tags))
	target := sharingTacticFixture(t, true)
	targetGlobals := uparams.BorrowProcessGlobalParamsService()
	cooperation, err := cooperate.NewCooperation(target.manager.Budget())
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, cooperation.Close()) })
	translated, err := original.TranslateWithServices(target.manager, target.context, targetGlobals, cooperation)
	sharingMust(t, err)
	sharingMust(t, translated.IncRef())
	t.Cleanup(func() { sharingMust(t, translated.DecRef()) })
	copy, ok := translated.(*BV1BlasterTactic)
	if !ok || copy.context != target.context || copy.cooperation != cooperation || copy.trace != tags || copy.current.maxSteps != 1234 || !copy.current.produceModels {
		t.Fatalf("translated blaster lost target services or policy: %T", translated)
	}
	sharingMust(t, original.Close())
	goal := bv1NativeGoal(t, copy)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, copy.Apply(goal, &result))
	if result.ModelConverter == nil {
		t.Fatal("translated blaster did not preserve model conversion")
	}
}
