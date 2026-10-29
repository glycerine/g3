package tactic

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSubpavingPublicSimplifyParameterStages(t *testing.T) {
	budget := arena.NewBudget(1 << 22)
	sy, err := symbol.NewManager(symbol.Config{Budget: budget, PageSize: 32, DescriptorPageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: budget, Text: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := c.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Del(&source); err != nil {
			t.Error(err)
		}
		if err := c.Close(); err != nil {
			t.Error(err)
		}
		if err := q.Close(); err != nil {
			t.Error(err)
		}
		if err := sy.Close(); err != nil {
			t.Error(err)
		}
	})
	key := "print_nodes"
	if err := c.SetBoolName(&source, &key, true); err != nil {
		t.Fatal(err)
	}
	first, second, err := newPublicSimplifyParams(c, &source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closePublicSimplifyParams(c, &first, &second); err != nil {
			t.Fatal(err)
		}
	})
	for _, name := range []string{"arith_lhs", "expand_power", "som", "eq2ineq", "elim_and", "blast_distinct"} {
		value, err := c.GetBoolName(&first, &name, false)
		if err != nil || !value {
			t.Fatalf("first stage %s=%v err=%v", name, value, err)
		}
	}
	name := "max_power"
	value, err := c.GetUIntName(&first, &name, 0)
	if err != nil || value != math.MaxUint32 {
		t.Fatalf("max_power=%d err=%v", value, err)
	}
	name = "mul_to_power"
	mul, err := c.GetBoolName(&second, &name, false)
	if err != nil || !mul {
		t.Fatalf("mul_to_power=%v err=%v", mul, err)
	}
	for _, p := range []*params.ParamsRef{&first, &second} {
		name = "print_nodes"
		inherited, err := c.GetBoolName(p, &name, false)
		if err != nil || !inherited {
			t.Fatalf("source setting not copied: %v %v", inherited, err)
		}
	}
}
