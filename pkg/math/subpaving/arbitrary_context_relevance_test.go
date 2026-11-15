package subpaving

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestArbitraryContextSourceCandidateRelevanceAndNormalization(t *testing.T) {
	t.Run("zero epsilon and conflict first", func(t *testing.T) {
		f := newArbitraryClauseFixture(t)
		c := f.context
		root, err := c.Root()
		if err != nil {
			t.Fatal(err)
		}
		var beyond numeral.ArbitraryFloat
		if err := f.numbers.SetInt32(&beyond, 101); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := f.numbers.Del(&beyond); err != nil {
				t.Error(err)
			}
		})
		if relevant, err := c.RelevantNewBound(root, f.x, &beyond, true, false); err != nil || relevant {
			t.Fatalf("max-bound refusal: %v, %v", relevant, err)
		}
		if err := c.PublishBound(root, f.x, &f.four, false, false, true); err != nil {
			t.Fatal(err)
		}
		if relevant, err := c.RelevantNewBound(root, f.x, &beyond, true, false); err != nil || !relevant {
			t.Fatalf("conflict before max-bound gate: %v, %v", relevant, err)
		}
		if err := c.PublishBound(root, f.y, &f.four, true, false, true); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			value *numeral.ArbitraryFloat
			open  bool
			want  bool
		}{{&f.four, false, false}, {&f.four, true, true}, {&f.five, false, true}} {
			got, err := c.RelevantNewBound(root, f.y, tc.value, true, tc.open)
			if err != nil || got != tc.want {
				t.Fatalf("lower improvement %v: %v, %v", tc.open, got, err)
			}
		}
		before := len(c.queue)
		if relevant, err := c.PropagateCandidate(root, f.y, &f.four, true, false); err != nil || relevant || len(c.queue) != before {
			t.Fatalf("repeat candidate does not publish: %v, %v, %d", relevant, err, len(c.queue))
		}
		if relevant, err := c.PropagateCandidate(root, f.y, &f.five, true, false); err != nil || !relevant || len(c.queue) != before+1 {
			t.Fatalf("improved candidate publishes: %v, %v, %d", relevant, err, len(c.queue))
		}
	})
	t.Run("positive epsilon gap", func(t *testing.T) {
		f := newArbitraryClauseFixtureWithPolicy(t, 2, false)
		c := f.context
		root, err := c.Root()
		if err != nil {
			t.Fatal(err)
		}
		var six, seven, eight, ten numeral.ArbitraryFloat
		for _, tc := range []struct {
			out *numeral.ArbitraryFloat
			val int32
		}{{&six, 6}, {&seven, 7}, {&eight, 8}, {&ten, 10}} {
			if err := f.numbers.SetInt32(tc.out, tc.val); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() {
			for _, x := range []*numeral.ArbitraryFloat{&six, &seven, &eight, &ten} {
				if err := f.numbers.Del(x); err != nil {
					t.Error(err)
				}
			}
		})
		if err := c.PublishBound(root, f.x, &f.four, true, false, true); err != nil {
			t.Fatal(err)
		}
		if err := c.PublishBound(root, f.x, &ten, false, false, true); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			value *numeral.ArbitraryFloat
			lower bool
			want  bool
		}{{&f.five, true, false}, {&seven, true, true}, {&eight, false, false}, {&six, false, true}} {
			got, err := c.RelevantNewBound(root, f.x, tc.value, tc.lower, false)
			if err != nil || got != tc.want {
				t.Fatalf("epsilon-gap candidate: %v, %v, %v", got, tc.want, err)
			}
		}
	})
	t.Run("strict integer normalization", func(t *testing.T) {
		f := newArbitraryClauseFixtureWithPolicy(t, 0, true)
		c := f.context
		root, err := c.Root()
		if err != nil {
			t.Fatal(err)
		}
		relevant, err := c.PropagateCandidate(root, f.y, &f.five, true, true)
		if err != nil || !relevant || root.lower[f.y] == nil || root.lower[f.y].open {
			t.Fatalf("strict integer candidate: %v, %v, %+v", relevant, err, root.lower[f.y])
		}
		var six numeral.ArbitraryFloat
		if err := f.numbers.SetInt32(&six, 6); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := f.numbers.Del(&six); err != nil {
				t.Error(err)
			}
		})
		equal, err := f.numbers.Equal(&root.lower[f.y].value, &six)
		if err != nil || !equal {
			t.Fatalf("integer >5 becomes >=6: %v, %v", equal, err)
		}
	})
}

// AUDIT.md math_subpaving_01-F3: relevant_new_bound catches C::exception.
// On a fixed variable sub(upper, lower) is zero, which f2n<mpf> rejects as
// non-regular; the candidate is refused and the context marked failed.
func TestArbitraryRelevantNewBoundCatchesNonRegular(t *testing.T) {
	f := newArbitraryClauseFixtureWithPolicy(t, 2, false)
	c := f.context
	root, err := c.Root()
	if err != nil {
		t.Fatal(err)
	}
	var three numeral.ArbitraryFloat
	if err := f.numbers.SetInt32(&three, 3); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.numbers.Del(&three); err != nil {
			t.Error(err)
		}
	})
	if err := c.PublishBound(root, f.x, &f.four, true, false, true); err != nil {
		t.Fatal(err)
	}
	if err := c.PublishBound(root, f.x, &f.four, false, false, true); err != nil {
		t.Fatal(err)
	}
	if c.arithmeticFailed {
		t.Fatal("arithmetic failed before the candidate")
	}
	relevant, err := c.RelevantNewBound(root, f.x, &three, true, false)
	if err != nil || relevant || !c.arithmeticFailed {
		t.Fatalf("non-regular gap: relevant=%v err=%v failed=%v", relevant, err, c.arithmeticFailed)
	}
}
