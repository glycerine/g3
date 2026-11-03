package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func checkDatalogUDocBV1Tuples(t *testing.T, m *ast.ASTManager, e *PropositionalEngine, declaration ast.ASTHandle, arity int, wanted [][]uint64) {
	t.Helper()
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	status, err := e.QueryPredicate(declaration)
	must(err)
	wantStatus := lbool.False
	if len(wanted) != 0 {
		wantStatus = lbool.True
	}
	if status != wantStatus {
		t.Fatal("UDoc tuple query status", declaration, status, wantStatus)
	}
	answer, err := e.Answer(declaration)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	formula, err := m.RefGet(answer)
	must(err)
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(substitution.Close()) }()
	limit := 1 << arity
	if arity == 0 {
		limit = 1
	}
	for code := 0; code < limit; code++ {
		tuple := make([]uint64, arity)
		values := make([]ast.ASTHandle, arity)
		roots := make([]ast.ASTRef, arity)
		for column := range tuple {
			tuple[column] = uint64(code >> (arity - 1 - column) & 1)
			values[column], err = m.BitVectorNumeralUnsigned(uint32(tuple[column]), 1)
			must(err)
			roots[column], err = m.NewRef(values[column])
			must(err)
		}
		want := false
		for _, candidate := range wanted {
			match := len(candidate) == len(tuple)
			for column := range tuple {
				match = match && candidate[column] == tuple[column]
			}
			want = want || match
		}
		ground, cause := m.NewRef(ast.ASTHandle{})
		must(cause)
		must(substitution.Substitute(formula, values, ground))
		expression, cause := m.RefGet(ground)
		must(cause)
		reduced, cause := m.NewRef(ast.ASTHandle{})
		must(cause)
		must(e.guard.RewriteWithBindings(expression, nil, reduced))
		truth, cause := m.RefGet(reduced)
		must(cause)
		if m.IsTrue(truth) != want || !m.IsTrue(truth) && !m.IsFalse(truth) {
			t.Fatal("UDoc answer tuple", declaration, tuple, want)
		}
		must(m.DelRef(&reduced))
		must(m.DelRef(&ground))
		for index := len(roots) - 1; index >= 0; index-- {
			must(m.DelRef(&roots[index]))
		}
	}
}

func TestDatalogUDocInterpretedAndHeadOnlyFullColumns(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	domain, err := m.BitVectorSort(1)
	must(err)
	declare := func(label string, arity int) ast.ASTHandle {
		name, cause := m.Symbols.Intern(label)
		must(cause)
		sorts := make([]ast.ASTHandle, arity)
		for index := range sorts {
			sorts[index] = domain
		}
		declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	seed := declare("doc-full-column-seed", 1)
	headOnly := declare("doc-full-column-head-only", 2)
	repeated := declare("doc-full-column-repeated", 2)
	emptyBody := declare("doc-full-column-empty-body", 2)
	mixedEmpty := declare("doc-full-column-mixed-empty", 2)
	interpretedOnly := declare("doc-full-column-interpreted", 1)
	interpretedProjected := declare("doc-full-column-interpreted-projected", 0)
	must(e.AddTableFact(seed, []uint32{1}))
	pin := func(term ast.ASTHandle, cause error) ast.ASTHandle {
		t.Helper()
		must(cause)
		root, cause := m.NewRef(term)
		must(cause)
		t.Cleanup(func() { must(m.DelRef(&root)) })
		return term
	}
	x := pin(m.MkVar(0, domain))
	y := pin(m.MkVar(1, domain))
	one := pin(m.BitVectorNumeralUnsigned(1, 1))
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		term, cause := m.MkApp(declaration, arguments)
		must(cause)
		return term
	}
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, name string) {
		rule, cause := e.rules.NewRule(head, tails, nil, name)
		must(cause)
		must(e.set.Add(rule))
		must(rule.Close())
	}
	add(app(headOnly, y, x), []ast.ASTHandle{app(seed, x)}, "head-only-full-column")
	add(app(repeated, y, y), []ast.ASTHandle{app(seed, x)}, "repeated-head-only-full-column")
	add(app(emptyBody, y, y), nil, "empty-body-full-column")
	add(app(mixedEmpty, one, y), nil, "mixed-empty-body-full-column")
	yIsOne := pin(m.MkEq(y, one))
	add(app(interpretedOnly, y), []ast.ASTHandle{yIsOne}, "interpreted-only-full-column")
	add(app(interpretedProjected), []ast.ASTHandle{yIsOne}, "interpreted-only-projected-column")
	checkDatalogUDocBV1Tuples(t, m, e, headOnly, 2, [][]uint64{{0, 1}, {1, 1}})
	checkDatalogUDocBV1Tuples(t, m, e, repeated, 2, [][]uint64{{0, 0}, {1, 1}})
	checkDatalogUDocBV1Tuples(t, m, e, emptyBody, 2, [][]uint64{{0, 0}, {1, 1}})
	checkDatalogUDocBV1Tuples(t, m, e, mixedEmpty, 2, [][]uint64{{1, 0}, {1, 1}})
	checkDatalogUDocBV1Tuples(t, m, e, interpretedOnly, 1, [][]uint64{{1}})
	checkDatalogUDocBV1Tuples(t, m, e, interpretedProjected, 0, [][]uint64{{}})
}
