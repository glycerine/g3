package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogBoundSimilarityPublicQueriesAndUpdates(t *testing.T) {
	for _, favourite := range []string{"bound_relation", "pentagon"} {
		for _, mode := range []string{"enabled", "disabled", "strict-threshold", "queried-output"} {
			t.Run(favourite+"/"+mode, func(t *testing.T) {
				m, globals := newPDRLifecycleServices(t)
				must := func(err error) { t.Helper(); fragmentMust(t, err) }
				e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
				must(err)
				defer func() { must(e.Close()) }()
				must(e.SelectEngine("datalog"))
				must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", favourite))
				must(e.globals.SourceSetName("fixedpoint.datalog.similarity_compressor_threshold", "2"))
				if mode == "disabled" {
					must(e.globals.SourceSetName("fixedpoint.datalog.similarity_compressor", "false"))
				} else if mode == "strict-threshold" {
					must(e.globals.SourceSetName("fixedpoint.datalog.similarity_compressor_threshold", "3"))
				}
				family, err := m.RegisterArithmeticPlugin()
				must(err)
				integer, err := m.MkFamilySort(family, 1, nil)
				must(err)
				boolean, err := m.MkBoolSort()
				must(err)
				declare := func(name string) ast.ASTHandle {
					symbol, err := m.Symbols.Intern(name)
					must(err)
					declaration, err := m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
					must(err)
					must(e.RegisterPredicate(declaration))
					return declaration
				}
				p, q := declare("public-bound-similarity-P"), declare("public-bound-similarity-Q")
				addRule := func(text string) {
					t.Helper()
					value, err := m.ArithmeticIntegerNumeralText(text)
					must(err)
					atom, err := m.MkApp(p, []ast.ASTHandle{value})
					must(err)
					must(e.addRuleEager(atom, "bound-source"))
				}
				for _, text := range []string{"0", "2", "4"} {
					addRule(text)
				}
				x, err := m.MkVar(0, integer)
				must(err)
				body, err := m.MkApp(p, []ast.ASTHandle{x})
				must(err)
				head, err := m.MkApp(q, []ast.ASTHandle{x})
				must(err)
				step, err := m.MkImplies(body, head)
				must(err)
				must(e.addRuleEager(step, "bound-step"))
				query := q
				if mode == "queried-output" {
					query = p
				}
				originalPredicates := len(e.predicates)
				for iteration := 0; iteration < 3; iteration++ {
					if iteration == 2 {
						addRule("6")
					}
					status, err := e.QueryPredicate(query)
					must(err)
					if status != lbool.Undef {
						t.Fatal("nonempty bound approximation changed query precision", status)
					}
					if len(e.predicates) != originalPredicates || len(e.auxiliary) != 0 || len(e.boundDatalog.entries) != originalPredicates {
						t.Fatal("query-local bound auxiliaries escaped into original state")
					}
					program := e.boundDatalog.similarity
					wantMerge := mode == "enabled" || mode == "strict-threshold" && iteration == 2
					if mode == "disabled" {
						if program != nil {
							t.Fatal("disabled compressor produced a program")
						}
					} else if program == nil || program.Modified() != wantMerge || (len(program.AuxiliaryPredicates()) == 1) != wantMerge {
						t.Fatal("bound compressor option/threshold/output guard", program, wantMerge)
					}
					if wantMerge {
						seed, found := program.AuxiliaryRelation(program.AuxiliaryPredicates()[0])
						if !found || seed.Empty() {
							t.Fatal("compressed constants did not become bound facts")
						}
						strict, weak, err := seed.Bounds(0)
						must(err)
						if len(strict)+len(weak) != 0 {
							t.Fatal("bound facts acquired constraints absent in source", strict, weak)
						}
					} else if program != nil {
						rules, err := program.Rules()
						must(err)
						for i, want := range []string{"0", "2", "4"} {
							rule, err := rules.Rule(i)
							must(err)
							head, err := rule.Head()
							must(err)
							constant, err := m.GetArg(head, 0)
							must(err)
							text, _, matched, err := m.ArithmeticNumeralText(constant)
							must(err)
							must(rule.Close())
							if !matched || text != want {
								t.Fatal("unchanged bound program reordered source rules", i, text, want)
							}
						}
					}
					if count, err := e.set.Len(); err != nil || count != 4+iteration/2 {
						t.Fatal("query replaced original bound rules", count, err)
					}
					answer, err := e.Answer(query)
					must(err)
					must(e.boundDatalog.discardSimilarity())
					root, err := m.RefGet(answer)
					must(err)
					if !m.IsTrue(root) {
						t.Fatal("bound answer lost source coarse fact semantics")
					}
					must(m.DelRef(&answer))
				}
			})
		}
	}
}
