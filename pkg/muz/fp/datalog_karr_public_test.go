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

func TestDatalogKarrPublicFactAndIdentity(t *testing.T) {
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
	declare := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("karr-public-p"), declare("karr-public-q")
	must(e.AddTableFact(p, []uint32{1}))
	x, err := m.MkVar(0, integer)
	must(err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	qx, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(px, qx)
	must(err)
	must(e.AddRule(rule, "karr-forward"))
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.Undef {
		t.Fatal("Karr is imprecise", status)
	}
	emptyStatus, err := e.QueryPredicates(nil)
	must(err)
	if emptyStatus != lbool.True {
		t.Fatal("zero-output query", emptyStatus)
	}
	status, err = e.QueryPredicate(q)
	must(err)
	// Force cancellation inside the lazy basis-to-inequality conversion. The
	// source-style undefined conversion must not corrupt the stored snapshot.
	armed, fired := false, false
	check := func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	}
	source := rel.NewKarrEmpty(1, check)
	must(source.AddFact([]*big.Rat{big.NewRat(1, 1)}))
	basis := rel.NewKarrEmpty(1, check)
	_, err = basis.Union(source)
	must(err)
	e.karrDatalog.results[q] = basis
	armed = true
	failedAnswer, err := e.Answer(q)
	m.Limit.ResetCancel()
	armed = false
	if !fired || err == nil || failedAnswer != (ast.ASTRef{}) {
		t.Fatal("answer dualization cancellation", fired, err)
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	formula, err := m.RefGet(answer)
	must(err)
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(substitution.Close()) }()
	for _, sample := range []struct {
		text string
		want bool
	}{{"1", true}, {"2", false}} {
		number, err := m.ArithmeticIntegerNumeralText(sample.text)
		must(err)
		ground, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(substitution.Substitute(formula, []ast.ASTHandle{number}, ground))
		h, err := m.RefGet(ground)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		value, err := m.RefGet(reduced)
		must(err)
		if m.IsTrue(value) != sample.want || (!m.IsTrue(value) && !m.IsFalse(value)) {
			t.Fatal("answer", sample, value)
		}
		must(m.DelRef(&reduced))
		must(m.DelRef(&ground))
	}
	if err := e.AddTableFact(p, []uint32{2}); !errors.Is(err, ErrKarrDatalogFragment) {
		t.Fatal("source single direct fact boundary", err)
	}
	status, err = e.QueryPredicate(q)
	must(err)
	if status != lbool.Undef {
		t.Fatal("failed fact changed seed", status)
	}
	// A direct predicate expression has an identity synthetic head; its clone
	// receives independent fact owners and retains an owning answer.
	status, err = e.QueryExpression(qx)
	must(err)
	if status != lbool.Undef || e.queryClone == nil || e.queryClone.karrDatalog == e.karrDatalog {
		t.Fatal("query clone", status)
	}
	held, err := e.ExpressionAnswer()
	must(err)
	must(e.clearQueryResources())
	if _, err = m.RefGet(held); err != nil {
		t.Fatal("query answer lost with clone", err)
	}
	must(m.DelRef(&held))
	m.Limit.Cancel()
	status, err = e.QueryPredicate(q)
	m.Limit.ResetCancel()
	if err == nil || status != lbool.Undef {
		t.Fatal("cancellation", status, err)
	}
	status, err = e.QueryPredicate(q)
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry", status)
	}

}

func TestDatalogKarrSelectionAndUnsupportedRule(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(e.SelectEngine("datalog"))
			must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
			if enabled {
				must(g.SourceSetName("fixedpoint.xform.karr", "true"))
			}
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("karr-select-p")
			must(err)
			p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			status, err := e.QueryPredicate(p)
			if !enabled {
				if status != lbool.Undef || !errors.Is(err, ErrKarrDatalogFragment) {
					t.Fatal("disabled registration", status, err)
				}
				return
			}
			must(err)
			if status != lbool.False {
				t.Fatal("empty relation", status)
			}
			one, err := m.ArithmeticIntegerNumeralText("1")
			must(err)
			fact, err := m.MkApp(p, []ast.ASTHandle{one})
			must(err)
			must(e.AddRule(fact, "ground-unsupported"))
			status, err = e.QueryPredicate(p)
			if status != lbool.Undef || !errors.Is(err, ErrKarrDatalogFragment) {
				t.Fatal("unsupported ground rule", status, err)
			}
			if e.karrDatalog.results != nil {
				t.Fatal("failed query published results")
			}
		})
	}
}

func TestDatalogKarrExplicitRepresentation(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("karr-explicit-p")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	must(e.SetPredicateRepresentation(p, []string{"karr_relation"}))
	must(e.AddTableFact(p, []uint32{3}))
	status, err := e.QueryPredicate(p)
	must(err)
	if status != lbool.Undef || e.karrDatalog == nil {
		t.Fatal("explicit kind not selected", status)
	}
}

func TestDatalogKarrRefusesReplacingOwnedArithmeticFacts(t *testing.T) {
	for _, favourite := range []string{"interval_relation", "bound_relation"} {
		t.Run(favourite, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(e.SelectEngine("datalog"))
			must(g.SourceSetName("fixedpoint.datalog.default_relation", favourite))
			must(g.SourceSetName("fixedpoint.xform.karr", "true"))
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("karr-owned-facts")
			must(err)
			p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			must(e.AddTableFact(p, []uint32{7}))
			if err := e.SetPredicateRepresentation(p, []string{"karr_relation"}); !errors.Is(err, ErrKarrDatalogFragment) {
				t.Fatal("switched away from owned facts", err)
			}
			if e.karrDatalog != nil {
				t.Fatal("installed conflicting owner")
			}
			status, err := e.QueryPredicate(p)
			must(err)
			if status != lbool.Undef {
				t.Fatal("original fact lost", status)
			}
			answer, err := e.Answer(p)
			must(err)
			must(m.DelRef(&answer))
		})
	}
}

func TestDatalogKarrRefusesReplacingOwnedDocOrProductFacts(t *testing.T) {
	for _, kind := range []string{"doc", "product"} {
		t.Run(kind, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(e.SelectEngine("datalog"))
			must(g.SourceSetName("fixedpoint.xform.karr", "true"))
			boolean, err := m.MkBoolSort()
			must(err)
			var sorts []ast.ASTHandle
			var fact []uint32
			if kind == "doc" {
				must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
				bv, err := m.BitVectorSort(2)
				must(err)
				sorts, fact = []ast.ASTHandle{bv}, []uint32{1}
			} else {
				family, err := m.RegisterArithmeticPlugin()
				must(err)
				integer, err := m.MkFamilySort(family, 1, nil)
				must(err)
				sorts, fact = []ast.ASTHandle{boolean, integer}, []uint32{1, 7}
			}
			name, err := m.Symbols.Intern("karr-owned-" + kind)
			must(err)
			p, err := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			if kind == "product" {
				must(e.SetPredicateRepresentation(p, []string{"interval_relation", "tr_sparse"}))
			}
			must(e.AddTableFact(p, fact))
			if err := e.SetPredicateRepresentation(p, []string{"karr_relation"}); !errors.Is(err, ErrKarrDatalogFragment) {
				t.Fatal("switched away from owned facts", err)
			}
			status, err := e.QueryPredicate(p)
			must(err)
			if status == lbool.False {
				t.Fatal("original fact lost", status)
			}
			answer, err := e.Answer(p)
			must(err)
			must(m.DelRef(&answer))
		})
	}
}

func TestDatalogKarrLargeFactOverflowDoesNotPublish(t *testing.T) {
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
	decl := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := decl("karr-overflow-p"), decl("karr-overflow-q")
	value, err := m.ArithmeticIntegerNumeralText("9223372036854775808")
	must(err)
	must(e.AddFact(p, []ast.ASTHandle{value}))
	x, err := m.MkVar(0, integer)
	must(err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	qx, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(px, qx)
	must(err)
	must(e.AddRule(rule, "overflow-forward"))
	for i := 0; i < 2; i++ {
		status, err := e.QueryPredicate(q)
		if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || e.karrDatalog.results != nil {
			t.Fatal("overflow must fail without snapshot", status, err)
		}
		matrix, err := e.karrDatalog.seeds[p].Inequalities()
		must(err)
		expected, _ := new(big.Rat).SetString("-9223372036854775808")
		if matrix.Len() != 1 || matrix.B[0].Cmp(expected) != 0 {
			t.Fatal("overflow damaged seed", matrix)
		}
	}
}

func TestDatalogKarrRelationsAnswerApproximateAndCancellation(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("karr-relations-p"), declare("karr-relations-q")
	must(e.AddTableFact(p, nil))
	must(e.AddTableFact(q, nil))
	for _, predicates := range [][]ast.ASTHandle{{p}, {p, q}} {
		status, err := e.QueryPredicates(predicates)
		must(err)
		if status != lbool.Undef {
			t.Fatal("approximate status", status)
		}
		answer, err := e.RelationsAnswer()
		must(err)
		h, err := m.RefGet(answer)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		value, err := m.RefGet(reduced)
		must(err)
		if !m.IsTrue(value) {
			t.Fatal("nonempty nullary conjunction", value)
		}
		must(m.DelRef(&reduced))
		must(m.DelRef(&answer))
	}
	m.Limit.Cancel()
	_, err = e.QueryPredicates([]ast.ASTHandle{p, q})
	m.Limit.ResetCancel()
	if err == nil {
		t.Fatal("cancellation accepted")
	}
	if _, err := e.RelationsAnswer(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("failed query exposed relation answer", err)
	}
}
