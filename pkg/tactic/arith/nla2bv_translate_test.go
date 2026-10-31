package arith

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func nla2BVIndependentTarget(t *testing.T) (*ast.ASTManager, *FixDLVarTactic) {
	t.Helper()
	base, m := fixDLFixture(t, false)
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget(), Params: base.context, Globals: base.globals})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, target.Close()) })
	return target, base
}
func TestNLA2BVTranslationIndependentManagersAndPolicies(t *testing.T) {
	source := nla2BVFixture(t, false)
	target, base := nla2BVIndependentTarget(t)
	policies := []struct {
		name  string
		value uint32
	}{{"nla2bv_root", 3}, {"nla2bv_divisor", 5}, {"nla2bv_max_bv_size", 17}, {"nla2bv_bv_size", 6}, {"max_bv_size", 23}}
	for _, field := range policies {
		name := field.name
		equationMust(t, source.context.SetUIntName(&source.params, &name, field.value))
	}
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	equationMust(t, source.BindTrace(tags))
	translated, err := source.Translate(target)
	equationMust(t, err)
	equationMust(t, translated.IncRef())
	t.Cleanup(func() { equationMust(t, translated.DecRef()) })
	got := translated.(*NLA2BVTactic)
	if got.manager != target || got.context != base.context || got.globals != base.globals || got.services.Memory != target.NumeralService().RuntimeOwner() || got.trace != tags {
		t.Fatal("translation uses foreign services or lost explicit trace")
	}
	for _, field := range policies {
		name := field.name
		value, err := got.context.GetUIntName(&got.params, &name, 0)
		equationMust(t, err)
		if value != field.value {
			t.Fatal("lost source policy", name, value, field.value)
		}
	}
	// The translated snapshot survives later source updates and tactic close.
	name := "nla2bv_bv_size"
	equationMust(t, source.context.SetUIntName(&source.params, &name, 2))
	equationMust(t, source.Close())
	invocation, err := got.newInvocation()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, invocation.close()) })
	root, divisor, _, err := invocation.real.Defaults()
	equationMust(t, err)
	maxReal, err := invocation.real.MaxNumBits()
	equationMust(t, err)
	maxInt, err := invocation.integer.MaxNumBits()
	equationMust(t, err)
	if root.RatString() != "3" || divisor.RatString() != "5" || maxReal != 17 || maxInt != 23 || invocation.numBits != 6 {
		t.Fatal("translated consumers used different policy", root.RatString(), divisor.RatString(), maxReal, maxInt, invocation.numBits)
	}
	equationMust(t, invocation.close())
	x := fixDLConst(t, target, true, "translated-x")
	three, nine := fixDLNumber(t, target, true, "3"), fixDLNumber(t, target, true, "9")
	goal := fixDLGoal(t, target, false, true, false, fixDLApp(t, target, 3, x, three), fixDLApp(t, target, 2, x, three), mustNLA2BVEq(t, target, fixDLApp(t, target, 9, x, x), nine))
	result := fmResult(t)
	equationMust(t, got.Apply(goal, result))
	if result.ModelConverter == nil {
		t.Fatal("translated tactic did not publish model conversion")
	}
	md, err := model.NewModel(target)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	var ref tactic.ModelRef
	equationMust(t, ref.Set(md.Core()))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	equationMust(t, result.ModelConverter.ApplyModel(&ref, 0))
	core, err := ref.Get()
	equationMust(t, err)
	if value := fmValue(t, target, core, x); value != "3" {
		t.Fatal("target model reconstruction", value)
	}
}
func TestNLA2BVTranslationAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 256; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			source := nla2BVFixture(t, false)
			target, _ := nla2BVIndependentTarget(t)
			name := "nla2bv_bv_size"
			equationMust(t, source.context.SetUIntName(&source.params, &name, 6))
			target.Budget().FailAfter(stop)
			translated, err := source.Translate(target)
			target.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
				equationMust(t, translated.IncRef())
				equationMust(t, translated.DecRef())
			} else {
				var failure umemory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != umemory.FailureMemout || !target.NumeralService().RuntimeOwner().State().OutOfMemory) {
					t.Fatal(err)
				}
				if translated != nil {
					t.Fatal("failed translation returned a partially owned interface")
				}
			}
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("translation allocation success never reached")
	}
}
