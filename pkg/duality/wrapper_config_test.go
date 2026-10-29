package duality

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func dualityParamsFixture(t *testing.T) (*uparams.ParamsContext, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(^uint64(0))
	sy, err := usymbol.NewManager(usymbol.Config{Budget: b, PageSize: 8, DescriptorPageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	q, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{
		Cells: arena.Config{PageSize: 2, Budget: b},
		Words: arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 2, Budget: b}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := uparams.NewParamsContext(sy, q, uparams.ParamsConfig{
		Budget: b, PageEntries: 2,
		Text: arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 2, Budget: b}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := q.Close(); err != nil {
			t.Error(err)
		}
		if err := sy.Close(); err != nil {
			t.Error(err)
		}
		if used := b.Used(); used != 0 {
			t.Errorf("retained %d bytes", used)
		}
	})
	return p, b
}

func closeConfig(t *testing.T, c *Config) {
	t.Helper()
	t.Cleanup(func() {
		if !c.closed {
			if err := c.Close(); err != nil {
				t.Error(err)
			}
		}
	})
}

func TestConfigSettersAndSignedUintConversion(t *testing.T) {
	p, _ := dualityParamsFixture(t)
	c, err := NewConfig(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	closeConfig(t, c)
	if err = c.SetString("engine", "duality"); err != nil {
		t.Fatal(err)
	}
	if err = c.SetBool("mbqi", true); err != nil {
		t.Fatal(err)
	}
	if err = c.SetInt("limit", -1); err != nil {
		t.Fatal(err)
	}
	r, err := c.Ref()
	if err != nil {
		t.Fatal(err)
	}
	name, def := "engine", ""
	if got, present, err := p.GetStringName(r, &name, &def); err != nil || !present || got != "duality" {
		t.Fatal("string", got, present, err)
	}
	name = "mbqi"
	if got, err := p.GetBoolName(r, &name, false); err != nil || !got {
		t.Fatal("bool", got, err)
	}
	name = "limit"
	if got, err := p.GetUIntName(r, &name, 0); err != nil || got != ^uint32(0) {
		t.Fatal("unsigned conversion", got, err)
	}
}

func TestConfigCloneSharesThenDetaches(t *testing.T) {
	p, _ := dualityParamsFixture(t)
	a, err := NewConfig(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	closeConfig(t, a)
	if err = a.SetBool("flag", true); err != nil {
		t.Fatal(err)
	}
	b, err := a.Clone()
	if err != nil {
		t.Fatal(err)
	}
	closeConfig(t, b)
	if err = b.SetBool("flag", false); err != nil {
		t.Fatal(err)
	}
	ar, _ := a.Ref()
	br, _ := b.Ref()
	name := "flag"
	av, err := p.GetBoolName(ar, &name, false)
	if err != nil {
		t.Fatal(err)
	}
	bv, err := p.GetBoolName(br, &name, true)
	if err != nil {
		t.Fatal(err)
	}
	if !av || bv {
		t.Fatal("copy-on-write configuration", av, bv)
	}
}

func TestConfigOwnerAndLifetimeGuards(t *testing.T) {
	p, _ := dualityParamsFixture(t)
	c, err := NewConfig(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	copyValue := *c
	if err := copyValue.SetBool("x", true); err == nil {
		t.Fatal("shallow copy accepted")
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	if err = c.SetBool("x", true); err == nil {
		t.Fatal("closed configuration accepted")
	}
	if err = c.Close(); err == nil {
		t.Fatal("double close accepted")
	}

	foreign, _ := dualityParamsFixture(t)
	v, err := foreign.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := foreign.Del(&v); err != nil {
			t.Error(err)
		}
	}()
	if _, err = NewConfig(p, &v); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("foreign params_ref", err)
	}
}

func TestWrapperKindsRetainSourceOrdinals(t *testing.T) {
	if DeclTrue != 0 || DeclInterp != 12 || DeclUninterpreted != 44 || DeclOther != 48 {
		t.Fatal("declaration kind ordinals", DeclTrue, DeclInterp, DeclUninterpreted, DeclOther)
	}
	if SortBool != 0 || SortUninterpreted != 4 || SortUnknown != 5 {
		t.Fatal("sort kind ordinals", SortBool, SortUninterpreted, SortUnknown)
	}
}
