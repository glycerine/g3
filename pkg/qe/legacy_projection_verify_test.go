package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyProjectionModelSanityRejectsFalseAndUnknown(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	truth := f.own(m.MkTrue())
	falsity := f.own(m.MkFalse())
	unknown := f.constant("sanity-unknown", f.boolean)
	holds, err := p.projectionModelHolds(md.Core(), truth)
	fmMust(t, err)
	if !holds {
		t.Fatal("true formula rejected")
	}
	for _, formula := range []struct {
		name       string
		falseValue bool
	}{{"false", true}, {"unknown", false}} {
		h := unknown
		if formula.falseValue {
			h = falsity
		}
		if holds, err := p.projectionModelHolds(md.Core(), h); err != nil || holds {
			t.Fatalf("%s projection sanity = %v", formula.name, err)
		}
	}
	decl, err := m.GetDecl(unknown)
	fmMust(t, err)
	present, err := md.Core().HasConstantInterpretation(decl)
	fmMust(t, err)
	if present {
		t.Fatal("sanity check completed the model")
	}
}
