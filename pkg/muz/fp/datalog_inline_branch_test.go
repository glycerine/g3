package fp

// Source: dl_mk_rule_inliner.cpp::inline_linear expands shared definitions
// only with xform.inline_linear_branch and deletes only the final use.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogLinearInliningBranchParametersAndFacts(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		module, local, value, facts bool
	}{
		{name: "default"},
		{name: "module", module: true},
		{name: "local", local: true, value: true},
		{name: "local-disables-module", module: true, local: true},
		{name: "extensional-definition", module: true, facts: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(globals.SourceSetName("fixedpoint.xform.inline_eager", "false"))
			if tc.module {
				must(globals.SourceSetName("fixedpoint.xform.inline_linear_branch", "true"))
			}
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(e.Close()) })
			InstallContextModelConverter(e) // a prior context::query installed m_mc
			must(e.SelectEngine("datalog"))
			if tc.local {
				local := globals.Context.EmptyValue()
				key := "xform.inline_linear_branch"
				must(globals.Context.SetBoolName(&local, &key, tc.value))
				must(e.SetQueryParameters(&local))
				must(globals.Context.Del(&local))
			}
			boolean, err := m.MkBoolSort()
			must(err)
			declare := func(name string) ast.ASTHandle {
				s, err := m.Symbols.Intern("linear-branch-" + name)
				must(err)
				d, err := m.MkFuncDecl(s, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				return d
			}
			seed, middle, left, right := declare("seed"), declare("middle"), declare("left"), declare("right")
			must(e.AddTableFact(seed, []uint32{0}))
			if tc.facts {
				must(e.AddTableFact(middle, []uint32{1}))
			}
			x, err := m.MkVar(0, boolean)
			must(err)
			app := func(d ast.ASTHandle) ast.ASTHandle {
				a, err := m.MkApp(d, []ast.ASTHandle{x})
				must(err)
				return a
			}
			for _, pair := range [][2]ast.ASTHandle{{seed, middle}, {middle, left}, {middle, right}} {
				rule, err := m.MkImplies(app(pair[0]), app(pair[1]))
				must(err)
				must(e.addRuleEager(rule, ""))
			}
			status, err := e.QueryPredicates([]ast.ASTHandle{left, right})
			must(err)
			if status != lbool.True || !e.compiled {
				t.Fatal("branch query", status, e.compiled)
			}
			for _, d := range []ast.ASTHandle{left, right} {
				rows, err := e.predicates[d].table.Facts()
				must(err)
				want := 1
				if tc.facts {
					want = 2
				}
				seen := make(map[uint64]bool)
				for _, row := range rows {
					if len(row) != 1 {
						t.Fatal("branch output arity", row)
					}
					seen[uint64(row[0])] = true
				}
				if len(rows) != want || !seen[0] || seen[1] != tc.facts {
					t.Fatal("branch changed output tuples", rows)
				}
			}
			eliminated, err := e.hornInlineEliminated(e.datalogConversions, middle)
			must(err)
			wantEliminated := (tc.module && !tc.local || tc.local && tc.value) && !tc.facts
			if eliminated != wantEliminated {
				t.Fatal("linear branch selection/fact protection", eliminated, wantEliminated)
			}
			status, err = e.QueryPredicate(middle)
			must(err)
			if status != lbool.True {
				t.Fatal("later query lost original definition", status)
			}
			count, err := e.set.Len()
			must(err)
			if count != 3 {
				t.Fatal("branch inlining escaped query scope", count)
			}
		})
	}
}
