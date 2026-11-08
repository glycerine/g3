package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestKarrDatalogIdentitySnapshotAndGuards(t *testing.T) {
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
	declare := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("karr-activation-p"), declare("karr-activation-q")
	x, err := m.MkVar(0, integer)
	must(err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	qx, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(px, qx)
	must(err)
	must(e.addRuleEager(rule, "karr-forward"))
	must(e.flushPendingRules())
	a, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q})
	must(err)
	defer func() { must(a.close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	must(a.addFact(p, []ast.ASTHandle{one}))
	status, err := a.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef || a.results[q].Empty() {
		t.Fatal("imprecise nonempty result", status)
	}
	matrix, err := a.results[q].Inequalities()
	must(err)
	for _, sample := range []struct {
		value int64
		want  bool
	}{{1, true}, {2, false}} {
		matches := true
		for i, row := range matrix.A {
			value := new(big.Rat).Mul(&row[0], big.NewRat(sample.value, 1))
			value.Add(value, &matrix.B[i])
			if matrix.Eq[i] && value.Sign() != 0 || !matrix.Eq[i] && value.Sign() < 0 {
				matches = false
			}
		}
		if matches != sample.want {
			t.Fatal("forwarded source fact", sample.value, matrix)
		}
	}
	saved := a.results[q].Clone()
	if err := a.addFact(p, []ast.ASTHandle{one}); !errors.Is(err, ErrKarrDatalogFragment) {
		t.Fatal("second direct fact must refuse", err)
	}
	clone, err := a.clone(e.set)
	must(err)
	defer func() { must(clone.close()) }()
	must(a.reset())
	status, err = a.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.False || saved.Empty() {
		t.Fatal("snapshot reset", status)
	}
	status, err = clone.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("clone lost seeds", status)
	}
	m.Limit.Cancel()
	status, err = clone.query([]ast.ASTHandle{q})
	m.Limit.ResetCancel()
	if err == nil || status != lbool.Undef || clone.results != nil {
		t.Fatal("cancel published result", status, err)
	}
	status, err = clone.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry", status)
	}
	// Cancellation fires inside Hilbert dualization, after the query's entry
	// checkpoint and before Union returns its source-compatible undefined basis.
	fired := false
	seed := rel.NewKarrEmpty(1, func() bool { fired = true; m.Limit.Cancel(); return false })
	must(seed.AddFact([]*big.Rat{big.NewRat(1, 1)}))
	clone.seeds[p] = seed
	status, err = clone.query([]ast.ASTHandle{q})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || clone.results != nil {
		t.Fatal("mid-dualization cancellation published snapshot", fired, status, err)
	}
	clone.seeds[p] = a.seeds[p].Clone()
	must(clone.seeds[p].AddFact([]*big.Rat{big.NewRat(1, 1)}))
	status, err = clone.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef || clone.results[q].Empty() {
		t.Fatal("dualization cancellation retry", status)
	}

	back, err := m.MkImplies(qx, px)
	must(err)
	must(e.addRuleEager(back, "karr-cycle"))
	must(e.flushPendingRules())
	status, err = clone.query([]ast.ASTHandle{q})
	if status != lbool.Undef || !errors.Is(err, ErrKarrDatalogFragment) || clone.results != nil {
		t.Fatal("recursive program must refuse without publishing", status, err)
	}

}
