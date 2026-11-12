package protomodel

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func datatypeFactoryTraceFixture(t *testing.T, out io.Writer) (*ast.ASTManager, *DatatypeFactory, *diagnostic.TraceTags, ast.ASTHandle, []ast.ASTHandle) {
	t.Helper()
	owner, _, config := protoFixtureWithConfig(t)
	previous, err := uparams.BindProcessGlobalParamsService(config.Globals)
	protoMust(t, err)
	t.Cleanup(func() { _, err := uparams.BindProcessGlobalParamsService(previous); protoMust(t, err) })
	tags := diagnostic.NewTraceTagsWithWriter(out)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: owner.Symbols, Numerals: owner.NumeralService(), Budget: owner.Budget(), TraceTags: tags})
	protoMust(t, err)
	protoMust(t, m.RegisterDeclarationPlugins())
	t.Cleanup(func() { protoMust(t, m.Close()) })
	p, err := NewProtoModel(m, config)
	protoMust(t, err)
	t.Cleanup(func() { protoMust(t, p.Close()) })
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: protoSymbol(t, m, "TraceNat"), Constructors: []ast.DatatypeConstructorDefinition{
		{Name: protoSymbol(t, m, "tz"), Recognizer: protoSymbol(t, m, "is-tz")},
		{Name: protoSymbol(t, m, "ts"), Recognizer: protoSymbol(t, m, "is-ts"), Accessors: []ast.DatatypeAccessorDefinition{{Name: protoSymbol(t, m, "tp"), Recursive: true}}},
	}}})
	protoMust(t, err)
	protoPin(t, m, sorts[0])
	f, err := NewDatatypeFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	constructors, err := m.DatatypeConstructors(sorts[0])
	protoMust(t, err)
	return m, f, tags, sorts[0], constructors
}

func TestDatatypeFactoryColdFreshExactTrace(t *testing.T) {
	var out bytes.Buffer
	m, f, tags, sort, _ := datatypeFactoryTraceFixture(t, &out)
	tags.Enable("datatype")
	v, err := f.FreshValue(sort)
	protoMust(t, err)
	if v.IsNull() || f.last[sort] != v {
		t.Fatal("cold fresh state")
	}
	want := ""
	if tags.ShouldTrace("datatype") {
		want = "-------- [datatype] get_fresh_value src/smt/proto_model/datatype_factory.cpp:139 ---------\ngenerating fresh value for: TraceNat\n------------------------------------------------\n" +
			"-------- [datatype] get_some_value src/smt/proto_model/datatype_factory.cpp:41 ---------\ntz\n------------------------------------------------\n" +
			"-------- [datatype] get_fresh_value src/smt/proto_model/datatype_factory.cpp:147 ---------\n0. result: tz\n------------------------------------------------\n"
	}
	if out.String() != want {
		t.Fatalf("datatype source trace\ngot %q\nwant %q", out.String(), want)
	}
	before := m.Stats().LiveReferences
	_, err = f.SomeValue(sort)
	protoMust(t, err)
	if out.String() != want || m.Stats().LiveReferences != before {
		t.Fatal("warm SomeValue traced or retained owners")
	}
	fm, err := m.GetFormatManager()
	protoMust(t, err)
	if fm.Stats().LiveReferences != 0 {
		t.Fatal("factory trace retained a format_ref")
	}
}

func TestDatatypeFactoryRecursiveTraceAndNestedLastValue(t *testing.T) {
	var out bytes.Buffer
	_, f, tags, sort, _ := datatypeFactoryTraceFixture(t, &out)
	tags.Enable("datatype")
	for i := 0; i < 3; i++ {
		v, err := f.FreshValue(sort)
		protoMust(t, err)
		if v.IsNull() {
			t.Fatal("recursive fresh refusal")
		}
	}
	if !tags.ShouldTrace("datatype") {
		if out.Len() != 0 {
			t.Fatal("erased factory trace emitted output")
		}
		return
	}
	for _, part := range []string{":180 ---------\n1. result: (ts tz)", ":191 ---------\n-------- [datatype] get_last_fresh_value", ":197 ---------\nchecking constructor: ts", ":200 ---------\nTraceNat TraceNat are_siblings 1  is_datatype 1 found_sibling 0", ":71 ---------\n(ts (ts tz)) in (ts tz) 0", ":129 ---------\nalmost fresh: (ts (ts tz))", ":232 ---------\npotential new value: (ts (ts (ts tz)))", ":236 ---------\n2. result: (ts (ts (ts tz)))"} {
		if !strings.Contains(out.String(), part) {
			t.Fatalf("missing source trace %q in\n%s", part, out.String())
		}
	}
}

type datatypeFactoryTraceStopWriter struct {
	bytes.Buffer
	at      string
	action  func()
	refuse  bool
	failed  bool
	after   int
	observe func([]byte)
}

func (w *datatypeFactoryTraceStopWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}
func (w *datatypeFactoryTraceStopWriter) Write(p []byte) (int, error) {
	if w.observe != nil {
		w.observe(p)
	}
	if w.failed {
		w.after++
		return 0, io.ErrClosedPipe
	}
	text := w.String() + string(p)
	if w.action != nil && strings.Contains(text, w.at) {
		action := w.action
		w.action = nil
		action()
		if w.refuse {
			w.failed = true
			return 0, io.ErrClosedPipe
		}
	}
	return w.Buffer.Write(p)
}

func TestDatatypeFactoryEnabledLoopTraceSeedsLastValueBeforeFormatterFailure(t *testing.T) {
	w := &datatypeFactoryTraceStopWriter{at: "get_fresh_value src/smt/proto_model/datatype_factory.cpp:191 ---------\n"}
	m, f, tags, sort, constructors := datatypeFactoryTraceFixture(t, w)
	zero, err := m.MkApp(constructors[0], nil)
	protoMust(t, err)
	protoMust(t, f.RegisterValue(zero))
	one, err := m.MkApp(constructors[1], []ast.ASTHandle{zero})
	protoMust(t, err)
	protoMust(t, f.RegisterValue(one))
	_, err = f.utility.Constructors(sort)
	protoMust(t, err)
	w.action = func() { m.Budget().FailAfter(0) }
	tags.Enable("datatype")
	v, err := f.FreshValue(sort)
	m.Budget().FailAfter(-1)
	if tags.ShouldTrace("datatype") {
		if !errors.Is(err, arena.ErrInjectedFailure) || !v.IsNull() || f.last[sort] != zero || len(f.pending) != 0 {
			t.Fatal("enabled get_last_fresh_value operand lost its publication prefix", v, err, f.last[sort])
		}
	} else {
		protoMust(t, err)
		if v.IsNull() {
			t.Fatal("erased loop trace refused generation")
		}
	}
}

func TestDatatypeFactoryNestedTraceRefusalRemainsSticky(t *testing.T) {
	w := &datatypeFactoryTraceStopWriter{at: "get_last_fresh_value src/smt/proto_model/datatype_factory.cpp:51", action: func() {}, refuse: true}
	_, f, tags, sort, _ := datatypeFactoryTraceFixture(t, w)
	tags.Enable("datatype")
	for i := 0; i < 3; i++ {
		v, err := f.FreshValue(sort)
		protoMust(t, err)
		if v.IsNull() {
			t.Fatal("stream refusal changed fresh generation")
		}
	}
	if tags.ShouldTrace("datatype") && (!w.failed || w.after != 0) {
		t.Fatal("outer record wrote after nested stream failure", w.failed, w.after)
	}
}

func TestDatatypeFactoryTraceUsesActualBvPrinterParameters(t *testing.T) {
	for _, mode := range []string{"decimal-bv", "negative-bv"} {
		t.Run(mode, func(t *testing.T) {
			var out bytes.Buffer
			m, f, tags, _, _ := datatypeFactoryTraceFixture(t, &out)
			bv, err := m.BitVectorSort(8)
			protoMust(t, err)
			fid, err := m.GetFamilyID(bv)
			protoMust(t, err)
			value, err := m.BitVectorNumeralUint64(200, 8)
			protoMust(t, err)
			protoPin(t, m, value)
			protoMust(t, f.model.RegisterFactory(&datatypeBVProvider{family: fid, values: []ast.ASTHandle{value}}))
			sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: protoSymbol(t, m, "TraceBits"), Constructors: []ast.DatatypeConstructorDefinition{{Name: protoSymbol(t, m, "trace-bits"), Recognizer: protoSymbol(t, m, "is-trace-bits"), Accessors: []ast.DatatypeAccessorDefinition{{Name: protoSymbol(t, m, "bits"), Range: bv}}}}}})
			protoMust(t, err)
			g := uparams.BorrowProcessGlobalParamsService()
			want := "(trace-bits (_ bv200 8))"
			if mode == "decimal-bv" {
				protoMust(t, g.SourceSetName("pp.bv_literals", "false"))
			} else {
				protoMust(t, g.SourceSetName("pp.bv_neg", "true"))
				want = "(trace-bits (bvneg #x38))"
			}
			tags.Enable("datatype")
			_, err = f.SomeValue(sorts[0])
			protoMust(t, err)
			if tags.ShouldTrace("datatype") && !strings.Contains(out.String(), want+"\n") {
				t.Fatal("trace replaced its actual printer module values with defaults", out.String(), want)
			}
		})
	}
}

func TestDatatypeFactoryTraceUsesActualAliasDepth(t *testing.T) {
	var out bytes.Buffer
	_, f, tags, sort, _ := datatypeFactoryTraceFixture(t, &out)
	protoMust(t, uparams.BorrowProcessGlobalParamsService().SourceSetName("pp.max_depth", "1"))
	tags.Enable("datatype")
	for i := 0; i < 3; i++ {
		_, err := f.FreshValue(sort)
		protoMust(t, err)
	}
	if tags.ShouldTrace("datatype") && (!strings.Contains(out.String(), "(let ((a!1 (ts tz)))") || !strings.Contains(out.String(), "(ts a!1))")) {
		t.Fatal("actual pp.max_depth did not reach expression aliasing", out.String())
	}
}

func TestDatatypeFactoryConditionalTracePrecedesDistinctnessAssertion(t *testing.T) {
	var out bytes.Buffer
	m, f, tags, _, _ := datatypeFactoryTraceFixture(t, &out)
	bv, err := m.BitVectorSort(8)
	protoMust(t, err)
	fid, err := m.GetFamilyID(bv)
	protoMust(t, err)
	value, err := m.BitVectorNumeralUint64(0, 8)
	protoMust(t, err)
	protoPin(t, m, value)
	protoMust(t, f.model.RegisterFactory(&datatypeBVProvider{family: fid, values: []ast.ASTHandle{value}}))
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: protoSymbol(t, m, "DuplicateTrace"), Constructors: []ast.DatatypeConstructorDefinition{{Name: protoSymbol(t, m, "duplicate-trace"), Recognizer: protoSymbol(t, m, "is-duplicate-trace"), Accessors: []ast.DatatypeAccessorDefinition{{Name: protoSymbol(t, m, "dup-bits"), Range: bv}}}}}})
	protoMust(t, err)
	cs, err := f.utility.Constructors(sorts[0])
	protoMust(t, err)
	old, err := m.MkApp(cs[0], []ast.ASTHandle{value})
	protoMust(t, err)
	protoMust(t, f.RegisterValue(old))
	scalarFactoryDebugActions(t, "T")
	tags.Enable("datatype")
	_, _, err = f.freshConstructor(sorts[0], cs[0], f.sets[sorts[0]])
	if diagnostic.DebugBuild {
		if err == nil {
			t.Fatal("duplicate source assertion did not abort")
		}
	} else {
		protoMust(t, err)
	}
	if tags.ShouldTrace("datatype") && !strings.Contains(out.String(), ":174 ---------\n(duplicate-trace #x00)\n") {
		t.Fatal("conditional trace did not run before SASSERT175", out.String())
	}
}

func TestDatatypeFactoryTraceFiniteSiblingFallback(t *testing.T) {
	var out bytes.Buffer
	m, f, tags, _, _ := datatypeFactoryTraceFixture(t, &out)
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{
		{Name: protoSymbol(t, m, "FallbackA"), Constructors: []ast.DatatypeConstructorDefinition{
			{Name: protoSymbol(t, m, "fa0"), Recognizer: protoSymbol(t, m, "is-fa0")},
			{Name: protoSymbol(t, m, "fa1"), Recognizer: protoSymbol(t, m, "is-fa1"), Accessors: []ast.DatatypeAccessorDefinition{{Name: protoSymbol(t, m, "finite"), Recursive: true, RecursiveIndex: 1}, {Name: protoSymbol(t, m, "recursive"), Recursive: true}}},
		}},
		{Name: protoSymbol(t, m, "FallbackB"), Constructors: []ast.DatatypeConstructorDefinition{{Name: protoSymbol(t, m, "fb"), Recognizer: protoSymbol(t, m, "is-fb")}}},
	})
	protoMust(t, err)
	a, err := f.utility.Constructors(sorts[0])
	protoMust(t, err)
	b, err := f.utility.Constructors(sorts[1])
	protoMust(t, err)
	zero, err := m.MkApp(a[0], nil)
	protoMust(t, err)
	finite, err := m.MkApp(b[0], nil)
	protoMust(t, err)
	protoMust(t, f.RegisterValue(zero))
	protoMust(t, f.RegisterValue(finite))
	// Approach 1 builds (fa1 fb some) from *set->begin(), which follows the
	// obj_hashtable order: extend the chain zero, (fa1 fb zero), ... until
	// that rebuilt value is already registered.
	registered := map[ast.ASTHandle]bool{zero: true}
	for {
		next, err := m.MkApp(a[1], []ast.ASTHandle{finite, f.sets[sorts[0]].begin()})
		protoMust(t, err)
		if registered[next] {
			break
		}
		registered[next] = true
		protoMust(t, f.RegisterValue(next))
	}
	tags.Enable("datatype")
	value, err := f.FreshValue(sorts[0])
	protoMust(t, err)
	if value.IsNull() || registered[value] {
		t.Fatal("finite-sibling fallback prevented recursive growth")
	}
	if tags.ShouldTrace("datatype") && !strings.Contains(out.String(), ":215 ---------\nno argument found for FallbackB\n") {
		t.Fatal("missing source null almost-fresh fallback trace", out.String())
	}
}

func TestDatatypeFactoryTraceBorrowsInputAndOwnsDocument(t *testing.T) {
	w := &datatypeFactoryTraceStopWriter{}
	m, f, tags, sort, _ := datatypeFactoryTraceFixture(t, w)
	observed := false
	w.observe = func(p []byte) {
		if string(p) != "tz" || !strings.Contains(w.String(), "datatype_factory.cpp:41") {
			return
		}
		observed = true
		set := f.sets[sort]
		if set == nil || len(set.values) != 1 {
			t.Fatal("source trace ran before registration")
		}
		count, err := m.GetRefCount(set.values[0])
		protoMust(t, err)
		if count != 1 {
			t.Fatalf("mk_pp borrowed input has %d references; want factory's single reference", count)
		}
		fm, err := m.GetFormatManager()
		protoMust(t, err)
		if fm.Stats().LiveReferences != 1 {
			t.Fatal("trace document has no lexical format_ref owner", fm.Stats())
		}
	}
	tags.Enable("datatype")
	_, err := f.SomeValue(sort)
	protoMust(t, err)
	if observed != tags.ShouldTrace("datatype") {
		t.Fatal("input/document observation disagrees with trace gate")
	}
	fm, err := m.GetFormatManager()
	protoMust(t, err)
	if fm.Stats().LiveReferences != 0 {
		t.Fatal("format_ref survived trace expression")
	}
}
