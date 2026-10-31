package qe

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Exhaustively compare two-variable quantified Boolean formulas with their
// truth tables in both projection and context-simplification modes.
func TestSATSolverQuantifiedBooleanTruthTables(t *testing.T) {
	for _, full := range []bool{false, true} {
		for _, local := range []bool{false, true} {
			for _, forall := range []bool{false, true} {
				for table := 0; table < 16; table++ {
					t.Run(fmt.Sprintf("full=%t/local=%t/forall=%t/table=%x", full, local, forall, table), func(t *testing.T) {
						f := qsatKernelFixture(t)
						m := f.r.manager
						x := f.own(m.MkVar(1, f.boolean))
						y := f.own(m.MkVar(0, f.boolean))
						notX := f.own(m.MkNot(x))
						notY := f.own(m.MkNot(y))
						var clauses []ast.ASTHandle
						for xv := 0; xv < 2; xv++ {
							for yv := 0; yv < 2; yv++ {
								if table&(1<<(xv*2+yv)) == 0 {
									continue
								}
								a, b := notX, notY
								if xv == 1 {
									a = x
								}
								if yv == 1 {
									b = y
								}
								clauses = append(clauses, f.own(m.MkAnd(a, b)))
							}
						}
						body := f.own(m.MkOrArgs(clauses))
						inner := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: !forall, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: body}))
						formula := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: inner}))
						alt, err := extractSATAlternation(m, f.r.config, formula)
						fmMust(t, err)
						defer func() { fmMust(t, alt.Close()) }()
						options := newSATTacticOptions()
						options.projection = full
						options.simplifyLocal = local
						search, err := newSATPartialSearch(m, f.r.config, alt, options)
						fmMust(t, err)
						defer func() { fmMust(t, search.Close()) }()
						truth := f.own(m.MkTrue())
						result, err := search.qt(0, truth)
						fmMust(t, err)
						defer func() { fmMust(t, result.Close()) }()
						h, err := m.RefGet(result.formula)
						fmMust(t, err)
						expected := forall
						for xv := 0; xv < 2; xv++ {
							innerExpected := !forall
							for yv := 0; yv < 2; yv++ {
								value := table&(1<<(xv*2+yv)) != 0
								if forall {
									innerExpected = innerExpected || value
								} else {
									innerExpected = innerExpected && value
								}
							}
							if forall {
								expected = expected && innerExpected
							} else {
								expected = expected || innerExpected
							}
						}
						if m.IsFalse(h) == expected {
							t.Fatalf("table %x: satisfiable=%t, want %t", table, !m.IsFalse(h), expected)
						}
					})
				}
			}
		}
	}
}
