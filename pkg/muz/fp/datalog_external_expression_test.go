package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogExternalFavouriteHonorsFiniteQueryKind(t *testing.T) {
	for _, kind := range []string{"tr_hashtable", "tr_sparse"} {
		t.Run(kind, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(g.SourceSetName("fixedpoint.datalog.default_relation", "external_relation"))
			must(e.SelectEngine("datalog"))
			emptyCalls := 0
			must(e.SetExternalRelationCallbacks(rel.ExternalRelationCallbacks{ReduceAssign: func(d ast.ASTHandle, args, outputs []ast.ASTHandle) error {
				op, err := m.GetDeclKind(d)
				if err != nil {
					return err
				}
				if op == ast.DatalogOpEmpty {
					emptyCalls++
				}
				return nil
			}}))
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("external-favourite-explicit-finite")
			must(err)
			p, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			must(e.SetPredicateRepresentation(p, []string{kind}))
			otherName, err := m.Symbols.Intern("external-favourite-other-finite")
			must(err)
			other, err := m.MkFuncDecl(otherName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(other))
			otherKind := "tr_sparse"
			if kind == "tr_sparse" {
				otherKind = "tr_hashtable"
			}
			must(e.SetPredicateRepresentation(other, []string{otherKind}))
			must(e.AddTableFact(other, []uint32{1}))
			must(e.AddTableFact(p, []uint32{1}))
			x, err := m.MkVar(0, boolean)
			must(err)
			query, err := m.MkApp(p, []ast.ASTHandle{x})
			must(err)
			queryRef, err := m.NewRef(query)
			must(err)
			result, err := e.QueryExpression(query)
			must(err)
			must(m.DelRef(&queryRef))
			if result != lbool.True || emptyCalls != 0 {
				t.Fatal("external favourite replaced valid finite query kind", result, emptyCalls)
			}
			inherited := e.queryClone.predicates[e.queryClonePredicate]
			if !inherited.explicitFiniteRepresentation || len(inherited.requestedRepresentations) != 1 || inherited.requestedRepresentations[0] != kind {
				t.Fatal("finite query kind not inherited")
			}
			if kind == "tr_hashtable" {
				if _, ok := inherited.table.(*rel.HashTable); !ok {
					t.Fatalf("Hash query became %T", inherited.table)
				}
			}
			otherQuery, err := m.MkApp(other, []ast.ASTHandle{x})
			must(err)
			combined, err := m.MkAnd(query, otherQuery)
			must(err)
			combinedRef, err := m.NewRef(combined)
			must(err)
			result, err = e.QueryExpression(combined)
			must(err)
			must(m.DelRef(&combinedRef))
			if result != lbool.True {
				t.Fatal("mixed finite query", result)
			}
			names := e.queryClone.predicates[e.queryClonePredicate].requestedRepresentations
			if len(names) != 1 || names[0] != otherKind {
				t.Fatal("query did not use source swap-last conjunction order", names, otherKind)
			}
			if kind == "tr_sparse" {
				truth, err := m.MkTrue()
				must(err)
				ground, err := m.MkApp(p, []ast.ASTHandle{truth})
				must(err)
				groundRef, err := m.NewRef(ground)
				must(err)
				result, err = e.QueryExpression(ground)
				must(m.DelRef(&groundRef))
				// The nullary query head inherits tr_sparse, which cannot
				// handle it: mk_empty_relation gives it the external
				// favourite's relation. The fixed fork's C API (default
				// relation external_relation, Z3_fixedpoint_init,
				// Z3_fixedpoint_query of (p true)) makes two external
				// relations (OP_RA_EMPTY) and answers sat.
				if err != nil || result != lbool.True || emptyCalls != 2 {
					t.Fatal("incompatible sparse nullary did not fall through to the external favourite", err, result, emptyCalls)
				}
			}
		})
	}
}
