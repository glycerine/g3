package portfolio_test

// Source: src/test/pb2bv.cpp test3 and test_solver_semantics.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func TestPB2BVSourceDriverFDSolverExhaustive(t *testing.T) {
	f := enumSolverFixture(t)
	m := f.manager
	vars := pbAtoms(t, m, 4)
	negatives := make([]ast.ASTHandle, 4)
	for i, v := range vars {
		h, e := m.MkNot(v)
		enumMust(t, e)
		enumPin(t, m, h)
		negatives[i] = h
	}
	values := make([]unumeral.OwnedRational, 5)
	for i := range values {
		v, e := m.NumeralService().NewValue()
		enumMust(t, e)
		values[i] = v
		defer func(i int) { enumMust(t, m.NumeralService().Del(&values[i])) }(i)
	}
	for coeff := 0; coeff < 16; coeff++ {
		weights := [4]int64{}
		for i := range weights {
			weights[i] = 2
			if coeff&(1<<i) != 0 {
				weights[i] = 1
			}
			enumMust(t, m.NumeralService().SetInt64(&values[i], weights[i]))
		}
		for bound := int64(0); bound <= 4; bound++ {
			enumMust(t, m.NumeralService().SetInt64(&values[4], bound))
			for kind := 0; kind < 3; kind++ {
				var pb ast.ASTHandle
				var e error
				switch kind {
				case 0:
					pb, e = m.MkPseudoBooleanGE(values[:4], vars, &values[4])
				case 1:
					pb, e = m.MkPseudoBooleanLE(values[:4], vars, &values[4])
				case 2:
					pb, e = m.MkPseudoBooleanEQ(values[:4], vars, &values[4])
				}
				enumMust(t, e)
				enumPin(t, m, pb)
				boolSort, e := m.MkBoolSort()
				enumMust(t, e)
				result, e := m.MkFreshConst("xx", boolSort)
				enumMust(t, e)
				enumPin(t, m, result)
				equal, e := m.MkEq(result, pb)
				enumMust(t, e)
				enumPin(t, m, equal)
				negResult, e := m.MkNot(result)
				enumMust(t, e)
				enumPin(t, m, negResult)
				solver, e := portfolio.NewFDSolver(m, nil, f.services)
				enumMust(t, e)
				enumMust(t, solver.Assert(equal))
				for mask := 0; mask < 16; mask++ {
					enumMust(t, solver.Push())
					sum := int64(0)
					for i, v := range vars {
						if mask&(1<<i) == 0 {
							v = negatives[i]
						} else {
							sum += weights[i]
						}
						enumMust(t, solver.Assert(v))
					}
					status, e := solver.CheckSat(nil)
					enumMust(t, e)
					if status != lbool.True {
						t.Fatalf("coeff %d bound %d kind %d mask %d: base %v", coeff, bound, kind, mask, status)
					}
					truth := sum >= bound
					if kind == 1 {
						truth = sum <= bound
					}
					if kind == 2 {
						truth = sum == bound
					}
					opposite := result
					if truth {
						opposite = negResult
					}
					enumMust(t, solver.Assert(opposite))
					status, e = solver.CheckSat(nil)
					enumMust(t, e)
					if status != lbool.False {
						t.Fatalf("coeff %d bound %d kind %d mask %d sum %d: %v", coeff, bound, kind, mask, sum, status)
					}
					enumMust(t, solver.Pop(1))
				}
				enumMust(t, solver.Close())
			}
		}
	}
}

func TestPB2BVSourceDriverRewriterExhaustive(t *testing.T) {
	f := enumSolverFixture(t)
	m := f.manager
	params, err := f.services.Context.NewValue()
	enumMust(t, err)
	defer func() { enumMust(t, f.services.Context.Del(&params)) }()
	vars := pbAtoms(t, m, 4)
	negatives := make([]ast.ASTHandle, 4)
	for i, v := range vars {
		h, e := m.MkNot(v)
		enumMust(t, e)
		enumPin(t, m, h)
		negatives[i] = h
	}
	values := make([]unumeral.OwnedRational, 5)
	for i := range values {
		v, e := m.NumeralService().NewValue()
		enumMust(t, e)
		values[i] = v
		defer func(i int) { enumMust(t, m.NumeralService().Del(&values[i])) }(i)
	}
	for coeff := 0; coeff < 16; coeff++ {
		weights := [4]int64{}
		for i := range weights {
			weights[i] = 2
			if coeff&(1<<i) != 0 {
				weights[i] = 1
			}
			enumMust(t, m.NumeralService().SetInt64(&values[i], weights[i]))
		}
		for bound := int64(0); bound <= 4; bound++ {
			enumMust(t, m.NumeralService().SetInt64(&values[4], bound))
			for kind := 0; kind < 3; kind++ {
				var pb ast.ASTHandle
				var e error
				switch kind {
				case 0:
					pb, e = m.MkPseudoBooleanGE(values[:4], vars, &values[4])
				case 1:
					pb, e = m.MkPseudoBooleanLE(values[:4], vars, &values[4])
				case 2:
					pb, e = m.MkPseudoBooleanEQ(values[:4], vars, &values[4])
				}
				enumMust(t, e)
				enumPin(t, m, pb)
				rw, e := rewriter.NewPB2BVRewriter(m, f.services.Context, &params)
				enumMust(t, e)
				out, e := m.NewRef(ast.ASTHandle{})
				enumMust(t, e)
				enumMust(t, rw.Rewrite(pb, out, ast.ASTRef{}))
				encoded, e := m.RefGet(out)
				enumMust(t, e)
				opposite, e := m.MkNot(encoded)
				enumMust(t, e)
				enumPin(t, m, opposite)
				clauses, e := rw.FlushSideConstraints()
				enumMust(t, e)
				s, e := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: f.services.Context, Globals: f.services.Globals, ModelEvaluator: f.services})
				enumMust(t, e)
				for _, clause := range clauses {
					enumMust(t, s.Assert(clause))
					enumMust(t, m.DecRef(clause))
				}
				for mask := 0; mask < 16; mask++ {
					enumMust(t, s.Push())
					sum := int64(0)
					for i, v := range vars {
						if mask&(1<<i) == 0 {
							v = negatives[i]
						} else {
							sum += weights[i]
						}
						enumMust(t, s.Assert(v))
					}
					status, e := s.CheckSat(nil)
					enumMust(t, e)
					if status != lbool.True {
						t.Fatalf("coeff %d bound %d kind %d mask %d: base %v", coeff, bound, kind, mask, status)
					}
					truth := sum >= bound
					if kind == 1 {
						truth = sum <= bound
					}
					if kind == 2 {
						truth = sum == bound
					}
					contradict := encoded
					if truth {
						contradict = opposite
					}
					enumMust(t, s.Assert(contradict))
					status, e = s.CheckSat(nil)
					enumMust(t, e)
					if status != lbool.False {
						t.Fatalf("coeff %d bound %d kind %d mask %d sum %d: %v", coeff, bound, kind, mask, sum, status)
					}
					enumMust(t, s.Pop(1))
				}
				enumMust(t, s.Close())
				enumMust(t, m.DelRef(&out))
				enumMust(t, rw.Close())
			}
		}
	}
}

func TestPB2BVSourceDriverCardinality(t *testing.T) {
	f := enumSolverFixture(t)
	m := f.manager
	vars := pbAtoms(t, m, 5)
	params, err := f.services.Context.NewValue()
	enumMust(t, err)
	defer func() { enumMust(t, f.services.Context.Del(&params)) }()
	rw, err := rewriter.NewPB2BVRewriter(m, f.services.Context, &params)
	enumMust(t, err)
	defer func() { enumMust(t, rw.Close()) }()
	out, err := m.NewRef(ast.ASTHandle{})
	enumMust(t, err)
	defer func() { enumMust(t, m.DelRef(&out)) }()
	for k := uint32(1); k <= 5; k++ {
		h, e := m.MkPseudoBooleanAtLeast(vars, k)
		enumMust(t, e)
		enumPin(t, m, h)
		enumMust(t, rw.Rewrite(h, out, ast.ASTRef{}))
		_, e = m.RefGet(out)
		enumMust(t, e)
	}
	clauses, err := rw.FlushSideConstraints()
	enumMust(t, err)
	for _, h := range clauses {
		enumMust(t, m.DecRef(h))
	}
}
