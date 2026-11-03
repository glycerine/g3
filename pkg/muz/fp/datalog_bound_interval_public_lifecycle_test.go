package fp

// Source: rel_context.cpp product selection and query ownership;
// dl_context.cpp named-rule replacement and engine lifetime.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func boundIntervalLifecycleDeclaration(t *testing.T, f *boundIntervalPublicFixture, name string, domain []ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	symbol, err := f.m.Symbols.Intern("bound-interval-lifecycle-" + name)
	fragmentMust(t, err)
	declaration := f.hold(f.m.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(t, f.e.RegisterPredicate(declaration))
	return declaration
}

func TestDatalogBoundIntervalPublicMigratesEagerNullaryFact(t *testing.T) {
	f := newBoundIntervalPublicFixture(t, []string{"interval_relation", "bound_relation"})
	unit := boundIntervalLifecycleDeclaration(t, f, "unit", nil)
	fragmentMust(t, f.e.AddFact(unit, nil))
	fragmentMust(t, f.e.AddTableFact(unit, nil))
	if count, err := f.e.predicates[unit].table.Len(); err != nil || count != 1 || f.e.boundIntervalDatalog != nil {
		t.Fatal("fixture did not create one eager finite nullary fact", count, err)
	}
	fragmentMust(t, f.e.SetPredicateRepresentation(unit, f.kinds))
	output := f.declare("nullary-output", 0)
	fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(f.app(unit), f.app(output))), "nullary-copy"))
	for _, declaration := range []ast.ASTHandle{output, unit, output} {
		status, err := f.e.QueryPredicate(declaration)
		fragmentMust(t, err)
		if status != lbool.Undef {
			t.Fatal("eager nullary fact was lost or exposed as a precise table", status)
		}
		answer := f.own(f.e.Answer(declaration))
		f.checkFormula(answer, nil, true)
	}
	if seed := f.e.boundIntervalDatalog.entries[unit].seed; seed.Empty() {
		t.Fatal("nullary tuple was not copied into product seed storage")
	}
	if f.e.predicates[unit].table != nil || f.e.predicates[output].table != nil {
		t.Fatal("product activation left eager finite tables exposed")
	}
	if _, err := f.e.Model(output); !errors.Is(err, ErrBoundIntervalDatalogFragment) {
		t.Fatal("product model used stale finite nullary tables", err)
	}
	// Resetting the selected store must not revive the old eager table tuple
	// when the next public query synchronizes its predicate declarations.
	fragmentMust(t, f.e.boundIntervalDatalog.reset())
	status, err := f.e.QueryPredicate(output)
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("repeated activation replayed a detached eager seed", status)
	}
}

func TestDatalogBoundIntervalPublicRejectsExistingPureOwner(t *testing.T) {
	for _, kind := range []string{"interval_relation", "bound_relation"} {
		t.Run(kind, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
			p := boundIntervalLifecycleDeclaration(t, f, "pure", []ast.ASTHandle{f.integer})
			if kind == "interval_relation" {
				fragmentMust(t, f.e.SetPredicateRepresentation(p, []string{kind}))
			} else {
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.default_relation", kind))
			}
			fragmentMust(t, f.e.AddFact(p, []ast.ASTHandle{f.numeral(7)}))
			interval, bound := f.e.intervalDatalog, f.e.boundDatalog
			if kind == "interval_relation" && interval == nil || kind == "bound_relation" && bound == nil {
				t.Fatal("fixture did not activate selected pure owner")
			}
			before := append([]string(nil), f.e.predicates[p].requestedRepresentations...)
			if err := f.e.SetPredicateRepresentation(p, f.kinds); !errors.Is(err, ErrBoundIntervalDatalogFragment) {
				t.Fatal("coarse pure owner was silently replaced by product", err)
			}
			if f.e.boundIntervalDatalog != nil || f.e.intervalDatalog != interval || f.e.boundDatalog != bound || !boundIntervalKindsEqual(before, f.e.predicates[p].requestedRepresentations) {
				t.Fatal("failed product selection changed prior kind or fact owner")
			}
			status, err := f.e.QueryPredicate(p)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("refused product selection discarded existing pure facts", status)
			}
		})
	}
}

func TestDatalogBoundIntervalPublicKeepsActiveSelectionAndReleasesBackend(t *testing.T) {
	f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
	p := f.declare("stable-selection", 1)
	f.fact(p, 7)
	owner := f.e.boundIntervalDatalog
	if owner == nil {
		t.Fatal("direct fact did not activate product")
	}
	for _, kinds := range [][]string{{"interval_relation", "bound_relation"}, {"interval_relation"}, {"doc"}, {"tr_hashtable"}} {
		if err := f.e.SetPredicateRepresentation(p, kinds); !errors.Is(err, ErrBoundIntervalDatalogFragment) {
			t.Fatal("active product accepted a different ordered selection", kinds, err)
		}
		if f.e.boundIntervalDatalog != owner || !boundIntervalKindsEqual(f.e.predicates[p].requestedRepresentations, f.kinds) || owner.entries[p].seed.Empty() {
			t.Fatal("failed selection replaced product owner, kind, or seeds", kinds)
		}
	}
	fragmentMust(t, f.e.SetPredicateRepresentation(p, f.kinds))
	status, err := f.e.QueryPredicate(p)
	fragmentMust(t, err)
	if status != lbool.Undef || f.e.boundIntervalDatalog != owner {
		t.Fatal("same-kind selection rebuilt or lost active product", status)
	}
	answer := f.own(f.e.Answer(p))
	f.checkFormula(answer, []int{7}, true)
	f.checkFormula(answer, []int{8}, false)
	fragmentMust(t, f.e.SelectEngine("bmc"))
	if f.e.boundIntervalDatalog != nil || !owner.closed || owner.entries != nil {
		t.Fatal("backend change did not release product owner")
	}
	if _, err := f.e.Answer(p); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("backend change exposed old product result", err)
	}
	f.checkFormula(answer, []int{7}, true)
	f.checkFormula(answer, []int{8}, false)
}

func TestDatalogBoundIntervalPublicNamedUpdateKeepsSeeds(t *testing.T) {
	f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
	p, q, gate := f.declare("update-P", 1), f.declare("update-Q", 1), f.declare("update-Gate", 1)
	f.fact(p, 7)
	x := f.hold(f.m.MkVar(0, f.integer))
	px, qx, gx := f.app(p, x), f.app(q, x), f.app(gate, x)
	fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(px, qx)), "product-step"))
	status, err := f.e.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("initial product copy did not derive output", status)
	}
	answer := f.own(f.e.Answer(q))
	f.checkFormula(answer, []int{7}, true)
	owner, source := f.e.boundIntervalDatalog, f.e.set
	body := f.hold(f.m.MkAnd(px, gx))
	fragmentMust(t, f.e.UpdateRule(f.hold(f.m.MkImplies(body, qx)), "product-step"))
	if f.e.set == source || f.e.boundIntervalDatalog != owner || owner.rules != f.e.set || owner.evaluated != nil {
		t.Fatal("named replacement did not rebind and invalidate product evaluation")
	}
	if _, err := f.e.Answer(q); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("named replacement exposed a stale result", err)
	}
	status, err = f.e.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("strengthened rule ignored empty gate", status)
	}
	f.checkFormula(f.own(f.e.Answer(q)), []int{7}, false)
	f.checkFormula(answer, []int{7}, true)
	fragmentMust(t, f.e.AddTableFact(gate, []uint32{7}))
	status, err = f.e.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("named replacement discarded original direct seeds", status)
	}
	current := f.own(f.e.Answer(q))
	f.checkFormula(current, []int{7}, true)
	f.checkFormula(current, []int{8}, false)
}
