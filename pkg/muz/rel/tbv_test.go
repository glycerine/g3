package rel

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func newTestTBVManager(t *testing.T, width uint32) (*TBVManager, *arena.Budget) {
	t.Helper()
	budget := arena.NewBudget(1 << 20)
	manager, err := NewTBVManager(width, arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 8, Budget: budget}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		manager.Close()
		if used := budget.Used(); used != 0 {
			t.Errorf("TBV manager retained %d bytes", used)
		}
	})
	return manager, budget
}

func tbvString(t *testing.T, manager *TBVManager, vector *TBV) string {
	t.Helper()
	text, err := manager.String(vector)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestTBVSourceEncodingRangesAndSetOperations(t *testing.T) {
	manager, _ := newTestTBVManager(t, 6)
	zero, err := manager.Allocate0()
	if err != nil {
		t.Fatal(err)
	}
	one, err := manager.Allocate1()
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := manager.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	if tbvString(t, manager, zero) != "000000" || tbvString(t, manager, one) != "111111" || tbvString(t, manager, unknown) != "xxxxxx" {
		t.Fatal("source fill encoding")
	}
	if err = manager.SetUint64(unknown, 0b1011, 4, 1); err != nil {
		t.Fatal(err)
	}
	if got := tbvString(t, manager, unknown); got != "x1011x" {
		t.Fatal("range set", got)
	}
	copy, err := manager.AllocateCopy(unknown)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := manager.Equals(copy, unknown)
	if err != nil || !equal {
		t.Fatal("copy equality", equal, err)
	}
	firstHash, err := manager.Hash(copy)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := manager.Hash(unknown)
	if err != nil || firstHash != secondHash {
		t.Fatal("copy hash", firstHash, secondHash, err)
	}
	intersection, err := manager.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	wellFormed, err := manager.Intersect(zero, one, intersection)
	if err != nil || wellFormed || tbvString(t, manager, intersection) != "zzzzzz" {
		t.Fatal("disjoint source intersection", wellFormed, tbvString(t, manager, intersection), err)
	}
	if err = manager.Copy(intersection, zero); err != nil {
		t.Fatal(err)
	}
	if err = manager.SetOr(intersection, one); err != nil || tbvString(t, manager, intersection) != "xxxxxx" {
		t.Fatal("source union", tbvString(t, manager, intersection), err)
	}
	contains, err := manager.Contains(intersection, zero)
	if err != nil || !contains {
		t.Fatal("unknown contains zero", contains, err)
	}
	contains, err = manager.Contains(zero, intersection)
	if err != nil || contains {
		t.Fatal("zero does not contain unknown", contains, err)
	}
}

func TestTBVSourceParsingPermutationComplementAndProjection(t *testing.T) {
	sourceManager, _ := newTestTBVManager(t, 5)
	parsed, err := sourceManager.AllocateString("  01*x!")
	if err != nil {
		t.Fatal(err)
	}
	if got := tbvString(t, sourceManager, parsed); got != "xxx10" {
		t.Fatal("source parsing stores the first character at bit zero", got)
	}
	permuted, err := sourceManager.AllocatePermutation(parsed, []uint32{4, 3, 2, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if got := tbvString(t, sourceManager, permuted); got != "01xxx" {
		t.Fatal("permutation", got)
	}
	complements, err := sourceManager.Complement(parsed)
	if err != nil || len(complements) != 2 {
		t.Fatal("complement count", len(complements), err)
	}
	if got := tbvString(t, sourceManager, complements[0]); got != "xxx11" {
		t.Fatal("first complement", got)
	}
	if got := tbvString(t, sourceManager, complements[1]); got != "xxx00" {
		t.Fatal("second complement", got)
	}

	destinationBudget := arena.NewBudget(1 << 20)
	destinationManager, err := NewTBVManager(3, arena.VectorConfig{PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: destinationBudget}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		destinationManager.Close()
		if used := destinationBudget.Used(); used != 0 {
			t.Errorf("projection manager retained %d bytes", used)
		}
	})
	projected, err := destinationManager.Project([]bool{false, true, false, true, false}, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if got := tbvString(t, destinationManager, projected); got != "xx0" {
		t.Fatal("cross-manager projection", got)
	}
	matched, err := sourceManager.ContainsColumns(parsed, []uint32{2, 0}, projected, []uint32{1, 0})
	if err != nil || !matched {
		t.Fatal("cross-manager column containment", matched, err)
	}

	negative, err := sourceManager.AllocateBigInt(big.NewInt(-2))
	if err != nil {
		t.Fatal(err)
	}
	if got := tbvString(t, sourceManager, negative); got != "11110" {
		t.Fatal("two's-complement rational bit extraction", got)
	}
}

func TestTBVFormulaAndLifetimeBoundaries(t *testing.T) {
	manager, _ := newTestTBVManager(t, 3)
	vector, err := manager.AllocateString("01x")
	if err != nil {
		t.Fatal(err)
	}

	budget := arena.NewBudget(1 << 24)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	astManager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := astManager.Close(); err != nil {
			t.Error(err)
		}
		if err := numerals.Close(); err != nil {
			t.Error(err)
		}
		if err := symbols.Close(); err != nil {
			t.Error(err)
		}
		if used := budget.Used(); used != 0 {
			t.Errorf("AST owner graph retained %d bytes", used)
		}
	})
	formula, err := manager.ToFormula(astManager, vector)
	if err != nil {
		t.Fatal(err)
	}
	arguments, err := astManager.AppArgumentView(formula)
	if err != nil || arguments.Count() != 2 {
		t.Fatal("formula contains one literal for each definite bit", arguments.Count(), err)
	}

	if err = manager.Deallocate(vector); err != nil {
		t.Fatal(err)
	}
	if err = manager.Deallocate(vector); !errors.Is(err, ErrTBV) {
		t.Fatal("double deallocation must be rejected", err)
	}
	manager.Reset()
	stale, err := manager.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	manager.Reset()
	if _, err = manager.String(stale); !errors.Is(err, ErrTBV) {
		t.Fatal("reset must invalidate old vectors", err)
	}
}

func TestTBVZeroWidthAndBounds(t *testing.T) {
	manager, _ := newTestTBVManager(t, 0)
	vector, err := manager.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	if got := tbvString(t, manager, vector); got != "[]" || manager.SizeEstimateBytes() != 0 {
		t.Fatal("zero width", got, manager.SizeEstimateBytes())
	}
	if err = manager.Set(vector, 0, TBit0); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("zero-width write", err)
	}
	if err = manager.Deallocate(vector); err != nil {
		t.Fatal(err)
	}
	if _, err = NewTBVManager(^uint32(0), arena.VectorConfig{Descriptors: arena.Config{Budget: arena.NewBudget(1 << 10)}}); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("width overflow", err)
	}
}
