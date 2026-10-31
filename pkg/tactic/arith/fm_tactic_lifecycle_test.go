package arith

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestFMTacticCleanupReplacementFailurePreservesImplementation(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	x := fixDLConst(t, m, false, "x")
	g := fixDLGoal(t, m, false, true, false, fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4")))
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	old := fm.run
	parser := old.parser
	converter := old.model
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	err := fm.Cleanup()
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || fm.run != old || old.parser != parser || old.model != converter || m.Budget().Used() != before {
		t.Fatal("failed replacement changed private implementation", err)
	}
	equationMust(t, fm.Cleanup())
	if fm.run == old || old.bytes != 0 || fm.run.bytes == 0 || fm.run.parser != nil || fm.run.counter != 0 {
		t.Fatal("missing fresh implementation")
	}
	if r.ModelConverter != converter {
		t.Fatal("cleanup changed result converter")
	}
	equationMust(t, fm.Apply(g, r))
}

func TestFMTacticParamsSnapshotAndCleanupPolicy(t *testing.T) {
	fm, _ := fmTacticFixture(t, false)
	p, err := fm.context.NewValue()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, fm.context.Del(&p)) })
	name := "fm_limit"
	equationMust(t, fm.context.SetUIntName(&p, &name, 7))
	equationMust(t, fm.UpdateParams(&p))
	equationMust(t, fm.context.SetUIntName(&p, &name, 99))
	// Cleanup reconstructs policy from the tactic's independent snapshot.
	fm.limit = 123
	equationMust(t, fm.Cleanup())
	if fm.limit != 7 {
		t.Fatal("borrowed caller params or stale policy", fm.limit)
	}
	copy := *fm
	if err = copy.Cleanup(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owner", err)
	}
}

func TestFMTacticAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "apply", "cleanup"} {
		succeeded := false
		for stop := int64(0); stop < 256; stop++ {
			t.Run(fmt.Sprintf("%s/%d", phase, stop), func(t *testing.T) {
				fm, m := fmTacticFixture(t, false)
				if phase == "constructor" {
					m.Budget().FailAfter(stop)
					fresh, err := NewFMTactic(m, fm.context, fm.globals, nil, fm.services)
					m.Budget().FailAfter(-1)
					if err == nil {
						succeeded = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
					if fresh != nil {
						equationMust(t, fresh.Close())
					}
					return
				}
				x := fixDLConst(t, m, false, "x")
				g := fixDLGoal(t, m, false, true, true, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "2")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4")))
				r := fmResult(t)
				if phase == "cleanup" {
					equationMust(t, fm.Apply(g, r))
				}
				m.Budget().FailAfter(stop)
				var err error
				if phase == "apply" {
					err = fm.Apply(g, r)
				} else {
					err = fm.Cleanup()
				}
				m.Budget().FailAfter(-1)
				if err == nil {
					succeeded = true
				} else if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
				equationMust(t, fm.Cleanup())
				equationMust(t, fm.Apply(g, r))
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("no successful reservation frontier", phase)
		}
	}
}

func TestFMTacticTranslationUsesIndependentTargetServices(t *testing.T) {
	fm, from := fmTacticFixture(t, false)
	targetBase, baseManager := fixDLFixture(t, false)
	to, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: baseManager.Symbols, Numerals: baseManager.NumeralService(), Budget: baseManager.Budget(), Params: targetBase.context, Globals: targetBase.globals})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, to.Close()) })
	_, err = to.RegisterArithmeticPlugin()
	equationMust(t, err)
	_, err = to.RegisterBitVectorPlugin()
	equationMust(t, err)
	name := "fm_real_only"
	equationMust(t, fm.context.SetBoolName(&fm.params, &name, false))
	equationMust(t, fm.UpdateParams(&fm.params))
	translated, err := fm.Translate(to)
	equationMust(t, err)
	equationMust(t, translated.IncRef())
	t.Cleanup(func() { equationMust(t, translated.DecRef()) })
	typed := translated.(*FMTactic)
	if typed.context != targetBase.context || typed.services.Memory != to.NumeralService().RuntimeOwner() || typed.realOnly {
		t.Fatal("foreign parameter/numeral owner or lost policy")
	}
	equationMust(t, fm.context.SetBoolName(&fm.params, &name, true))
	equationMust(t, fm.UpdateParams(&fm.params))
	equationMust(t, fm.Close())
	x := fixDLConst(t, to, true, "translated-x")
	goal := fixDLGoal(t, to, false, true, false, fixDLApp(t, to, 3, x, fixDLNumber(t, to, true, "2")), fixDLApp(t, to, 2, x, fixDLNumber(t, to, true, "4")))
	r := fmResult(t)
	equationMust(t, translated.Apply(goal, r))
	size, err := r.Goals[0].Size()
	equationMust(t, err)
	if size != 0 || r.ModelConverter == nil {
		t.Fatal("translated integer policy", size)
	}
	md, err := model.NewModel(to)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	core := fmApply(t, r.ModelConverter.(*FMModelConverter), md)
	if got := fmValue(t, to, core, x); got != "2" {
		t.Fatal("target integer model", got)
	}
	// Translate the populated converter back to a manager with different symbols
	// and verify that it evaluates through the destination parameter owner.
	back, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: from.Symbols, Numerals: from.NumeralService(), Budget: from.Budget(), Params: fm.context, Globals: fm.globals})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, back.Close()) })
	_, err = back.RegisterArithmeticPlugin()
	equationMust(t, err)
	_, err = back.RegisterBitVectorPlugin()
	equationMust(t, err)
	tr, err := ast.NewASTTranslation(to, back, true)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, tr.Close()) })
	converter, err := r.ModelConverter.Translate(tr)
	equationMust(t, err)
	equationMust(t, converter.IncRef())
	t.Cleanup(func() { equationMust(t, converter.DecRef()) })
	bx, err := tr.Translate(x)
	equationMust(t, err)
	fixDLPin(t, back, bx)
	bm, err := model.NewModel(back)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, bm.Close()) })
	bc := fmApply(t, converter.(*FMModelConverter), bm)
	if got := fmValue(t, back, bc, bx); got != "2" {
		t.Fatal("translated saved model", got)
	}
}

func TestFMTacticTranslationAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 256; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			fm, _ := fmTacticFixture(t, false)
			base, bm := fixDLFixture(t, false)
			to, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: bm.Symbols, Numerals: bm.NumeralService(), Budget: bm.Budget(), Params: base.context, Globals: base.globals})
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, to.Close()) })
			to.Budget().FailAfter(stop)
			translated, err := fm.Translate(to)
			to.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
				equationMust(t, translated.IncRef())
				equationMust(t, translated.DecRef())
			} else if !errors.Is(err, arena.ErrInjectedFailure) || translated != nil {
				t.Fatal("translation failure", translated, err)
			}
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("no successful translation frontier")
	}
}

func TestFMModelReconstructionAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 512; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			fm, m := fmTacticFixture(t, false)
			x := fixDLConst(t, m, false, "x")
			g := fixDLGoal(t, m, false, true, false, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "2")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4")))
			r := fmResult(t)
			equationMust(t, fm.Apply(g, r))
			md, err := model.NewModel(m)
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, md.Close()) })
			var ref tactic.ModelRef
			equationMust(t, ref.Set(&md.CoreField))
			t.Cleanup(func() { equationMust(t, ref.Close()) })
			m.Budget().FailAfter(stop)
			err = r.ModelConverter.ApplyModel(&ref, 0)
			m.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("no successful model-reconstruction frontier")
	}
}
