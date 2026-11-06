package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestExplanationFormulaScopeFiniteSortsExplicitBypassAndLifetime(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "favourite", true: "explicit"}[explicit], func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.check_relation", "tr_hashtable"))
			boolean, err := m.MkBoolSort()
			fragmentMust(t, err)
			bv, err := m.BitVectorSort(2)
			fragmentMust(t, err)
			name, err := m.Symbols.Intern("formula-scope-mixed")
			fragmentMust(t, err)
			p, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, bv}, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			fragmentMust(t, e.RegisterPredicate(p))
			if explicit {
				for _, d := range e.predicateOrder {
					fragmentMust(t, e.SetPredicateRepresentation(d, []string{"tr_hashtable"}))
				}
			}
			fragmentMust(t, e.AddTableFact(p, []uint32{1, 3}))
			fragmentMust(t, e.AddRule(f.applications["q"], "formula-scope-rule"))
			scope, enabled, err := e.beginDatalogExplanationFormulaScope()
			fragmentMust(t, err)
			if !enabled || scope == nil {
				t.Fatal("formula scope inactive")
			}
			transformer, err := rel.NewExplanationTransformer(m, e.rules)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, transformer.Close()) }()
			target, err := transformer.TransformRulesOnly(e.set)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, target.Close()) }()
			fragmentMust(t, scope.transferFacts(transformer, e.set, target))
			product, found, err := transformer.RelationProduct(p)
			fragmentMust(t, err)
			if !found {
				t.Fatal("missing explained product")
			}
			checked, err := product.FormulaChecked()
			fragmentMust(t, err)
			if checked == explicit {
				t.Fatal("favourite checker ignored explicit representation", checked, explicit)
			}
			fragmentMust(t, scope.requireProduct(p, product))
			if err := scope.requireProduct(ast.ASTHandle{}, product); !errors.Is(err, rel.ErrRelationVerification) {
				t.Fatal("unregistered checker expectation silently bypassed", err)
			}
			fragmentMust(t, scope.Close())
			fragmentMust(t, scope.Close())
			truth, err := m.MkTrue()
			fragmentMust(t, err)
			three, err := m.BitVectorNumeralUnsigned(3, 2)
			fragmentMust(t, err)
			explanation, found, err := product.Explanation([]ast.ASTHandle{truth, three})
			fragmentMust(t, err)
			if !found || explanation.IsNull() {
				t.Fatal("mixed-sort fact/proof lost after scope retirement")
			}
			if err := scope.requireProduct(p, product); !errors.Is(err, rel.ErrCompilerUnsupported) {
				t.Fatal("retired scope admitted verification", err)
			}
		})
	}
}

func TestExplanationFormulaScopeDispatchesSelectedBacking(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	scope, enabled, err := e.beginDatalogExplanationFormulaScope()
	fragmentMust(t, err)
	if scope != nil || enabled {
		t.Fatal("default unexpectedly installs formula checker")
	}
	for _, name := range []string{"tr_sparse", "tr_lazy_sparse"} {
		fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.check_relation", name))
		scope, enabled, err = e.beginDatalogExplanationFormulaScope()
		if err != nil || scope == nil || !enabled {
			t.Fatal("selected finite checker was not dispatched", name, scope, enabled, err)
		}
		want := "sparse"
		if name == "tr_lazy_sparse" {
			want = "lazy_sparse"
		}
		if scope.backing != want {
			t.Fatal("selected finite backing changed", name, scope.backing, want)
		}
		fragmentMust(t, scope.Close())
	}
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.check_relation", "doc"))
	scope, enabled, err = e.beginDatalogExplanationFormulaScope()
	fragmentMust(t, err)
	if !enabled || scope == nil || scope.backing != "doc" {
		t.Fatal("selected checked UDoc backing was not dispatched", scope, enabled)
	}
	factory, found, err := e.relations.RelationExplanationUDocFactory()
	fragmentMust(t, err)
	if !found || !factory.FormulaChecked() {
		t.Fatal("selected doc explanation factory is not formula checked", found, factory)
	}
	fragmentMust(t, scope.Close())
}
