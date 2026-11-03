package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocConditionOnlyFiniteColumns(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	domain, err := m.BitVectorSort(2)
	must(err)
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		must(cause)
		sorts := make([]ast.ASTHandle, arity)
		for i := range sorts {
			sorts[i] = domain
		}
		d, cause := m.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(d))
		return d
	}
	p := declare("doc-witness-P", 1)
	projected := declare("doc-witness-projected", 1)
	head := declare("doc-witness-head", 2)
	impossible := declare("doc-witness-impossible", 1)
	must(e.AddTableFact(p, []uint32{2}))
	x, err := m.MkVar(1, domain)
	must(err)
	witness, err := m.MkVar(7, domain)
	must(err)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, cause := m.MkApp(d, args)
		must(cause)
		return h
	}
	low, err := m.MkBitVectorExtract(0, 0, witness)
	must(err)
	one, err := m.BitVectorNumeralUnsigned(1, 1)
	must(err)
	guard, err := m.MkEq(low, one)
	must(err)
	body, err := m.MkAnd(app(p, x), guard)
	must(err)
	add := func(body, head ast.ASTHandle, name string) {
		r, cause := m.MkImplies(body, head)
		must(cause)
		must(e.AddRule(r, name))
	}
	add(body, app(projected, x), "project-witness")
	add(body, app(head, witness, x), "keep-witness-permutation")
	zero, err := m.BitVectorNumeralUnsigned(0, 1)
	must(err)
	contradict, err := m.MkEq(low, zero)
	must(err)
	falseBody, err := m.MkAnd(body, contradict)
	must(err)
	add(falseBody, app(impossible, x), "contradict-witness")
	for _, d := range []ast.ASTHandle{projected, head} {
		status, cause := e.QueryPredicate(d)
		must(cause)
		if status != lbool.True {
			t.Fatal("finite witness rejected", status)
		}
		answer, cause := e.Answer(d)
		must(cause)
		formula, cause := m.RefGet(answer)
		must(cause)
		substitution, cause := astrewriter.NewVarSubstitution(m, false)
		must(cause)
		arity, cause := m.GetArity(d)
		must(cause)
		for left := uint32(0); left < 4; left++ {
			for right := uint32(0); right < 4; right++ {
				if arity == 1 && right != 0 {
					continue
				}
				values := []uint32{left}
				if arity == 2 {
					values = append(values, right)
				}
				args := make([]ast.ASTHandle, len(values))
				roots := make([]ast.ASTRef, len(values))
				for i, value := range values {
					args[i], cause = m.BitVectorNumeralUnsigned(value, 2)
					must(cause)
					roots[i], cause = m.NewRef(args[i])
					must(cause)
				}
				ground, cause := m.NewRef(ast.ASTHandle{})
				must(cause)
				must(substitution.Substitute(formula, args, ground))
				expression, cause := m.RefGet(ground)
				must(cause)
				reduced, cause := m.NewRef(ast.ASTHandle{})
				must(cause)
				must(e.guard.RewriteWithBindings(expression, nil, reduced))
				value, cause := m.RefGet(reduced)
				must(cause)
				want := left == 2
				if arity == 2 {
					want = (left == 1 || left == 3) && right == 2
				}
				if m.IsTrue(value) != want || !m.IsTrue(value) && !m.IsFalse(value) {
					t.Fatalf("witness answer %v: want %v", values, want)
				}
				must(m.DelRef(&reduced))
				must(m.DelRef(&ground))
				for i := range roots {
					must(m.DelRef(&roots[i]))
				}
			}
		}
		must(substitution.Close())
		must(m.DelRef(&answer))
	}
	status, err := e.QueryPredicate(impossible)
	must(err)
	if status != lbool.False {
		t.Fatal("contradictory full-domain witness", status)
	}
}
