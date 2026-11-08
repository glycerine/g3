package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogIntervalSimilarityPublicQueriesAndUpdates(t *testing.T) {
	for _, mode := range []string{"enabled", "disabled", "strict-threshold", "queried-output"} {
		t.Run(mode, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
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
			p, q := declare("public-interval-similarity-P"), declare("public-interval-similarity-Q")
			addRule := func(text string) {
				t.Helper()
				value, err := m.ArithmeticIntegerNumeralText(text)
				must(err)
				atom, err := m.MkApp(p, []ast.ASTHandle{value})
				must(err)
				must(e.addRuleEager(atom, "arithmetic-source"))
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
			must(e.addRuleEager(step, "arithmetic-step"))
			seed, err := m.ArithmeticIntegerNumeralText("-2")
			must(err)
			seedRef, err := m.NewRef(seed)
			must(err)
			defer func() { must(m.DelRef(&seedRef)) }()
			must(e.AddFact(p, []ast.ASTHandle{seed}))
			query := q
			if mode == "queried-output" {
				query = p
			}
			originalPredicates := len(e.predicates)
			for iteration := 0; iteration < 3; iteration++ {
				upper := "4/1"
				if iteration == 2 {
					addRule("6")
					upper = "6/1"
				}
				status, err := e.QueryPredicate(query)
				must(err)
				if status != lbool.Undef {
					t.Fatal("nonempty interval approximation changed query precision", status)
				}
				if len(e.predicates) != originalPredicates || len(e.auxiliary) != 0 {
					t.Fatal("arithmetic query published query-local similarity predicates")
				}
				program := e.intervalDatalog.similarity
				wantMerge := mode == "enabled" || mode == "strict-threshold" && iteration == 2
				if mode == "disabled" {
					if program != nil {
						t.Fatal("disabled compressor still produced a program")
					}
				} else if program == nil || program.Modified() != wantMerge || (len(program.AuxiliaryPredicates()) == 1) != wantMerge {
					t.Fatal("arithmetic compressor option/threshold/output guard", program, wantMerge)
				}
				result, err := e.intervalDatalog.relation(query)
				must(err)
				lower, loOpen, high, hiOpen, err := result.Bounds(0)
				must(err)
				if lower == nil || high == nil || lower.String() != "-2/1" || high.String() != upper || loOpen || hiOpen {
					t.Fatal("arithmetic similarity lost rules, seed or update", lower, high, upper)
				}
				if count, err := e.set.Len(); err != nil || count != 4+iteration/2 {
					t.Fatal("query replaced original arithmetic rules", count, err)
				}
				answer, err := e.Answer(query)
				must(err)
				must(e.intervalDatalog.discardEvaluator())
				if _, err = m.RefGet(answer); err != nil {
					t.Fatal("owning arithmetic answer died with similarity program", err)
				}
				must(m.DelRef(&answer))
			}
		})
	}
}
