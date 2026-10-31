package sat

// Source: src/test/cnf_backbones.cpp, DIMACS and get_consequences mode.

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCNFBackbonesSourceConsequences(t *testing.T) {
	const input = "p cnf 4 4\n1 0\n-1 2 0\n-2 3 0\n4 -4 0\n"
	solver, _, _ := satSolverCheckFixture(t)
	satOK(t, ParseDIMACS(strings.NewReader(input), solver))
	status, rows, err := solver.GetConsequences(nil, []BoolVar{1, 2, 3, 4})
	satOK(t, err)
	if status != lbool.True {
		t.Fatal("consequence status", status)
	}
	seen := make(map[BoolVar]bool)
	for _, row := range rows {
		if len(row) != 1 || row[0].sign() || row[0].variable() < 1 || row[0].variable() > 3 {
			t.Fatal("unexpected backbone", row)
		}
		seen[row[0].variable()] = true
	}
	if len(seen) != 3 {
		t.Fatal("missing backbone", seen, rows)
	}
}

// sourceCNFChunking translates the separate test driver's core-based
// chunking algorithm, whose output consists of backbone literals.
func sourceCNFChunking(t *testing.T, solver *Solver, vars []BoolVar, assumptions []Literal, chunkSize int) (lbool.LBool, []Literal) {
	t.Helper()
	status, err := solver.Check(assumptions)
	satOK(t, err)
	if status != lbool.True {
		return status, nil
	}
	model, err := solver.AppendModel(nil)
	satOK(t, err)
	var lambda, backbones []Literal
	for _, v := range vars {
		lambda = append(lambda, MakeLiteral(v, model[v] == lbool.False))
	}
	remove := func(values []Literal, lit Literal) []Literal {
		for i := len(values) - 1; i >= 0; i-- {
			if values[i] == lit {
				values[i] = values[len(values)-1]
				return values[:len(values)-1]
			}
		}
		t.Fatalf("source chunk literal missing: %v", lit)
		return nil
	}
	contains := func(values []Literal, lit Literal) bool {
		for _, v := range values {
			if v == lit {
				return true
			}
		}
		return false
	}
	for len(lambda) != 0 {
		count := chunkSize
		if count > len(lambda) {
			count = len(lambda)
		}
		var gamma, omega []Literal
		for i := 0; i < count; i++ {
			lit := lambda[len(lambda)-i-1]
			gamma = append(gamma, lit)
			omega = append(omega, lit.Negated())
		}
		for {
			query := append(append([]Literal(nil), assumptions...), omega...)
			status, err = solver.Check(query)
			satOK(t, err)
			if status == lbool.True {
				model, err = solver.AppendModel(nil)
				satOK(t, err)
				for i := 0; i < len(lambda); {
					lit := lambda[i]
					if (model[lit.variable()] == lbool.False) != lit.sign() {
						lambda[i] = lambda[len(lambda)-1]
						lambda = lambda[:len(lambda)-1]
					} else {
						i++
					}
				}
				break
			}
			if status != lbool.False {
				t.Fatal("chunk solver returned unknown")
			}
			core, err := solver.AppendCore(nil)
			satOK(t, err)
			var occurs []Literal
			for _, lit := range omega {
				if contains(core, lit) {
					occurs = append(occurs, lit)
				}
			}
			if len(occurs) == 1 {
				backbone := occurs[0].Negated()
				backbones = append(backbones, backbone)
				lambda = remove(lambda, backbone)
				gamma = remove(gamma, backbone)
				satOK(t, solver.MkClause([]Literal{backbone}))
			}
			for i := 0; i < len(omega); {
				if contains(occurs, omega[i]) {
					omega[i] = omega[len(omega)-1]
					omega = omega[:len(omega)-1]
				} else {
					i++
				}
			}
			if len(occurs) == 0 {
				t.Fatal("source chunk core did not intersect assumptions")
			}
			if len(omega) == 0 && len(occurs) > 1 {
				for _, lit := range gamma {
					query := append(append([]Literal(nil), assumptions...), lit.Negated())
					status, err = solver.Check(query)
					satOK(t, err)
					if status == lbool.False {
						backbones = append(backbones, lit)
					}
					lambda = remove(lambda, lit)
				}
				break
			}
		}
	}
	return lbool.True, backbones
}

func TestCNFBackbonesSourceChunking(t *testing.T) {
	for _, size := range []int{1, 100} {
		t.Run(map[int]string{1: "single", 100: "source"}[size], func(t *testing.T) {
			solver, _, _ := satSolverCheckFixture(t)
			satOK(t, ParseDIMACS(strings.NewReader("p cnf 4 4\n1 0\n-1 2 0\n-2 3 0\n4 -4 0\n"), solver))
			status, backbone := sourceCNFChunking(t, solver, []BoolVar{1, 2, 3, 4}, nil, size)
			if status != lbool.True {
				t.Fatal(status)
			}
			seen := make(map[BoolVar]bool)
			for _, lit := range backbone {
				if lit.sign() || lit.variable() < 1 || lit.variable() > 3 {
					t.Fatal("unexpected backbone", lit)
				}
				seen[lit.variable()] = true
			}
			if len(seen) != 3 {
				t.Fatal("missing backbones", backbone)
			}
		})
	}
}

func TestCNFBackbonesSourceChunkingExhaustiveBinaryCNFs(t *testing.T) {
	for mask := uint32(0); mask < 16; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			solver, _, _ := satSolverCheckFixture(t)
			for i := 0; i < 2; i++ {
				_, err := solver.MkVar(true, true)
				satOK(t, err)
			}
			var clauses [][]Literal
			for i := uint32(0); i < 4; i++ {
				if mask&(1<<i) != 0 {
					clause := []Literal{MakeLiteral(0, i&1 != 0), MakeLiteral(1, i&2 != 0)}
					clauses = append(clauses, clause)
					satOK(t, solver.MkClause(append([]Literal(nil), clause...)))
				}
			}
			models := satConsequenceModels(clauses, nil, 2)
			status, backbone := sourceCNFChunking(t, solver, []BoolVar{0, 1}, nil, 2)
			if len(models) == 0 {
				if status != lbool.False {
					t.Fatal("unsatisfiable CNF", status)
				}
				return
			}
			if status != lbool.True {
				t.Fatal("satisfiable CNF", status)
			}
			actual := make(map[Literal]bool)
			for _, lit := range backbone {
				actual[lit] = true
			}
			want := make(map[Literal]bool)
			for v := BoolVar(0); v < 2; v++ {
				first := models[0]&(1<<v) != 0
				fixed := true
				for _, model := range models[1:] {
					if (model&(1<<v) != 0) != first {
						fixed = false
					}
				}
				if fixed {
					want[MakeLiteral(v, !first)] = true
				}
			}
			if !reflect.DeepEqual(actual, want) {
				t.Fatal("incorrect backbone", clauses, models, actual, want)
			}
		})
	}
}
