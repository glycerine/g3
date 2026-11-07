package smtcore

// Source: src/smt/theory_arith_pp.h. Only the parts with retained callers
// are ported: collect_statistics and the theory name.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (*SMTTheoryArith) Name() string { return "arithmetic" }

func (t *SMTTheoryArith) CollectStatistics(st *statistics.Statistics) error {
	if t == nil || st == nil {
		return ast.ErrASTHandle
	}
	for _, s := range []struct {
		name  statistics.Key
		value uint32
	}{
		{"arith conflicts", t.stats.conflicts},
		{"arith add rows", t.stats.addRows},
		{"arith pivots", t.stats.pivots},
		{"arith assert lower", t.stats.assertLower},
		{"arith assert upper", t.stats.assertUpper},
		{"arith assert diseq", t.stats.assertDiseq},
		{"arith bound prop", t.stats.boundProps},
		{"arith fixed eqs", t.stats.fixedEqs},
		{"arith offset eqs", t.stats.offsetEqs},
		{"arith gcd tests", t.stats.gcdTests},
		{"arith ineq splits", t.stats.branches},
		{"arith gomory cuts", t.stats.gomoryCuts},
		{"arith max-min", t.stats.maxMin},
		{"arith grobner", t.stats.gbComputeBasis},
		{"arith pseudo nonlinear", t.stats.nlLinear},
		{"arith nonlinear bounds", t.stats.nlBounds},
		{"arith nonlinear horner", t.stats.nlCrossNested},
	} {
		st.UpdateUint64(s.name, uint64(s.value))
	}
	// m_arith_eq_adapter.collect_statistics
	st.UpdateUint64("arith eq adapter", t.arithEqAdapter.NumAxioms())
	return nil
}

func (t *SMTTheoryArith) ResetStatistics() {
	t.stats = smtTheoryArithStats{}
	t.arithEqAdapter.ResetStatistics()
}
