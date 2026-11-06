package smtcore

import (
	"fmt"
	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"

	"errors"

	"os"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usexpr "github.com/glycerine/g3/pkg/util/sexpr"
	"path/filepath"
)

func astTestMust(t testing.TB, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}

func astTestSymbol(t testing.TB, m *ast.ASTManager, text string) usymbol.Symbol {
	t.Helper()
	s, e := m.Symbols.Intern(text)
	astTestMust(t, e)
	return s
}
func astTestManager(t testing.TB) *ast.ASTManager {
	t.Helper()
	s, q, b := astTestOwners(t)
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: s, Numerals: q, Budget: b})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	return m
}
func astTracePath(t *testing.T) string {
	t.Helper()
	f, e := os.CreateTemp(".", "ast-trace-*")
	if e != nil {
		t.Fatal(e)
	}
	name, e := filepath.Abs(f.Name())
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Remove(name) })
	return name
}
func astTestSymbolGraph(s *usymbol.Manager) uint64 {
	x := s.Stats()
	d := x.Region.Descriptors
	return x.OwnerBytes + x.Table.OwnerBytes + x.Table.TableBytes + x.Table.DirectoryBytes + x.Region.OwnerBytes + x.Region.PayloadBytes + x.Region.PageHeaderBytes + x.Region.DirectoryBytes + x.Region.LogBytes + x.Region.ScopeBytes + d.OwnerBytes + d.RecordBytes + d.MetadataBytes + d.DirectoryBytes
}

type upolyPhysicalExtraOwners struct {
	CallerBlocks         []umemory.Block // actual directly returned caller allocations only
	ASTManagers          []*ast.ASTManager
	DebugContexts        []*udiag.DebugContext
	ParamsRationalValues []paramsPhysicalRationalOwner
	ParamsValues         []paramsPhysicalValueOwner
	RationalServices     []*unumeral.RationalService
	SexprManagers        []*usexpr.SexprManager
	Services             []*polynomial.UPolynomialFactorizationService
	Integers             []*unumeral.Int
	Rationals            []*unumeral.Rational
	RationalManagers     []*unumeral.RationalManager
	BinaryValues         []*unumeral.BinaryRational
	BinaryManagers       []*unumeral.BinaryRationalManager
	BinaryVectors        []*polynomial.BinaryRationalVector
	Sequences            []*polynomial.UPolynomialSequence
}

// Read-only independent physical ownership audit. Reflection reads scalar
// metadata; it neither borrows mutable payloads nor repairs failed numerals.
func upolyHenselAuditPhysical(t *testing.T, cores []*polynomial.UPolynomialManager, factors []*polynomial.UPolynomialFactors, extras ...upolyPhysicalExtraOwners) upolyHenselPhysicalGraph {
	var e error
	cores, factors, e = upolyPhysicalRetainedOwners(cores, factors, extras)
	if e != nil {
		t.Fatal(e)
	}
	t.Helper()
	if e := upolyHenselCheckVectorOwners(cores, factors, extras...); e != nil {
		t.Fatal(e)
	}
	var graph upolyHenselPhysicalGraph
	z := reflect.ValueOf(cores[0].IntegerManager()).Elem()
	cells := testFieldBySourceName(z, "cells").Elem()
	pages := testFieldBySourceName(cells, "pages")
	pageSize := testFieldBySourceName(testFieldBySourceName(cells, "config"), "PageSize").Uint()
	slots := testFieldBySourceName(cells, "slots").Uint()
	failed := testFieldBySourceName(z, "failedRef").Uint()
	failedNull := testFieldBySourceName(z, "failedNullRef").Uint()
	named := map[uint64]bool{}
	add := func(ref uint64) {
		if ref == 0 || ref == failed || ref == failedNull {
			return
		}
		slot, gen := uint64(uint32(ref)), ref>>32
		if slot == 0 || slot > slots {
			t.Fatal("owning scalar ref outside numeric directory", ref)
		}
		state := testFieldBySourceName(pages.Index(int((slot-1)/pageSize)), "meta").Index(int((slot - 1) % pageSize))
		if !testFieldBySourceName(state, "live").Bool() || (testFieldBySourceName(z, "failedRef").Type().Size() == 8 && testFieldBySourceName(state, "generation").Uint() != gen) {
			t.Fatal("owning scalar ref stale", ref)
		}

		named[testFieldBySourceName(state, "generation").Uint()<<32|slot] = true
	}
	addInt := func(v reflect.Value) { add(testFieldBySourceName(v, "cell").Uint()) }
	for _, field := range []string{"tmp", "arg"} {
		v := testFieldBySourceName(z, field)
		for i := 0; i < v.Len(); i++ {
			add(v.Index(i).Uint())
		}
	}

	for _, field := range []string{"divisionRaw", "extendedRaw", "gcdRaw"} {
		scope := testFieldBySourceName(z, field)
		values := testFieldBySourceName(scope, "values")
		for i := uint64(0); i < testFieldBySourceName(scope, "count").Uint(); i++ {
			addInt(values.Index(int(i)))
		}
	}
	addMul := testFieldBySourceName(z, "addMulInvocation")
	for _, child := range []reflect.Value{testFieldBySourceName(testFieldBySourceName(z, "gcdRaw"), "addMul"), testFieldBySourceName(addMul, "receipt")} {
		if testFieldBySourceName(child, "manager").Pointer() == z.Addr().Pointer() && testFieldBySourceName(child, "frame").Pointer() == addMul.Addr().Pointer() && testFieldBySourceName(child, "invocation").Uint() != 0 && testFieldBySourceName(child, "invocation").Uint() == testFieldBySourceName(addMul, "invocation").Uint() && testFieldBySourceName(addMul, "active").Bool() {
			addInt(testFieldBySourceName(addMul, "tmp"))
		}
	}
	addInt(testFieldBySourceName(z, "intMin"))
	addInt(testFieldBySourceName(z, "two64"))
	rootRAW := testFieldBySourceName(testFieldBySourceName(z, "rootRaw"), "values")
	for i := 0; i < rootRAW.Len(); i++ {
		addInt(rootRAW.Index(i))
	}
	addInt(testFieldBySourceName(testFieldBySourceName(z, "powerRaw"), "value"))
	stores := map[uintptr]reflect.Value{}
	for _, owner := range cores {
		core := reflect.ValueOf(owner).Elem()
		store := testFieldBySourceName(core, "store")
		stores[store.Pointer()] = store.Elem()
		bindings := testFieldBySourceName(core, "bindings")
		for i := 0; i < bindings.Len(); i++ {
			store = testFieldBySourceName(bindings.Index(i), "store")
			stores[store.Pointer()] = store.Elem()
		}
		modular := testFieldBySourceName(core, "modular").Elem()
		for _, field := range []string{"p", "lower", "upper", "invTmp1", "invTmp2", "invTmp3", "divTmp"} {
			addInt(testFieldBySourceName(modular, field))
		}
	}

	for _, store := range stores {
		backings := testFieldBySourceName(store, "backings")
		for i := 0; i < backings.Len(); i++ {
			b := backings.Index(i)
			if !testFieldBySourceName(b, "live").Bool() {
				continue
			}
			values := testFieldBySourceName(b, "values")
			n := testFieldBySourceName(b, "initialized").Uint()
			if n > uint64(values.Len()) {
				t.Fatal("coefficient physical extent", n, values.Len())
			}
			for j := uint64(0); j < n; j++ {
				addInt(values.Index(int(j)))
			}
		}
	}
	for _, fs := range factors {
		addInt(testFieldBySourceName(reflect.ValueOf(fs).Elem(), "constant"))
	}
	upolyPhysicalExtraNumerals(extras, addInt)
	for _, extra := range extras {
		for _, service := range extra.Services {
			front := testFieldBySourceName(reflect.ValueOf(service).Elem(), "frontend")
			addInt(testFieldBySourceName(front, "content"))
			for _, x := range []struct{ scope, value string }{{"vectorNormalize", "g"}, {"vectorNormalize", "old"}, {"vectorNormalize", "tmp"}, {"modularPower", "power"}, {"flip", "c"}, {"image", "old"}, {"image", "c"}, {"remainder", "am"}, {"monic", "lc"}, {"monic", "inv"}, {"derivative", "d"}, {"divisibility", "ratio"}, {"primitive", "cont"}} {
				addInt(testFieldBySourceName(testFieldBySourceName(front, x.scope), x.value))
			}
			for _, name := range []string{"quadratic", "modularGCD", "cra"} {
				values := testFieldBySourceName(testFieldBySourceName(front, name), "values")
				for i := 0; i < values.Len(); i++ {
					addInt(values.Index(i))
				}
			}
			for r := testFieldBySourceName(testFieldBySourceName(front, "derivative"), "retired"); !r.IsNil(); r = testFieldBySourceName(r.Elem(), "next") {
				addInt(testFieldBySourceName(r.Elem(), "value"))
			}
			for pair := testFieldBySourceName(reflect.ValueOf(service).Elem(), "henselPairs"); !pair.IsNil(); pair = testFieldBySourceName(pair.Elem(), "next") {
				h := pair.Elem()
				if testFieldBySourceName(h, "amConstructed").Bool() {
					addInt(testFieldBySourceName(h, "am"))
				}
				check := testFieldBySourceName(h, "check")
				if testFieldBySourceName(check, "constructed").Uint() >= 1 {
					addInt(testFieldBySourceName(check, "br"))
				}
				if testFieldBySourceName(check, "transportBound").Bool() {
					addInt(testFieldBySourceName(check, "transport"))
				}
			}
			scope := testFieldBySourceName(reflect.ValueOf(service).Elem(), "scope")
			if scope.IsNil() {
				continue
			}
			scope = scope.Elem()
			caller := testFieldBySourceName(scope, "hensel")
			for _, n := range []struct{ flag, value string }{{"amConstructed", "am"}, {"initialLCConstructed", "initialLC"}, {"finalLCInvConstructed", "finalLCInv"}, {"checkPowerConstructed", "checkPower"}, {"transportBound", "transport"}} {
				if testFieldBySourceName(caller, n.flag).Bool() {
					addInt(testFieldBySourceName(caller, n.value))
				}
			}
			if testFieldBySourceName(caller, "extendedRawConstructed").Bool() {
				for i := 0; i < 2; i++ {
					addInt(testFieldBySourceName(caller, "monicRaw").Index(i))
				}
			}
			for i := 0; i < int(testFieldBySourceName(caller, "extendedScopedConstructed").Uint()); i++ {
				addInt(testFieldBySourceName(caller, "extendedScoped").Index(i))
			}
			prime := testFieldBySourceName(scope, "primeTrial")
			if testFieldBySourceName(prime, "transportBound").Bool() {
				addInt(testFieldBySourceName(prime, "transport"))
			}
			for _, field := range []string{"p", "bestP", "gcdTmp", "leading", "trialContent"} {
				addInt(testFieldBySourceName(scope, field))
			}
		}
	}
	runtime := testFieldBySourceName(z, "memory").Elem()
	blocks := testFieldBySourceName(runtime, "blocks")
	allocator := testFieldBySourceName(z, "allocator").Elem()
	locations := testFieldBySourceName(allocator, "locations")
	objects := map[[4]uint64]bool{}
	rawRoots := map[[3]uint64]bool{}
	rawKey := func(v reflect.Value) [3]uint64 {
		return [3]uint64{testFieldBySourceName(v, "origin").Uint(), testFieldBySourceName(v, "slot").Uint(), testFieldBySourceName(v, "generation").Uint()}
	}
	addBacking := func(backing reflect.Value, complete bool) {
		object := testFieldBySourceName(backing, "object")
		key := [4]uint64{testFieldBySourceName(object, "owner").Uint(), testFieldBySourceName(object, "address").Uint(), testFieldBySourceName(object, "slot").Uint(), testFieldBySourceName(object, "generation").Uint()}
		if key == [4]uint64{} {
			return
		}
		if key[0] != testFieldBySourceName(allocator, "origin").Uint() || key[2] == 0 || key[2] > uint64(locations.Len()) {
			t.Fatal("physical object owner/slot", key)
		}
		loc := locations.Index(int(key[2] - 1))
		if !testFieldBySourceName(loc, "live").Bool() || testFieldBySourceName(loc, "free").Bool() || testFieldBySourceName(loc, "generation").Uint() != key[3] || testFieldBySourceName(loc, "address").Uint() != key[1] {
			t.Fatal("physical owned object lifetime", key, loc)
		}
		if objects[key] {
			t.Fatal("two numeric owners of one physical object", key)
		}
		objects[key] = true
		raw := rawKey(testFieldBySourceName(loc, "raw"))
		if raw[0] != testFieldBySourceName(runtime, "origin").Uint() || raw[1] == 0 || raw[1] > uint64(blocks.Len()) {
			t.Fatal("owned cell raw block", raw)
		}
		rawBlock := blocks.Index(int(raw[1] - 1))
		if !testFieldBySourceName(rawBlock, "live").Bool() || testFieldBySourceName(rawBlock, "generation").Uint() != raw[2] {
			t.Fatal("owned cell raw lifetime", raw)
		}
		extent := testFieldBySourceName(loc, "extent").Uint()
		if testFieldBySourceName(loc, "kind").Uint() == 3 {
			width := testFieldBySourceName(testFieldBySourceName(runtime, "config"), "SizeTBits").Uint() / 8
			data := testFieldBySourceName(rawBlock, "data")
			initialized := testFieldBySourceName(rawBlock, "initialized")
			extent = 0
			for i := uint64(0); i < width; i++ {
				if initialized.Index(int(i/64)).Uint()&(uint64(1)<<(i%64)) == 0 {
					t.Fatal("direct runtime header not initialized")
				}
				extent |= data.Index(int(i)).Uint() << (8 * i)
			}
			extent -= width
		}
		if complete && (rawKey(testFieldBySourceName(loc, "raw")) != rawKey(testFieldBySourceName(backing, "raw")) || testFieldBySourceName(loc, "offset").Uint() != testFieldBySourceName(backing, "offset").Uint() || extent != testFieldBySourceName(backing, "extent").Uint() || testFieldBySourceName(loc, "requested").Uint() != testFieldBySourceName(backing, "requested").Uint()) {
			t.Fatal("physical cell bounds/location", key)
		}
	}
	live := uint32(0)
	for slot := uint64(1); slot <= slots; slot++ {
		page := pages.Index(int((slot - 1) / pageSize))
		index := int((slot - 1) % pageSize)
		state := testFieldBySourceName(page, "meta").Index(index)
		if !testFieldBySourceName(state, "live").Bool() {
			continue
		}
		live++
		ref := testFieldBySourceName(state, "generation").Uint()<<32 | slot
		record := testFieldBySourceName(page, "records").Index(index)
		if named[ref] {
			graph.Named++
		} else if testFieldBySourceName(record, "unpublished").Bool() {
			graph.Unpublished++
		} else if testFieldBySourceName(record, "cleanupPending").Bool() {
			graph.Cleanup++
		} else {
			t.Fatal("anonymous published numeric descriptor", ref)
		}
		if !testFieldBySourceName(record, "released").Bool() {
			addBacking(testFieldBySourceName(record, "backing"), true)
		}
	}
	if live != graph.Named+graph.Unpublished+graph.Cleanup || uint32(len(named)) != graph.Named || uint64(live) != testFieldBySourceName(cells, "live").Uint() {
		t.Fatal("exact descriptor root census", live, graph, len(named))
	}
	pending := testFieldBySourceName(z, "pendingCell")
	if testFieldBySourceName(testFieldBySourceName(pending, "object"), "slot").Uint() != 0 {
		graph.Pending++
		addBacking(pending, false)
	}
	graph.Objects = uint32(len(objects))
	for i := 0; i < locations.Len(); i++ {
		loc := locations.Index(i)
		if !testFieldBySourceName(loc, "live").Bool() {
			continue
		}
		if !testFieldBySourceName(loc, "free").Bool() && testFieldBySourceName(loc, "kind").Uint() != 1 {
			key := [4]uint64{testFieldBySourceName(allocator, "origin").Uint(), testFieldBySourceName(loc, "address").Uint(), uint64(i + 1), testFieldBySourceName(loc, "generation").Uint()}
			if !objects[key] {
				t.Fatal("anonymous live allocator object", key)
			}
		}
		rawRoots[rawKey(testFieldBySourceName(loc, "raw"))] = true
	}
	key := rawKey(testFieldBySourceName(allocator, "pending"))
	if key[1] != 0 {
		rawRoots[key] = true
	}

	for _, extra := range extras {
		for _, b := range extra.CallerBlocks {
			key := [3]uint64{b.OwnerID(), uint64(b.Slot()), uint64(b.Generation())}
			if key[0] != testFieldBySourceName(runtime, "origin").Uint() || key[1] == 0 || key[1] > uint64(blocks.Len()) || rawRoots[key] {
				t.Fatal("invalid or duplicate caller raw acquisition", key)
			}
			block := blocks.Index(int(key[1] - 1))
			if !testFieldBySourceName(block, "live").Bool() || testFieldBySourceName(block, "generation").Uint() != key[2] {
				t.Fatal("ended caller raw acquisition", key)
			}
			rawRoots[key] = true
		}
		for _, d := range extra.DebugContexts {
			if d == nil || d.Self != d {
				t.Fatal("invalid named DebugContext")
			}
		}
	}

	formatOwner := testFieldBySourceName(z, "natural").Elem()
	addFormattingRaw := func(v reflect.Value) {
		for _, field := range []string{"raw", "pending"} {
			key := rawKey(testFieldBySourceName(v, field))
			if key[1] != 0 {
				if key[0] != testFieldBySourceName(runtime, "origin").Uint() || rawRoots[key] {
					t.Fatal("foreign or duplicate lexical formatting root", key)
				}
				rawRoots[key] = true
			}
		}
	}
	formatting := testFieldBySourceName(formatOwner, "formatting")
	if testFieldBySourceName(formatting, "active").Bool() || testFieldBySourceName(formatting, "pending").Bool() || testFieldBySourceName(formatting, "completed").Uint() != 0 {
		locals := testFieldBySourceName(formatting, "locals")
		for i := 0; i < locals.Len(); i++ {
			addFormattingRaw(locals.Index(i))
		}
	}
	for i := 0; i < blocks.Len(); i++ {
		block := blocks.Index(i)
		if testFieldBySourceName(block, "live").Bool() {
			key := [3]uint64{testFieldBySourceName(runtime, "origin").Uint(), uint64(i + 1), testFieldBySourceName(block, "generation").Uint()}
			if !rawRoots[key] {
				t.Fatal("anonymous raw numeric allocation", key)
			}
		}
	}

	return graph
}
func astTestOwners(t testing.TB) (*usymbol.Manager, *unumeral.RationalService, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(1 << 30)
	s, e := usymbol.NewManager(usymbol.Config{Budget: b})
	astTestMust(t, e)
	q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	astTestMust(t, e)
	t.Cleanup(func() {
		astTestMust(t, q.Close())
		astTestMust(t, s.Close())
		if b.Used() != 0 {
			t.Errorf("owner graph retained %d bytes", b.Used())
		}
	})
	return s, q, b
}

type upolyHenselPhysicalGraph struct{ Named, Unpublished, Cleanup, Pending, Objects uint32 }

// Only actual declared retained lexical owner fields are roots. Accessors borrow
// the charged owners; no live-descriptor registry can manufacture ownership.
func upolyPhysicalRetainedOwners(cores []*polynomial.UPolynomialManager, factors []*polynomial.UPolynomialFactors, extras []upolyPhysicalExtraOwners) ([]*polynomial.UPolynomialManager, []*polynomial.UPolynomialFactors, error) {
	cs := append([]*polynomial.UPolynomialManager(nil), cores...)
	fs := append([]*polynomial.UPolynomialFactors(nil), factors...)
	for _, extra := range extras {
		for _, service := range extra.Services {
			for i := uint32(0); ; i++ {
				c, e := service.RetainedCleanupCore(i)
				if errors.Is(e, arena.ErrBounds) {
					break
				}
				if e != nil {
					return nil, nil, e
				}
				found := false
				for _, old := range cs {
					if old == c {
						found = true
					}
				}
				if !found {
					cs = append(cs, c)
				}
			}
			for i := uint32(0); i < 3; i++ {
				f, e := service.RetainedCleanupFactors(i)
				if errors.Is(e, arena.ErrBounds) {
					break
				}
				if e != nil {
					return nil, nil, e
				}
				found := false
				for _, old := range fs {
					if old == f {
						found = true
					}
				}
				if !found {
					fs = append(fs, f)
				}
			}
		}
	}
	return cs, fs, nil
}

// Every live coefficient buffer must be reachable through a concrete source
// vector state; the storage registry is physical metadata, not a vector owner.
func upolyHenselCheckVectorOwners(cores []*polynomial.UPolynomialManager, factors []*polynomial.UPolynomialFactors, extras ...upolyPhysicalExtraOwners) error {
	var e error
	cores, factors, e = upolyPhysicalRetainedOwners(cores, factors, extras)
	if e != nil {
		return e
	}
	stores := map[uint64]reflect.Value{}
	for _, core := range cores {
		v := reflect.ValueOf(core).Elem()
		store := testFieldBySourceName(v, "store").Elem()
		stores[testFieldBySourceName(store, "origin").Uint()] = store
		bindings := testFieldBySourceName(v, "bindings")
		for i := 0; i < bindings.Len(); i++ {
			store = testFieldBySourceName(bindings.Index(i), "store").Elem()
			stores[testFieldBySourceName(store, "origin").Uint()] = store
		}
	}
	owners := map[[3]uint64]string{}
	add := func(core reflect.Value, state reflect.Value, label string) error {
		backing := testFieldBySourceName(state, "backing").Uint()
		if backing == 0 {
			return nil
		}
		origin := testFieldBySourceName(state, "origin").Uint()
		if origin == 0 {
			origin = testFieldBySourceName(testFieldBySourceName(core, "store").Elem(), "origin").Uint()
		}
		key := [3]uint64{origin, backing, testFieldBySourceName(state, "generation").Uint()}
		store, ok := stores[origin]
		if !ok {
			return fmt.Errorf("%s vector has no bound physical origin %v", label, key)
		}
		backings := testFieldBySourceName(store, "backings")
		if backing > uint64(backings.Len()) {
			return fmt.Errorf("%s vector outside physical directory %v", label, key)
		}
		b := backings.Index(int(backing - 1))
		if !testFieldBySourceName(b, "live").Bool() || testFieldBySourceName(b, "generation").Uint() != key[2] {
			return fmt.Errorf("%s vector physical lifetime mismatch %v", label, key)
		}
		if previous, ok := owners[key]; ok {
			return fmt.Errorf("one buffer has two scalar vector owners %v: %s and %s", key, previous, label)
		}
		owners[key] = label
		if testFieldBySourceName(state, "capacity").Uint() != uint64(testFieldBySourceName(b, "values").Len()) || testFieldBySourceName(state, "size").Uint() > testFieldBySourceName(state, "capacity").Uint() {
			return fmt.Errorf("%s vector physical capacity mismatch %v", label, key)
		}
		return nil
	}
	for index, owner := range cores {
		core := reflect.ValueOf(owner).Elem()
		for _, field := range []string{"basic", "div1", "div2", "exactDiv", "gcd1", "gcd2", "cra", "sqf1", "sqf2", "pw", "db", "dbab1", "dbab2", "tr", "push"} {
			if e := add(core, testFieldBySourceName(core, field), fmt.Sprintf("core%d.%s", index, field)); e != nil {
				return e
			}
		}
		mgcd := testFieldBySourceName(core, "mgcd")
		for i := 0; i < mgcd.Len(); i++ {
			if e := add(core, mgcd.Index(i), fmt.Sprintf("core%d.mgcd%d", index, i)); e != nil {
				return e
			}
		}
		pages := testFieldBySourceName(core, "pages")
		for i := 0; i < pages.Len(); i++ {
			rows := testFieldBySourceName(pages.Index(i).Elem(), "rows")
			for j := 0; j < rows.Len(); j++ {
				row := rows.Index(j)
				if testFieldBySourceName(row, "live").Bool() {
					if e := add(core, testFieldBySourceName(row, "state"), fmt.Sprintf("core%d.polynomial%d", index, i*rows.Len()+j)); e != nil {
						return e
					}
				}
			}
		}
	}
	for index, owner := range factors {
		fs := reflect.ValueOf(owner).Elem()
		core := testFieldBySourceName(fs, "manager").Elem()
		states := testFieldBySourceName(fs, "factors")
		for i := uint64(0); i < testFieldBySourceName(fs, "count").Uint(); i++ {
			if e := add(core, states.Index(int(i)), fmt.Sprintf("factors%d.element%d", index, i)); e != nil {
				return e
			}
		}
	}
	if e := upolyPhysicalExtraVectors(extras, add); e != nil {
		return e
	}
	for _, extra := range extras {
		for _, service := range extra.Services {
			front := testFieldBySourceName(reflect.ValueOf(service).Elem(), "frontend")
			root := testFieldBySourceName(reflect.ValueOf(service).Elem(), "manager").Elem()
			for _, name := range []string{"pp", "prime", "A", "B", "D"} {
				if e := add(root, testFieldBySourceName(front, name), "actual factorFrontend."+name); e != nil {
					return e
				}
			}
			for _, name := range []string{"f1", "f2", "product"} {
				if e := add(root, testFieldBySourceName(testFieldBySourceName(front, "quadratic"), name), "actual factorFrontend.quadratic."+name); e != nil {
					return e
				}
			}
			for pair := testFieldBySourceName(reflect.ValueOf(service).Elem(), "henselPairs"); !pair.IsNil(); pair = testFieldBySourceName(pair.Elem(), "next") {
				h := pair.Elem()
				root := testFieldBySourceName(testFieldBySourceName(h, "service").Elem(), "manager").Elem()
				for _, field := range []string{"f", "Vf", "t", "S", "T", "tmp"} {
					owner := root
					if field != "f" && !testFieldBySourceName(h, "rm").IsNil() {
						owner = testFieldBySourceName(h, "rm").Elem()
					}
					if e := add(owner, testFieldBySourceName(h, field), "retained henselPair."+field); e != nil {
						return e
					}
				}
				check := testFieldBySourceName(h, "check")
				for _, field := range []string{"test1", "test2a", "test2b", "test3a", "test3b"} {
					if e := add(root, testFieldBySourceName(check, field), "retained henselCheck."+field); e != nil {
						return e
					}
				}
			}
			scope := testFieldBySourceName(reflect.ValueOf(service).Elem(), "scope")
			if scope.IsNil() {
				continue
			}
			scope = scope.Elem()
			caller := testFieldBySourceName(scope, "hensel")
			for _, name := range []string{"outer", "quadratic", "iteration"} {
				group := testFieldBySourceName(caller, name)
				for i := 0; i < group.Len(); i++ {
					if e := add(testFieldBySourceName(scope, "manager").Elem(), group.Index(i), "retained henselCaller."+name); e != nil {
						return e
					}
				}
			}
			prime := testFieldBySourceName(scope, "primeTrial")
			if !testFieldBySourceName(prime, "zp").IsNil() {
				if e := add(testFieldBySourceName(prime, "zp").Elem(), testFieldBySourceName(prime, "fzp"), "retained primeTrial.f_pp_zp"); e != nil {
					return e
				}
			}
			core := testFieldBySourceName(scope, "manager").Elem()
			for _, field := range []string{"fpp", "trial", "quotient"} {
				if e := add(core, testFieldBySourceName(scope, field), "retained factorSquareFree."+field); e != nil {
					return e
				}
			}
		}
	}
	for origin, store := range stores {
		backings := testFieldBySourceName(store, "backings")
		for i := 0; i < backings.Len(); i++ {
			b := backings.Index(i)
			if testFieldBySourceName(b, "live").Bool() {
				key := [3]uint64{origin, uint64(i + 1), testFieldBySourceName(b, "generation").Uint()}
				if _, ok := owners[key]; !ok {
					return fmt.Errorf("anonymous live coefficient vector %v", key)
				}
			}
		}
	}
	return nil
}
func upolyPhysicalExtraNumerals(extras []upolyPhysicalExtraOwners, addInt func(reflect.Value)) {
	for _, x := range extras {

		addRat := func(v reflect.Value) {
			addInt(testFieldBySourceName(v, "num"))
			addInt(testFieldBySourceName(v, "den"))
		}
		addManager := func(v reflect.Value) {
			for _, f := range []string{"nTmp", "addTmp1", "addTmp2"} {
				addInt(testFieldBySourceName(v, f))
			}
			for _, f := range []string{"addMulTmp", "ltTmp1", "ltTmp2"} {
				addRat(testFieldBySourceName(v, f))
			}
			scope := testFieldBySourceName(v, "lexical")
			for i := 0; i < int(testFieldBySourceName(scope, "count").Uint()); i++ {
				addRat(testFieldBySourceName(scope, "roots").Index(i))
			}
			raw := testFieldBySourceName(v, "rawArithmetic")
			if testFieldBySourceName(raw, "divActive").Bool() {
				addInt(testFieldBySourceName(raw, "div"))
			}
			if testFieldBySourceName(raw, "powerActive").Bool() {
				addRat(testFieldBySourceName(raw, "power"))
			}
			if testFieldBySourceName(raw, "addActive").Bool() {
				addInt(testFieldBySourceName(raw, "addOne"))
				addInt(testFieldBySourceName(raw, "addTwo"))
			}
			if testFieldBySourceName(raw, "compareActive").Bool() {
				addRat(testFieldBySourceName(raw, "compareOne"))
				addRat(testFieldBySourceName(raw, "compareTwo"))
			}
			if testFieldBySourceName(raw, "normActive").Bool() {
				addInt(testFieldBySourceName(raw, "norm"))
			}
		}

		for _, owner := range x.ASTManagers {
			if owner == nil || owner.Self != owner || owner.Closed {
				panic(ast.ErrASTHandle)
			}
			for _, page := range owner.Parameters {
				if page == nil {
					continue
				}
				for i := range page {
					value := &page[i]
					if value.Live && value.Kind == ast.ASTParameterRational {
						addRat(testFieldBySourceName(reflect.ValueOf(&value.Rational).Elem(), "raw"))
					}
				}
			}
			for f := owner.ArrayCardinality; f != nil; f = f.Next {
				if f.Self != f || f.Owner != owner || f.Invocation == 0 {
					panic(ast.ErrASTHandle)
				}
				for i, owned := range f.Acquired {
					if owned {
						addRat(testFieldBySourceName(reflect.ValueOf(&f.Values[i]).Elem(), "raw"))
					}
				}
			}
			if owner.LowLevel.Active && owner.LowLevel.NumeralAcquired {
				addRat(testFieldBySourceName(reflect.ValueOf(&owner.LowLevel.Numeral).Elem(), "raw"))
			}
		}
		for _, owner := range x.ParamsRationalValues {
			if owner.Service == nil || owner.Value == nil {
				panic(unumeral.ErrRationalServiceOwner)
			}
			service := reflect.ValueOf(owner.Service).Elem()
			value := reflect.ValueOf(owner.Value).Elem()
			if testFieldBySourceName(service, "self").Pointer() != reflect.ValueOf(owner.Service).Pointer() || testFieldBySourceName(value, "ownerID").Uint() == 0 || testFieldBySourceName(value, "ownerID").Uint() != testFieldBySourceName(service, "ownerID").Uint() {
				panic(unumeral.ErrRationalServiceOwner)
			}
			addRat(testFieldBySourceName(value, "raw"))
		}
		paramsContexts := make(map[*uparams.ParamsContext]bool)
		for _, owner := range x.ParamsValues {
			if owner.Context == nil || owner.Value == nil {
				panic(uparams.ErrParamsOwner)
			}
			c, v := owner.Context, owner.Value
			if !paramsContexts[c] {
				paramsContexts[c] = true
				for i, frame := range c.SetFrames {
					if !frame.Active {
						continue
					}
					if !frame.Acquired || frame.Invocation == 0 || i >= len(c.Blocks) || frame.Generation != c.Blocks[i].Generation || frame.Next.Kind != uparams.ParamNumeral {
						panic(uparams.ErrParamsOwner)
					}
					raw := reflect.ValueOf(&frame.Next.Rat).Elem()
					if testFieldBySourceName(raw, "ownerID").Uint() != testFieldBySourceName(reflect.ValueOf(c.NumeralsField).Elem(), "ownerID").Uint() {
						panic(unumeral.ErrRationalServiceOwner)
					}
					addRat(testFieldBySourceName(raw, "raw"))
				}
				for _, frame := range c.SourceFrames {
					if !frame.Active {
						continue
					}
					if frame.Invocation == 0 || frame.HeldSlot == 0 || int(frame.HeldSlot) > len(c.Blocks) {
						panic(uparams.ErrParamsOwner)
					}
					held := &c.Blocks[frame.HeldSlot-1]
					if !held.Live || held.Generation != frame.HeldGeneration {
						panic(uparams.ErrParamsOwner)
					}
					for i := uint32(0); i < held.Size; i++ {
						entry := c.Entry(held, i)
						if entry.Kind == uparams.ParamNumeral {
							addRat(testFieldBySourceName(reflect.ValueOf(&entry.Rat).Elem(), "raw"))
						}
					}
				}
			}
			if c.Self != c || c.Closed || v.Owner != c.Owner {
				panic(uparams.ErrParamsOwner)
			}
			if v.Slot == 0 {
				continue
			}
			if int(v.Slot) > len(c.Blocks) {
				panic(uparams.ErrParamsOwner)
			}
			b := &c.Blocks[v.Slot-1]
			if !b.Live || b.Generation != v.Generation {
				panic(uparams.ErrParamsOwner)
			}

			for i := uint32(0); i < b.Size; i++ {
				e := c.Entry(b, i)
				if e.Kind == uparams.ParamNumeral {
					addRat(testFieldBySourceName(reflect.ValueOf(&e.Rat).Elem(), "raw"))
				}
			}
		}
		for _, s := range x.RationalServices {
			v := reflect.ValueOf(s).Elem()
			addManager(testFieldBySourceName(v, "manager").Elem())
			for _, f := range []string{"zero", "one", "minusOne"} {
				addRat(testFieldBySourceName(v, f))
			}
			cache := testFieldBySourceName(v, "cache")
			for i := 0; i < cache.Len(); i++ {
				addRat(cache.Index(i))
			}
			scope := testFieldBySourceName(v, "lexical")
			for i := 0; i < int(testFieldBySourceName(scope, "count").Uint()); i++ {
				addRat(testFieldBySourceName(scope, "roots").Index(i))
			}
		}
		for _, s := range x.SexprManagers {
			for _, page := range s.Pages {
				for i := range page.Nodes {
					r := &page.Nodes[i]
					if r.Live && (r.Kind == usexpr.SexprNumeral || r.Kind == usexpr.SexprBVNumeral) {
						addRat(testFieldBySourceName(reflect.ValueOf(r.Numeral), "raw"))
					}
				}
			}
		}
		for _, n := range x.Integers {
			addInt(reflect.ValueOf(n).Elem())
		}
		for _, r := range x.Rationals {
			v := reflect.ValueOf(r).Elem()
			addInt(testFieldBySourceName(v, "num"))
			addInt(testFieldBySourceName(v, "den"))
		}
		for _, m := range x.RationalManagers {
			addManager(reflect.ValueOf(m).Elem())
		}
		for _, b := range x.BinaryValues {
			addInt(testFieldBySourceName(reflect.ValueOf(b).Elem(), "num"))
		}
		for _, m := range x.BinaryManagers {
			v := reflect.ValueOf(m).Elem()
			for _, f := range []string{"tmp", "tmp2", "selectIntTmp1", "selectIntTmp2", "selectSmallTmp", "divTmp1", "divTmp2", "divTmp3"} {
				addInt(testFieldBySourceName(v, f))
			}
			for _, f := range []string{"addMulTmp", "selectSmallTmp1", "selectSmallTmp2"} {
				addInt(testFieldBySourceName(testFieldBySourceName(v, f), "num"))
			}
		}
		for _, b := range x.BinaryVectors {
			state := testFieldBySourceName(reflect.ValueOf(b).Elem(), "state")
			values := testFieldBySourceName(state, "values")
			for i := uint64(0); i < testFieldBySourceName(state, "initialized").Uint(); i++ {
				addInt(testFieldBySourceName(values.Index(int(i)), "num"))
			}
		}
	}
}

func upolyPhysicalExtraVectors(extras []upolyPhysicalExtraOwners, add func(reflect.Value, reflect.Value, string) error) error {
	for _, x := range extras {
		for i, s := range x.Sequences {
			v := reflect.ValueOf(s).Elem()
			if e := add(testFieldBySourceName(v, "manager").Elem(), testFieldBySourceName(testFieldBySourceName(v, "state"), "coefficients"), fmt.Sprintf("sequence%d", i)); e != nil {
				return e
			}
		}
	}
	return nil
}
