package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocInterpretedJoinColumnsAndContradiction(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	small, err := m.BitVectorSort(2)
	must(err)
	wide, err := m.BitVectorSort(4)
	must(err)
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(n, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p := declare("doc-guard-P", []ast.ASTHandle{small, wide})
	r := declare("doc-guard-R", []ast.ASTHandle{wide, small})
	q := declare("doc-guard-Q", []ast.ASTHandle{small})
	contradiction := declare("doc-guard-contradiction", []ast.ASTHandle{small})
	bad := declare("doc-guard-unbound", []ast.ASTHandle{small})
	for _, row := range [][]uint32{{2, 9}, {3, 9}, {1, 8}} {
		must(e.AddTableFact(p, row))
	}
	for _, row := range [][]uint32{{9, 3}, {8, 1}} {
		must(e.AddTableFact(r, row))
	}
	x, err := m.MkVar(5, small)
	must(err)
	y, err := m.MkVar(0, wide)
	must(err)
	z, err := m.MkVar(2, small)
	must(err)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, args)
		must(err)
		return h
	}
	pa, ra, qa := app(p, x, y), app(r, y, z), app(q, x)
	yLow, err := m.MkBitVectorExtract(1, 0, y)
	must(err)
	one, err := m.BitVectorNumeralUnsigned(1, 2)
	must(err)
	yNine, err := m.MkEq(yLow, one)
	must(err)
	zLow, err := m.MkBitVectorExtract(0, 0, z)
	must(err)
	oneLow, err := m.BitVectorNumeralUnsigned(1, 1)
	must(err)
	zThree, err := m.MkEq(zLow, oneLow)
	must(err)
	body, err := m.MkAndArgs([]ast.ASTHandle{pa, ra, yNine, zThree})
	must(err)
	rule, err := m.MkImplies(body, qa)
	must(err)
	must(e.AddRule(rule, "doc-guard-join"))
	zero, err := m.BitVectorNumeralUnsigned(0, 2)
	must(err)
	yEight, err := m.MkEq(yLow, zero)
	must(err)
	impossible, err := m.MkAndArgs([]ast.ASTHandle{pa, yNine, yEight})
	must(err)
	rule, err = m.MkImplies(impossible, app(contradiction, x))
	must(err)
	must(e.AddRule(rule, "doc-guard-false"))
	lowered, err := e.set.Rule(0)
	must(err)
	interpretedEnd, err := lowered.TailSize()
	must(err)
	interpretedStart, err := lowered.UninterpretedTailSize()
	must(err)
	must(lowered.Close())
	if interpretedEnd == interpretedStart {
		t.Fatal("fixture normalization removed interpreted conditions")
	}
	result, err := e.QueryPredicate(q)
	must(err)
	if result != lbool.True {
		t.Fatal("bound UDoc guard rejected matching rows", result)
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	formula, err := m.RefGet(answer)
	must(err)
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(substitution.Close()) }()
	for _, value := range []uint32{0, 1, 2, 3} {
		numeral, err := m.BitVectorNumeralUnsigned(value, 2)
		must(err)
		root, err := m.NewRef(numeral)
		must(err)
		ground, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(substitution.Substitute(formula, []ast.ASTHandle{numeral}, ground))
		h, err := m.RefGet(ground)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		truth, err := m.RefGet(reduced)
		must(err)
		if m.IsTrue(truth) != (value == 2 || value == 3) || !m.IsTrue(truth) && !m.IsFalse(truth) {
			t.Fatal("UDoc variable/column remap", value)
		}
		must(m.DelRef(&reduced))
		must(m.DelRef(&ground))
		must(m.DelRef(&root))
	}
	result, err = e.QueryPredicate(contradiction)
	must(err)
	if result != lbool.False {
		t.Fatal("conjunctive UDoc guard contradiction", result)
	}
	// A variable absent from every positive relation remains an explicit
	// unsupported condition; a failed query must not expose the prior answer.
	unbound, err := m.MkVar(7, wide)
	must(err)
	lowBit, err := m.MkBitVectorExtract(0, 0, unbound)
	must(err)
	oneBit, err := m.BitVectorNumeralUnsigned(1, 1)
	must(err)
	unboundEq, err := m.MkEq(lowBit, oneBit)
	must(err)
	badBody, err := m.MkAnd(pa, unboundEq)
	must(err)
	rule, err = m.MkImplies(badBody, app(bad, x))
	must(err)
	must(e.AddRule(rule, "doc-guard-unbound"))
	_, err = e.QueryPredicate(q)
	if !errors.Is(err, ErrUDocDatalogFragment) {
		t.Fatal("unbound interpreted guard not rejected", err)
	}
	if _, err = e.Answer(q); err == nil {
		t.Fatal("failed interpreted query exposed stale result")
	}
	if len(e.udocDatalog.guardCleanup) != 0 {
		t.Fatal("ordinary interpreted failure left temporary roots")
	}
}
