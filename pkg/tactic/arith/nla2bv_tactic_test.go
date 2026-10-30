package arith

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func nla2BVFixture(t *testing.T, proofs bool) *NLA2BVTactic {
	t.Helper()
	base, m := fixDLFixture(t, proofs)
	r, err := NewNLA2BVTactic(m, base.context, base.globals, nil, base.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	return r
}

func nla2BVGoalWidth(t *testing.T, r *NLA2BVTactic, number string) uint32 {
	t.Helper()
	m := r.manager
	x := fixDLConst(t, m, true, "width_x")
	eq, err := m.MkEq(fixDLApp(t, m, 9, x, x), fixDLNumber(t, m, true, number))
	equationMust(t, err)
	fixDLPin(t, m, eq)
	g := fixDLGoal(t, m, false, true, false, eq)
	var out tactic.TacticResult
	t.Cleanup(func() { equationMust(t, out.Close()) })
	equationMust(t, r.Apply(g, &out))
	c := &nla2BVInvocation{t: r}
	terms, err := c.postorder(g)
	equationMust(t, err)
	for _, h := range terms {
		kind, err := m.GetKind(h)
		equationMust(t, err)
		if kind != ast.ASTApp {
			continue
		}
		family, err := m.GetFamilyID(h)
		equationMust(t, err)
		if family != ast.NullFamilyID {
			continue
		}
		sort, err := m.GetSort(h)
		equationMust(t, err)
		bv, err := m.IsBitVectorSort(sort)
		equationMust(t, err)
		if bv {
			width, err := m.BitVectorSize(h)
			equationMust(t, err)
			return width
		}
	}
	t.Fatal("missing generated BV constant")
	return 0
}

func TestNLA2BVParametersFreshInvocationsCleanupAndTranslation(t *testing.T) {
	r := nla2BVFixture(t, false)
	p, err := r.context.NewValue()
	equationMust(t, err)
	defer func() { equationMust(t, r.context.Del(&p)) }()
	key := "nla2bv_bv_size"
	equationMust(t, r.context.SetUIntName(&p, &key, 2))
	equationMust(t, r.UpdateParams(&p))
	equationMust(t, r.context.SetUIntName(&p, &key, 3))
	if got := nla2BVGoalWidth(t, r, "0"); got != 2 {
		t.Fatal("parameter snapshot", got)
	}
	if got := nla2BVGoalWidth(t, r, "8"); got != 4 {
		t.Fatal("integer constant width growth", got)
	}
	equationMust(t, r.Cleanup())
	if got := nla2BVGoalWidth(t, r, "0"); got != 2 {
		t.Fatal("invocation state or cleanup changed params", got)
	}
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: r.manager.Symbols, Numerals: r.manager.NumeralService(), Budget: r.manager.Budget()})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, m.Close()) })
	translated, err := r.Translate(m)
	equationMust(t, err)
	other := translated.(*NLA2BVTactic)
	t.Cleanup(func() { equationMust(t, other.Close()) })
	if got := nla2BVGoalWidth(t, other, "0"); got != 2 {
		t.Fatal("translation lost updated snapshot", got)
	}
	copy := *r
	if err := copy.Cleanup(); err == nil {
		t.Fatal("copied tactic accepted")
	}
}

func TestNLA2BVProofAndCoreGuardsPreserveResult(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		r := nla2BVFixture(t, proofs)
		m := r.manager
		x := fixDLConst(t, m, true, "guard_x")
		g := fixDLGoal(t, m, proofs, true, !proofs, fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "0")))
		sentinel := fixDLGoal(t, m, false, false, false)
		var result tactic.TacticResult
		t.Cleanup(func() { equationMust(t, result.Close()) })
		equationMust(t, result.Prepare(m))
		equationMust(t, result.AddGoal(sentinel))
		if err := r.Apply(g, &result); err == nil {
			t.Fatal("proof/core production accepted")
		}
		if len(result.Goals) != 1 || result.Goals[0] != sentinel {
			t.Fatal("guard reset prior result")
		}
		if r.active != nil || len(r.pending) != 0 {
			t.Fatal("guard constructed algorithm state")
		}
	}
}

func TestNLA2BVConstructorAndApplicationAllocationPrefixes(t *testing.T) {
	lastApplicationRefusal := int64(-1)
	for _, phase := range []string{"constructor", "apply"} {
		stops := []int64{0, 1, 2, 3, 4, 8, 16, 32, 64, 128, 256, 512, -1}
		if phase == "apply" {
			stops = make([]int64, 259)
			for i := int64(0); i <= 257; i++ {
				stops[i] = i
			}
			stops[258] = -1
		}
		for _, stop := range stops {
			t.Run(fmt.Sprintf("%s/%d", phase, stop), func(t *testing.T) {
				r := nla2BVFixture(t, false)
				m := r.manager
				if phase == "constructor" {
					m.Budget().FailAfter(stop)
					fresh, err := NewNLA2BVTactic(m, r.context, r.globals, &r.params, r.services)
					m.Budget().FailAfter(-1)
					if err != nil && !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
					if fresh != nil {
						equationMust(t, fresh.Close())
					}
					return
				}
				x := fixDLConst(t, m, true, "failure_x")
				g := fixDLGoal(t, m, false, true, false, fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "0")))
				var result tactic.TacticResult
				t.Cleanup(func() { equationMust(t, result.Close()) })
				m.Budget().FailAfter(stop)
				err := r.Apply(g, &result)
				m.Budget().FailAfter(-1)
				if errors.Is(err, arena.ErrInjectedFailure) {
					lastApplicationRefusal = stop
				}
				if err != nil && !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
				if r.active != nil {
					t.Fatal("scoped implementation pointer retained")
				}
				equationMust(t, r.Cleanup())
				if len(r.pending) != 0 {
					t.Fatal("invocation cleanup retained")
				}
				if errors.Is(err, arena.ErrInjectedFailure) {
					retryX := fixDLConst(t, m, true, "failure_retry_x")
					retry := fixDLGoal(t, m, false, true, false, fixDLApp(t, m, 3, retryX, fixDLNumber(t, m, true, "0")))
					var retryResult tactic.TacticResult
					if retryErr := r.Apply(retry, &retryResult); retryErr != nil {
						t.Fatal("application did not recover after allocation refusal", retryErr)
					}
					equationMust(t, retryResult.Close())
				}
			})
		}
	}
	if lastApplicationRefusal < 8 {
		t.Fatalf("allocation refusal sweep reached only boundary %d", lastApplicationRefusal)
	}
}
