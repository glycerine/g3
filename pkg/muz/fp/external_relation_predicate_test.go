package fp_test

// Source: rel_context relation-manager dispatch and
// external_relation_plugin::mk_empty.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointExternalPredicateRelationDispatch(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	emptyCalls := 0
	fpMust(t, engine.SetExternalRelationCallbacks(rel.ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, err := manager.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			if kind == ast.DatalogOpEmpty {
				emptyCalls++
			}
			return nil
		},
	}))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("fixedpoint-external-predicate")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	if _, err = engine.ExternalPredicateRelation(predicate); err == nil {
		t.Fatal("unregistered predicate received an external relation")
	}
	fpMust(t, engine.RegisterPredicate(predicate))
	first, err := engine.ExternalPredicateRelation(predicate)
	fpMust(t, err)
	second, err := engine.ExternalPredicateRelation(predicate)
	fpMust(t, err)
	if first != second || emptyCalls != 1 {
		t.Fatal("fixedpoint dispatch did not reuse the manager relation", first == second, emptyCalls)
	}
	fpMust(t, engine.Close())
	if _, err = first.Relation(); err == nil {
		t.Fatal("fixedpoint close left manager-owned external relation open")
	}
	fpMust(t, context.Close())
}

func TestFixedpointExternalPredicateFactsAndQueries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	nonempty := make(map[ast.ASTHandle]bool)
	fpMust(t, engine.SetExternalRelationCallbacks(rel.ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, err := manager.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			switch kind {
			case ast.DatalogOpEmpty:
				nonempty[outputs[0]] = false
			case ast.DatalogOpStore:
				nonempty[outputs[0]] = true
			default:
				t.Fatalf("unexpected assignment operation %d", kind)
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, err := manager.GetDeclKind(declaration)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if kind != ast.DatalogOpIsEmpty || len(arguments) != 1 {
				t.Fatalf("unexpected reduction operation %d", kind)
			}
			if nonempty[arguments[0]] {
				return manager.MkFalse()
			}
			return manager.MkTrue()
		},
	}))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	makePredicate := func(name string) ast.ASTHandle {
		symbol, err := manager.Symbols.Intern(name)
		fpMust(t, err)
		declaration, err := manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, err)
		fpMust(t, engine.RegisterPredicate(declaration))
		fpMust(t, engine.SetPredicateRepresentation(declaration, []string{"external_relation"}))
		return declaration
	}
	p := makePredicate("external-fact-p")
	q := makePredicate("external-fact-q")
	status, err := engine.QueryPredicate(q)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("empty external predicate query", status)
	}
	truth, err := manager.MkTrue()
	fpMust(t, err)
	fpMust(t, engine.AddFact(p, []ast.ASTHandle{truth}))
	status, err = engine.QueryPredicate(p)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("stored external predicate query", status)
	}
	fpMust(t, engine.AddTableFact(q, []uint32{1}))
	status, err = engine.QueryPredicates([]ast.ASTHandle{q, p})
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("mixed external predicate query", status)
	}
	answer, err := engine.Answer(p)
	fpMust(t, err)
	relation, err := engine.ExternalPredicateRelation(p)
	fpMust(t, err)
	want, err := relation.Relation()
	fpMust(t, err)
	got, err := manager.RefGet(answer)
	fpMust(t, err)
	if got != want {
		t.Fatal("external query answer is not its relation expression", got, want)
	}
	fpMust(t, manager.DelRef(&answer))
	fpMust(t, engine.Close())
	fpMust(t, context.Close())
}

func TestFixedpointExternalGroundAndIdentityRuleExecution(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	nonempty := make(map[ast.ASTHandle]bool)
	stores, clones, unions := 0, 0, 0
	fpMust(t, engine.SetExternalRelationCallbacks(rel.ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, err := manager.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			switch kind {
			case ast.DatalogOpEmpty:
				nonempty[outputs[0]] = false
			case ast.DatalogOpStore:
				stores++
				nonempty[outputs[0]] = true
			case ast.DatalogOpClone:
				clones++
				nonempty[outputs[0]] = nonempty[arguments[0]]
			case ast.DatalogOpUnion:
				unions++
				nonempty[outputs[0]] = nonempty[outputs[0]] || nonempty[arguments[1]]
			default:
				t.Fatalf("unexpected assignment operation %d", kind)
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, err := manager.GetDeclKind(declaration)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if kind != ast.DatalogOpIsEmpty {
				t.Fatalf("unexpected reduction operation %d", kind)
			}
			if nonempty[arguments[0]] {
				return manager.MkFalse()
			}
			return manager.MkTrue()
		},
	}))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	makePredicate := func(name string, external bool) ast.ASTHandle {
		symbol, err := manager.Symbols.Intern(name)
		fpMust(t, err)
		declaration, err := manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, err)
		fpMust(t, engine.RegisterPredicate(declaration))
		if external {
			fpMust(t, engine.SetPredicateRepresentation(declaration, []string{"external_relation"}))
		}
		return declaration
	}
	p := makePredicate("external-rule-p", true)
	q := makePredicate("external-rule-q", true)
	truth, err := manager.MkTrue()
	fpMust(t, err)
	pFact, err := manager.MkApp(p, []ast.ASTHandle{truth})
	fpMust(t, err)
	fpMust(t, engine.AddRule(pFact, "external-fact"))
	x, err := manager.MkVar(0, boolean)
	fpMust(t, err)
	px, err := manager.MkApp(p, []ast.ASTHandle{x})
	fpMust(t, err)
	qx, err := manager.MkApp(q, []ast.ASTHandle{x})
	fpMust(t, err)
	identity, err := manager.MkImplies(px, qx)
	fpMust(t, err)
	fpMust(t, engine.AddRule(identity, "external-identity"))
	for attempt := 0; attempt < 2; attempt++ {
		status, err := engine.QueryPredicate(q)
		fpMust(t, err)
		if status != lbool.True {
			t.Fatal("external rule query", attempt, status)
		}
	}
	if stores != 2 || clones == 0 || unions == 0 {
		t.Fatal("external rule operations", stores, clones, unions)
	}

	finite := makePredicate("external-rule-finite", false)
	finiteX, err := manager.MkApp(finite, []ast.ASTHandle{x})
	fpMust(t, err)
	mixed, err := manager.MkImplies(qx, finiteX)
	fpMust(t, err)
	fpMust(t, engine.AddRule(mixed, "mixed-representation"))
	if _, err = engine.QueryPredicate(q); err == nil {
		t.Fatal("mixed finite/external rule program was silently reinterpreted")
	}
	fpMust(t, engine.Close())
	fpMust(t, context.Close())
}
