package spacer

// Source: src/muz/spacer/spacer_context.cpp context::solve simplifies every
// unreachable result and then runs the guarded original-rule validation.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func installEmptyFixtureQuery(t *testing.T, fixture *contextAnswerFixture) {
	t.Helper()
	truth, err := fixture.context.manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	if err = fixture.context.AddCover(0, fixture.other, truth); err != nil {
		t.Fatal(err)
	}
	if err = fixture.context.SetQuery(fixture.other); err != nil {
		t.Fatal(err)
	}
	// Updating to the live rules removes the cover-only query transformer
	// while preserving the selected query declaration, as source update_rules
	// does. This exercises absence, not a live tail-only predicate.
	if err = fixture.context.UpdateRules(fixture.rules); err != nil {
		t.Fatal(err)
	}
	if fixture.context.transformers[fixture.other] != nil {
		t.Fatal("query transformer survived rule update")
	}
}

func TestContextEarlyUnreachableSimplifiesModelFrames(t *testing.T) {
	fixture := newContextAnswerFixture(t, true)
	context, manager := fixture.context, fixture.context.manager
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	var refs []ast.ASTRef
	defer func() {
		if err := releaseNormalizeRefs(manager, refs); err != nil {
			t.Fatal(err)
		}
	}()
	own := func(handle ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		reference, err := manager.NewRef(handle)
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, reference)
		return handle
	}
	a := own(manager.MkFreshConst("early-unreachable-a", boolean))
	b := own(manager.MkFreshConst("early-unreachable-b", boolean))
	weak := own(manager.MkOr(a, b))
	transformer := context.transformers[fixture.query]
	for _, formula := range []ast.ASTHandle{weak, a} {
		if _, err = transformer.AddLemma(formula, InfinityLevel); err != nil {
			t.Fatal(err)
		}
	}
	installEmptyFixtureQuery(t, fixture)
	transformer = context.transformers[fixture.query]

	status, err := context.Solve(0, 1)
	if err != nil || status != lbool.False {
		t.Fatal("empty query solve", status, err)
	}
	lemmas, err := transformer.frames.FrameLemmas(InfinityLevel)
	if err != nil || len(lemmas) != 1 || lemmas[0] != a {
		t.Fatal("early result did not simplify subsumed frames", lemmas, err)
	}
	inductive, err := context.Model(fixture.other)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := inductive.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	interpretation, err := inductive.Core().ConstantInterpretation(fixture.query)
	if err != nil || interpretation != a {
		t.Fatal("simplified model interpretation", interpretation, a, err)
	}
}

func TestContextEarlyUnreachablePreservesValidationGuard(t *testing.T) {
	for _, validate := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[validate], func(t *testing.T) {
			fixture := newContextAnswerFixture(t, false)
			context, manager := fixture.context, fixture.context.manager
			falsehood, err := manager.MkFalse()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = context.transformers[fixture.query].AddLemma(falsehood, InfinityLevel); err != nil {
				t.Fatal(err)
			}
			installEmptyFixtureQuery(t, fixture)
			context.validateResult = validate
			status, err := context.Solve(0, 1)
			if status != lbool.False {
				t.Fatal("empty query status", status, err)
			}
			if validate && err == nil {
				t.Fatal("guarded validation accepted a model that violates the original fact")
			}
			if !validate && err != nil {
				t.Fatal("disabled validation ran", err)
			}
		})
	}
}
