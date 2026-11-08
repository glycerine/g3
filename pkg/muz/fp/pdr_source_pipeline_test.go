package fp

// Sources: src/muz/pdr/pdr_dl_interface.cpp:80-105 and
// src/muz/rel/rel_context.cpp:297-301 (separate relational normalization).

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRQueryPreservesOriginalRulesAcrossSourcePipeline(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		for _, shape := range []string{"constant-tail", "three-tails"} {
			for _, reachable := range []bool{false, true} {
				name := backend + "/" + shape + map[bool]string{false: "/unsat", true: "/sat"}[reachable]
				t.Run(name, func(t *testing.T) {
					m, globals := newPDRLifecycleServices(t)
					for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.slice", "fixedpoint.xform.subsumption_checker"} {
						fragmentMust(t, globals.SourceSetName(key, "false"))
					}
					e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
					fragmentMust(t, err)
					defer func() { fragmentMust(t, e.Close()) }()
					fragmentMust(t, e.SelectEngine(backend))
					var pins []ast.ASTRef
					defer func() {
						for i := len(pins) - 1; i >= 0; i-- {
							fragmentMust(t, m.DelRef(&pins[i]))
						}
					}()
					own := func(h ast.ASTHandle, err error) ast.ASTHandle {
						fragmentMust(t, err)
						r, err := m.NewRef(h)
						fragmentMust(t, err)
						pins = append(pins, r)
						return h
					}
					boolean := own(m.MkBoolSort())
					declare := func(label string, domain []ast.ASTHandle) ast.ASTHandle {
						symbol, err := m.Symbols.Intern("pdr-pipeline-" + label)
						fragmentMust(t, err)
						d := own(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
						fragmentMust(t, e.RegisterPredicate(d))
						return d
					}
					goal := declare("goal", nil)
					goalApp := own(m.MkApp(goal, nil))
					var atoms []ast.ASTHandle
					if shape == "constant-tail" {
						zero, one := own(m.ArithmeticIntegerNumeralText("0")), own(m.ArithmeticIntegerNumeralText("1"))
						integer, err := m.GetSort(zero)
						fragmentMust(t, err)
						p := declare("p", []ast.ASTHandle{integer})
						p0, p1 := own(m.MkApp(p, []ast.ASTHandle{zero})), own(m.MkApp(p, []ast.ASTHandle{one}))
						fact := p1
						if reachable {
							fact = p0
						}
						fragmentMust(t, e.addRuleEager(fact, "pipeline-fact"))
						fragmentMust(t, e.addRuleEager(own(m.MkImplies(p0, goalApp)), "pipeline-goal"))
						atoms = []ast.ASTHandle{goalApp, p1, p0}
					} else {
						for _, label := range []string{"a", "b", "c"} {
							d := declare(label, nil)
							atoms = append(atoms, own(m.MkApp(d, nil)))
						}
						for i := 0; i < 2; i++ {
							fragmentMust(t, e.addRuleEager(atoms[i], "pipeline-fact"))
						}
						if reachable {
							fragmentMust(t, e.addRuleEager(atoms[2], "pipeline-c"))
						} else {
							fragmentMust(t, e.addRuleEager(own(m.MkImplies(atoms[2], atoms[2])), "pipeline-c-recursive"))
						}
						fragmentMust(t, e.addRuleEager(own(m.MkImplies(own(m.MkAndArray(atoms)), goalApp)), "pipeline-goal"))
					}
					original, names, err := e.NamedRules()
					fragmentMust(t, err)
					defer func() {
						for i := range original {
							fragmentMust(t, m.DelRef(&original[i]))
						}
					}()
					originals := make([]ast.ASTHandle, len(original))
					for i, ref := range original {
						originals[i], err = m.RefGet(ref)
						fragmentMust(t, err)
					}
					for repeat := 0; repeat < 2; repeat++ {
						status, err := e.QueryPredicate(goal)
						fragmentMust(t, err)
						want := lbool.False
						if reachable {
							want = lbool.True
						}
						if status != want {
							t.Fatal("source pipeline result", status, want)
						}
						current, currentNames, err := e.NamedRules()
						fragmentMust(t, err)
						same := len(current) == len(original) && reflect.DeepEqual(currentNames, names)
						for i, ref := range current {
							h, err := m.RefGet(ref)
							fragmentMust(t, err)
							if i >= len(originals) || h != originals[i] {
								same = false
							}
						}
						for i := range current {
							fragmentMust(t, m.DelRef(&current[i]))
						}
						if !same {
							t.Fatal("PDR query rewrote original rules", len(current), len(original), currentNames, names)
						}
					}
					if !reachable {
						md, err := e.Model(goal)
						fragmentMust(t, err)
						defer func() { fragmentMust(t, md.Close()) }()
						cooperation, err := cooperate.NewCooperation(m.Budget())
						fragmentMust(t, err)
						defer func() { fragmentMust(t, cooperation.Close()) }()
						evaluator, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: globals.Context, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
						fragmentMust(t, err)
						defer func() { fragmentMust(t, evaluator.Close()) }()
						fragmentMust(t, evaluator.SetModelCompletion(false))
						probes := atoms
						expected := []bool{true, false}
						if shape == "constant-tail" {
							// context::get_level_property excludes the query predicate.
							present, err := md.Core().HasInterpretation(goal)
							fragmentMust(t, err)
							if present {
								t.Fatal("unconverted query predicate unexpectedly interpreted")
							}
							probes = atoms[1:]
						}
						if shape == "three-tails" {
							probes = append(append([]ast.ASTHandle(nil), atoms...), goalApp)
							expected = []bool{true, true, false, false}
						}
						for i, app := range probes {
							value, err := evaluator.EvaluateValue(app)
							fragmentMust(t, err)
							h, err := m.RefGet(value)
							fragmentMust(t, err)
							valid := m.IsFalse(h)
							if expected[i] {
								valid = m.IsTrue(h)
							}
							fragmentMust(t, m.DelRef(&value))
							if !valid {
								t.Fatal("original model interpretation", i, expected[i])
							}
						}
					}
				})
			}
		}
	}
}
