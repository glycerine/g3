package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/params"
)

func TestArithmeticContextTranslationCopiesParametersAndServices(t *testing.T) {
	source, sourceManager := fixDLFixture(t, false)
	p, err := source.context.NewValue()
	equationMust(t, err)
	key := "recover_01_max_bits"
	equationMust(t, source.context.SetUIntName(&p, &key, 3))
	equationMust(t, source.UpdateParams(&p))
	recoverTactic, err := NewRecover01Tactic(sourceManager, source.context, source.globals, &p, source.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, recoverTactic.Close()) })
	equationMust(t, source.context.Del(&p))
	target, targetManager := fixDLFixture(t, false)
	for _, original := range []tactic.Tactic{source, recoverTactic} {
		translated, matched, err := TranslateContextLeaf(original, targetManager, target.context, target.globals, target.services)
		equationMust(t, err)
		if !matched {
			t.Fatal("arithmetic tactic was not translated")
		}
		equationMust(t, translated.IncRef())
		t.Cleanup(func() { equationMust(t, translated.DecRef()) })
		var copied *params.ParamsRef
		switch v := translated.(type) {
		case *FixDLVarTactic:
			if v.context != target.context || v.services.Cooperation != target.services.Cooperation {
				t.Fatal("fix-dl-var retained source services")
			}
			copied = &v.params
		case *Recover01Tactic:
			if v.context != target.context || v.services.Cooperation != target.services.Cooperation || v.maxBits != 3 {
				t.Fatal("recover-01 lost target services or policy")
			}
			copied = &v.params
		default:
			t.Fatalf("unexpected translated tactic %T", translated)
		}
		value, err := target.context.GetUIntName(copied, &key, 0)
		equationMust(t, err)
		if value != 3 {
			t.Fatal("translated parameter", value)
		}
	}
}

func TestBoundTacticContextTranslationPreservesSourcePolicies(t *testing.T) {
	source, manager := fixDLFixture(t, false)
	normalizeParams, err := source.context.NewValue()
	equationMust(t, err)
	normKey := "norm_int_only"
	equationMust(t, source.context.SetBoolName(&normalizeParams, &normKey, false))
	normalize, err := NewNormalizeBoundsTactic(manager, source.context, source.globals, &normalizeParams, source.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, normalize.Close()) })
	equationMust(t, source.context.Del(&normalizeParams))
	liveUpdate, err := source.context.NewValue()
	equationMust(t, err)
	equationMust(t, source.context.SetBoolName(&liveUpdate, &normKey, true))
	equationMust(t, normalize.UpdateParams(&liveUpdate))
	equationMust(t, source.context.Del(&liveUpdate))
	if !normalize.integerOnly {
		t.Fatal("live normalize-bounds update was not applied")
	}
	diffParams, err := source.context.NewValue()
	equationMust(t, err)
	diffKey := "diff_neq_max_k"
	equationMust(t, source.context.SetUIntName(&diffParams, &diffKey, 3))
	diff, err := NewDiffNeqTactic(manager, source.context, source.globals, &diffParams)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, diff.Close()) })
	equationMust(t, source.context.Del(&diffParams))
	target, targetManager := fixDLFixture(t, false)
	for _, original := range []tactic.Tactic{normalize, diff} {
		translated, matched, err := TranslateContextLeaf(original, targetManager, target.context, target.globals, target.services)
		equationMust(t, err)
		if !matched {
			t.Fatal("bound tactic was not translated")
		}
		equationMust(t, translated.IncRef())
		t.Cleanup(func() { equationMust(t, translated.DecRef()) })
		switch copy := translated.(type) {
		case *NormalizeBoundsTactic:
			if copy.context != target.context || copy.services.Cooperation != target.services.Cooperation || copy.integerOnly {
				t.Fatal("normalize-bounds did not restore source constructor policy")
			}
		case *DiffNeqTactic:
			if copy.context != target.context || copy.current.maxK != 3 || copy.current.maxNegK != -3 {
				t.Fatal("diff-neq policy or parameter owner was lost")
			}
		default:
			t.Fatalf("unexpected tactic %T", translated)
		}
	}
}
