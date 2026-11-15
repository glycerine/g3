package bv

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func bv1Fixture(t *testing.T) *BV1BlasterTactic {
	t.Helper()
	base := sharingTacticFixture(t, true)
	r, err := NewBV1BlasterTactic(base.manager, base.context, uparams.BorrowProcessGlobalParamsService(), nil, nil)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, r.Close()) })
	return r
}
func bv1NativeGoal(t *testing.T, r *BV1BlasterTactic) *tactic.Goal {
	t.Helper()
	m := r.manager
	sort, err := m.BitVectorSort(4)
	sharingMust(t, err)
	name, err := m.Symbols.Intern("bv1-native-x")
	sharingMust(t, err)
	x, err := m.MkConst(name, sort)
	sharingMust(t, err)
	value, err := m.BitVectorNumeralUnsigned(10, 4)
	sharingMust(t, err)
	eq, err := m.MkEq(x, value)
	sharingMust(t, err)
	g, err := tactic.NewGoal(m, true, true, true)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	pr, err := m.MkProofAsserted(eq)
	sharingMust(t, err)
	dep, err := m.DependencyLeaf(eq)
	sharingMust(t, err)
	sharingMust(t, g.AssertWithProof(eq, pr, dep))
	return g
}
func TestBV1BlasterCachePolicyTranslationAndCleanup(t *testing.T) {
	r := bv1Fixture(t)
	g := bv1NativeGoal(t, r)
	var first tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, first.Close()) })
	sharingMust(t, r.Apply(g, &first))
	if len(r.current.bindings()) != 1 {
		t.Fatal("wide constant cache")
	}
	binding := r.current.bindings()[0]
	steps, err := r.NumSteps()
	sharingMust(t, err)
	if steps == 0 {
		t.Fatal("missing accumulated rewrite steps")
	}
	var second tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, second.Close()) })
	g2 := bv1NativeGoal(t, r)
	sharingMust(t, r.Apply(g2, &second))
	if len(r.current.bindings()) != 1 || r.current.bindings()[0] != binding {
		t.Fatal("raw apply discarded the source mapping")
	}
	settings, err := r.context.NewValue()
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, r.context.Del(&settings)) })
	key := "max_steps"
	sharingMust(t, r.context.SetUIntName(&settings, &key, 0))
	sharingMust(t, r.UpdateParams(&settings))
	if r.current.maxSteps != 0 {
		t.Fatal("live parameter update")
	}
	old := r.current
	r.manager.Budget().FailAfter(0)
	err = r.Cleanup()
	r.manager.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || r.current != old {
		t.Fatal("cleanup replaced the live implementation before construction succeeded", err)
	}
	sharingMust(t, r.Cleanup())
	if len(r.current.bindings()) != 0 || r.current.maxSteps != 0 {
		t.Fatal("cleanup did not reset cache and preserve parameters")
	}
	var display bytes.Buffer
	sharingMust(t, first.ModelConverter.Display(&display))
	if !strings.Contains(display.String(), "bv1-native-x") {
		t.Fatal("converter lost old bindings", display.String())
	}
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: r.manager.Symbols, Numerals: r.manager.NumeralService(), Budget: r.manager.Budget(), ProofMode: ast.ASTProofEnabled})
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, target.Close()) })
	sharingMust(t, target.RegisterDeclarationPlugins())
	other, err := r.Translate(target)
	sharingMust(t, err)
	translated := other.(*BV1BlasterTactic)
	t.Cleanup(func() { sharingMust(t, translated.Close()) })
	sharingMust(t, r.context.SetUIntName(&settings, &key, 200))
	sharingMust(t, r.UpdateParams(&settings))
	if translated.current.maxSteps != 0 || translated.manager != target {
		t.Fatal("translated policy aliases the caller")
	}
	tr, err := ast.NewASTTranslation(r.manager, target, true)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, tr.Close()) })
	copy, err := g2.Translate(tr)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, copy.Close()) })
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	err = translated.Apply(copy, &out)
	if !rewriter.IsRewriterException(err) {
		t.Fatal("zero-step translated policy did not retain the source rewriter exception", err)
	}
}

type bv1ClosedWriter struct{}

func (bv1ClosedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestBV1BlasterCancellationMemoryAndCooperationGuards(t *testing.T) {
	base := sharingTacticFixture(t, true)
	m := base.manager
	cooperation, err := cooperate.NewCooperation(m.Budget())
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, cooperation.Close()) })
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("cooperate_detail")
	sharingMust(t, cooperation.BindTrace(tags))
	sharingMust(t, cooperation.BeginSection())
	t.Cleanup(func() { sharingMust(t, cooperation.EndSection()) })
	r, err := NewBV1BlasterTactic(m, base.context, uparams.BorrowProcessGlobalParamsService(), nil, cooperation)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, r.Close()) })
	g := bv1NativeGoal(t, r)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	m.ResourceLimit().IncCancel()
	err = r.Apply(g, &result)
	m.ResourceLimit().DecCancel()
	if !rewriter.IsRewriterException(err) || !strings.Contains(err.Error(), "canceled") {
		t.Fatal("source cancellation exception", err)
	}
	sharingMust(t, r.Cleanup())
	sharingMust(t, r.Apply(g, &result))
	if tags.ShouldTrace("cooperate_detail") && !strings.Contains(trace.String(), "bv1 blaster") {
		t.Fatal("wrong cooperation task", trace.String())
	}
	closed := diagnostic.NewTraceTagsWithWriter(bv1ClosedWriter{})
	closed.Enable("cooperate_detail")
	sharingMust(t, cooperation.BindTrace(closed))
	sharingMust(t, r.Cleanup())
	sharingMust(t, r.Apply(bv1NativeGoal(t, r), &result))
	settings, err := r.context.NewValue()
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, r.context.Del(&settings)) })
	key := "max_memory"
	sharingMust(t, r.context.SetUIntName(&settings, &key, 0))
	sharingMust(t, r.UpdateParams(&settings))
	sharingMust(t, r.Cleanup())
	err = r.Apply(bv1NativeGoal(t, r), &result)
	var failure tactic.TacticException
	if !errors.As(err, &failure) || failure.Message() != "max. memory exceeded" {
		t.Fatal("source memory exception", err)
	}
}

func TestBV1BlasterApplicationAndCleanupAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"apply", "cleanup"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 4096; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					r := bv1Fixture(t)
					g := bv1NativeGoal(t, r)
					var result tactic.TacticResult
					t.Cleanup(func() { sharingMust(t, result.Close()) })
					if phase == "cleanup" {
						sharingMust(t, r.Apply(g, &result))
					}
					r.manager.Budget().FailAfter(stop)
					var err error
					if phase == "apply" {
						err = r.Apply(g, &result)
					} else {
						err = r.Cleanup()
					}
					r.manager.Budget().FailAfter(-1)
					if err == nil {
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
					sharingMust(t, r.Cleanup())
				})
				if success {
					break
				}
				if t.Failed() {
					t.Fatal("allocation frontier", phase, stop)
				}
			}
			if !success {
				t.Fatal("no successful allocation frontier")
			}
		})
	}
}

// AUDIT.md tactic_bv_01-F3: mk_bv1_blaster_model_converter copies
// m_const2bits (obj_map<func_decl, expr*>) in table order, not in the order
// the constants were first blasted.
func TestBV1BlasterModelConverterObjMapOrder(t *testing.T) {
	r := bv1Fixture(t)
	m := r.manager
	sort, err := m.BitVectorSort(2)
	sharingMust(t, err)
	value, err := m.BitVectorNumeralUnsigned(1, 2)
	sharingMust(t, err)
	g, err := tactic.NewGoal(m, false, true, false)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	want := ast.NewObjMap(m)
	var inserted []ast.ASTHandle
	for i := 0; i < 11; i++ {
		name, err := m.Symbols.Intern(fmt.Sprintf("bv1-order-%d", i))
		sharingMust(t, err)
		x, err := m.MkConst(name, sort)
		sharingMust(t, err)
		d, err := m.GetDecl(x)
		sharingMust(t, err)
		sharingMust(t, want.Insert(d, [2]ast.ASTHandle{}))
		inserted = append(inserted, d)
		eq, err := m.MkEq(x, value)
		sharingMust(t, err)
		sharingMust(t, g.Assert(eq))
	}
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, r.Apply(g, &out))
	mc, ok := out.ModelConverter.(*BitBlasterModelConverter)
	if !ok {
		t.Fatalf("model converter %T", out.ModelConverter)
	}
	entries := want.Entries()
	if len(mc.variables) != len(entries) {
		t.Fatal(len(mc.variables), len(entries))
	}
	same := true
	for i, entry := range entries {
		if mc.variables[i] != entry.Key {
			t.Fatal("converter order differs from obj_map order at", i)
		}
		same = same && entry.Key == inserted[i]
	}
	if same {
		t.Fatal("fixture does not distinguish obj_map order from insertion order")
	}
}
