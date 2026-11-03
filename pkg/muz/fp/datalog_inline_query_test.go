package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogInliningPreservesFactsModelsAndQueryScope(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		serviceFree, disabled, facts bool
		moduleDisabled, localEnable  bool
		eagerOnly, linearOnly        bool
	}{
		{name: "default"},
		{name: "without-services", serviceFree: true},
		{name: "disabled", disabled: true},
		{name: "facts-and-rule", facts: true},
		{name: "module-disabled", moduleDisabled: true},
		{name: "local-override", moduleDisabled: true, localEnable: true},
		{name: "eager-only", eagerOnly: true},
		{name: "linear-only", linearOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			var budget *arena.Budget
			t.Cleanup(func() {
				if budget != nil && budget.Used() != 0 {
					t.Fatal("inline query leaked bytes", budget.Used())
				}
			})
			m, globals := newPDRLifecycleServices(t)
			budget = m.Budget()
			if tc.moduleDisabled {
				for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
					must(globals.SourceSetName(key, "false"))
				}
			}
			var e *PropositionalEngine
			var err error
			if tc.serviceFree {
				e, err = NewPropositionalEngine(m)
			} else {
				e, err = NewPropositionalEngineWithServices(m, globals.Context, globals)
			}
			must(err)
			t.Cleanup(func() { must(e.Close()) })
			must(e.SelectEngine("datalog"))
			if tc.disabled || tc.localEnable || tc.eagerOnly || tc.linearOnly {
				local := globals.Context.EmptyValue()
				for _, key := range []string{"xform.inline_eager", "xform.inline_linear"} {
					enabled := tc.localEnable || key == "xform.inline_eager" && tc.eagerOnly || key == "xform.inline_linear" && tc.linearOnly
					must(globals.Context.SetBoolName(&local, &key, enabled))
				}
				must(e.SetQueryParameters(&local))
				must(globals.Context.Del(&local))
			}
			boolean, err := m.MkBoolSort()
			must(err)
			declare := func(name string) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(name)
				must(err)
				declaration, err := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(declaration))
				return declaration
			}
			seed, middle, output := declare("inline-seed"), declare("inline-middle"), declare("inline-output")
			must(e.AddTableFact(seed, []uint32{0}))
			if tc.facts {
				must(e.AddTableFact(middle, []uint32{1}))
			}
			x, err := m.MkVar(0, boolean)
			must(err)
			app := func(declaration, argument ast.ASTHandle) ast.ASTHandle {
				result, err := m.MkApp(declaration, []ast.ASTHandle{argument})
				must(err)
				return result
			}
			for _, pair := range [][2]ast.ASTHandle{{seed, middle}, {middle, output}} {
				rule, err := m.MkImplies(app(pair[0], x), app(pair[1], x))
				must(err)
				must(e.AddRule(rule, ""))
			}
			status, err := e.QueryPredicate(output)
			must(err)
			if status != lbool.True || !e.compiled {
				t.Fatal("inlined finite query", status, e.compiled)
			}
			rows, err := e.predicates[output].table.Facts()
			must(err)
			wantRows := 1
			if tc.facts {
				wantRows = 2
			}
			if len(rows) != wantRows {
				t.Fatal("inlining dropped extensional rows", rows)
			}
			enabled := !tc.disabled && (!tc.moduleDisabled || tc.localEnable) && !tc.facts
			eliminated, err := e.hornInlineEliminated(e.datalogConversions, middle)
			must(err)
			if eliminated != enabled {
				t.Fatal("default inlining or fact/parameter protection", eliminated, enabled)
			}
			inductive, err := e.Model(output)
			must(err)
			cooperation, err := cooperate.NewCooperation(budget)
			must(err)
			evaluator, err := model.NewModelEvaluator(inductive.Core(), model.ModelEvaluatorConfig{Context: globals.Context, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
			must(err)
			for _, value := range []bool{false, true} {
				argument, err := m.MkFalse()
				if value {
					argument, err = m.MkTrue()
				}
				must(err)
				result, err := evaluator.EvaluateValue(app(middle, argument))
				must(err)
				handle, err := m.RefGet(result)
				must(err)
				// Source inline_linear records del_rule(..., true), which
				// supplies a false default for its removed definition.
				wantTrue := (!value || tc.facts) && !tc.linearOnly
				if m.IsTrue(handle) != wantTrue || !m.IsTrue(handle) && !m.IsFalse(handle) {
					t.Fatal("inline model did not reconstruct the removed relation", value, handle)
				}
				must(m.DelRef(&result))
			}
			must(evaluator.Close())
			must(cooperation.Close())
			must(inductive.Close())
			status, err = e.QueryPredicate(middle)
			must(err)
			if status != lbool.True {
				t.Fatal("later query could not expose previously inlined predicate", status)
			}
			count, err := e.set.Len()
			must(err)
			if count != 2 || len(e.predicates) != 3 || tc.serviceFree && (e.params != nil || e.globals != nil || e.guard != nil) {
				t.Fatal("query-local inlining changed persistent engine state")
			}
		})
	}
}
