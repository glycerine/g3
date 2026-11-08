package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogKarrGroundRulesAndDirectFacts(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
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
	decl := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p := decl("karr-ground-p", []ast.ASTHandle{integer, integer})
	unit := decl("karr-ground-unit", nil)
	must(e.AddTableFact(p, []uint32{1, 2}))
	fact := func(d ast.ASTHandle, values ...string) {
		args := make([]ast.ASTHandle, len(values))
		for i, v := range values {
			args[i], err = m.ArithmeticIntegerNumeralText(v)
			must(err)
		}
		f, err := m.MkApp(d, args)
		must(err)
		must(e.addRuleEager(f, ""))
	}
	fact(p, "3", "4")
	fact(unit)
	status, err := e.QueryPredicate(p)
	must(err)
	if status != lbool.Undef {
		t.Fatal("ground rule status", status)
	}
	answer, err := e.Answer(p)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	h, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	check := func(values []string, want bool) {
		args := make([]ast.ASTHandle, len(values))
		for i, v := range values {
			args[i], err = m.ArithmeticIntegerNumeralText(v)
			must(err)
		}
		out, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&out)) }()
		must(subst.Substitute(h, args, out))
		expr, err := m.RefGet(out)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&reduced)) }()
		must(e.guard.RewriteWithBindings(expr, nil, reduced))
		expr, err = m.RefGet(reduced)
		must(err)
		if (want && !m.IsTrue(expr)) || (!want && !m.IsFalse(expr)) {
			t.Fatalf("answer %v want %v got %v", values, want, expr)
		}
	}
	check([]string{"1", "2"}, true)
	check([]string{"3", "4"}, true)
	check([]string{"5", "6"}, true) // affine hull of the two facts
	check([]string{"5", "7"}, false)
	status, err = e.QueryPredicate(unit)
	must(err)
	if status != lbool.Undef || e.karrDatalog.results[unit].Empty() {
		t.Fatal("nullary ground rule", status)
	}
	m.Limit.Cancel()
	status, err = e.QueryPredicate(p)
	m.Limit.ResetCancel()
	if err == nil || status != lbool.Undef || e.karrDatalog.results != nil {
		t.Fatal("cancelled ground query published", status, err)
	}
	status, err = e.QueryPredicate(p)
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry", status)
	}
}

func TestKarrGroundRuleKeepsInequalitiesLazyUntilUnion(t *testing.T) {
	value, ok := new(big.Rat).SetString("9223372036854775808")
	if !ok {
		t.Fatal("invalid test numeral")
	}
	candidate, err := (karrGroundRule{values: []*big.Rat{value}}).apply(nil)
	fragmentMust(t, err)
	inequalities, err := candidate.Inequalities()
	fragmentMust(t, err)
	if inequalities.Len() != 1 || inequalities.B[0].Cmp(new(big.Rat).Neg(value)) != 0 {
		t.Fatal("singleton inequality", inequalities)
	}
	target := rel.NewKarrEmpty(1, nil)
	if _, err = target.Union(candidate); !errors.Is(err, hilbert.ErrOverflow) {
		t.Fatal("expected lazy dualization overflow", err)
	}
	if !target.Empty() {
		t.Fatal("failed union changed target")
	}
	inequalities, err = candidate.Inequalities()
	fragmentMust(t, err)
	if inequalities.Len() != 1 {
		t.Fatal("failed union changed candidate", inequalities)
	}
}
