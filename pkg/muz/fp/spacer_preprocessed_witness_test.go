package fp

// Sources: spacer_dl_interface.cpp SAT export forwarding, dl_mk_slice.cpp
// column projection and dl_mk_rule_inliner.cpp source-name composition.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerPreprocessedSharedWitnessExports(t *testing.T) {
	for _, mode := range []string{"plain", "slice", "inline"} {
		t.Run(mode, func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.slice", "fixedpoint.xform.subsumption_checker", "fixedpoint.xform.coalesce_rules"} {
				fragmentMust(t, globals.SourceSetName(key, "false"))
			}
			fragmentMust(t, globals.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
			if mode == "slice" {
				fragmentMust(t, globals.SourceSetName("fixedpoint.xform.slice", "true"))
			}
			if mode == "inline" {
				for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
					fragmentMust(t, globals.SourceSetName(key, "true"))
				}
			}
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, e.Close()) }()
			fragmentMust(t, e.SelectEngine("spacer"))
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
			zero := own(m.ArithmeticIntegerNumeralText("0"))
			integer, err := m.GetSort(zero)
			fragmentMust(t, err)
			variable := own(m.MkVar(0, integer))
			labels := []string{"goal", "root", "left", "right", "mid", "leaf"}
			decls, apps := make([]ast.ASTHandle, len(labels)), make([]ast.ASTHandle, len(labels))
			for i, label := range labels {
				name, err := m.Symbols.Intern("preprocessed-witness-" + label)
				fragmentMust(t, err)
				var domain, args []ast.ASTHandle
				if label == "leaf" {
					domain = []ast.ASTHandle{integer, integer}
					args = []ast.ASTHandle{zero, variable}
				}
				decls[i] = own(m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(decls[i]))
				apps[i] = own(m.MkApp(decls[i], args))
			}
			tails := [][]int{{1}, {2, 3}, {5}, {4}, {5}, nil}
			names := []string{"goal-rule", "root-rule", "left-rule", "right-rule", "mid-rule", "leaf-fact"}
			for i, indices := range tails {
				formula := apps[i]
				if len(indices) != 0 {
					body := make([]ast.ASTHandle, len(indices))
					for j, index := range indices {
						body[j] = apps[index]
					}
					formula = own(m.MkImplies(own(m.MkAndArray(body)), formula))
				}
				if i == 2 || i == 4 || i == 5 {
					formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: formula}))
				}
				fragmentMust(t, e.AddRule(formula, names[i]))
			}
			inputCount, err := e.set.Len()
			fragmentMust(t, err)
			status, err := e.QueryPredicate(decls[0])
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("public nonlinear query", status)
			}
			afterCount, err := e.set.Len()
			fragmentMust(t, err)
			if afterCount != inputCount {
				t.Fatal("Spacer query rewrote original rule set", afterCount, inputCount)
			}
			answer, err := e.GroundSatAnswer(decls[0])
			fragmentMust(t, err)
			defer func() { fragmentMust(t, m.DelRef(&answer)) }()
			root, err := m.RefGet(answer)
			fragmentMust(t, err)
			forms, traceNames, err := e.RulesAlongTrace()
			fragmentMust(t, err)
			defer func() {
				for i := range forms {
					fragmentMust(t, m.DelRef(&forms[i]))
				}
			}()
			if mode == "inline" {
				if !m.IsTrue(root) || len(forms) != 1 {
					t.Fatal("inlined fact export", len(forms))
				}
				got := map[string]int{}
				for _, name := range strings.Split(traceNames[0], ";") {
					if name != "" {
						got[name]++
					}
				}
				want := map[string]int{"goal-rule": 1, "root-rule": 1, "left-rule": 1, "right-rule": 1, "mid-rule": 1, "leaf-fact": 2}
				if !reflect.DeepEqual(got, want) {
					t.Fatal("source inlined name occurrences", traceNames, got, want)
				}
			} else {
				// Source HNF flatten_and moves the final appended body child
				// into the conjunction slot, so right precedes left here.
				wantNames := []string{"", "goal-rule", "root-rule", "right-rule", "left-rule", "mid-rule", "leaf-fact", "leaf-fact"}
				if !reflect.DeepEqual(traceNames, wantNames) {
					t.Fatal("source queue trace names", traceNames, wantNames)
				}
				count, err := m.GetNumArgs(root)
				fragmentMust(t, err)
				order := []int{0, 1, 3, 2, 4, 5, 5}
				if int(count) != len(order) {
					t.Fatal("ground witness occurrences", count, len(order))
				}
				leaf := decls[5]
				if mode == "slice" {
					if !slicedPredicateHasRemovedColumns(e.spacerSlices, leaf) {
						t.Fatal("fixture did not exercise slicing")
					}
					leaf = e.spacerPredicate(leaf)
				}
				for i, index := range order {
					app, err := m.GetArg(root, uint32(i))
					fragmentMust(t, err)
					decl, err := m.GetDecl(app)
					fragmentMust(t, err)
					want := decls[index]
					if index == 5 {
						want = leaf
					}
					if decl != want {
						t.Fatal("transformed witness predicate/order", i, decl, want)
					}
					if index == 5 {
						arity, err := m.GetNumArgs(app)
						fragmentMust(t, err)
						if mode == "plain" && arity != 2 {
							t.Fatal("original leaf arity", arity)
						}
						if mode == "slice" && arity != 1 {
							t.Fatal("sliced leaf arity", arity)
						}
						value, err := m.GetArg(app, 0)
						fragmentMust(t, err)
						if value != zero {
							t.Fatal("ground constrained argument", value, zero)
						}
						if mode == "plain" {
							unconstrained, err := m.GetArg(app, 1)
							fragmentMust(t, err)
							_, integer, matched, err := m.ArithmeticNumeralText(unconstrained)
							fragmentMust(t, err)
							if !integer || !matched {
								t.Fatal("unconstrained argument was not completed to an integer")
							}
						}
					}
				}
			}
			// Exported owners survive a replacement query and disposal of the engine.
			status, err = e.QueryPredicate(decls[0])
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			fragmentMust(t, e.Close())
			if got, err := m.RefGet(answer); err != nil || got != root {
				t.Fatal("prior witness owner lost", got, err)
			}
			for _, form := range forms {
				if _, err := m.RefGet(form); err != nil {
					t.Fatal("prior exported rule owner lost", err)
				}
			}
		})
	}
}
