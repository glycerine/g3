package fp

// Source: rel_context::transform_rules runs similarity at priority 5000,
// before constant-tail filtering (2000), binary joins (1000), and unbound (500).

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogSimilarityPrecedesConstantTailFilters(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(e.Close()) })
			must(e.SelectEngine("datalog"))
			local := globals.Context.EmptyValue()
			// Keep the shared internal definition visible to the similarity stage.
			for _, key := range []string{"xform.inline_eager", "xform.inline_linear"} {
				must(globals.Context.SetBoolName(&local, &key, false))
			}
			key := "datalog.similarity_compressor"
			must(globals.Context.SetBoolName(&local, &key, enabled))
			key = "datalog.similarity_compressor_threshold"
			must(globals.Context.SetUIntName(&local, &key, 2))
			must(e.SetQueryParameters(&local))
			must(globals.Context.Del(&local))
			domainName, err := m.Symbols.Intern("similarity-order-domain")
			must(err)
			domain, err := m.MkDatalogFiniteSort(domainName, 4)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			declare := func(name string, arity int) ast.ASTHandle {
				n, err := m.Symbols.Intern("similarity-order-" + name)
				must(err)
				sorts := make([]ast.ASTHandle, arity)
				for i := range sorts {
					sorts[i] = domain
				}
				d, err := m.MkFuncDecl(n, sorts, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				return d
			}
			p, r, goal := declare("P", 2), declare("R", 1), declare("Goal", 1)
			for _, row := range [][]uint32{{0, 0}, {1, 1}, {2, 1}, {3, 2}} {
				must(e.AddTableFact(p, row))
			}
			x, err := m.MkVar(0, domain)
			must(err)
			ra, err := m.MkApp(r, []ast.ASTHandle{x})
			must(err)
			for c := uint64(0); c < 3; c++ {
				value, err := m.MkDatalogNumeral(c, domain)
				must(err)
				body, err := m.MkApp(p, []ast.ASTHandle{value, x})
				must(err)
				rule, err := m.MkImplies(body, ra)
				must(err)
				must(e.AddRule(rule, ""))
			}
			ga, err := m.MkApp(goal, []ast.ASTHandle{x})
			must(err)
			rule, err := m.MkImplies(ra, ga)
			must(err)
			must(e.AddRule(rule, ""))
			for query := 0; query < 2; query++ {
				if query == 1 {
					must(e.AddTableFact(p, []uint32{2, 2}))
				}
				status, err := e.QueryPredicate(goal)
				must(err)
				if status != lbool.True || !e.compiled {
					t.Fatal("similarity query did not compile", status, e.compiled)
				}
				rows, err := e.predicates[goal].table.Facts()
				must(err)
				seen := [4]bool{}
				for _, row := range rows {
					if len(row) != 1 || row[0] > 3 {
						t.Fatal("unexpected output tuple", row)
					}
					seen[row[0]] = true
				}
				if len(rows) != 2+query || seen != [4]bool{true, true, query == 1, false} {
					t.Fatal("constant membership changed query", rows)
				}
				joins := uint32(0)
				for i := 0; i < e.datalogStats.Size(); i++ {
					if e.datalogStats.Key(i) == "dl.join_project" {
						joins += e.datalogStats.UintValue(i)
					}
				}
				if (joins != 0) != enabled {
					t.Fatal("similarity membership join was lost before filtering", enabled, joins)
				}
				count, err := e.set.Len()
				must(err)
				if count != 4 || len(e.predicates) != 3 || len(e.auxiliary) != 0 {
					t.Fatal("ordered transformations escaped query scope", count, len(e.predicates), len(e.auxiliary))
				}
			}
		})
	}
}
