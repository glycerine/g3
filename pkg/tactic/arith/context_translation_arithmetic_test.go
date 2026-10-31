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
