package fp

// Sources: dl_mk_rule_inliner.cpp rule_unifier::apply name concatenation,
// dl_mk_subsumption_checker.cpp target-name preservation, and Spacer's direct
// get_rules_along_trace forwarding of the transformed reach-fact rules.

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerExpressionQueryTraceNamesFollowInlining(t *testing.T) {
	for _, inline := range []bool{false, true} {
		for _, checker := range []bool{false, true} {
			t.Run("inline="+strconv.FormatBool(inline)+"/checker="+strconv.FormatBool(checker), func(t *testing.T) {
				m, globals := newPDRLifecycleServices(t)
				for _, setting := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
					fragmentMust(t, globals.SourceSetName(setting, strconv.FormatBool(inline)))
				}
				fragmentMust(t, globals.SourceSetName("fixedpoint.xform.slice", "false"))
				fragmentMust(t, globals.SourceSetName("fixedpoint.xform.subsumption_checker", strconv.FormatBool(checker)))
				e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
				fragmentMust(t, err)
				defer func() { fragmentMust(t, e.Close()) }()
				fragmentMust(t, e.SelectEngine("spacer"))
				boolean, err := m.MkBoolSort()
				fragmentMust(t, err)
				declare := func(name string) ast.ASTHandle {
					symbol, err := m.Symbols.Intern(name)
					fragmentMust(t, err)
					decl, err := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
					fragmentMust(t, err)
					fragmentMust(t, e.RegisterPredicate(decl))
					app, err := m.MkApp(decl, nil)
					fragmentMust(t, err)
					return app
				}
				p, q := declare("trace-name-p"), declare("trace-name-q")
				fragmentMust(t, e.AddRule(p, "fact"))
				step, err := m.MkImplies(p, q)
				fragmentMust(t, err)
				fragmentMust(t, e.AddRule(step, "step"))
				status, err := e.QueryExpression(q)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("expression query", status)
				}
				forms, names, err := e.RulesAlongTrace()
				fragmentMust(t, err)
				defer func() {
					for i := range forms {
						fragmentMust(t, m.DelRef(&forms[i]))
					}
				}()
				want := []string{"<null>", "step", "fact"}
				if inline {
					want = []string{"<null>;step;fact"}
				}
				if checker {
					want = []string{"<null>"}
				}
				if !reflect.DeepEqual(names, want) {
					t.Fatalf("trace names = %q, want %q", names, want)
				}
				status, err = e.QueryExpression(p)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("replacement expression query", status)
				}
				for _, form := range forms {
					if _, err := m.RefGet(form); err != nil {
						t.Fatal("prior trace lost ownership after query-clone replacement", err)
					}
				}
				if !reflect.DeepEqual(names, want) {
					t.Fatal("prior trace names changed after replacement query", names)
				}
			})
		}
	}
}
