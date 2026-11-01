package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyProjectionRejectedResultReleasesFormula(t *testing.T) {
	if !ast.ASTSourceDebug {
		t.Skip("final source model sanity check requires g3debug")
	}
	f := newFMFixture(t)
	m := f.r.manager
	unknown := f.constant("rejected-projection", f.boolean)
	truth := f.own(m.MkTrue())
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for attempt := 0; attempt < 3; attempt++ {
		before := m.Stats().LiveReferences
		p, err := NewLegacyProjection(m, f.r.config)
		fmMust(t, err)
		result, err := p.Project(md.Core(), nil, unknown, false, true)
		if result != nil || !errors.Is(err, ast.ErrASTDomain) {
			t.Fatalf("rejected result = %v, %v", result, err)
		}
		result, err = p.Project(md.Core(), nil, truth, false, true)
		fmMust(t, err)
		handle, err := result.Formula()
		fmMust(t, err)
		if handle != truth {
			t.Fatal("retry changed true formula")
		}
		fmMust(t, result.Close())
		fmMust(t, p.Close())
		if got := m.Stats().LiveReferences; got != before {
			t.Fatalf("attempt %d: live references = %d, want %d", attempt, got, before)
		}
	}
}

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
