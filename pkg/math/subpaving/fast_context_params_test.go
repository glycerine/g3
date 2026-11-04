package subpaving

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFastContextNamedParamsSinglePassAndStoppedPrefix(t *testing.T) {
	m, base, _, _ := fastMonomialFixture(t, nil)
	b := arena.NewBudget(1 << 25)
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	if err != nil {
		t.Fatal(err)
	}
	pc, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	p, err := pc.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	d, err := params.NewParamDescrs(sy, params.ParamDescrsConfig{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, e := range []error{d.Close(), pc.Del(&p), pc.Close(), q.Close(), sy.Close()} {
			if e != nil {
				t.Error(e)
			}
		}
		if b.Used() != 0 {
			t.Error("retained parameter owners", b.Used())
		}
	})
	set := func(name string, value uint32) {
		t.Helper()
		if err := pc.SetUIntName(&p, &name, value); err != nil {
			t.Fatal(err)
		}
	}
	set("epsilon", 4)
	set("max_bound", 2)
	set("max_depth", 7)
	set("max_nodes", 9)
	set("max_memory", 3)
	set("nth_root_precision", 16)
	c, err := NewFastContextFromParams(m, base.limit, pc, &p)
	if err != nil {
		t.Fatal("single pass", err)
	}
	if c.root != nil {
		t.Fatal("source context constructor allocated root before initialization")
	}
	t.Cleanup(func() {
		if e := c.Close(); e != nil {
			t.Error(e)
		}
	})
	var want numeral.FastFloat
	t.Cleanup(func() {
		if e := m.Del(&want); e != nil {
			t.Error(e)
		}
	})
	check := func(got *numeral.FastFloat, n int32, den uint32) {
		t.Helper()
		if err := m.SetFraction32(&want, n, den); err != nil {
			t.Fatal(err)
		}
		equal, err := m.Equal(got, &want)
		if err != nil || !equal {
			t.Fatal("parameter numeral", n, den, equal, err)
		}
	}
	if c.zeroEpsilon || c.maxDepth != 7 || c.maxNodes != 9 || c.maxMemory != 3*1024*1024 {
		t.Fatal("named MPFF scalar policy", c.maxDepth, c.maxNodes, c.maxMemory)
	}
	check(c.epsilon, 1, 4)
	check(c.maxBound, 100, 1)
	check(c.minusMaxBound, -100, 1)
	check(c.rootPrecision, 1, 16)
	if err := CollectFastParamDescrs(d); err != nil {
		t.Fatal(err)
	}
	if size, err := d.Size(); err != nil || size != 5 {
		t.Fatal("source descriptors", size, err)
	}
	set("epsilon", 2)
	set("max_bound", 1<<31)
	partial, err := NewFastContextFromParams(m, base.limit, pc, &p)
	if err == nil || partial == nil {
		t.Fatal("refused owner", partial, err)
	}
	if partial.root != nil {
		t.Fatal("refused constructor allocated root before parameter pass completed")
	}
	t.Cleanup(func() {
		if e := partial.Close(); e != nil {
			t.Error(e)
		}
	})
	if partial.zeroEpsilon || partial.maxDepth != 0 || partial.maxNodes != 0 || partial.maxMemory != 0 {
		t.Fatal("stopped before later named fields")
	}
	check(partial.epsilon, 1, 2)
	if err := pc.Reset(&p); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateParamsRef(pc, &p); err != nil {
		t.Fatal("default update", err)
	}
	if c.zeroEpsilon || c.maxDepth != 128 || c.maxNodes != 8192 || c.maxMemory != math.MaxUint64 {
		t.Fatal("source defaults", c.maxDepth, c.maxNodes, c.maxMemory)
	}
	set("epsilon", 0)
	set("max_bound", 0)
	set("nth_root_precision", 0)
	if err := c.UpdateParamsRef(pc, &p); err != nil {
		t.Fatal("zero branches", err)
	}
	if !c.zeroEpsilon {
		t.Fatal("zero epsilon")
	}
	check(c.maxBound, 1, 1)
	check(c.rootPrecision, 1, 1)
	if err := c.InitializeRoot(); err != nil || c.root == nil || c.initStage != 3 {
		t.Fatal("source root allocation during initialization", c.root, c.initStage, err)
	}
}
