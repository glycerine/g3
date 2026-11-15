package opt

// Source: src/opt/maxres.cpp literal/new_assumption state.

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResBaseAggregatesSortsAndRemovesAssumptions(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "maxres-base-a")
	b := maxResTestBool(t, manager, "maxres-base-b")
	engine := &maxResEngine{solver: &ArithmeticSolver{manager: manager}, trail: maxSMTASTOwner{manager: manager}, weights: make(map[ast.ASTHandle]*big.Rat)}
	defer engine.close()
	// new_assumption never aggregates: m_asm2weight.insert replaces
	// (AUDIT.md FUZZ_J-F2); add_soft is the aggregating entry point.
	if err := engine.addAssumption(a, big.NewRat(1, 1)); err != nil {
		t.Fatal(err)
	}
	if err := engine.addAssumption(b, big.NewRat(5, 7)); err != nil {
		t.Fatal(err)
	}
	if engine.sortedAssumptions(engine.active)[0] != a || engine.weights[a].Cmp(big.NewRat(1, 1)) != 0 || engine.stats.Assumptions != 2 {
		t.Fatal("MaxRes base", engine.active, engine.weights, engine.stats)
	}
	removed, err := engine.removeAssumption(a)
	if err != nil || removed.Cmp(big.NewRat(1, 1)) != 0 || len(engine.active) != 1 {
		t.Fatal("MaxRes removal", removed, engine.active, err)
	}
}

// opt01F3Weights and opt01F3Order: the tied weights of AUDIT.md opt_01-F3
// and the index permutation g++ 13.3 std::sort leaves with the "greater"
// comparator (compare_asm / maxsmt_compare_soft).
var (
	opt01F3Weights = []int64{2, 2, 2, 3, 1, 1, 3, 1, 2, 3, 3, 3, 3, 3, 1, 2, 1}
	opt01F3Order   = []int{13, 3, 12, 11, 6, 10, 9, 1, 15, 8, 2, 0, 7, 5, 4, 14, 16}
)

// TestMaxResSortAssumptionsMatchesStdSort: maxres.cpp:417 sort_assumptions
// uses std::sort, whose introsort permutes the 17 tied weights; the Go port
// must leave the libstdc++ order (AUDIT.md opt_01-F3).
func TestMaxResSortAssumptionsMatchesStdSort(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	engine := &maxResEngine{weights: make(map[ast.ASTHandle]*big.Rat)}
	asms := make([]ast.ASTHandle, len(opt01F3Weights))
	for i, w := range opt01F3Weights {
		asms[i] = maxResTestBool(t, manager, fmt.Sprintf("maxres-std-sort-%d", i))
		engine.weights[asms[i]] = big.NewRat(w, 1)
	}
	got := engine.sortedAssumptions(asms)
	for i, j := range opt01F3Order {
		if got[i] != asms[j] {
			t.Fatalf("position %d: got %v, want asms[%d]", i, got[i], j)
		}
	}
	rats := make(map[ast.ASTHandle]*big.Rat, len(asms))
	ints := make(map[ast.ASTHandle]*big.Int, len(asms))
	for i, a := range asms {
		rats[a] = big.NewRat(opt01F3Weights[i], 1)
		ints[a] = big.NewInt(opt01F3Weights[i])
	}
	ratMutex := append([]ast.ASTHandle(nil), asms...)
	sortMaxResMutex(ratMutex, rats)
	intMutex := append([]ast.ASTHandle(nil), asms...)
	maxSMTSortMutex(intMutex, ints)
	for i, j := range opt01F3Order {
		if ratMutex[i] != asms[j] || intMutex[i] != asms[j] {
			t.Fatalf("mutex position %d: want asms[%d]", i, j)
		}
	}
}
