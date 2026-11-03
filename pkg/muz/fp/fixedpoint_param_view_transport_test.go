package fp

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestFixedpointParamViewTransportsStoppedOwnerThroughDeferredError(t *testing.T) {
	numeralBudget := arena.NewBudget(1 << 26)
	paramsBudget := arena.NewBudget(1 << 24)
	symbolBudget := arena.NewBudget(1 << 24)
	runtime, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: numeralBudget, SizeTBits: 64, ThreadLocal: true})
	fragmentMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: runtime, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{PageSize: 2, Budget: numeralBudget}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: numeralBudget}}}})
	fragmentMust(t, err)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: symbolBudget, PageSize: 64, DescriptorPageSize: 2})
	fragmentMust(t, err)
	context, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: paramsBudget, PageEntries: 1, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: paramsBudget}}})
	fragmentMust(t, err)
	defer func() {
		finish := func(close func() error) {
			for i := 0; i < 64; i++ {
				cause := close()
				if cause == nil {
					return
				}
				if !errors.Is(cause, arena.ErrBounds) {
					t.Fatal(cause)
				}
			}
			t.Fatal("owner cleanup did not converge")
		}
		finish(context.Close)
		finish(numerals.Close)
		finish(runtime.Close)
		fragmentMust(t, symbols.Close())
	}()
	module, err := context.NewValue()
	fragmentMust(t, err)
	name := "fixedpoint-view-large-numeral"
	integers := context.NumeralsField.Manager.Z
	scoped, err := unumeral.NewScopedInt(integers)
	fragmentMust(t, err)
	value, err := scoped.Get()
	fragmentMust(t, err)
	digits := make([]uint32, 32768)
	digits[0], digits[len(digits)-1] = 17, 1
	fragmentMust(t, integers.SetLimbs(value, digits))
	rational, err := context.NumeralsField.NewFromScopedInt(scoped)
	fragmentMust(t, err)
	fragmentMust(t, scoped.Close())
	fragmentMust(t, context.SetRatName(&module, &name, &rational))
	fragmentMust(t, context.NumeralsField.Del(&rational))

	a, err := runtime.Allocate(8)
	fragmentMust(t, err)
	b, err := runtime.Allocate(8)
	fragmentMust(t, err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("runtime refusal priming", err)
	}
	fragmentMust(t, runtime.Deallocate(&a))
	state := runtime.State()
	delta = math.MinInt64 + int64(1000) - state.GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("runtime lower-bound priming", err)
	}
	fragmentMust(t, runtime.Deallocate(&b))
	integerBudget := integers.BudgetField
	integerBudget.FailAfter(0)
	paramsBudget.FailAfter(0)
	err = func() (result error) {
		view := &fixedpointParamView{context: context, global: module}
		module = uparams.ParamsRef{}
		defer func() { result = errors.Join(result, view.Close()) }()
		return nil
	}()
	integerBudget.FailAfter(-1)
	paramsBudget.FailAfter(-1)
	if !errors.Is(err, arena.ErrBounds) {
		t.Fatal("parameter view cleanup did not expose native refusal", err)
	}
	var owner umemory.ConstructionFailure
	if !errors.As(err, &owner) || owner.RuntimeOwner() != runtime {
		t.Fatal("deferred parameter cleanup lost its owner", err)
	}
	if owner.OriginalCause() == nil || owner.CleanupCause() == nil {
		t.Fatal("cleanup transport lost original cause")
	}
	fragmentMust(t, owner.Cleanup())
	if owner.CleanupCause() != nil || owner.RuntimeOwner() != runtime {
		t.Fatal("completed cleanup lost stable runtime or kept stale cause")
	}
	fragmentMust(t, owner.Cleanup())
}
