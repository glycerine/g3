package fp

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source horn_tactic.cpp check_predicate treats theory atoms and predicate
// arguments as opaque, and visits only the branches of an ITE.
func TestHornTacticPredicateRegistrationBoundaries(t *testing.T) {
	for _, mode := range []string{"predicate-argument", "theory-equality", "ite-condition", "connectives"} {
		t.Run(mode, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			horn, err := NewHornTactic(m, g.Context, g)
			fragmentMust(t, err)
			fragmentMust(t, horn.IncRef())
			defer func() { fragmentMust(t, horn.DecRef()) }()
			engine, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, engine.Close()) }()
			var refs []ast.ASTRef
			defer func() {
				for i := range refs {
					fragmentMust(t, m.DelRef(&refs[i]))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				fragmentMust(t, err)
				r, e := m.NewRef(h)
				fragmentMust(t, e)
				refs = append(refs, r)
				return h
			}
			boolean := own(m.MkBoolSort())
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				s, e := m.Symbols.Intern(name)
				fragmentMust(t, e)
				return own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
			}
			p, q, k := decl("boundary-p", nil), decl("boundary-q", nil), decl("boundary-k", nil)
			pa, qa, ka := own(m.MkApp(p, nil)), own(m.MkApp(q, nil)), own(m.MkApp(k, nil))
			var formula ast.ASTHandle
			var want []ast.ASTHandle
			switch mode {
			case "predicate-argument":
				r := decl("boundary-r", []ast.ASTHandle{boolean})
				formula = own(m.MkApp(r, []ast.ASTHandle{ka}))
				want = []ast.ASTHandle{r}
			case "theory-equality":
				formula = own(m.MkEq(pa, qa))
			case "ite-condition":
				formula = own(m.MkIte(ka, pa, qa))
				want = []ast.ASTHandle{q, p}
			case "connectives":
				formula = own(m.MkImplies(own(m.MkAnd(pa, own(m.MkNot(qa)))), ka))
				want = []ast.ASTHandle{k, q, p}
			}
			fragmentMust(t, horn.registerPredicates(engine, formula, make(map[ast.ASTHandle]bool)))
			got, e := engine.Predicates()
			fragmentMust(t, e)
			if len(got) != len(want) || len(want) > 0 && !reflect.DeepEqual(got, want) {
				t.Fatalf("registered %v, want %v", got, want)
			}
		})
	}
}
