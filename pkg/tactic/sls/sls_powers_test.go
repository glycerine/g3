package sls

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func powersManager(t *testing.T, limit uint64) (*numeral.IntegerManager, *arena.Budget) {
	t.Helper()
	budget := arena.NewBudget(limit)
	manager, err := numeral.NewIntegerManager(numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget},
		Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager, budget
}

func TestPowersSourceLazyCacheAndBorrowIdentity(t *testing.T) {
	manager, budget := powersManager(t, 1<<28)
	p, err := newPowers(manager)
	if err != nil {
		t.Fatal(err)
	}

	want := map[uint32]string{0: "1", 1: "2", 31: "2147483648", 80: "1208925819614629174706176"}
	for _, shift := range []uint32{80, 0, 31, 1} {
		value, err := p.borrow(shift)
		if err != nil {
			t.Fatal(err)
		}
		text, err := manager.String(value)
		if err != nil || text != want[shift] {
			t.Fatalf("2^%d = %q, %v", shift, text, err)
		}
		before := budget.Used()
		again, err := p.borrow(shift)
		if err != nil {
			t.Fatal(err)
		}
		if again != value || budget.Used() != before {
			t.Fatalf("cache hit for 2^%d replaced or allocated", shift)
		}
	}

	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.borrow(0); !errors.Is(err, errPowersOwner) {
		t.Fatalf("closed cache borrow: %v", err)
	}
	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatalf("cache/manager leaked %d bytes", budget.Used())
	}
}

func TestPowersMissFailureDoesNotPublishCacheEntry(t *testing.T) {
	manager, budget := powersManager(t, 1<<28)
	p, err := newPowers(manager)
	if err != nil {
		t.Fatal(err)
	}

	budget.FailAfter(0)
	if value, err := p.borrow(100000); err == nil || value != nil {
		t.Fatalf("allocation refusal returned %v, %v", value, err)
	}
	if len(p.byShift) != 0 || len(p.owned) != 0 {
		t.Fatal("failed source construction published a cache entry")
	}
	budget.FailAfter(-1)
	value, err := p.borrow(100)
	if err != nil {
		t.Fatal(err)
	}
	text, err := manager.String(value)
	if err != nil || text != "1267650600228229401496703205376" {
		t.Fatalf("retry 2^100 = %q, %v", text, err)
	}

	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatalf("failed miss leaked %d bytes", budget.Used())
	}
}

func TestPowersRejectsMissingAndEndedManager(t *testing.T) {
	if _, err := newPowers(nil); !errors.Is(err, errPowersOwner) {
		t.Fatalf("nil owner: %v", err)
	}
	manager, _ := powersManager(t, 1<<28)
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if _, err := newPowers(manager); !errors.Is(err, errPowersOwner) {
		t.Fatalf("ended owner: %v", err)
	}
}
