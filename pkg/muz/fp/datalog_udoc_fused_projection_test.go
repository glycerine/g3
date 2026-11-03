package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocFusesFiniteEqualityWithDeadColumns(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("doc-fused-domain")
	must(err)
	finite, err := m.MkDatalogFiniteSort(name, 5)
	must(err)
	bits, err := m.BitVectorSort(3)
	must(err)
	declare := func(label string, sorts []ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		name, cause := m.Symbols.Intern(label)
		must(cause)
		declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("doc-fused-P", []ast.ASTHandle{finite, bits, finite, finite})
	key := declare("doc-fused-key", []ast.ASTHandle{bits})
	q := declare("doc-fused-Q", []ast.ASTHandle{bits, finite, finite, finite})
	bad := declare("doc-fused-no-dead-variable", []ast.ASTHandle{finite, bits, finite, finite, finite})
	for _, row := range [][]uint32{{0, 1, 0, 0}, {1, 2, 2, 2}, {3, 4, 3, 3}, {4, 5, 4, 0}} {
		must(e.AddTableFact(p, row))
	}
	for _, value := range []uint32{1, 2, 4, 5} {
		must(e.AddTableFact(key, []uint32{value}))
	}
	x, err := m.MkVar(7, finite)
	must(err)
	b, err := m.MkVar(2, bits)
	must(err)
	y, err := m.MkVar(5, finite)
	must(err)
	four, err := m.MkDatalogNumeral(4, finite)
	must(err)
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		term, cause := m.MkApp(declaration, arguments)
		must(cause)
		return term
	}
	body, keyBody := app(p, x, b, y, y), app(key, b)
	equality, err := m.MkEq(x, y)
	must(err)
	// Normalized rule copies own fresh variable expressions. Keep the original
	// fixture terms alive for the later ordinary-filter boundary rule as well.
	for _, term := range []ast.ASTHandle{body, keyBody, equality} {
		root, cause := m.NewRef(term)
		must(cause)
		defer func() { must(m.DelRef(&root)) }()
	}
	addCompiledRule := func(head ast.ASTHandle, label string) {
		t.Helper()
		// Keep the compiler input's equality explicit so this reaches the
		// registered fused operation rather than a preceding equality rewrite.
		rule, cause := e.rules.NewRule(head, []ast.ASTHandle{body, keyBody, equality}, nil, label)
		must(cause)
		must(e.set.Add(rule))
		must(rule.Close())
	}
	addCompiledRule(app(q, b, x, four, x), "doc-fused-rule")
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.True {
		t.Fatal("fused UDoc finite equality query", status)
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	formula, err := m.RefGet(answer)
	must(err)
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(substitution.Close()) }()
	for _, tc := range []struct {
		row  [4]uint64
		want bool
	}{{[4]uint64{1, 0, 4, 0}, true}, {[4]uint64{4, 3, 4, 3}, true}, {[4]uint64{2, 1, 4, 1}, false}, {[4]uint64{1, 0, 3, 0}, false}, {[4]uint64{1, 0, 4, 1}, false}, {[4]uint64{4, 0, 4, 0}, false}, {[4]uint64{5, 4, 4, 4}, false}} {
		var roots []ast.ASTRef
		values := make([]ast.ASTHandle, 4)
		for index, value := range tc.row {
			sort := finite
			if index == 0 {
				sort = bits
			}
			values[index], err = m.MkDatalogNumeral(value, sort)
			must(err)
			root, cause := m.NewRef(values[index])
			must(cause)
			roots = append(roots, root)
		}
		ground, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(substitution.Substitute(formula, values, ground))
		handle, err := m.RefGet(ground)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(handle, nil, reduced))
		truth, err := m.RefGet(reduced)
		must(err)
		if m.IsTrue(truth) != tc.want || !m.IsTrue(truth) && !m.IsFalse(truth) {
			t.Fatal("fused finite equality/head projection tuple", tc)
		}
		must(m.DelRef(&reduced))
		must(m.DelRef(&ground))
		for index := len(roots) - 1; index >= 0; index-- {
			must(m.DelRef(&roots[index]))
		}
	}
	// Every body variable appears in this head, so source compile_rule must
	// use ordinary filtering even though final assembly appends a constant and
	// duplicates y. Ordinary finite equality remains an unsupported guard.
	addCompiledRule(app(bad, x, b, y, y, four), "doc-fused-boundary")
	_, err = e.QueryPredicate(bad)
	if !errors.Is(err, ErrUDocDatalogFragment) {
		t.Fatal("finite equality without a dead variable used fused filtering", err)
	}
	if _, err = e.Answer(bad); err == nil {
		t.Fatal("unsupported ordinary finite guard exposed a result")
	}
	if _, err = m.RefGet(answer); err != nil {
		t.Fatal("owning fused answer did not survive a later failed query", err)
	}
}
