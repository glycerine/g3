package ddnf

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestDDNFRegistrySourceWidthsLookupAndFreeze(t *testing.T) {
	budget := arena.NewBudget(1 << 22)
	registry := NewRegistry(arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 8, Budget: budget}})
	t.Cleanup(func() {
		registry.Close()
		if used := budget.Used(); used != 0 {
			t.Errorf("registry retained %d bytes", used)
		}
	})

	one, err := registry.AllocateUint64Range(1, 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	oneNode, err := registry.Insert(1, one)
	if err != nil || oneNode.ID() != 1 {
		t.Fatal("one-bit insertion", oneNode, err)
	}
	three, err := registry.AllocateUint64Range(3, 2, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	threeNode, err := registry.Insert(3, three)
	if err != nil || threeNode.ID() != 1 {
		t.Fatal("three-bit insertion", threeNode, err)
	}
	large, err := registry.AllocateUint64Range(65, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.Insert(65, large); err != nil {
		t.Fatal(err)
	}
	if got := registry.Widths(); len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 65 {
		t.Fatal("source lazy width order", got)
	}
	firstCore, err := registry.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	again, err := registry.Get(1)
	if err != nil || firstCore != again {
		t.Fatal("width core reuse", err)
	}
	descendants, err := registry.Lookup(1, one)
	if err != nil || len(descendants) != 1 || descendants[0] != oneNode {
		t.Fatal("lookup", descendants, err)
	}
	otherOne, err := firstCore.Manager().Allocate1()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.Insert(1, otherOne); !errors.Is(err, ErrCore) {
		t.Fatal("lookup freezes its width", err)
	}
	otherThree, err := registry.AllocateUint64Range(3, 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.Insert(3, otherThree); err != nil {
		t.Fatal("other widths remain mutable", err)
	}
	display, err := registry.Display()
	if err != nil {
		t.Fatal(err)
	}
	if i1, i3, i65 := strings.Index(display, "node[0: x"), strings.Index(display, "node[0: xxx"), strings.Index(display, "node[0: "+strings.Repeat("x", 65)); i1 < 0 || i3 <= i1 || i65 <= i3 {
		t.Fatal("stable width display order")
	}
}

func TestDDNFRegistryConstructionFailureDoesNotPublishWidth(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	registry := NewRegistry(arena.VectorConfig{PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: budget}})
	if _, err := registry.Get(2); err != nil {
		t.Fatal(err)
	}
	used := budget.Used()
	budget.FailAfter(0)
	if _, err := registry.Get(4); !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("injected construction failure", err)
	}
	if got := registry.Widths(); len(got) != 1 || got[0] != 2 || budget.Used() != used {
		t.Fatal("failed width publication", got, used, budget.Used())
	}
	budget.FailAfter(-1)
	if _, err := registry.Get(4); err != nil {
		t.Fatal(err)
	}
	registry.Close()
	if used := budget.Used(); used != 0 {
		t.Fatal("registry close", used)
	}
	if _, err := registry.Get(2); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed registry", err)
	}
}
