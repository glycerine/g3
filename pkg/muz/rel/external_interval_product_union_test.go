package rel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalIntervalProductUnionWidenDeltaAndRefusal(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := f.ast.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	astTestPin(t, f.ast, realSort)
	one, err := f.ast.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	three, err := f.ast.ArithmeticRealNumeralText("3")
	astTestMust(t, err)
	five, err := f.ast.ArithmeticRealNumeralText("5")
	astTestMust(t, err)
	seven, err := f.ast.ArithmeticRealNumeralText("7")
	astTestMust(t, err)
	for _, value := range []ast.ASTHandle{one, three, five, seven} {
		astTestPin(t, f.ast, value)
	}
	type unionEvent struct {
		op              int32
		inputs, outputs int
	}
	var events []unionEvent
	refusal := errors.New("mixed product union refused")
	failAt := 0
	callbacks := ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
		op, cause := f.ast.GetDeclKind(declaration)
		if cause != nil {
			return cause
		}
		if op == ast.DatalogOpUnion || op == ast.DatalogOpWiden {
			events = append(events, unionEvent{op: op, inputs: len(arguments), outputs: len(outputs)})
			if failAt != 0 && len(events) == failAt {
				return refusal
			}
		}
		return nil
	}}
	plugin, err := NewExternalRelationPlugin(f.relations, callbacks)
	astTestMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	makeProduct := func(value ast.ASTHandle, add bool) *ExternalIntervalProduct {
		t.Helper()
		product, selected, cause := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{realSort}, kinds)
		astTestMust(t, cause)
		if !selected {
			t.Fatal("mixed union product not selected")
		}
		if add {
			astTestMust(t, product.AddFact([]ast.ASTHandle{value}))
		}
		return product
	}
	target, source, delta := makeProduct(one, true), makeProduct(three, true), makeProduct(five, true)
	defer func() {
		astTestMust(t, delta.Close())
		astTestMust(t, source.Close())
		astTestMust(t, target.Close())
		astTestMust(t, plugin.Close())
	}()
	bound := func(product *ExternalIntervalProduct) (string, string) {
		t.Helper()
		lo, _, hi, _, cause := product.interval.Bounds(0)
		astTestMust(t, cause)
		lower, upper := "-oo", "oo"
		if lo != nil {
			lower = lo.RatString()
		}
		if hi != nil {
			upper = hi.RatString()
		}
		return lower, upper
	}

	events = nil
	astTestMust(t, target.Union(source, delta, false))
	if len(events) != 2 || events[0] != (unionEvent{op: ast.DatalogOpUnion, inputs: 2, outputs: 2}) || events[1] != (unionEvent{op: ast.DatalogOpUnion, inputs: 2, outputs: 1}) {
		t.Fatal("mixed union callback shape", events)
	}
	if lo, hi := bound(target); lo != "1" || hi != "3" {
		t.Fatal("mixed union target bounds", lo, hi)
	}
	if lo, hi := bound(delta); lo != "1" || hi != "5" {
		t.Fatal("mixed union delta bounds", lo, hi)
	}
	if lo, hi := bound(source); lo != "3" || hi != "3" {
		t.Fatal("mixed union changed source", lo, hi)
	}

	widenSource := makeProduct(seven, true)
	defer func() { astTestMust(t, widenSource.Close()) }()
	failAt = 1
	events = nil
	if err = target.Union(widenSource, delta, true); !errors.Is(err, refusal) {
		t.Fatal("mixed widen refusal not propagated", err)
	}
	if len(events) != 1 || events[0] != (unionEvent{op: ast.DatalogOpWiden, inputs: 2, outputs: 2}) {
		t.Fatal("mixed widen callback shape", events)
	}
	if lo, hi := bound(target); lo != "1" || hi != "3" {
		t.Fatal("refused widen published target interval", lo, hi)
	}
	if lo, hi := bound(delta); lo != "1" || hi != "5" {
		t.Fatal("refused widen published delta interval", lo, hi)
	}
	failAt = 0
	events = nil
	astTestMust(t, target.Union(widenSource, delta, true))
	if len(events) != 2 || events[0] != (unionEvent{op: ast.DatalogOpWiden, inputs: 2, outputs: 2}) || events[1] != (unionEvent{op: ast.DatalogOpUnion, inputs: 2, outputs: 1}) {
		t.Fatal("mixed successful widen/delta accumulation callbacks", events)
	}
	if lo, hi := bound(target); lo != "1" || hi != "oo" {
		t.Fatal("mixed widen target bounds", lo, hi)
	}
	if lo, hi := bound(delta); lo != "1" || hi != "oo" {
		t.Fatal("mixed widen delta bounds", lo, hi)
	}
	lateTarget, lateDelta := makeProduct(one, true), makeProduct(five, true)
	defer func() {
		astTestMust(t, lateDelta.Close())
		astTestMust(t, lateTarget.Close())
	}()
	events = nil
	failAt = 2
	if err = lateTarget.Union(widenSource, lateDelta, true); !errors.Is(err, refusal) {
		t.Fatal("late delta accumulation refusal not propagated", err)
	}
	failAt = 0
	if lo, hi := bound(lateTarget); lo != "1" || hi != "oo" {
		t.Fatal("late refusal did not publish completed target update", lo, hi)
	}
	if lo, hi := bound(lateDelta); lo != "5" || hi != "5" {
		t.Fatal("late refusal published caller delta candidate", lo, hi)
	}
	emptySource := makeProduct(ast.ASTHandle{}, false)
	defer func() { astTestMust(t, emptySource.Close()) }()
	events = nil
	astTestMust(t, target.Union(emptySource, delta, false))
	if len(events) != 2 || events[0].outputs != 2 || events[1].outputs != 1 {
		t.Fatal("empty-source delta callback sequence", events)
	}
	if lo, hi := bound(delta); lo != "1" || hi != "oo" {
		t.Fatal("empty source discarded accumulated delta", lo, hi)
	}

	reversed, selected, err := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{realSort}, []string{"interval_relation", "external_relation"})
	astTestMust(t, err)
	if !selected {
		t.Fatal("reversed mixed product not selected")
	}
	defer func() { astTestMust(t, reversed.Close()) }()
	events = nil
	astTestMust(t, target.Union(reversed, nil, false))
	if len(events) != 1 || events[0].op != ast.DatalogOpUnion {
		t.Fatal("unaligned kind order did not reach the typed union", events)
	}
	if got := target.Kinds(); len(got) != 2 || got[0] != "external_relation" || got[1] != "interval_relation" {
		t.Fatal("unaligned target did not keep the common specification", got)
	}
	if got := reversed.Kinds(); len(got) != 2 || got[0] != "interval_relation" || got[1] != "external_relation" {
		t.Fatal("unaligned union changed its source specification", got)
	}
	events = nil
	if err = target.Union(source, target, false); !errors.Is(err, ast.ErrASTDomain) || len(events) != 0 {
		t.Fatal("aliased delta reached callback", err, events)
	}
}

func TestExternalIntervalProductUnalignedUnionCanonicalizesWritableSpecs(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := f.ast.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	astTestPin(t, f.ast, realSort)
	value := func(text string) ast.ASTHandle {
		numeral, cause := f.ast.ArithmeticRealNumeralText(text)
		astTestMust(t, cause)
		astTestPin(t, f.ast, numeral)
		return numeral
	}
	one, three, five, seven := value("1"), value("3"), value("5"), value("7")
	type event struct{ op int32 }
	var events []event
	failAt := 0
	refusal := errors.New("unaligned mixed product union refused")
	callbacks := ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, _, _ []ast.ASTHandle) error {
		op, cause := f.ast.GetDeclKind(declaration)
		if cause != nil {
			return cause
		}
		if op == ast.DatalogOpUnion || op == ast.DatalogOpWiden {
			events = append(events, event{op: op})
			if failAt != 0 && len(events) == failAt {
				return refusal
			}
		}
		return nil
	}}
	plugin, err := NewExternalRelationPlugin(f.relations, callbacks)
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	externalFirst := []string{"external_relation", "interval_relation"}
	intervalFirst := []string{"interval_relation", "external_relation"}
	makeProduct := func(kinds []string, numeral ast.ASTHandle) *ExternalIntervalProduct {
		t.Helper()
		product, selected, cause := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{realSort}, kinds)
		astTestMust(t, cause)
		if !selected {
			t.Fatal("unaligned product not selected")
		}
		astTestMust(t, product.AddFact([]ast.ASTHandle{numeral}))
		return product
	}
	assertKinds := func(product *ExternalIntervalProduct, want []string) {
		t.Helper()
		got := product.Kinds()
		if len(got) != len(want) {
			t.Fatal("product kind count", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatal("product kinds", got, want)
			}
		}
	}
	bound := func(product *ExternalIntervalProduct) (string, string) {
		t.Helper()
		lo, _, hi, _, cause := product.interval.Bounds(0)
		astTestMust(t, cause)
		lower, upper := "-oo", "oo"
		if lo != nil {
			lower = lo.RatString()
		}
		if hi != nil {
			upper = hi.RatString()
		}
		return lower, upper
	}

	for _, test := range []struct {
		name                          string
		targetKinds, sourceKinds      []string
		deltaKinds                    []string
		source                        ast.ASTHandle
		sourceText                    string
		widen                         bool
		wantTargetLow, wantTargetHigh string
		wantDeltaLow, wantDeltaHigh   string
	}{
		{name: "union-target-external-first", targetKinds: externalFirst, sourceKinds: intervalFirst, deltaKinds: intervalFirst, source: three, sourceText: "3", wantTargetLow: "1", wantTargetHigh: "3", wantDeltaLow: "1", wantDeltaHigh: "5"},
		{name: "widen-target-interval-first", targetKinds: intervalFirst, sourceKinds: externalFirst, deltaKinds: intervalFirst, source: seven, sourceText: "7", widen: true, wantTargetLow: "1", wantTargetHigh: "oo", wantDeltaLow: "1", wantDeltaHigh: "oo"},
		{name: "delta-forces-common-spec", targetKinds: intervalFirst, sourceKinds: intervalFirst, deltaKinds: externalFirst, source: three, sourceText: "3", wantTargetLow: "1", wantTargetHigh: "3", wantDeltaLow: "1", wantDeltaHigh: "5"},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := makeProduct(test.targetKinds, one)
			source := makeProduct(test.sourceKinds, test.source)
			delta := makeProduct(test.deltaKinds, five)
			defer func() {
				astTestMust(t, delta.Close())
				astTestMust(t, source.Close())
				astTestMust(t, target.Close())
			}()
			events = nil
			failAt = 0
			astTestMust(t, target.Union(source, delta, test.widen))
			wantOp := int32(ast.DatalogOpUnion)
			if test.widen {
				wantOp = ast.DatalogOpWiden
			}
			if len(events) != 2 || events[0].op != wantOp || events[1].op != ast.DatalogOpUnion {
				t.Fatal("unaligned callback sequence", events)
			}
			assertKinds(target, externalFirst)
			assertKinds(delta, externalFirst)
			assertKinds(source, test.sourceKinds)
			if lo, hi := bound(target); lo != test.wantTargetLow || hi != test.wantTargetHigh {
				t.Fatal("unaligned target bounds", lo, hi)
			}
			if lo, hi := bound(delta); lo != test.wantDeltaLow || hi != test.wantDeltaHigh {
				t.Fatal("unaligned delta bounds", lo, hi)
			}
			if lo, hi := bound(source); lo != test.sourceText || hi != test.sourceText {
				t.Fatal("unaligned union changed source", lo, hi)
			}
		})
	}

	t.Run("aligned-reversed-remains-reversed", func(t *testing.T) {
		target, source, delta := makeProduct(intervalFirst, one), makeProduct(intervalFirst, three), makeProduct(intervalFirst, five)
		defer func() { astTestMust(t, delta.Close()); astTestMust(t, source.Close()); astTestMust(t, target.Close()) }()
		events, failAt = nil, 0
		astTestMust(t, target.Union(source, delta, false))
		assertKinds(target, intervalFirst)
		assertKinds(delta, intervalFirst)
		assertKinds(source, intervalFirst)
	})

	for _, refusalAt := range []int{1, 2} {
		t.Run(fmt.Sprintf("callback-refusal-%d", refusalAt), func(t *testing.T) {
			target, source, delta := makeProduct(intervalFirst, one), makeProduct(externalFirst, three), makeProduct(intervalFirst, five)
			defer func() { astTestMust(t, delta.Close()); astTestMust(t, source.Close()); astTestMust(t, target.Close()) }()
			events, failAt = nil, refusalAt
			if cause := target.Union(source, delta, false); !errors.Is(cause, refusal) {
				t.Fatal("unaligned callback refusal", cause)
			}
			assertKinds(target, externalFirst)
			assertKinds(delta, externalFirst)
			assertKinds(source, externalFirst)
			if lo, hi := bound(target); refusalAt == 1 && (lo != "1" || hi != "1") || refusalAt == 2 && (lo != "1" || hi != "3") {
				t.Fatal("unaligned refusal target bounds", refusalAt, lo, hi)
			}
			if lo, hi := bound(delta); lo != "5" || hi != "5" {
				t.Fatal("unaligned refusal published delta", refusalAt, lo, hi)
			}
			failAt = 0
		})
	}

	t.Run("alias-validates-before-conversion", func(t *testing.T) {
		target, source := makeProduct(intervalFirst, one), makeProduct(externalFirst, three)
		defer func() { astTestMust(t, source.Close()); astTestMust(t, target.Close()) }()
		events, failAt = nil, 0
		if cause := target.Union(source, target, false); !errors.Is(cause, ast.ErrASTDomain) || len(events) != 0 {
			t.Fatal("aliased unaligned delta reached conversion or callback", cause, events)
		}
		assertKinds(target, intervalFirst)
		assertKinds(source, externalFirst)
	})
}
