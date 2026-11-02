package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp add_cover/check_reset and
// spacer_context.cpp context::add_cover/get_num_levels.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerCoverBeforeFirstQuerySurvivesRuleInstallation(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(e.SelectEngine("spacer"))
	for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
		must(e.globals.SourceSetName(key, "false"))
	}
	u := f.applications["u"]
	recursive, err := m.MkImplies(u, u)
	must(err)
	must(e.AddRule(recursive, "covered-recursive-predicate"))
	addSpacerLifecycleIntegerRules(t, e, u)
	parameters, err := e.params.NewValue()
	must(err)
	defer func() { must(e.params.Del(&parameters)) }()
	slicing := "xform.slice"
	must(e.params.SetBoolName(&parameters, &slicing, false))
	maximum := "spacer.max_level"
	must(e.params.SetUIntName(&parameters, &maximum, 0))
	must(e.SetQueryParameters(&parameters))
	query := f.declarations["u"]
	levels, err := e.NumLevels(query)
	must(err)
	if levels != 0 {
		t.Fatal("unknown predicate should initially have zero frames", levels)
	}
	falsehood, err := m.MkFalse()
	must(err)
	must(e.AddCover(2, query, falsehood))
	levels, err = e.NumLevels(query)
	must(err)
	if levels != 3 {
		t.Fatal("cover before first query did not create the requested frame", levels)
	}
	context := e.spacer
	assertCover := func(expectedFalse bool) {
		t.Helper()
		// Result converters can reconstruct a false relation even after a
		// frame is lost. Check the stored frame as well as the public cover.
		raw, err := e.spacer.CoverDelta(2, e.spacerPredicate(query))
		must(err)
		defer func() { must(m.DelRef(&raw)) }()
		rawHandle, err := m.RefGet(raw)
		must(err)
		if (expectedFalse && !m.IsFalse(rawHandle)) || (!expectedFalse && !m.IsTrue(rawHandle)) {
			t.Fatal("stored cover frame differs from source rule-reset policy", expectedFalse, rawHandle)
		}
		cover, err := e.CoverDelta(2, query)
		must(err)
		defer func() { must(m.DelRef(&cover)) }()
		h, err := m.RefGet(cover)
		must(err)
		if (expectedFalse && !m.IsFalse(h)) || (!expectedFalse && !m.IsTrue(h)) {
			t.Fatal("cover frame differs from source rule-reset policy", expectedFalse, h)
		}
	}
	assertCover(true)
	status, err := e.QueryPredicate(query)
	must(err)
	if status == lbool.True {
		t.Fatal("recursive-only predicate cannot be reachable", status)
	}
	if e.spacer != context {
		t.Fatal("installing the first rules replaced the context containing its cover")
	}
	// No search level can create/promote a replacement lemma at max_level=0,
	// so this frame must have survived initial rule installation itself.
	assertCover(true)
	must(e.AddRule(u, "cover-invalidating-fact"))
	_, err = e.QueryPredicate(query)
	must(err)
	assertCover(false)
}
