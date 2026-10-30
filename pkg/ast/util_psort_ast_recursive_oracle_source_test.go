package ast

// Source: src/util/sorting_network.h:190-260,597-1046.

import (
	"fmt"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"testing"
)

// The test oracle independently lowers actual Boolean AST gates into their
// truth-table CNF and exhausts assignments with unit propagation. It has no
// cardinality/sorting knowledge and is not a production solver substitution.
type psortOracleCNF struct {
	ids     map[ASTHandle]int
	clauses [][]int
	count   int
}

func (q *psortOracleCNF) gate(t *testing.T, m *ASTManager, h ASTHandle) int {
	if id, ok := q.ids[h]; ok {
		return id
	}
	q.count++
	id := q.count
	q.ids[h] = id
	r, e := m.internal(h)
	astTestMust(t, e)
	app := m.appAt(r.slot)
	info, e := m.funcInfo(m.declAt(app.decl.slot).info)
	astTestMust(t, e)
	var args []int
	for i := uint32(0); i < app.count; i++ {
		r, e := m.readInternal(app.args, i)
		astTestMust(t, e)
		args = append(args, q.gate(t, m, m.external(r)))
	}
	if info.family != 0 {
		if len(args) != 0 {
			t.Fatal("oracle unsupported nonbasic compound")
		}
		return id
	}
	switch info.kind {
	case ASTOpTrue:
		q.clauses = append(q.clauses, []int{id})
	case ASTOpFalse:
		q.clauses = append(q.clauses, []int{-id})
	case ASTOpNot:
		if len(args) != 1 {
			t.Fatal("oracle NOT arity")
		}
		q.clauses = append(q.clauses, []int{-id, -args[0]}, []int{id, args[0]})
	case ASTOpAnd:
		reverse := []int{id}
		for _, a := range args {
			q.clauses = append(q.clauses, []int{-id, a})
			reverse = append(reverse, -a)
		}
		q.clauses = append(q.clauses, reverse)
	case ASTOpOr:
		reverse := []int{-id}
		for _, a := range args {
			q.clauses = append(q.clauses, []int{id, -a})
			reverse = append(reverse, a)
		}
		q.clauses = append(q.clauses, reverse)
	default:
		t.Fatal("oracle unsupported basic operation", info.kind)
	}
	return id
}
func psortOracleSAT(clauses [][]int, values []int8) bool {
	for {
		changed := false
		for _, clause := range clauses {
			satisfied := false
			unknown, last := 0, 0
			for _, lit := range clause {
				id := lit
				if id < 0 {
					id = -id
				}
				v := values[id]
				if v == 0 {
					unknown++
					last = lit
				} else if v > 0 && lit > 0 || v < 0 && lit < 0 {
					satisfied = true
					break
				}
			}
			if satisfied {
				continue
			}
			if unknown == 0 {
				return false
			}
			if unknown == 1 {
				id := last
				v := int8(1)
				if id < 0 {
					id = -id
					v = -1
				}
				if values[id] == 0 {
					values[id] = v
					changed = true
				} else if values[id] != v {
					return false
				}
			}
		}
		if !changed {
			break
		}
	}
	pick := 0
	for _, clause := range clauses {
		satisfied := false
		candidate := 0
		for _, lit := range clause {
			id := lit
			if id < 0 {
				id = -id
			}
			v := values[id]
			if v > 0 && lit > 0 || v < 0 && lit < 0 {
				satisfied = true
				break
			}
			if v == 0 && candidate == 0 {
				candidate = id
			}
		}
		if !satisfied && candidate != 0 {
			pick = candidate
			break
		}
	}
	if pick == 0 {
		return true
	}
	next := append([]int8(nil), values...)
	next[pick] = 1
	if psortOracleSAT(clauses, next) {
		return true
	}
	next = append([]int8(nil), values...)
	next[pick] = -1
	return psortOracleSAT(clauses, next)
}
func TestPsortASTNetworkRecursiveFamiliesIndependentClauseOracle(t *testing.T) {
	cases := [][2]uint32{{5, 1}, {5, 2}, {10, 3}, {10, 5}, {12, 4}, {12, 10}}
	for _, tc := range cases {
		for mode := 0; mode < 3; mode++ {
			for _, full := range []bool{false, true} {
				n, k := tc[0], tc[1]
				m := astTestManager(t)
				c, e := NewPsortASTContext(m)
				astTestMust(t, e)
				r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
				astTestMust(t, e)
				s, e := NewPsortASTNetwork(c, r, m.BudgetField, nil)
				astTestMust(t, e)
				in, e := NewASTLiteralVector(m.BudgetField)
				astTestMust(t, e)
				sort, e := m.MkBoolSort()
				astTestMust(t, e)
				var inputs []ASTHandle
				for i := uint32(0); i < n; i++ {
					h, e := m.MkFreshConst("a", sort)
					astTestMust(t, e)
					astTestPin(t, m, h)
					inputs = append(inputs, h)
					astTestMust(t, in.Push(h))
				}
				p, e := in.Span()
				astTestMust(t, e)
				var result ASTHandle
				switch mode {
				case 0:
					result, e = s.Ge(full, k, n, p)
				case 1:
					result, e = s.Le(full, k, n, p)
				case 2:
					result, e = s.Eq(full, k, n, p)
				}
				if e != nil {
					t.Fatal(n, k, mode, full, e)
				}
				q := psortOracleCNF{ids: map[ASTHandle]int{}}
				rid := q.gate(t, m, result)
				for _, h := range c.clauses {
					id := q.gate(t, m, h)
					q.clauses = append(q.clauses, []int{id})
				}
				for _, h := range inputs {
					q.gate(t, m, h)
				}
				for pop := uint32(0); pop <= n; pop++ {
					values := make([]int8, q.count+1)
					for i, h := range inputs {
						values[q.ids[h]] = -1
						if uint32(i) < pop {
							values[q.ids[h]] = 1
						}
					}
					want := pop >= k
					if mode == 1 {
						want = pop <= k
					} else if mode == 2 {
						want = pop == k
					}
					yes := append([]int8(nil), values...)
					yes[rid] = 1
					got := psortOracleSAT(q.clauses, yes)
					if got != want {
						t.Fatal("recursive existential oracle", n, k, mode, full, pop, got, want)
					}
					if full {
						no := append([]int8(nil), values...)
						no[rid] = -1
						if got := psortOracleSAT(q.clauses, no); got == want {
							t.Fatal("recursive negative oracle", n, k, mode, pop, got, want)
						}
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

func TestPsortASTNetworkRegisteredOddDriverCampaign(t *testing.T) {
	// src/test/sorting_network.cpp runs test_sorting5 for every odd n from 3
	// through 19 and every interior k, in LE/EQ/GE order. Check the emitted
	// clauses with the independent SAT oracle under every input population.
	for n := uint32(3); n < 20; n += 2 {
		for k := uint32(1); k < n; k++ {
			for _, mode := range []struct {
				name string
				kind int
				full bool
			}{{"le", 1, false}, {"eq", 2, true}, {"ge", 0, false}} {
				t.Run(fmt.Sprintf("n=%d/k=%d/%s", n, k, mode.name), func(t *testing.T) {
					m := astTestManager(t)
					c, err := NewPsortASTContext(m)
					astTestMust(t, err)
					r, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
					astTestMust(t, err)
					network, err := NewPsortASTNetwork(c, r, m.BudgetField, nil)
					astTestMust(t, err)
					input, err := NewASTLiteralVector(m.BudgetField)
					astTestMust(t, err)
					sort, err := m.MkBoolSort()
					astTestMust(t, err)
					inputs := make([]ASTHandle, 0, n)
					for i := uint32(0); i < n; i++ {
						h, err := m.MkFreshConst("a", sort)
						astTestMust(t, err)
						astTestPin(t, m, h)
						inputs = append(inputs, h)
						astTestMust(t, input.Push(h))
					}
					span, err := input.Span()
					astTestMust(t, err)
					var result ASTHandle
					switch mode.kind {
					case 0:
						result, err = network.Ge(mode.full, k, n, span)
					case 1:
						result, err = network.Le(mode.full, k, n, span)
					case 2:
						result, err = network.Eq(mode.full, k, n, span)
					}
					if err != nil {
						t.Fatal("registered source bound", err)
					}
					oracle := psortOracleCNF{ids: map[ASTHandle]int{}}
					resultID := oracle.gate(t, m, result)
					for _, clause := range c.clauses {
						id := oracle.gate(t, m, clause)
						oracle.clauses = append(oracle.clauses, []int{id})
					}
					for _, h := range inputs {
						oracle.gate(t, m, h)
					}
					for population := uint32(0); population <= n; population++ {
						values := make([]int8, oracle.count+1)
						for index, h := range inputs {
							values[oracle.ids[h]] = -1
							if uint32(index) < population {
								values[oracle.ids[h]] = 1
							}
						}
						want := population <= k
						if mode.kind == 0 {
							want = population >= k
						} else if mode.kind == 2 {
							want = population == k
						}
						yes := append([]int8(nil), values...)
						yes[resultID] = 1
						if got := psortOracleSAT(oracle.clauses, yes); got != want {
							t.Fatalf("source positive bound: population=%d got=%v want=%v", population, got, want)
						}
						if mode.full {
							no := append([]int8(nil), values...)
							no[resultID] = -1
							if got := psortOracleSAT(oracle.clauses, no); got == want {
								t.Fatalf("source full equivalence: population=%d false-result satisfiable=%v want=%v", population, got, !want)
							}
						}
					}
					astTestMust(t, network.Close())
					astTestMust(t, input.Close())
					astTestMust(t, r.Close())
					astTestMust(t, c.Close())
				})
			}
		}
	}
}

func TestPsortASTNetworkRegisteredAtMostOneCampaign(t *testing.T) {
	// src/test/sorting_network.cpp runs both full modes for n=1..16. In full
	// mode it proves equivalence to the pairwise formula; for n<9 it then
	// checks every assignment. In one-way mode it checks every violating
	// assignment by asserting the encoded bound and expecting UNSAT.
	for n := uint32(1); n <= 16; n++ {
		for _, full := range []bool{true, false} {
			t.Run(fmt.Sprintf("n=%d/full=%v", n, full), func(t *testing.T) {
				m := astTestManager(t)
				c, err := NewPsortASTContext(m)
				astTestMust(t, err)
				r, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
				astTestMust(t, err)
				network, err := NewPsortASTNetwork(c, r, m.BudgetField, nil)
				astTestMust(t, err)
				input, err := NewASTLiteralVector(m.BudgetField)
				astTestMust(t, err)
				sort, err := m.MkBoolSort()
				astTestMust(t, err)
				inputs := make([]ASTHandle, 0, n)
				for i := uint32(0); i < n; i++ {
					h, err := m.MkFreshConst("a", sort)
					astTestMust(t, err)
					astTestPin(t, m, h)
					inputs = append(inputs, h)
					astTestMust(t, input.Push(h))
				}
				span, err := input.Span()
				astTestMust(t, err)
				result, err := network.Le(full, 1, n, span)
				astTestMust(t, err)
				astTestPin(t, m, result)
				oracle := psortOracleCNF{ids: map[ASTHandle]int{}}
				resultID := oracle.gate(t, m, result)
				for _, clause := range c.clauses {
					id := oracle.gate(t, m, clause)
					oracle.clauses = append(oracle.clauses, []int{id})
				}
				for _, h := range inputs {
					oracle.gate(t, m, h)
				}

				if full {
					// This is the source's solver proof that result1 and
					// naive_at_most1 agree for every assignment.
					var pairClauses []ASTHandle
					for i := 0; i < len(inputs); i++ {
						for j := i + 1; j < len(inputs); j++ {
							both, err := m.MkAnd(inputs[i], inputs[j])
							astTestMust(t, err)
							astTestPin(t, m, both)
							notBoth, err := m.MkNot(both)
							astTestMust(t, err)
							astTestPin(t, m, notBoth)
							pairClauses = append(pairClauses, notBoth)
						}
					}
					naive, err := m.MkAndArgs(pairClauses)
					astTestMust(t, err)
					astTestPin(t, m, naive)
					naiveID := oracle.gate(t, m, naive)
					// (result OR naive) AND (!result OR !naive) is true
					// exactly when the two Boolean results differ.
					oracle.clauses = append(oracle.clauses, []int{resultID, naiveID}, []int{-resultID, -naiveID})
					if psortOracleSAT(oracle.clauses, make([]int8, oracle.count+1)) {
						t.Fatal("full at-most-one encoding differs from pairwise source formula")
					}
				} else if n < 9 {
					// The registered source loop skips satisfying assignments and
					// verifies only that every population above one is rejected.
					for bits := uint32(0); bits < 1<<n; bits++ {
						population := 0
						values := make([]int8, oracle.count+1)
						for i, h := range inputs {
							id := oracle.ids[h]
							values[id] = -1
							if bits&(1<<i) != 0 {
								values[id] = 1
								population++
							}
						}
						if population <= 1 {
							continue
						}
						values[resultID] = 1
						if psortOracleSAT(oracle.clauses, values) {
							t.Fatalf("source one-way bound accepted mask %#x with population %d", bits, population)
						}
					}
				}
				astTestMust(t, network.Close())
				astTestMust(t, input.Close())
				astTestMust(t, r.Close())
				astTestMust(t, c.Close())
			})
		}
	}
}

func TestPsortASTNetworkRegisteredEvenEQOneCampaign(t *testing.T) {
	// The C++ driver separately checks test_sorting_eq(n, 1) for every n=2..19.
	// Odd sizes are also covered by the full semantic campaign above; exercise
	// the missing even-size solver assertions here in their source order.
	for n := uint32(2); n < 20; n += 2 {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			m := astTestManager(t)
			c, err := NewPsortASTContext(m)
			astTestMust(t, err)
			r, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
			astTestMust(t, err)
			network, err := NewPsortASTNetwork(c, r, m.BudgetField, nil)
			astTestMust(t, err)
			input, err := NewASTLiteralVector(m.BudgetField)
			astTestMust(t, err)
			sort, err := m.MkBoolSort()
			astTestMust(t, err)
			inputs := make([]ASTHandle, 0, n)
			for i := uint32(0); i < n; i++ {
				h, err := m.MkFreshConst("a", sort)
				astTestMust(t, err)
				astTestPin(t, m, h)
				inputs = append(inputs, h)
				astTestMust(t, input.Push(h))
			}
			span, err := input.Span()
			astTestMust(t, err)
			result, err := network.Eq(true, 1, n, span)
			astTestMust(t, err)
			astTestPin(t, m, result)
			oracle := psortOracleCNF{ids: map[ASTHandle]int{}}
			resultID := oracle.gate(t, m, result)
			for _, clause := range c.clauses {
				id := oracle.gate(t, m, clause)
				oracle.clauses = append(oracle.clauses, []int{id})
			}
			for _, h := range inputs {
				oracle.gate(t, m, h)
			}
			base := make([]int8, oracle.count+1)
			base[resultID] = 1
			if !psortOracleSAT(oracle.clauses, append([]int8(nil), base...)) {
				t.Fatal("source equality bound is not satisfiable")
			}
			first := append([]int8(nil), base...)
			first[oracle.ids[inputs[0]]] = 1
			if !psortOracleSAT(oracle.clauses, first) {
				t.Fatal("source one-true prefix should remain satisfiable")
			}
			disallowed := append([]int8(nil), first...)
			disallowed[oracle.ids[inputs[1]]] = 1
			if psortOracleSAT(oracle.clauses, disallowed) {
				t.Fatal("source two-true prefix should be rejected")
			}
			astTestMust(t, network.Close())
			astTestMust(t, input.Close())
			astTestMust(t, r.Close())
			astTestMust(t, c.Close())
		})
	}
}

func TestASTBooleanSortingNetworkRegisteredSourceDisplay(t *testing.T) {
	// test_sorting3 invokes the expr* extension over seven fresh Boolean
	// constants and prints the resulting comparator network. Check that same
	// operation's outputs for every assignment; stream formatting is diagnostic.
	const n = 7
	m := astTestManager(t)
	network, err := NewASTBooleanSortingNetwork(m)
	astTestMust(t, err)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	var inputs []ASTHandle
	for i := uint32(0); i < n; i++ {
		h, err := m.MkFreshConst("a", sort)
		astTestMust(t, err)
		astTestPin(t, m, h)
		inputs = append(inputs, h)
	}
	output, err := network.Sort(inputs)
	astTestMust(t, err)
	if len(output) != 8 {
		t.Fatalf("source power-of-two output size=%d want=8", len(output))
	}
	for bits := uint32(0); bits < 1<<n; bits++ {
		env := make(map[ASTHandle]bool, n)
		for i, h := range inputs {
			env[h] = bits&(1<<i) != 0
		}
		previous := false
		for i := uint32(0); i < uint32(len(output)); i++ {
			h, err := m.RefGet(output[i])
			astTestMust(t, err)
			value := psortOracleEval(t, m, h, env)
			if previous && !value {
				t.Fatalf("Boolean sorting output descends at input mask %#x, output index %d", bits, i)
			}
			previous = value
		}
	}
	for i := len(output) - 1; i >= 0; i-- {
		astTestMust(t, m.DelRef(&output[i]))
	}
}

func TestPsortASTNetworkRegisteredDuplicateAtMostOneInput(t *testing.T) {
	// test_at_most1 overwrites input[4] with input[3] before constructing the
	// full at-most-one network. Preserve that repeated-handle source case.
	m := astTestManager(t)
	c, err := NewPsortASTContext(m)
	astTestMust(t, err)
	r, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
	astTestMust(t, err)
	network, err := NewPsortASTNetwork(c, r, m.BudgetField, nil)
	astTestMust(t, err)
	input, err := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, err)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	var inputs []ASTHandle
	for i := uint32(0); i < 4; i++ {
		h, err := m.MkFreshConst("a", sort)
		astTestMust(t, err)
		astTestPin(t, m, h)
		inputs = append(inputs, h)
		astTestMust(t, input.Push(h))
	}
	astTestMust(t, input.Push(inputs[3]))
	span, err := input.Span()
	astTestMust(t, err)
	result, err := network.Le(true, 1, 5, span)
	astTestMust(t, err)
	astTestPin(t, m, result)
	oracle := psortOracleCNF{ids: map[ASTHandle]int{}}
	resultID := oracle.gate(t, m, result)
	for _, clause := range c.clauses {
		id := oracle.gate(t, m, clause)
		oracle.clauses = append(oracle.clauses, []int{id})
	}
	for _, h := range inputs {
		oracle.gate(t, m, h)
	}
	for bits := uint32(0); bits < 1<<4; bits++ {
		values := make([]int8, oracle.count+1)
		pop := 0
		for i, h := range inputs {
			id := oracle.ids[h]
			values[id] = -1
			if bits&(1<<i) != 0 {
				values[id] = 1
				pop++
			}
		}
		if bits&(1<<3) != 0 {
			pop++ // input[4] aliases input[3] in the registered C++ case.
		}
		values[resultID] = 1
		if got := psortOracleSAT(oracle.clauses, values); got != (pop <= 1) {
			t.Fatalf("duplicate source input mask=%#x population=%d result-true satisfiable=%v", bits, pop, got)
		}
	}
	astTestMust(t, network.Close())
	astTestMust(t, input.Close())
	astTestMust(t, r.Close())
	astTestMust(t, c.Close())
}
