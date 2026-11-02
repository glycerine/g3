package pdr

import "testing"

func TestPDRContextPropertiesBeforeRuleInitialization(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
	must := func(err error) { t.Helper(); closureMust(t, err) }
	context, err := NewContext(m, nil, 2, ReachableNoCache, owner.contexts.config)
	must(err)
	defer func() {
		must(context.Close())
	}()
	declaration, err := transformer.Head()
	must(err)
	levels, err := context.NumLevels(declaration)
	must(err)
	if levels != 0 {
		t.Fatal("unknown predicate has levels", levels)
	}
	delta, err := context.CoverDelta(-1, declaration)
	must(err)
	h, err := m.RefGet(delta)
	must(err)
	if !m.IsTrue(h) {
		t.Fatal("unknown predicate cover is not true")
	}
	must(m.DelRef(&delta))
	falsity, err := m.MkFalse()
	must(err)
	must(context.AddCover(2, declaration, falsity))
	levels, err = context.NumLevels(declaration)
	must(err)
	if levels != 3 {
		t.Fatal("pre-initialization cover frame was lost", levels)
	}
	delta, err = context.CoverDelta(2, declaration)
	must(err)
	defer func() { must(m.DelRef(&delta)) }()
	h, err = m.RefGet(delta)
	must(err)
	if !m.IsFalse(h) {
		t.Fatal("pre-initialization cover changed")
	}
}
