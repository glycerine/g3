package bv

import (
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func TestBVSliceSourceTranslateParamsAndFactory(t *testing.T) {
	f := newBVSliceFixture(t)
	p := f.base.context.EmptyValue()
	defer func() { sharingMust(t, f.base.context.Del(&p)) }()
	sharingMust(t, f.tactic.UpdateParams(&p))
	translated, e := f.tactic.Translate(f.m)
	sharingMust(t, e)
	sharingMust(t, translated.(*BVSliceTactic).Close())
	made, e := MakeBVSliceTactic(f.m, f.base.context, f.base.globals, &p, f.base.services)
	sharingMust(t, e)
	sharingMust(t, made.IncRef())
	sharingMust(t, made.DecRef())
	var _ tactic.Tactic = translated
}
