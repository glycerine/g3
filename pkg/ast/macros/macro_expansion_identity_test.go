package macros_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Each scope in the general rewriter has a fresh cache. The same subterm is
// rewritten once in each binder and once more outside both.
func TestMacroExpansionBinderResourceSteps(t *testing.T) {
	m, manager, cases := macroExpansionCases(t)
	chain := cases[1].term
	b, err := m.MkBoolSort()
	must(t, err)
	makeQuant := func(all bool) ast.ASTHandle {
		q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: all, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Sorts: []ast.ASTHandle{b}, Body: chain,
		})
		must(t, err)
		return q
	}
	both, err := m.MkAnd3(makeQuant(true), makeQuant(false), chain)
	must(t, err)
	ref, err := m.NewRef(both)
	must(t, err)
	defer func() { must(t, m.DelRef(&ref)) }()
	out, err := m.NewRef(ast.ASTHandle{})
	must(t, err)
	defer func() { must(t, m.DelRef(&out)) }()
	proof, err := m.NewRef(ast.ASTHandle{})
	must(t, err)
	defer func() { must(t, m.DelRef(&proof)) }()
	limit := m.ResourceLimit()
	for _, steps := range []uint32{35, 36} {
		must(t, limit.Push(steps))
		initial := limit.Count()
		_, err := manager.ExpandMacros(both, ast.ASTHandle{}, ast.ASTDependency{}, out, proof)
		cost := limit.Count() - initial
		limit.Pop()
		if (err == nil) != (steps == 36) || cost != 36 {
			t.Fatalf("limit %d, charged %d steps, error %v", steps, cost, err)
		}
	}
	got, err := m.RefGet(out)
	must(t, err)
	if got != both {
		t.Fatal("changed an untouched quantified formula")
	}
}

func TestMacroExpansionObservesCurrentDefinitionsAndAliasedOutput(t *testing.T) {
	m, manager, cases := macroExpansionCases(t)
	leaf, chain := cases[0].term, cases[1].term
	out, err := m.NewRef(ast.ASTHandle{})
	must(t, err)
	defer func() { must(t, m.DelRef(&out)) }()
	dep, err := m.DependencyLeaf(leaf)
	must(t, err)
	must(t, m.DependencyInc(dep))
	defer func() { must(t, m.DependencyDec(dep)) }()
	check := func(want ast.ASTHandle) {
		t.Helper()
		gotDep, err := manager.ExpandMacros(chain, ast.ASTHandle{}, dep, out, out)
		must(t, err)
		got, err := m.RefGet(out)
		must(t, err)
		if got != want || gotDep != dep {
			t.Fatalf("expanded to %v with dependency %v, want %v and %v", got, gotDep, want, dep)
		}
		must(t, m.DependencyDec(gotDep))
	}
	check(chain)
	b, err := m.MkBoolSort()
	must(t, err)
	x, err := m.MkVar(0, b)
	must(t, err)
	g, err := m.GetDecl(chain)
	must(t, err)
	must(t, manager.PushScope())
	ok, err := manager.Insert(g, quantifiedMacro(t, m, g, x, b), ast.ASTHandle{}, ast.ASTDependency{})
	must(t, err)
	if !ok {
		t.Fatal("failed to install scoped macro")
	}
	check(leaf)
	must(t, manager.PopScope(1))
	check(chain)
}

// Compare with the ordinary traversal on exactly the same live DAG. Arming
// the arena injector at an unreachable count selects that traversal.
func TestMacroExpansionMatchesTraversalOnSharedTerms(t *testing.T) {
	m, manager, cases := macroExpansionCases(t)
	rng := rand.New(rand.NewPCG(423, 771))
	terms := []ast.ASTHandle{cases[0].term, cases[1].term, cases[2].term}
	sort, err := m.MkBoolSort()
	must(t, err)
	pair, err := m.GetDecl(cases[2].term)
	must(t, err)
	macro, err := m.GetDecl(cases[3].term)
	must(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	must(t, err)
	defer func() { must(t, m.DelRef(&out)) }()
	proof, err := m.NewRef(ast.ASTHandle{})
	must(t, err)
	defer func() { must(t, m.DelRef(&proof)) }()
	pick := func() ast.ASTHandle { return terms[rng.IntN(len(terms))] }
	for i := range 220 {
		var term ast.ASTHandle
		switch rng.IntN(6) {
		case 0:
			term, err = m.MkApp(macro, []ast.ASTHandle{pick()})
		case 1:
			term, err = m.MkIte(pick(), pick(), pick())
		case 2:
			term, err = m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: rng.IntN(2) == 0, Names: []usymbol.Symbol{usymbol.Numerical(1)},
				Sorts: []ast.ASTHandle{sort}, Body: pick(),
			})
		case 3:
			same := pick()
			term, err = m.MkApp(pair, []ast.ASTHandle{same, same})
		case 4:
			term, err = m.MkVar(uint32(rng.IntN(3)), sort)
		case 5:
			term, err = m.MkApp(pair, []ast.ASTHandle{pick(), pick()})
		}
		must(t, err)
		root, err := m.NewRef(term)
		must(t, err)
		t.Cleanup(func() { must(t, m.DelRef(&root)) })
		terms = append(terms, term)
		var reference ast.ASTRef
		var steps uint64
		for _, original := range []bool{true, false} {
			if original {
				m.Budget().FailAfter(math.MaxInt64)
			}
			before := m.ResourceLimit().Count()
			_, err = manager.ExpandMacros(term, ast.ASTHandle{}, ast.ASTDependency{}, out, proof)
			cost := m.ResourceLimit().Count() - before
			m.Budget().FailAfter(-1)
			must(t, err)
			got, err := m.RefGet(out)
			must(t, err)
			if original {
				reference, err = m.NewRef(got)
				must(t, err)
				steps = cost
				continue
			}
			want, err := m.RefGet(reference)
			must(t, err)
			if got != want || cost != steps {
				t.Fatalf("term %d got %v in %d steps, ordinary traversal returned %v in %d", i, got, cost, want, steps)
			}
			must(t, m.DelRef(&reference))
		}
	}
}
