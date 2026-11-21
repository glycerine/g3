package smtcore

import (
	"fmt"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func sortedClauseInputs(t testing.TB, size int) (*SMTInternalizerState, []SMTLiteral) {
	t.Helper()
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	args := make([]SMTLiteral, size)
	for i := range args {
		atom := smtRelevancyBool(t, m, fmt.Sprintf("ordered-clause-%d", i))
		astTestMust(t, s.Internalize(atom, true))
		lit, err := s.Literal(atom)
		astTestMust(t, err)
		args[len(args)-1-i] = lit
	}
	return s, args
}

func BenchmarkSMTAuxiliaryClauseSimplification(b *testing.B) {
	for _, size := range []int{3, 8, 16, 32, 64} {
		for _, kind := range []SMTClauseKind{SMTClauseAux, SMTClauseAuxLemma} {
			b.Run(fmt.Sprintf("Width%d/Kind%d", size, kind), func(b *testing.B) {
				s, lits := sortedClauseInputs(b, size)
				// The sorted prefix must be visited before the last variable and its
				// complement cancel the clause. No clause is retained between probes.
				lits = append(lits, lits[0].Negated())
				original := slices.Clone(lits)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := s.addClauseKind(lits, false, kind, ast.ASTHandle{}); err != nil {
						b.Fatal(err)
					}
				}
				if s.numClauseRecords() != 0 || s.inconsistent || !slices.Equal(original, lits) {
					b.Fatal("tautology changed solver or caller data")
				}
			})
		}
	}
}

func TestSMTAuxiliaryClauseSortedDuplicates(t *testing.T) {
	for _, kind := range []SMTClauseKind{SMTClauseAux, SMTClauseAuxLemma} {
		for _, width := range []int{4, 32} {
			for _, tautology := range []bool{false, true} {
				t.Run(fmt.Sprint(kind, "/", width, "/", tautology), func(t *testing.T) {
					s, keys := sortedClauseInputs(t, width)
					var input []SMTLiteral
					for i, v := range keys {
						input = append(input, v, keys[(i+3)%width])
					}
					if tautology {
						input = append(input, keys[width-2].Negated())
					}
					original := slices.Clone(input)
					astTestMust(t, s.addClauseKind(input, false, kind, ast.ASTHandle{}))
					if !slices.Equal(original, input) || s.inconsistent {
						t.Fatalf("clause changed its caller or caused a conflict")
					}
					cs := s.testClauses()
					if tautology {
						if len(cs) != 0 {
							t.Fatalf("stored tautology: %v", cs)
						}
						return
					}
					if len(cs) != 1 {
						t.Fatalf("got %d clauses", len(cs))
					}
					got := slices.Clone(cs[0].Literals)
					slices.Sort(got)
					slices.Sort(keys)
					if !slices.Equal(got, keys) {
						t.Fatalf("literals %v, want %v", got, keys)
					}
				})
			}
		}
	}
}
