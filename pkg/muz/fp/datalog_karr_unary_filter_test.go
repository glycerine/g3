package fp

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source dl_compiler selects each constant and immediately projects it before
// building filter_identical groups and choosing the last binding for the head.
func TestDatalogKarrUnaryConstantsRepeatedColumnsAndSibling(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fact    []uint32
		q7, q42 bool
	}{
		{"matching", []uint32{2, 1, 7, 1, 2}, true, false},
		{"constant mismatch", []uint32{2, 3, 7, 1, 2}, true, true},
		{"repeated mismatch", []uint32{2, 1, 7, 1, 3}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(e.SelectEngine("datalog"))
			must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
			must(g.SourceSetName("fixedpoint.xform.karr", "true"))
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			decl := func(name string, n int) ast.ASTHandle {
				s, err := m.Symbols.Intern(name)
				must(err)
				sorts := make([]ast.ASTHandle, n)
				for i := range sorts {
					sorts[i] = integer
				}
				d, err := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				return d
			}
			p, q, sibling := decl("karr-filter-p", 5), decl("karr-filter-q", 2), decl("karr-filter-sibling", 5)
			must(e.AddTableFact(p, tc.fact))
			x, err := m.MkVar(0, integer)
			must(err)
			y, err := m.MkVar(1, integer)
			must(err)
			nine, err := m.ArithmeticIntegerNumeralText("9")
			must(err)
			eight, err := m.ArithmeticIntegerNumeralText("8")
			must(err)
			body, err := m.MkApp(p, []ast.ASTHandle{x, nine, y, eight, x})
			must(err)
			head, err := m.MkApp(q, []ast.ASTHandle{y, x})
			must(err)
			rule, err := m.MkImplies(body, head)
			must(err)
			must(e.addRuleEager(rule, "filtered"))
			vars := make([]ast.ASTHandle, 5)
			for i := range vars {
				vars[i], err = m.MkVar(uint32(i), integer)
				must(err)
			}
			copyBody, err := m.MkApp(p, vars)
			must(err)
			copyHead, err := m.MkApp(sibling, vars)
			must(err)
			copyRule, err := m.MkImplies(copyBody, copyHead)
			must(err)
			must(e.addRuleEager(copyRule, "sibling"))
			must(e.flushPendingRules())
			// Verify the compiled columns, including the LAST occurrence of x.
			owner, err := newKarrDatalogActivation(m, e.set, e.predicateOrder)
			must(err)
			defer func() { must(owner.close()) }()
			sourceRule, err := e.set.Rule(0)
			must(err)
			_, compiled, err := owner.compileUnaryRule(sourceRule)
			must(err)
			must(sourceRule.Close())
			if len(compiled.constants) != 2 || compiled.constants[0] != 1 || compiled.constants[1] != 2 || len(compiled.identical) != 1 || len(compiled.identical[0]) != 2 || compiled.identical[0][0] != 0 || compiled.identical[0][1] != 2 || len(compiled.removed) != 1 || compiled.removed[0] != 0 {
				t.Fatal("source shifted/last columns", compiled)
			}
			status, err := e.QueryPredicate(q)
			must(err)
			if status != lbool.Undef {
				t.Fatal("status", status)
			}
			answer, err := e.Answer(q)
			must(err)
			defer func() { must(m.DelRef(&answer)) }()
			formula, err := m.RefGet(answer)
			must(err)
			subst, err := astrewriter.NewVarSubstitution(m, false)
			must(err)
			defer func() { must(subst.Close()) }()
			evaluate := func(formula ast.ASTHandle, values []uint32) bool {
				args := make([]ast.ASTHandle, len(values))
				for i, v := range values {
					args[i], err = m.ArithmeticIntegerNumeralText(fmt.Sprint(v))
					must(err)
				}
				ground, err := m.NewRef(ast.ASTHandle{})
				must(err)
				defer func() { must(m.DelRef(&ground)) }()
				must(subst.Substitute(formula, args, ground))
				h, err := m.RefGet(ground)
				must(err)
				reduced, err := m.NewRef(ast.ASTHandle{})
				must(err)
				defer func() { must(m.DelRef(&reduced)) }()
				must(e.guard.RewriteWithBindings(h, nil, reduced))
				h, err = m.RefGet(reduced)
				must(err)
				if !m.IsTrue(h) && !m.IsFalse(h) {
					t.Fatal("unreduced", h)
				}
				return m.IsTrue(h)
			}
			if got := evaluate(formula, []uint32{7, 2}); got != tc.q7 {
				t.Fatal("source filter quirk", got)
			}
			if got := evaluate(formula, []uint32{42, 2}); got != tc.q42 {
				t.Fatal("lazy empty basis", got)
			}
			// The filtering operation must never alter the source used by a sibling.
			status, err = e.QueryPredicate(sibling)
			must(err)
			if status != lbool.Undef {
				t.Fatal(status)
			}
			siblingAnswer, err := e.Answer(sibling)
			must(err)
			defer func() { must(m.DelRef(&siblingAnswer)) }()
			siblingFormula, err := m.RefGet(siblingAnswer)
			must(err)
			if !evaluate(siblingFormula, tc.fact) {
				t.Fatal("sibling seed lost")
			}
			changed := append([]uint32(nil), tc.fact...)
			changed[0] = 99
			if evaluate(siblingFormula, changed) {
				t.Fatal("filter mutated sibling seed")
			}
			m.Limit.Cancel()
			status, err = e.QueryPredicate(q)
			m.Limit.ResetCancel()
			if err == nil || status != lbool.Undef {
				t.Fatal("cancel", status, err)
			}
			status, err = e.QueryPredicate(q)
			must(err)
			if status != lbool.Undef {
				t.Fatal("retry", status)
			}
		})
	}
}

func TestKarrUnaryEmptySourceSkipsFilters(t *testing.T) {
	// Existing concrete empty relation is a nil source register in the compiler.
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, n int) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("karr-filter-empty-p", 2), declare("karr-filter-empty-q", 1)
	x, err := m.MkVar(0, integer)
	must(err)
	nine, err := m.ArithmeticIntegerNumeralText("9")
	must(err)
	body, err := m.MkApp(p, []ast.ASTHandle{nine, x})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, ""))
	for i := 0; i < 2; i++ {
		status, err := e.QueryPredicate(q)
		must(err)
		if status != lbool.False {
			t.Fatal("empty relation", status)
		}
	}
}

func TestKarrUnaryConstantFilterMidCancellationAndOverflowRetry(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(name string, n int) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := decl("karr-filter-cancel-p", 2), decl("karr-filter-cancel-q", 1)
	x, err := m.MkVar(0, integer)
	must(err)
	nine, err := m.ArithmeticIntegerNumeralText("9")
	must(err)
	body, err := m.MkApp(p, []ast.ASTHandle{nine, x})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, ""))
	must(e.flushPendingRules())
	a, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q})
	must(err)
	defer func() { must(a.close()) }()
	armed, fired := true, false
	seed := rel.NewKarrEmpty(2, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	must(seed.AddFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(7, 1)}))
	a.seeds[p] = seed
	status, err := a.query([]ast.ASTHandle{q})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || a.results != nil {
		t.Fatal("filter cancellation", fired, status, err)
	}
	armed = false
	status, err = a.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef || a.results[q].Empty() {
		t.Fatal("filter retry", status)
	}
	large := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 63))
	oversized := rel.NewKarrEmpty(2, nil)
	must(oversized.AddFact([]*big.Rat{big.NewRat(1, 1), large}))
	a.seeds[p] = oversized
	status, err = a.query([]ast.ASTHandle{q})
	if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || a.results != nil {
		t.Fatal("filter overflow", status, err)
	}
	a.seeds[p] = seed
	status, err = a.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef || a.results[q].Empty() {
		t.Fatal("overflow retry", status)
	}
}
