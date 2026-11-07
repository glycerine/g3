package numeral

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func smallPathManager(t testing.TB) *IntegerManager {
	t.Helper()
	budget := arena.NewBudget(1 << 24)
	m, err := NewIntegerManager(IntegerConfig{Cells: arena.Config{PageSize: 16, Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func smallPathValue(t *testing.T, m *IntegerManager, a *Int) *big.Int {
	t.Helper()
	text, err := m.String(a)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		t.Fatal("unparsable", text)
	}
	return value
}

// Results must match big.Int across the int32 boundary, where the small path
// must hand over to cells, and for operands aliasing the destination.
func TestIntegerSmallPathMatchesBigInt(t *testing.T) {
	m := smallPathManager(t)
	edges := []int32{0, 1, -1, 2, -2, 46340, 46341, -46341, math.MaxInt32, math.MaxInt32 - 1, math.MinInt32, math.MinInt32 + 1}
	r := rand.New(rand.NewSource(17))
	pick := func() int32 {
		if r.Intn(2) == 0 {
			return edges[r.Intn(len(edges))]
		}
		return int32(r.Uint32())
	}
	type operation struct {
		name  string
		apply func(a, b, c *Int) error
		want  func(z, x, y *big.Int) *big.Int
	}
	operations := []operation{
		{"add", m.Add, (*big.Int).Add},
		{"sub", m.Sub, (*big.Int).Sub},
		{"mul", m.Mul, (*big.Int).Mul},
	}
	for round := 0; round < 20000; round++ {
		x, y := pick(), pick()
		for _, op := range operations {
			want := op.want(new(big.Int), big.NewInt(int64(x)), big.NewInt(int64(y)))
			a, b, c := NewInt(x), NewInt(y), NewInt(0)
			if err := op.apply(&a, &b, &c); err != nil {
				t.Fatal(op.name, x, y, err)
			}
			if got := smallPathValue(t, m, &c); got.Cmp(want) != 0 {
				t.Fatalf("%s(%d,%d) = %v, want %v", op.name, x, y, got, want)
			}
			if a.Val != x || b.Val != y || !a.IsSmall() || !b.IsSmall() {
				t.Fatalf("%s changed an operand", op.name)
			}
			if err := m.Del(&c); err != nil {
				t.Fatal(err)
			}
			aliased := NewInt(x)
			other := NewInt(y)
			if err := op.apply(&aliased, &other, &aliased); err != nil {
				t.Fatal(op.name, "aliased", err)
			}
			if got := smallPathValue(t, m, &aliased); got.Cmp(want) != 0 {
				t.Fatalf("aliased %s(%d,%d) = %v, want %v", op.name, x, y, got, want)
			}
			if err := m.Del(&aliased); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// A destination owning a cell takes the slow path, which releases it.
func TestIntegerSmallPathReleasesLargeDestination(t *testing.T) {
	m := smallPathManager(t)
	big1, big2, c := NewInt(math.MaxInt32), NewInt(math.MaxInt32), NewInt(0)
	if err := m.Mul(&big1, &big2, &c); err != nil {
		t.Fatal(err)
	}
	if c.IsSmall() {
		t.Fatal("product past int32 should own a cell")
	}
	a, b := NewInt(2), NewInt(3)
	if err := m.Add(&a, &b, &c); err != nil {
		t.Fatal(err)
	}
	if !c.IsSmall() || c.Val != 5 {
		t.Fatal("destination did not return to a small value", c)
	}
}

func TestIntegerSmallPathRefusesAClosedManager(t *testing.T) {
	m := smallPathManager(t)
	a, b, c := NewInt(1), NewInt(2), NewInt(0)
	m.closed = true
	defer func() { m.closed = false }()
	for name, apply := range map[string]func(a, b, c *Int) error{"add": m.Add, "sub": m.Sub, "mul": m.Mul} {
		if err := apply(&a, &b, &c); !errors.Is(err, arena.ErrClosed) {
			t.Fatal(name, err)
		}
	}
}

func TestIntegerSmallPathDoesNotAllocate(t *testing.T) {
	m := smallPathManager(t)
	a, b, c := NewInt(7), NewInt(-3), NewInt(0)
	if allocs := testing.AllocsPerRun(100, func() {
		_ = m.Add(&a, &b, &c)
		_ = m.Sub(&a, &b, &c)
		_ = m.Mul(&a, &b, &c)
	}); allocs != 0 {
		t.Fatalf("%v allocations", allocs)
	}
}

func BenchmarkIntegerSmallAddMul(b *testing.B) {
	m := smallPathManager(b)
	x, y, z := NewInt(7), NewInt(-3), NewInt(0)
	for i := 0; i < b.N; i++ {
		_ = m.Add(&x, &y, &z)
		_ = m.Mul(&x, &y, &z)
	}
}
