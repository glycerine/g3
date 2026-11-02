package rel

import "testing"

func TestLazySparseProjectAllUsesNullSignatureFallback(t *testing.T) {
	for _, nonempty := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "nonempty"}[nonempty], func(t *testing.T) {
			engine := NewLazySparseEngine()
			defer engine.Close()
			table, err := engine.New(lazySignature(t, 2), 0)
			if err != nil {
				t.Fatal(err)
			}
			if nonempty {
				if _, err = table.Add(Fact{1}); err != nil {
					t.Fatal(err)
				}
			}
			projected, err := table.Project([]int{0})
			if err != nil {
				t.Fatal(err)
			}
			if projected.Forced() {
				t.Fatal("null-signature projection was eager")
			}
			facts, err := projected.Facts()
			if err != nil {
				t.Fatal(err)
			}
			if len(facts) != map[bool]int{false: 0, true: 1}[nonempty] || len(facts) == 1 && len(facts[0]) != 0 {
				t.Fatal("null-signature projection", facts)
			}
			if projected.root.nullary == nil || projected.root.evaluated != nil {
				t.Fatal("projection did not use concrete null-signature fallback")
			}
		})
	}
}

func TestLazySparseJoinProjectAllKeepsJoinDeferred(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	left, _ := engine.New(lazySignature(t, 2), 0)
	right, _ := engine.New(lazySignature(t, 2), 0)
	_, _ = left.Add(Fact{1})
	_, _ = right.Add(Fact{1})
	joined, err := left.Join(right, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	projected, err := joined.Project([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := projected.Facts()
	if err != nil || len(facts) != 1 || len(facts[0]) != 0 {
		t.Fatal("nullary join/project", facts, err)
	}
	stats := engine.Statistics()
	if joined.Forced() || stats.JoinProjects != 1 || stats.Joins != 0 {
		t.Fatal("nullary join/project materialized shared join", stats, joined.Forced())
	}
}

func TestLazySparseNullaryProjectionAndProductRemainComposable(t *testing.T) {
	for _, truth := range []bool{false, true} {
		t.Run(map[bool]string{false: "false", true: "true"}[truth], func(t *testing.T) {
			engine := NewLazySparseEngine()
			defer engine.Close()
			seed, _ := engine.New(lazySignature(t, 2), 0)
			if truth {
				_, _ = seed.Add(Fact{1})
			}
			nullary, err := seed.Project([]int{0})
			if err != nil {
				t.Fatal(err)
			}
			identity, err := nullary.Project(nil)
			if err != nil {
				t.Fatal("nullary identity project", err)
			}
			other, _ := engine.New(lazySignature(t, 3), 0)
			_, _ = other.Add(Fact{2})
			product, err := identity.Join(other, nil, nil)
			if err != nil {
				t.Fatal("nullary product", err)
			}
			if identity.Forced() || product.Forced() {
				t.Fatal("chained nullary operations were eager")
			}
			facts, err := product.Facts()
			if err != nil {
				t.Fatal(err)
			}
			if truth {
				instructionWantFacts(t, facts, Fact{2})
			} else if len(facts) != 0 {
				t.Fatal("false nullary product", facts)
			}
			identityFacts, err := identity.Facts()
			if err != nil || len(identityFacts) != map[bool]int{false: 0, true: 1}[truth] {
				t.Fatal("nullary identity projection", identityFacts, err)
			}
		})
	}
}

func TestLazySparseProjectionAfterNullaryProductUsesConcreteFallback(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	seed, _ := engine.New(lazySignature(t, 2), 0)
	_, _ = seed.Add(Fact{1})
	truth, err := seed.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := engine.New(lazySignature(t, 2, 3), 0)
	_, _ = other.Add(Fact{1, 2})
	product, err := truth.Join(other, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	projected, err := product.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	if truth.Forced() || product.Forced() || projected.Forced() {
		t.Fatal("mixed nullary product/projection was eager")
	}
	facts, err := projected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{2})
	stats := engine.Statistics()
	if stats.JoinProjects != 0 || stats.Joins != 1 || stats.Projects != 2 {
		t.Fatal("mixed nullary product used invalid direct fusion", stats)
	}
}
