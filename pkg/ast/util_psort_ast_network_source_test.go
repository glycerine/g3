package ast

// Source: src/util/sorting_network.h:120-1046; src/test/sorting_network.cpp:184-325.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"testing"
)

// This finite propositional oracle evaluates actual emitted AST terms. It is
// independent of the sorting/cardinality implementation and supplies no solver
// behavior to production. Registered SMT-kernel checks remain separate.
func psortOracleEval(t *testing.T, m *ASTManager, h ASTHandle, env map[ASTHandle]bool) bool {
	t.Helper()
	if b, ok := env[h]; ok {
		return b
	}
	r, e := m.internal(h)
	astTestMust(t, e)
	if m.nodeAt(r.slot).kind != ASTApp {
		t.Fatal("non-Boolean app oracle input")
	}
	app := m.appAt(r.slot)
	info, e := m.funcInfo(m.declAt(app.decl.slot).info)
	astTestMust(t, e)
	child := func(i uint32) bool {
		r, e := m.readInternal(app.args, i)
		astTestMust(t, e)
		return psortOracleEval(t, m, m.external(r), env)
	}
	if info.family == 0 {
		switch info.kind {
		case ASTOpTrue:
			return true
		case ASTOpFalse:
			return false
		case ASTOpNot:
			return !child(0)
		case ASTOpAnd:
			for i := uint32(0); i < app.count; i++ {
				if !child(i) {
					return false
				}
			}
			return true
		case ASTOpOr:
			for i := uint32(0); i < app.count; i++ {
				if child(i) {
					return true
				}
			}
			return false
		case ASTOpImplies:
			return !child(0) || child(1)
		case ASTOpIte:
			if child(0) {
				return child(1)
			}
			return child(2)
		}
	}
	t.Fatal("unassigned actual fresh variable", h)
	return false
}
func psortOracleVariables(t *testing.T, m *ASTManager, h ASTHandle, seen map[ASTHandle]bool, vars *[]ASTHandle) {
	t.Helper()
	if seen[h] {
		return
	}
	seen[h] = true
	r, e := m.internal(h)
	astTestMust(t, e)
	app := m.appAt(r.slot)
	if app.count == 0 {
		info, e := m.funcInfo(m.declAt(app.decl.slot).info)
		astTestMust(t, e)
		if info.family != 0 || info.kind != ASTOpTrue && info.kind != ASTOpFalse {
			*vars = append(*vars, h)
		}
		return
	}
	for i := uint32(0); i < app.count; i++ {
		r, e := m.readInternal(app.args, i)
		astTestMust(t, e)
		psortOracleVariables(t, m, m.external(r), seen, vars)
	}
}
func TestPsortASTNetworkFiniteCardinalityClauseSemantics(t *testing.T) {
	for n := uint32(0); n <= 4; n++ {
		for k := uint32(0); k <= n+1; k++ {
			for mode := 0; mode < 3; mode++ {
				for _, full := range []bool{false, true} {
					m := astTestManager(t)
					c, e := NewPsortASTContext(m)
					astTestMust(t, e)
					r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
					astTestMust(t, e)
					s, e := NewPsortASTNetwork(c, r, m.BudgetField, nil)
					astTestMust(t, e)
					in, e := NewASTLiteralVector(m.BudgetField)
					astTestMust(t, e)
					var inputs []ASTHandle
					sort, e := m.MkBoolSort()
					astTestMust(t, e)
					for i := uint32(0); i < n; i++ {
						h, e := m.MkFreshConst("a", sort)
						astTestMust(t, e)
						astTestPin(t, m, h)
						inputs = append(inputs, h)
						astTestMust(t, in.Push(h))
					}
					span, e := in.Span()
					astTestMust(t, e)
					var result ASTHandle
					switch mode {
					case 0:
						result, e = s.Ge(full, k, n, span)
					case 1:
						result, e = s.Le(full, k, n, span)
					case 2:
						result, e = s.Eq(full, k, n, span)
					}
					if n == 0 && k == 0 && mode == 2 {
						// Source eq(0,0) reaches out[0] on an empty vector;
						// this is a reached invalid-index domain, not true.
						if e == nil {
							t.Fatal("source empty Eq must reach actual index guard")
						}
						astTestMust(t, s.Close())
						astTestMust(t, in.Close())
						astTestMust(t, r.Close())
						astTestMust(t, c.Close())
						continue
					}
					if e != nil {
						t.Fatal("source bound call", n, k, mode, full, e)
					}
					var clauses, vars []ASTHandle
					seen := map[ASTHandle]bool{}
					psortOracleVariables(t, m, result, seen, &vars)
					for _, h := range c.clauses {
						clauses = append(clauses, h)
						psortOracleVariables(t, m, h, seen, &vars)
					}
					inputSet := map[ASTHandle]bool{}
					for _, h := range inputs {
						inputSet[h] = true
					}
					var aux []ASTHandle
					for _, h := range vars {
						if !inputSet[h] {
							aux = append(aux, h)
						}
					}
					if len(aux) > 16 {
						t.Fatal("bounded independent oracle extent", n, k, mode, full, len(aux))
					}
					for bits := uint32(0); bits < 1<<n; bits++ {
						env := map[ASTHandle]bool{}
						pop := uint32(0)
						for i, h := range inputs {
							env[h] = bits&(1<<i) != 0
							if env[h] {
								pop++
							}
						}
						positive, negative := false, false
						for a := uint32(0); a < 1<<len(aux); a++ {
							for i, h := range aux {
								env[h] = a&(1<<i) != 0
							}
							valid := true
							for _, h := range clauses {
								if !psortOracleEval(t, m, h, env) {
									valid = false
									break
								}
							}
							if valid {
								if psortOracleEval(t, m, result, env) {
									positive = true
								} else {
									negative = true
								}
							}
						}
						want := pop >= k
						if mode == 1 {
							want = pop <= k
						} else if mode == 2 {
							want = pop == k
						}
						if positive != want || full && negative == want {
							t.Fatal("independent existential clause semantics", n, k, mode, full, bits, pop, positive, negative, want)
						}
					}
					astTestMust(t, s.Close())
					astTestMust(t, in.Close())
					astTestMust(t, r.Close())
					astTestMust(t, c.Close())
				}
			}
		}
	}
}
