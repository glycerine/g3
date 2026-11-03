package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogExternalDefaultExpressionCloneOwnsFacts(t *testing.T) {
	for _, arithmetic := range []bool{false, true} {
		t.Run(fmt.Sprint(arithmetic), func(t *testing.T) {
			testDatalogExternalExpressionClone(t, arithmetic)
		})
	}
}
func testDatalogExternalExpressionClone(t *testing.T, arithmetic bool) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "external_relation"))
	must(e.SelectEngine("datalog"))
	nonempty := make(map[ast.ASTHandle]bool)
	clones := 0
	must(e.SetExternalRelationCallbacks(rel.ExternalRelationCallbacks{
		ReduceAssign: func(d ast.ASTHandle, args, outputs []ast.ASTHandle) error {
			op, err := m.GetDeclKind(d)
			if err != nil {
				return err
			}
			switch op {
			case ast.DatalogOpEmpty:
				nonempty[outputs[0]] = false
			case ast.DatalogOpStore:
				nonempty[outputs[0]] = true
			case ast.DatalogOpClone:
				clones++
				nonempty[outputs[0]] = nonempty[args[0]]
			case ast.DatalogOpUnion:
				changed := nonempty[args[1]] && !nonempty[outputs[0]]
				nonempty[outputs[0]] = nonempty[outputs[0]] || nonempty[args[1]]
				if len(outputs) > 1 {
					nonempty[outputs[1]] = changed
				}
			default:
				t.Fatalf("unexpected external expression operation %v", op)
			}
			return nil
		},
		Reduce: func(d ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
			op, err := m.GetDeclKind(d)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if op != ast.DatalogOpIsEmpty {
				t.Fatalf("unexpected external expression reduction %v", op)
			}
			if nonempty[args[0]] {
				return m.MkFalse()
			}
			return m.MkTrue()
		},
	}))
	boolean, err := m.MkBoolSort()
	must(err)
	argumentSort := boolean
	if arithmetic {
		family, err := m.RegisterArithmeticPlugin()
		must(err)
		argumentSort, err = m.MkFamilySort(family, 1, nil)
		must(err)
	}

	declare := func(name string) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(n, []ast.ASTHandle{argumentSort}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("external-expression-P"), declare("external-expression-Q")
	untouched := declare("external-expression-untouched")
	must(e.AddTableFact(p, []uint32{1}))
	x, err := m.MkVar(0, argumentSort)
	must(err)
	query, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	derived, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(query, derived)
	must(err)
	must(e.AddRule(rule, "external-expression-copy"))
	queryRef, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&queryRef)) }()
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True || clones == 0 {
		t.Fatal("default external expression lost callback facts", status, clones)
	}
	derivedEmpty, err := e.predicates[q].external.Empty()
	must(err)
	if derivedEmpty {
		t.Fatal("expression search did not publish original external result")
	}
	clone := e.queryClone
	if clone == nil || clone.external == nil || clone.external == e.external || clone.predicates[p].external == e.predicates[p].external {
		t.Fatal("external clone shares owners")
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	truth, err := m.MkDatalogNumeral(1, argumentSort)
	must(err)
	must(clone.predicates[untouched].external.AddFact([]ast.ASTHandle{truth}))
	empty, err := e.predicates[untouched].external.Empty()
	must(err)
	if !empty {
		t.Fatal("query clone mutation changed parent external relation")
	}
	must(e.clearQueryResources())
	if _, err = m.RefGet(answer); err != nil {
		t.Fatal("held external answer lost clone-owned expression", err)
	}
	must(m.DelRef(&answer))
}

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
				_, err = e.QueryExpression(ground)
				must(m.DelRef(&groundRef))
				if !errors.Is(err, ErrPropositionalHorn) || emptyCalls == 0 {
					t.Fatal("incompatible sparse nullary did not fall through external favourite before mixed-plugin guard", err, emptyCalls)
				}
			}
		})
	}
}
