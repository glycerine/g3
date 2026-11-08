package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogRelationExplanationFormulaCheckExplicitNegativeBoundary(t *testing.T) {
	for _, test := range []struct {
		name           string
		representation string
		nullary        bool
	}{
		{name: "hash", representation: "tr_hashtable"},
		{name: "sparse", representation: "tr_sparse"},
		{name: "nullary-hash", representation: "tr_hashtable", nullary: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
			must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
			must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "hashtable"))
			must(e.globals.SourceSetName("fixedpoint.datalog.unbound_compressor", "false"))

			boolean, err := m.MkBoolSort()
			must(err)
			declare := func(text string, domain []ast.ASTHandle) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(text)
				must(err)
				declaration, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(declaration))
				return declaration
			}
			p := declare("formula-negative-input-"+test.name, []ast.ASTHandle{boolean})
			q := declare("formula-negative-output-"+test.name, []ast.ASTHandle{boolean})
			negativeDomain := []ast.ASTHandle{boolean}
			if test.nullary {
				negativeDomain = nil
			}
			n := declare("formula-negative-explicit-"+test.name, negativeDomain)
			must(e.SetPredicateRepresentation(n, []string{test.representation}))

			truth, err := m.MkTrue()
			must(err)
			truthRef, err := m.NewRef(truth)
			must(err)
			defer func() { must(m.DelRef(&truthRef)) }()
			falsehood, err := m.MkFalse()
			must(err)
			falsehoodRef, err := m.NewRef(falsehood)
			must(err)
			defer func() { must(m.DelRef(&falsehoodRef)) }()
			must(e.AddFact(p, []ast.ASTHandle{truth}))
			must(e.AddFact(p, []ast.ASTHandle{falsehood}))
			if !test.nullary {
				must(e.AddFact(n, []ast.ASTHandle{truth}))
			}

			x, err := m.MkVar(0, boolean)
			must(err)
			app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
				application, err := m.MkApp(declaration, arguments)
				must(err)
				return application
			}
			var negativeApplication ast.ASTHandle
			if test.nullary {
				negativeApplication = app(n)
			} else {
				negativeApplication = app(n, x)
			}
			notNegative, err := m.MkNot(negativeApplication)
			must(err)
			body, err := m.MkAnd(app(p, x), notNegative)
			must(err)
			rule, err := m.MkImplies(body, app(q, x))
			must(err)
			proofName := "formula-negative-proof-" + test.name
			must(e.addRuleEager(rule, proofName))

			positiveValue := falsehood
			if test.nullary {
				positiveValue = truth
			}
			positiveQuery, err := m.NewRef(app(q, positiveValue))
			must(err)
			positiveRoot, err := m.RefGet(positiveQuery)
			must(err)
			status, err := e.QueryExpression(positiveRoot)
			must(m.DelRef(&positiveQuery))
			must(err)
			if status != lbool.True {
				t.Fatal("explicit negative boundary blocked expected tuple", status)
			}
			answer, err := e.ExpressionAnswer()
			must(err)
			root, err := m.RefGet(answer)
			must(err)
			if !datalogExplanationFormulaNames(t, m, root)[proofName] {
				t.Fatal("formula-checked negative result lost named proof")
			}
			must(m.DelRef(&answer))

			if test.nullary {
				must(e.AddFact(n, nil))
			}
			blockedValue := truth
			blockedQuery, err := m.NewRef(app(q, blockedValue))
			must(err)
			blockedRoot, err := m.RefGet(blockedQuery)
			must(err)
			status, err = e.QueryExpression(blockedRoot)
			must(m.DelRef(&blockedQuery))
			must(err)
			if status != lbool.False {
				t.Fatal("explicit negative boundary admitted blocked tuple", status)
			}
			answer, err = e.ExpressionAnswer()
			must(err)
			root, err = m.RefGet(answer)
			must(err)
			if root != falsehood {
				t.Fatal("blocked negative query published nonfalse explanation")
			}
			must(m.DelRef(&answer))

			switch table := e.predicates[n].table.(type) {
			case *rel.HashTable:
				if test.representation != "tr_hashtable" {
					t.Fatalf("explicit negative lost sparse representation: %T", table)
				}
			case *rel.SparseTable:
				if test.representation != "tr_sparse" || test.nullary {
					t.Fatalf("unexpected sparse negative representation: %T", table)
				}
			default:
				t.Fatalf("explicit negative became checked target: %T", table)
			}
		})
	}
}
