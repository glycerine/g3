package arith

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
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

func TestNLA2BVLog2SourceBoundaries(t *testing.T) {
	for denominator := int64(1); denominator <= 17; denominator++ {
		for numerator := int64(1); numerator <= 1024; numerator++ {
			n := new(big.Rat).SetFrac64(numerator, denominator)
			got := nla2BVLog2(n)
			upper := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(got)))
			lower := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(got-1)))
			if n.Cmp(upper) > 0 || got > 1 && n.Cmp(lower) <= 0 {
				t.Fatalf("ceil(log2(%s)) = %d violates power-of-two bounds [%s, %s)", n.RatString(), got, lower, upper)
			}
		}
	}
	for _, n := range []*big.Rat{big.NewRat(0, 1), big.NewRat(-1, 2), big.NewRat(-8, 1)} {
		if got := nla2BVLog2(n); got != 1 {
			t.Fatalf("source minimum for %s = %d, want 1", n, got)
		}
	}
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

func TestNLA2BVReferenceCountClosesAtLastRelease(t *testing.T) {
	r := nla2BVFixture(t, false)
	equationMust(t, r.IncRef())
	equationMust(t, r.IncRef())
	if err := r.DecRef(); err != nil || r.closed {
		t.Fatal("non-final release closed tactic", r.closed, err)
	}
	equationMust(t, r.DecRef())
	if !r.closed {
		t.Fatal("last release did not close tactic")
	}
	if err := r.IncRef(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed tactic accepted another reference", err)
	}
}

func TestNLA2BVParameterDescriptionsMatchSourceOrderAndMetadata(t *testing.T) {
	r := nla2BVFixture(t, false)
	descriptions, err := uparams.NewParamDescrs(r.manager.Symbols, uparams.ParamDescrsConfig{Budget: r.manager.Budget()})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, descriptions.Close()) })
	equationMust(t, r.CollectParamDescrs(descriptions))
	equationMust(t, r.CollectParamDescrs(descriptions))
	size, err := descriptions.Size()
	equationMust(t, err)
	if size != 4 {
		t.Fatalf("descriptor count = %d, want 4", size)
	}
	for index, want := range []struct {
		name, description string
	}{
		{"nla2bv_max_bv_size", "(default: inf) maximum bit-vector size used by nla2bv tactic"},
		{"nla2bv_bv_size", "(default: 4) default bit-vector size used by nla2bv tactic."},
		{"nla2bv_root", "(default: 2) nla2bv tactic encodes reals into bit-vectors using expressions of the form a+b*sqrt(c), this parameter sets the value of c used in the encoding."},
		{"nla2bv_divisor", "(default: 2) nla2bv tactic parameter."},
	} {
		name, e := descriptions.ParamName(uint32(index))
		equationMust(t, e)
		if got, e := r.manager.Symbols.String(name); e != nil || got != want.name {
			t.Fatalf("descriptor %d = %q, want %q: %v", index, got, want.name, e)
		}
		kind, e := descriptions.Kind(name)
		equationMust(t, e)
		if kind != uparams.ParamUInt {
			t.Fatalf("%s kind = %v, want unsigned integer", want.name, kind)
		}
		got, present, e := descriptions.Description(name)
		equationMust(t, e)
		if !present || got != want.description {
			t.Fatalf("%s description = %q, present %v", want.name, got, present)
		}
		if _, present, e = descriptions.Default(name); e != nil || present {
			t.Fatalf("%s unexpectedly has a separate default: %v, %v", want.name, present, e)
		}
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
