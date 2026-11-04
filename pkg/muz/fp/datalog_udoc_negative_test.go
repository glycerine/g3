package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocStratifiedNegativeRulesAndUpdates(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	sort, err := m.BitVectorSort(2)
	must(err)
	declare := func(label string, arity int) ast.ASTHandle {
		t.Helper()
		name, cause := m.Symbols.Intern(label)
		must(cause)
		sorts := make([]ast.ASTHandle, arity)
		for index := range sorts {
			sorts[index] = sort
		}
		declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	p, domain, seed := declare("doc-negative-P", 2), declare("doc-negative-domain", 1), declare("doc-negative-seed", 1)
	negative, more, output := declare("doc-negative-derived", 1), declare("doc-negative-more", 1), declare("doc-negative-output", 2)
	aligned, repeated := declare("doc-negative-aligned", 1), declare("doc-negative-repeated", 1)
	stop, ready := declare("doc-negative-stop", 0), declare("doc-negative-ready", 1)
	for _, row := range [][]uint32{{0, 0}, {0, 1}, {1, 1}, {2, 3}, {3, 3}} {
		must(e.AddTableFact(p, row))
	}
	for value := uint32(0); value < 4; value++ {
		must(e.AddTableFact(domain, []uint32{value}))
	}
	must(e.AddTableFact(seed, []uint32{1}))
	x, err := m.MkVar(0, sort)
	must(err)
	y, err := m.MkVar(1, sort)
	must(err)
	for _, variable := range []ast.ASTHandle{x, y} {
		root, cause := m.NewRef(variable)
		must(cause)
		defer func() { must(m.DelRef(&root)) }()
	}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		term, cause := m.MkApp(declaration, arguments)
		must(cause)
		return term
	}
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, flags []bool, name string) {
		rule, cause := e.rules.NewRule(head, tails, flags, name)
		must(cause)
		must(e.set.Add(rule))
		must(rule.Close())
	}
	// Put all negative consumers before the derived producer to prove strata,
	// rather than source rule order, determine when negation is evaluated.
	add(app(output, x, y), []ast.ASTHandle{app(p, x, y), app(negative, y)}, []bool{false, true}, "negative-consumer")
	add(app(aligned, x), []ast.ASTHandle{app(seed, x), app(negative, x)}, []bool{false, true}, "negative-aligned")
	add(app(repeated, x), []ast.ASTHandle{app(domain, x), app(p, x, x)}, []bool{false, true}, "negative-repeated")
	add(app(ready, x), []ast.ASTHandle{app(domain, x), app(stop)}, []bool{false, true}, "negative-nullary")
	add(app(negative, x), []ast.ASTHandle{app(seed, x)}, nil, "negative-producer")
	check := func(declaration ast.ASTHandle, arity int, wanted [][]uint32) {
		t.Helper()
		status, cause := e.QueryPredicate(declaration)
		must(cause)
		if (status == lbool.True) != (len(wanted) != 0) || status == lbool.Undef {
			t.Fatal("negative query status", status, wanted)
		}
		answer, cause := e.Answer(declaration)
		must(cause)
		defer func() { must(m.DelRef(&answer)) }()
		formula, cause := m.RefGet(answer)
		must(cause)
		substitution, cause := astrewriter.NewVarSubstitution(m, false)
		must(cause)
		defer func() { must(substitution.Close()) }()
		limit := 1 << (2 * arity)
		for code := 0; code < limit; code++ {
			values, row := make([]ast.ASTHandle, arity), make([]uint32, arity)
			var roots []ast.ASTRef
			for column := range row {
				row[column] = uint32(code>>(2*(arity-1-column))) & 3
				values[column], cause = m.BitVectorNumeralUnsigned(row[column], 2)
				must(cause)
				root, getErr := m.NewRef(values[column])
				must(getErr)
				roots = append(roots, root)
			}
			want := false
			for _, candidate := range wanted {
				match := len(candidate) == len(row)
				for column := range row {
					match = match && candidate[column] == row[column]
				}
				want = want || match
			}
			ground, getErr := m.NewRef(ast.ASTHandle{})
			must(getErr)
			must(substitution.Substitute(formula, values, ground))
			handle, getErr := m.RefGet(ground)
			must(getErr)
			reduced, getErr := m.NewRef(ast.ASTHandle{})
			must(getErr)
			must(e.guard.RewriteWithBindings(handle, nil, reduced))
			truth, getErr := m.RefGet(reduced)
			must(getErr)
			if m.IsTrue(truth) != want || !m.IsTrue(truth) && !m.IsFalse(truth) {
				t.Fatal("negative relation answer tuple", row, want)
			}
			must(m.DelRef(&reduced))
			must(m.DelRef(&ground))
			for index := len(roots) - 1; index >= 0; index-- {
				must(m.DelRef(&roots[index]))
			}
		}
	}
	check(output, 2, [][]uint32{{0, 0}, {2, 3}, {3, 3}})
	check(output, 2, [][]uint32{{0, 0}, {2, 3}, {3, 3}})
	check(aligned, 1, nil)
	check(repeated, 1, [][]uint32{{2}})
	check(ready, 1, [][]uint32{{0}, {1}, {2}, {3}})
	must(e.AddTableFact(stop, nil))
	check(ready, 1, nil)
	must(e.AddTableFact(seed, []uint32{3}))
	check(output, 2, [][]uint32{{0, 0}})
	strongerBody, err := m.MkAnd(app(seed, x), app(more, x))
	must(err)
	stronger, err := m.MkImplies(strongerBody, app(negative, x))
	must(err)
	must(e.UpdateRule(stronger, "negative-producer"))
	check(output, 2, [][]uint32{{0, 0}, {0, 1}, {1, 1}, {2, 3}, {3, 3}})
	add(app(negative, x), []ast.ASTHandle{app(domain, x), app(negative, x)}, []bool{false, true}, "negative-cycle")
	_, err = e.QueryPredicate(output)
	if !errors.Is(err, ErrUnstratifiedNegation) {
		t.Fatal("negative cycle accepted", err)
	}
	if _, err = e.Answer(output); err == nil {
		t.Fatal("negative cycle exposed stale answer")
	}
}
