package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestMaxBVSharingContextTranslationPreservesPolicy(t *testing.T) {
	source := sharingTacticFixture(t, true)
	p, err := source.context.NewValue()
	sharingMust(t, err)
	key := "max_args"
	sharingMust(t, source.context.SetUIntName(&p, &key, 2))
	sharingMust(t, source.UpdateParams(&p))
	sharingMust(t, source.context.SetUIntName(&p, &key, 9))
	sharingMust(t, source.context.Del(&p))
	tags := diagnostic.NewTraceTags()
	sharingMust(t, source.BindTrace(tags))
	target := sharingTacticFixture(t, true)
	cooperation, err := cooperate.NewCooperation(target.manager.Budget())
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, cooperation.Close()) })
	translated, err := source.TranslateWithServices(target.manager, target.context, cooperation)
	sharingMust(t, err)
	sharingMust(t, translated.IncRef())
	t.Cleanup(func() { sharingMust(t, translated.DecRef()) })
	copy, ok := translated.(*MaxBVSharingTactic)
	if !ok || copy.context != target.context || copy.cooperation != cooperation || copy.trace != tags || copy.current.configuration.reducer.trace != tags || copy.current.configuration.reducer.maxArgs != 2 {
		t.Fatal("translated sharing tactic lost target services or policy")
	}
	sharingMust(t, source.Close())
	goal, _ := sharingTacticGoal(t, copy, 4, true)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, copy.Apply(goal, &result))
	size, err := goal.Size()
	sharingMust(t, err)
	for j := uint32(0); j < size; j++ {
		p, e := goal.Proof(j)
		sharingMust(t, e)
		fact, e := copy.manager.GetProofFact(p)
		sharingMust(t, e)
		formula, e := goal.Form(j)
		sharingMust(t, e)
		if fact != formula {
			t.Fatalf("translated sharing proof %d conclusion", j)
		}
	}
}
