package substitution

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type collectingTreeVisitor struct {
	subst *Substitution
	found []ast.ASTHandle
	stop  int
	check func(ast.ASTHandle, *Substitution) error
}

func (v *collectingTreeVisitor) TreeSubstitution() *Substitution { return v.subst }
func (v *collectingTreeVisitor) VisitTreeExpression(h ast.ASTHandle) (bool, error) {
	if v.check != nil {
		if e := v.check(h, v.subst); e != nil {
			return false, e
		}
	}
	v.found = append(v.found, h)
	return v.stop == 0 || len(v.found) < v.stop, nil
}

func TestSubstitutionTreeRetainedUnifyBacktrackingBindingsAndStop(t *testing.T) {
	m := offsetManager(t)
	sort, e := m.MkUninterpretedSort(symbol.Numerical(7400), nil)
	if e != nil {
		t.Fatal(e)
	}
	substPin(t, m, sort)
	decl := func(id uint32, arity int) ast.ASTHandle {
		ds := make([]ast.ASTHandle, arity)
		for i := range ds {
			ds[i] = sort
		}
		d, e := m.MkFuncDecl(symbol.Numerical(id), ds, sort, ast.ASTFuncDeclInfo{})
		if e != nil {
			t.Fatal(e)
		}
		return substPin(t, m, d)
	}
	f := decl(7401, 2)
	constant := func(id uint32) ast.ASTHandle {
		d := decl(id, 0)
		h, e := m.MkApp(d, nil)
		if e != nil {
			t.Fatal(e)
		}
		return substPin(t, m, h)
	}
	a, b, c := constant(7402), constant(7403), constant(7404)
	v := func(i uint32) ast.ASTHandle {
		h, e := m.MkVar(i, sort)
		if e != nil {
			t.Fatal(e)
		}
		return substPin(t, m, h)
	}
	x, y := v(0), v(1)
	app := func(args ...ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(f, args)
		if e != nil {
			t.Fatal(e)
		}
		return substPin(t, m, h)
	}
	one, two, three := app(x, a), app(b, y), app(c, c)
	tree, e := NewSubstitutionTree(m)
	if e != nil {
		t.Fatal(e)
	}
	defer tree.Close()
	for _, h := range []ast.ASTHandle{one, two, three} {
		if e = tree.Insert(h); e != nil {
			t.Fatal(e)
		}
	}
	s, e := New(m)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Reserve(3, 16); e != nil {
		t.Fatal(e)
	}
	query := app(b, a)
	visitor := &collectingTreeVisitor{subst: s, check: func(h ast.ASTHandle, s *Substitution) error {
		switch h {
		case one:
			r, ok, e := s.Find(0, 1)
			if e != nil {
				return e
			}
			if !ok || r != NewExprOffset(b, 0) {
				t.Fatalf("first binding %v %v", r, ok)
			}
		case two:
			r, ok, e := s.Find(1, 1)
			if e != nil {
				return e
			}
			if !ok || r != NewExprOffset(a, 0) {
				t.Fatalf("second binding %v %v", r, ok)
			}
		}
		return nil
	}}
	if e = tree.Unify(query, visitor); e != nil {
		t.Fatal(e)
	}
	if len(visitor.found) != 2 {
		t.Fatal("unifier count", visitor.found)
	}
	visitor.found = nil
	visitor.stop = 1
	if e = tree.Unify(query, visitor); e != nil {
		t.Fatal(e)
	}
	if len(visitor.found) != 1 {
		t.Fatal("visitor stop", visitor.found)
	}
}

func TestSubstitutionTreeVariableCandidatesAndOffsetGuards(t *testing.T) {
	m := offsetManager(t)
	sort, e := m.MkBoolSort()
	if e != nil {
		t.Fatal(e)
	}
	v0, e := m.MkVar(0, sort)
	if e != nil {
		t.Fatal(e)
	}
	v1, e := m.MkVar(1, sort)
	if e != nil {
		t.Fatal(e)
	}
	substPin(t, m, v0)
	substPin(t, m, v1)
	tree, e := NewSubstitutionTree(m)
	if e != nil {
		t.Fatal(e)
	}
	defer tree.Close()
	if e = tree.Insert(v0); e != nil {
		t.Fatal(e)
	}
	s, e := New(m)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Reserve(3, 8); e != nil {
		t.Fatal(e)
	}
	visitor := &collectingTreeVisitor{subst: s}
	if e = tree.Unify(v1, visitor); e != nil {
		t.Fatal(e)
	}
	if len(visitor.found) != 1 || visitor.found[0] != v0 {
		t.Fatal("variable candidate", visitor.found)
	}
	small, e := New(m)
	if e != nil {
		t.Fatal(e)
	}
	defer small.Close()
	if e = small.Reserve(2, 8); e != nil {
		t.Fatal(e)
	}
	if e = tree.Unify(v1, &collectingTreeVisitor{subst: small}); e == nil {
		t.Fatal("offset capacity guard")
	}
}

// substitution_tree::visit stops the variable query's loop over the roots
// once the visitor asks to stop (AUDIT ast_sub_03-F4).
func TestSubstitutionTreeVarQueryStopsAcrossRoots(t *testing.T) {
	m := offsetManager(t)
	sort, e := m.MkUninterpretedSort(symbol.Numerical(7500), nil)
	if e != nil {
		t.Fatal(e)
	}
	substPin(t, m, sort)
	decl := func(id uint32, arity int) ast.ASTHandle {
		ds := make([]ast.ASTHandle, arity)
		for i := range ds {
			ds[i] = sort
		}
		d, e := m.MkFuncDecl(symbol.Numerical(id), ds, sort, ast.ASTFuncDeclInfo{})
		if e != nil {
			t.Fatal(e)
		}
		return substPin(t, m, d)
	}
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(d, args)
		if e != nil {
			t.Fatal(e)
		}
		return substPin(t, m, h)
	}
	f, g := decl(7501, 1), decl(7502, 1)
	a, b := app(decl(7503, 0)), app(decl(7504, 0))
	tree, e := NewSubstitutionTree(m)
	if e != nil {
		t.Fatal(e)
	}
	defer tree.Close()
	for _, h := range []ast.ASTHandle{app(f, a), app(g, b)} {
		if e = tree.Insert(h); e != nil {
			t.Fatal(e)
		}
	}
	s, e := New(m)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Reserve(3, 16); e != nil {
		t.Fatal(e)
	}
	x, e := m.MkVar(0, sort)
	if e != nil {
		t.Fatal(e)
	}
	substPin(t, m, x)
	visitor := &collectingTreeVisitor{subst: s}
	if e = tree.Unify(x, visitor); e != nil {
		t.Fatal(e)
	}
	if len(visitor.found) != 2 {
		t.Fatal("unifier count", visitor.found)
	}
	visitor.found = nil
	visitor.stop = 1
	if e = tree.Unify(x, visitor); e != nil {
		t.Fatal(e)
	}
	if len(visitor.found) != 1 {
		t.Fatal("visitor stop", visitor.found)
	}
}
