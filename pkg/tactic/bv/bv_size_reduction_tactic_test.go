package bv

import (
	"bytes"
	"errors"
	"fmt"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func bvSizeFixture(t *testing.T, proofs bool) *BVSizeReductionTactic {
	t.Helper()
	base := sharingTacticFixture(t, proofs)
	m := base.manager
	sharingMust(t, m.RegisterDeclarationPlugins())
	coop, err := cooperate.NewCooperation(m.Budget())
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, coop.Close()) })
	globals := uparams.BorrowProcessGlobalParamsService()
	services := model.ModelEvaluatorConfig{Context: base.context, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: coop}
	r, err := NewBVSizeReductionTactic(m, base.context, globals, services)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, r.Close()) })
	return r
}
func bvSizeNativeGoal(t *testing.T, r *BVSizeReductionTactic, lower, upper bool, models bool) (*tactic.Goal, ast.ASTHandle) {
	t.Helper()
	m := r.manager
	sort, err := m.BitVectorSort(8)
	sharingMust(t, err)
	name, err := m.Symbols.Intern("bv-size-x")
	sharingMust(t, err)
	x, err := m.MkConst(name, sort)
	sharingMust(t, err)
	g, err := tactic.NewGoal(m, false, models, false)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	for _, lo := range []bool{true, false} {
		if lo && !lower || !lo && !upper {
			continue
		}
		v := int64(2)
		if lo {
			v = -2
		}
		num, err := m.BitVectorNumeralFromRational(&unumeral.Rational{Num: unumeral.NewInt(int32(v)), Den: unumeral.NewInt(1)}, 8)
		sharingMust(t, err)
		args := []ast.ASTHandle{x, num}
		if lo {
			args = []ast.ASTHandle{num, x}
		}
		f, err := m.MkFamilyApp(r.current.family, 23, nil, args)
		sharingMust(t, err)
		sharingMust(t, g.Assert(f))
	}
	return g, x
}
func TestBVSizeReductionModelLifetimeFilteringAndPlacementCleanup(t *testing.T) {
	r := bvSizeFixture(t, false)
	m := r.manager
	g, x := bvSizeNativeGoal(t, r, true, true, true)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, r.Apply(g, &result))
	if result.ModelConverter == nil {
		t.Fatal("missing extension/filter converter")
	}
	var def, proof ast.ASTHandle
	found, err := m.ExprSubstitutionFindProof(r.current.active.value, x, &def, &proof)
	sharingMust(t, err)
	if !found {
		t.Fatal("missing narrowing definition")
	}
	fresh, err := m.GetArg(def, 0)
	sharingMust(t, err)
	width, err := m.BitVectorSize(fresh)
	sharingMust(t, err)
	if width != 3 {
		t.Fatal("source conservative mixed-sign width", width)
	}
	d, err := m.GetDecl(x)
	sharingMust(t, err)
	fd, err := m.GetDecl(fresh)
	sharingMust(t, err)
	old := r.current
	m.Budget().FailAfter(0)
	err = r.Cleanup()
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || r.open() == nil || old.ready {
		t.Fatal("placement-style cleanup preserved an executable old implementation", err)
	}
	sharingMust(t, r.Cleanup())
	if len(r.current.lower) != 0 || len(r.current.upper) != 0 {
		t.Fatal("rebuild retained old bound maps")
	}
	sharingMust(t, r.Close())
	md, err := model.NewModel(m)
	sharingMust(t, err)
	var out tactic.ModelRef
	sharingMust(t, out.Set(md.Core()))
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	value, err := m.BitVectorNumeralUnsigned(7, 3)
	sharingMust(t, err)
	sharingMust(t, md.Core().RegisterConstant(fd, value))
	sharingMust(t, result.ModelConverter.ApplyModel(&out, 0))
	reconstructed, err := out.Get()
	sharingMust(t, err)
	value, err = reconstructed.ConstantInterpretation(d)
	sharingMust(t, err)
	text, yes, err := m.BitVectorNumeralText(value)
	sharingMust(t, err)
	if !yes || text != "255" {
		t.Fatal("signed reconstruction after tactic destruction", text)
	}
	helpers, err := reconstructed.ConstantInterpretation(fd)
	sharingMust(t, err)
	if !helpers.IsNull() {
		t.Fatal("fresh helper escaped model filtering")
	}
}

func TestBVSizeReductionMetadataGuardsAndCancellation(t *testing.T) {
	r := bvSizeFixture(t, true)
	m := r.manager
	g, _ := bvSizeNativeGoal(t, r, true, true, true)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, r.Apply(g, &result))
	previous := result.ModelConverter
	for _, proofs := range []bool{true, false} {
		goal, err := tactic.NewGoal(m, proofs, false, !proofs)
		sharingMust(t, err)
		t.Cleanup(func() { sharingMust(t, goal.Close()) })
		err = r.Apply(goal, &result)
		var refusal tactic.TacticException
		if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "does not support") {
			t.Fatal("metadata refusal", err)
		}
		if result.ModelConverter != previous || len(result.Goals) != 1 || result.Goals[0] != g {
			t.Fatal("metadata refusal reset earlier result")
		}
	}
	sharingMust(t, r.Cleanup())
	oneSided, _ := bvSizeNativeGoal(t, r, true, false, false)
	m.ResourceLimit().IncCancel()
	err := r.Apply(oneSided, &result)
	m.ResourceLimit().DecCancel()
	sharingMust(t, err) // No substitution, hence no reached checkpoint.
	sharingMust(t, r.Cleanup())
	both, _ := bvSizeNativeGoal(t, r, true, true, true)
	m.ResourceLimit().IncCancel()
	err = r.Apply(both, &result)
	m.ResourceLimit().DecCancel()
	if !rewriter.IsRewriterException(err) || !strings.Contains(err.Error(), "canceled") {
		t.Fatal("default replacer cancellation transport", err)
	}
	sharingMust(t, r.Cleanup())
	sharingMust(t, r.Apply(both, &result))
}

func TestBVSizeReductionRawStateTranslationIgnoredParamsAndTrace(t *testing.T) {
	r := bvSizeFixture(t, false)
	m := r.manager
	g, _ := bvSizeNativeGoal(t, r, true, true, true)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	for _, name := range []string{"bv_size_reduction", "before_bv_size_reduction", "after_bv_size_reduction"} {
		tags.Enable(name)
	}
	sharingMust(t, r.BindTrace(tags))
	var first tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, first.Close()) })
	sharingMust(t, r.Apply(g, &first))
	if tags.ShouldTrace("bv_size_reduction") {
		for _, text := range []string{"collect_bounds", "bv_size_reduction_tactic.cpp:127", "bv_size_reduction_tactic.cpp:142", "m_signed_lowers:", "-2 <= bv-size-x <= 2", "3 bits"} {
			if !strings.Contains(output.String(), text) {
				t.Fatal("missing signed-bound trace", text, output.String())
			}
		}
	}
	secondGoal, _ := bvSizeNativeGoal(t, r, false, true, true)
	var second tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, second.Close()) })
	sharingMust(t, r.Apply(secondGoal, &second))
	if second.ModelConverter == nil {
		t.Fatal("raw tactic forgot its earlier signed lower bound")
	}
	settings, err := r.context.NewValue()
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, r.context.Del(&settings)) })
	name := "max_memory"
	sharingMust(t, r.context.SetUIntName(&settings, &name, 0))
	sharingMust(t, r.UpdateParams(&settings))
	desc, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, desc.Close()) })
	sharingMust(t, r.CollectParamDescrs(desc))
	size, err := desc.Size()
	sharingMust(t, err)
	if size != 0 {
		t.Fatal("invented tactic parameters")
	}
	sharingMust(t, r.Cleanup())
	closed := diagnostic.NewTraceTagsWithWriter(bv1ClosedWriter{})
	closed.Enable("bv_size_reduction")
	sharingMust(t, r.BindTrace(closed))
	fresh, _ := bvSizeNativeGoal(t, r, true, true, false)
	sharingMust(t, r.Apply(fresh, &second))
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, target.Close()) })
	sharingMust(t, target.RegisterDeclarationPlugins())
	copy, err := r.Translate(target)
	sharingMust(t, err)
	translated := copy.(*BVSizeReductionTactic)
	t.Cleanup(func() { sharingMust(t, translated.Close()) })
	tr, err := ast.NewASTTranslation(m, target, true)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, tr.Close()) })
	original, _ := bvSizeNativeGoal(t, r, true, true, false)
	goal, err := original.Translate(tr)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, goal.Close()) })
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, translated.Apply(goal, &result))
	if translated.current.active == nil {
		t.Fatal("translated tactic did not narrow target goal")
	}
}

func TestBVSizeReductionAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "apply", "cleanup"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 2048; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					r := bvSizeFixture(t, false)
					m := r.manager
					if phase == "constructor" {
						m.Budget().FailAfter(stop)
						fresh, err := NewBVSizeReductionTactic(m, r.context, r.globals, r.services)
						m.Budget().FailAfter(-1)
						if fresh != nil {
							sharingMust(t, fresh.Close())
						}
						var failed memory.ConstructionFailure
						if errors.As(err, &failed) {
							sharingMust(t, failed.Cleanup())
						}
						if err == nil {
							success = true
						} else if !errors.Is(err, arena.ErrInjectedFailure) {
							t.Fatal(err)
						}
						return
					}
					g, _ := bvSizeNativeGoal(t, r, true, true, true)
					var result tactic.TacticResult
					t.Cleanup(func() { sharingMust(t, result.Close()) })
					if phase == "cleanup" {
						sharingMust(t, r.Apply(g, &result))
					}
					m.Budget().FailAfter(stop)
					var err error
					if phase == "apply" {
						err = r.Apply(g, &result)
					} else {
						err = r.Cleanup()
					}
					m.Budget().FailAfter(-1)
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
					t.Fatal("failed reservation frontier", phase, stop)
				}
			}
			if !success {
				t.Fatal("no successful reservation frontier", phase)
			}
		})
	}
}
