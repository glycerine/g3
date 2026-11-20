package params

// Source: src/util/params.cpp action traces; added bounded ownership expectations.

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func paramsTestOwner(t testing.TB) (*ParamsContext, *usymbol.Manager, *unumeral.RationalService) {
	t.Helper()
	b := arena.NewBudget(^uint64(0))
	sy, e := usymbol.NewManager(usymbol.Config{Budget: b, PageSize: 64, DescriptorPageSize: 2})
	if e != nil {
		t.Fatal(e)
	}
	q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{PageSize: 2, Budget: b}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}}}})
	if e != nil {
		t.Fatal(e)
	}
	c, e := NewParamsContext(sy, q, ParamsConfig{Budget: b, PageEntries: 1, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := c.Close(); e != nil {
			t.Error(e)
		}
		if e := q.Close(); e != nil {
			t.Error(e)
		}
		if e := sy.Close(); e != nil {
			t.Error(e)
		}
		if b.Used() != 0 {
			t.Errorf("retained %d", b.Used())
		}
	})
	return c, sy, q
}
func paramsMust(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}

func TestParamsSourceStaticEmptyOwnerAdaptation(t *testing.T) {
	c, _, _ := paramsTestOwner(t)
	before := c.Stats()
	a, b := c.EmptyValue(), c.EmptyValue()
	if a.Owner == 0 || a != b || a.Slot != 0 || a.Generation != 0 {
		t.Fatal("source static null reference adaptation", a, b)
	}
	if e := c.Assign(&b, &a); e != nil {
		t.Fatal(e)
	}
	empty, e := c.Empty(&b)
	if e != nil || !empty || b != a || c.Stats() != before {
		t.Fatal("empty copy allocated or changed owner", empty, e, a, b, c.Stats(), before)
	}
}

// Source has no registered params unit-test assertions. These finite action
// traces are derived from complete params_ref bodies, not solver assertions.
func TestParamsReferenceSourceSharingAndReset(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	k := internSymbol(t, sy, "flag")
	a, e := c.NewValue()
	paramsMust(t, e)
	b, e := c.NewValue()
	paramsMust(t, e)
	empty, e := c.Empty(&a)
	paramsMust(t, e)
	if !empty {
		t.Fatal("default null")
	}
	paramsMust(t, c.SetBool(&a, k, true))
	paramsMust(t, c.Assign(&b, &a))
	paramsMust(t, c.Assign(&b, &b))
	paramsMust(t, c.ResetKey(&b, k))
	got, e := c.Contains(&a, k)
	paramsMust(t, e)
	if got {
		t.Fatal("reset must mutate shared backing")
	}
	paramsMust(t, c.SetBool(&a, k, true))
	got, e = c.GetBool(&b, k, false)
	paramsMust(t, e)
	if got {
		t.Fatal("setter must detach")
	}
	paramsMust(t, c.Assign(&b, &a))
	paramsMust(t, c.Reset(&a))
	empty, e = c.Empty(&b)
	paramsMust(t, e)
	if !empty {
		t.Fatal("reset all must remain shared")
	}
	paramsMust(t, c.Del(&a))
	paramsMust(t, c.Del(&b))
}

func TestParamsReferenceSourceCopyMergeAndSignedRat(t *testing.T) {
	c, sy, q := paramsTestOwner(t)
	x := internSymbol(t, sy, "x")
	y := internSymbol(t, sy, "y")
	a, e := c.NewValue()
	paramsMust(t, e)
	b, e := c.NewValue()
	paramsMust(t, e)
	paramsMust(t, c.SetUInt(&a, x, ^uint32(0)))
	paramsMust(t, c.SetBool(&b, y, true))
	paramsMust(t, c.Copy(&b, &a))
	paramsMust(t, c.Append(&a, &b))
	flag, e := c.GetBool(&b, y, false)
	paramsMust(t, e)
	if !flag {
		t.Fatal("copy is ordered merge")
	}
	def, e := q.NewValue()
	paramsMust(t, e)
	r, e := c.GetRat(&b, x, &def)
	paramsMust(t, e)
	text, e := q.String(&r)
	paramsMust(t, e)
	if text != "-1" {
		t.Fatal("source unsigned to signed32 cast", text)
	}
	paramsMust(t, q.Del(&r))
	paramsMust(t, q.Del(&def))
	paramsMust(t, c.Del(&a))
	paramsMust(t, c.Del(&b))
}

func TestParamsReferenceNameReadsNeverIntern(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	name := "missing"
	before := sy.Stats()
	got, e := c.GetBoolName(&a, &name, true)
	paramsMust(t, e)
	if !got {
		t.Fatal("default")
	}
	contains, e := c.ContainsName(&a, &name)
	paramsMust(t, e)
	if contains {
		t.Fatal("missing")
	}
	paramsMust(t, c.ResetKeyName(&a, &name))
	after := sy.Stats()
	if before != after {
		t.Fatal("read interned key")
	}
	paramsMust(t, c.Del(&a))
}

func TestParamsReferenceValidationSharedPrefixAndDuplicates(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	b, e := c.NewValue()
	paramsMust(t, e)
	d, e := NewParamDescrs(sy, ParamDescrsConfig{})
	paramsMust(t, e)
	defer d.Close()
	descr, module := "description", "m"
	x := internSymbol(t, sy, "x")
	bad := internSymbol(t, sy, "bad")
	paramsMust(t, d.Insert(x, ParamBool, &descr, nil, &module))
	paramsMust(t, c.SetBool(&a, internSymbol(t, sy, "m.x"), true))
	paramsMust(t, c.SetUInt(&a, x, 7))
	paramsMust(t, c.SetBool(&a, bad, false))
	paramsMust(t, c.Assign(&b, &a))
	e = c.Validate(&a, d)
	if e == nil || e.Error() != "Parameter x was given argument of type unsigned int, expected bool" {
		t.Fatal(e)
	}
	got, e := c.GetBool(&b, x, false)
	paramsMust(t, e)
	if !got {
		t.Fatal("shared key rewrite lost")
	}
	n, e := c.GetUInt(&b, x, 0)
	paramsMust(t, e)
	if n != 7 {
		t.Fatal("kind mismatch must continue", n)
	}
	paramsMust(t, c.ResetKey(&b, x))
	got, e = c.GetBool(&a, x, false)
	paramsMust(t, e)
	if got {
		t.Fatal("first duplicate only")
	}
	paramsMust(t, c.Del(&a))
	paramsMust(t, c.Del(&b))
}

func TestParamsReferenceExactDisplayAndKinds(t *testing.T) {
	c, sy, q := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	paramsMust(t, c.SetBool(&a, internSymbol(t, sy, "b"), true))
	paramsMust(t, c.SetUInt(&a, internSymbol(t, sy, "u"), 9))
	paramsMust(t, c.SetDouble(&a, internSymbol(t, sy, "d"), 1.23456789))
	text := "hello\x00ignored"
	paramsMust(t, c.SetString(&a, internSymbol(t, sy, "s"), &text))
	paramsMust(t, c.SetSymbol(&a, internSymbol(t, sy, "z"), internSymbol(t, sy, "sym")))
	r, e := q.NewValue()
	paramsMust(t, e)
	paramsMust(t, q.SetFractionSigned(&r, 1, 3))
	paramsMust(t, c.SetRat(&a, internSymbol(t, sy, "r"), &r))
	paramsMust(t, q.Del(&r))
	out, e := c.Display(&a)
	paramsMust(t, e)
	line, e := c.DebugString(&a)
	paramsMust(t, e)
	if line != out+"\n" {
		t.Fatal(line)
	}
	keyText, e := c.DisplayKey(&a, internSymbol(t, sy, "b"))
	paramsMust(t, e)
	if keyText != "true" {
		t.Fatal(keyText)
	}
	buf := make([]byte, 8)
	n, e := c.FormatKeyInto(&a, internSymbol(t, sy, "b"), buf)
	paramsMust(t, e)
	if string(buf[:n]) != keyText {
		t.Fatal("key buffer")
	}
	if out != "(params b true u 9 d 1.23457 s hello z sym r 1/3)" {
		t.Fatal(out)
	}
	dst := []byte("untouched")
	_, e = c.FormatInto(&a, dst)
	if e == nil || string(dst) != "untouched" {
		t.Fatal("short publication", e, string(dst))
	}
	paramsMust(t, c.Del(&a))
}

func TestParamsReferenceNumericalSymbolValueBareString(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	k := internSymbol(t, sy, "s")
	e = c.SetSymbol(&a, k, usymbol.Numerical(5))
	if udiag.DebugBuild {
		if e != usymbol.ErrInvalid || a.Slot == 0 {
			t.Fatal("source assertion must follow params_ref::init", e, a)
		}
		if got, e := c.GetSymbol(&a, k, usymbol.Symbol{}); e != nil || got != (usymbol.Symbol{}) {
			t.Fatal("assertion must leave the new block without a value", got, e)
		}
	} else {
		paramsMust(t, e)
		got, e := c.GetSymbol(&a, k, usymbol.Symbol{})
		paramsMust(t, e)
		spelling, present, e := sy.BareString(got)
		paramsMust(t, e)
		if !present || spelling != "" || got.IsNumerical() {
			t.Fatal("numerical value must be stored as the empty spelling", got, spelling, present)
		}
	}
	name := "named"
	e = c.SetSymbolName(&a, &name, usymbol.Numerical(9))
	if udiag.DebugBuild {
		if e != usymbol.ErrInvalid {
			t.Fatal("named setter must reach the same assertion", e)
		}
	} else {
		paramsMust(t, e)
		got, e := c.GetSymbolName(&a, &name, usymbol.Symbol{})
		paramsMust(t, e)
		spelling, present, e := sy.BareString(got)
		paramsMust(t, e)
		if !present || spelling != "" || got.IsNumerical() {
			t.Fatal("named setter must store the empty spelling", got, spelling, present)
		}
	}
	paramsMust(t, c.Del(&a))
}

func TestParamsReferenceNullableStreamAndDisplayName(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	name := "display-missing"
	before := sy.Stats()
	text, e := c.DisplayKeyName(&a, &name)
	paramsMust(t, e)
	if text != "default" || before == sy.Stats() {
		t.Fatal("display name must intern even null backing", text)
	}
	k := internSymbol(t, sy, "s")
	paramsMust(t, c.SetString(&a, k, nil))
	def := "fallback"
	text, present, e := c.GetString(&a, k, &def)
	paramsMust(t, e)
	if present || text != "" {
		t.Fatal("null stored string is not fallback")
	}
	text, e = c.Display(&a)
	if e != utext.ErrNullCString {
		t.Fatal(e)
	}
	if text != "(params s " {
		t.Fatal("null C string stream prefix", text)
	}
	paramsMust(t, c.SetSymbol(&a, k, usymbol.Symbol{}))
	text, e = c.Display(&a)
	paramsMust(t, e)
	if text != "(params s null)" {
		t.Fatal(text)
	}
	d, e := NewParamDescrs(sy, ParamDescrsConfig{})
	paramsMust(t, e)
	defer d.Close()
	descr := "description"
	paramsMust(t, d.Insert(k, ParamSymbol, &descr, nil, nil))
	module := "m"
	normal, e := c.DisplaySMT2(&a, &module, d)
	paramsMust(t, e)
	if normal != "(set-option :m.s null)\n" {
		t.Fatal(normal)
	}
	buffer := make([]byte, 128)
	n, e := c.FormatSMT2Into(&a, &module, d, buffer)
	paramsMust(t, e)
	if string(buffer[:n]) != normal {
		t.Fatal("SMT2 buffer")
	}
	text, e = c.DisplaySMT2(&a, nil, d)
	if e != utext.ErrNullCString {
		t.Fatal(e)
	}
	if text != "(set-option :" {
		t.Fatal("null module prefix", text)
	}
	paramsMust(t, c.Reset(&a))
	text, e = c.DisplaySMT2(&a, nil, d)
	paramsMust(t, e)
	if text != "" {
		t.Fatal("empty does not insert null module", text)
	}
	paramsMust(t, c.Del(&a))
}

func TestParamsReferenceFallbackAllKinds(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	b, e := c.NewValue()
	paramsMust(t, e)
	name := "key"
	k := internSymbol(t, sy, name)
	s := internSymbol(t, sy, "symbol")
	str := "text"
	paramsMust(t, c.SetUInt(&a, k, 1))
	paramsMust(t, c.SetBoolName(&b, &name, true))
	v, e := c.GetBoolFallbackName(&a, &name, &b, false)
	paramsMust(t, e)
	if !v {
		t.Fatal("wrong local kind must fall back")
	}
	paramsMust(t, c.SetUIntName(&b, &name, 42))
	n, e := c.GetUIntFallbackName(&a, &name, &b, 0)
	paramsMust(t, e)
	if n != 1 {
		t.Fatal("local precedence")
	}
	paramsMust(t, c.Reset(&a))
	n, e = c.GetUIntFallbackName(&a, &name, &b, 0)
	paramsMust(t, e)
	if n != 42 {
		t.Fatal(n)
	}
	paramsMust(t, c.SetDoubleName(&b, &name, -3.25))
	for _, byName := range []bool{false, true} {
		var got float64
		if byName {
			got, e = c.GetDoubleName(&b, &name, 0)
		} else {
			got, e = c.GetDouble(&b, k, 0)
		}
		paramsMust(t, e)
		if got != -3.25 {
			t.Fatal(got)
		}
	}
	f, e := c.GetDoubleFallbackName(&a, &name, &b, 0)
	paramsMust(t, e)
	if f != -3.25 {
		t.Fatal(f)
	}
	paramsMust(t, c.SetStringName(&b, &name, &str))
	namedString, p, e := c.GetStringName(&b, &name, nil)
	paramsMust(t, e)
	if !p || namedString != str {
		t.Fatal(namedString, p)
	}
	out, present, e := c.GetStringFallbackName(&a, &name, &b, nil)
	paramsMust(t, e)
	if !present || out != str {
		t.Fatal(out, present)
	}
	paramsMust(t, c.SetSymbolName(&b, &name, s))
	direct, e := c.GetSymbol(&b, k, usymbol.Symbol{})
	paramsMust(t, e)
	named, e := c.GetSymbolName(&b, &name, usymbol.Symbol{})
	paramsMust(t, e)
	if direct != s || named != s {
		t.Fatal("symbol overloads")
	}
	sym, e := c.GetSymbolFallbackName(&a, &name, &b, usymbol.Symbol{})
	paramsMust(t, e)
	if sym != s {
		t.Fatal(sym)
	}
	paramsMust(t, c.Del(&a))
	paramsMust(t, c.Del(&b))
}

func TestParamsReferenceFallbackReadOnlyAfterPrimaryMiss(t *testing.T) {
	c, _, _ := paramsTestOwner(t)
	primary, e := c.NewValue()
	paramsMust(t, e)
	name := "local"
	invalidFallback := ParamsRef{Owner: c.Owner + 1}
	paramsMust(t, c.SetBoolName(&primary, &name, true))
	got, e := c.GetBoolFallbackName(&primary, &name, &invalidFallback, false)
	if e != nil || !got {
		t.Fatal("primary match should return before fallback validation", got, e)
	}

	// A matching null string is still a match, even though it has no bytes.
	paramsMust(t, c.SetStringName(&primary, &name, nil))
	str, present, e := c.GetStringFallbackName(&primary, &name, &invalidFallback, nil)
	if e != nil || present || str != "" {
		t.Fatal("matching null string consulted fallback", str, present, e)
	}
	// The same key with a different requested kind takes the fallback path.
	if _, e = c.GetBoolFallbackName(&primary, &name, &invalidFallback, false); !errors.Is(e, ErrParamsOwner) {
		t.Fatal("wrong-kind primary did not consult fallback", e)
	}
	paramsMust(t, c.Del(&primary))
}

func TestParamsReferenceNullBackingSkipsKeyRead(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	_, foreignSymbols, _ := paramsTestOwner(t)
	foreignKey := internSymbol(t, foreignSymbols, "key")
	v, e := c.NewValue()
	paramsMust(t, e)
	before := c.Stats()
	got, e := c.GetBool(&v, foreignKey, true)
	if e != nil || !got {
		t.Fatal("null-backed get must return default before key read", got, e)
	}
	contains, e := c.Contains(&v, foreignKey)
	if e != nil || contains {
		t.Fatal("null-backed contains must not read key", contains, e)
	}
	paramsMust(t, c.ResetKey(&v, foreignKey))
	if c.Stats() != before {
		t.Fatal("null-backed reads or reset changed storage")
	}
	localKey := internSymbol(t, sy, "key")
	paramsMust(t, c.SetBool(&v, localKey, true))
	if _, e = c.GetBool(&v, foreignKey, false); !errors.Is(e, usymbol.ErrOwner) {
		t.Fatal("live backing skipped foreign key check", e)
	}
	paramsMust(t, c.Del(&v))
}

func TestParamsReferenceMatchedValuesSkipUnusedDefaults(t *testing.T) {
	c, sy, q := paramsTestOwner(t)
	_, foreignSymbols, foreignNumerals := paramsTestOwner(t)
	key := internSymbol(t, sy, "key")
	wantSymbol := internSymbol(t, sy, "value")
	foreignDefault := internSymbol(t, foreignSymbols, "unused")
	v, e := c.NewValue()
	paramsMust(t, e)
	paramsMust(t, c.SetSymbol(&v, key, wantSymbol))
	gotSymbol, e := c.GetSymbol(&v, key, foreignDefault)
	if e != nil || gotSymbol != wantSymbol {
		t.Fatal("matching symbol read consulted default", gotSymbol, e)
	}
	paramsMust(t, c.ResetKey(&v, key))
	gotSymbol, e = c.GetSymbol(&v, key, foreignDefault)
	if e != nil || gotSymbol != foreignDefault {
		t.Fatal("missing symbol read default instead of returning it", gotSymbol, e)
	}

	foreignRat, e := foreignNumerals.NewValue()
	paramsMust(t, e)
	paramsMust(t, c.SetUInt(&v, key, 7))
	gotRat, e := c.GetRat(&v, key, &foreignRat)
	paramsMust(t, e)
	text, e := q.String(&gotRat)
	paramsMust(t, e)
	if text != "7" {
		t.Fatal("matching unsigned rational conversion", text)
	}
	paramsMust(t, q.Del(&gotRat))
	localRat, e := q.NewValue()
	paramsMust(t, e)
	paramsMust(t, q.AssignInt(&localRat, 9))
	paramsMust(t, c.SetRat(&v, key, &localRat))
	gotRat, e = c.GetRat(&v, key, &foreignRat)
	paramsMust(t, e)
	text, e = q.String(&gotRat)
	paramsMust(t, e)
	if text != "9" {
		t.Fatal("matching numeral read consulted default", text)
	}
	paramsMust(t, q.Del(&gotRat))
	paramsMust(t, c.ResetKey(&v, key))
	if _, e = c.GetRat(&v, key, &foreignRat); !errors.Is(e, unumeral.ErrRationalServiceOwner) {
		t.Fatal("missing numeral did not validate used default", e)
	}
	paramsMust(t, q.Del(&localRat))
	paramsMust(t, foreignNumerals.Del(&foreignRat))
	paramsMust(t, c.Del(&v))
}

func TestParamsReferenceValidationUnknownExactOrdering(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	d, e := NewParamDescrs(sy, ParamDescrsConfig{})
	paramsMust(t, e)
	defer d.Close()
	descr, def := "description", "7"
	paramsMust(t, d.Insert(internSymbol(t, sy, "b-Z"), ParamUInt, &descr, &def, nil))
	paramsMust(t, d.Insert(internSymbol(t, sy, "A"), ParamBool, &descr, nil, nil))
	paramsMust(t, c.SetBool(&a, internSymbol(t, sy, "missing"), true))
	e = c.Validate(&a, d)
	want := "unknown parameter 'missing'\nLegal parameters are:\n  a (bool)\n  b_z (unsigned int) (default: 7)\n"
	if e == nil || e.Error() != want {
		t.Fatal(e)
	}
	paramsMust(t, c.Del(&a))
}

func TestParamsReferenceSourceGrowthAndFailurePrefixes(t *testing.T) {
	c, sy, q := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	var keys [12]usymbol.Symbol
	for i := range keys {
		keys[i] = usymbol.Numerical(uint32(i))
	}
	for i := 0; i < 12; i++ {
		paramsMust(t, c.SetUInt(&a, keys[i], uint32(i)))
		want := []uint32{2, 2, 3, 5, 5, 8, 8, 8, 12, 12, 12, 12}[i]
		if c.Blocks[a.Slot-1].capacity != want {
			t.Fatal("source requested capacity", i, c.Blocks[a.Slot-1].capacity)
		}
	}
	b, e := c.NewValue()
	paramsMust(t, e)
	paramsMust(t, c.Assign(&b, &a))
	refs := c.Stats().References
	key := internSymbol(t, sy, "new")
	c.BudgetField.FailAfter(0)
	e = c.SetBool(&a, key, true)
	c.BudgetField.FailAfter(-1)
	// The registry may have spare capacity. A detached block is published before
	// vector copying; both full or empty prefixes are legal based on reservations.
	if e != nil && e != arena.ErrInjectedFailure {
		t.Fatal(e)
	}
	retained := uint32(0)
	for _, frame := range c.SourceFrames {
		if frame.Active {
			retained++
		}
	}
	if c.Stats().References != refs+retained {
		t.Fatal("source published current and retained old references", c.Stats().References, refs, retained)
	}
	stats, e := q.Stats()
	paramsMust(t, e)
	if stats.LiveCells == 0 {
		t.Fatal("manager baseline missing")
	}
	paramsMust(t, c.Del(&a))
	paramsMust(t, c.Del(&b))
}

// Audit live roots BEFORE rendering or teardown, so Close cannot hide leaked
// unpublished numerals. In these sweeps every big rational is one big integer
// numerator with an inline denominator, hence owns exactly one numeric cell.
func paramsAuditRoots(t *testing.T, c *ParamsContext, q *unumeral.RationalService, base, basePayloads uint32, roots []ParamsRef) {
	t.Helper()
	var bigs uint32
	for i, frame := range c.replacementFrames {
		if frame.isActive() {
			if frame.invocation == 0 || frame.generation != c.Blocks[i].Generation || frame.index >= c.Blocks[i].Size {
				t.Fatal("invalid declared replacement frame")
			}
			if frame.next.text.present && !frame.next.text.borrowed {
				if _, e := frame.next.text.owned.read(); e != nil {
					t.Fatal("invalid prepared native string", e)
				}
			}
		}
	}
	auditNumeral := func(value *unumeral.OwnedRational) {
		big, e := q.IsBig(value)
		if e != nil {
			// Read-only test reflection checks the actual service-owned dead
			// descriptor published by numerator Copy refusal. Other invalid
			// roots remain errors; this fixture's denominator is inline one.
			service := reflect.ValueOf(q).Elem()
			z := testFieldBySourceName(testFieldBySourceName(service, "manager").Elem(), "z").Elem()
			owned := reflect.ValueOf(value).Elem()
			raw := testFieldBySourceName(owned, "raw")
			num, den := testFieldBySourceName(raw, "num"), testFieldBySourceName(raw, "den")
			if !errors.Is(e, arena.ErrInvalidSlot) || testFieldBySourceName(owned, "ownerID").Uint() != testFieldBySourceName(service, "ownerID").Uint() || testFieldBySourceName(num, "val").Int() != 1 || testFieldBySourceName(num, "cell").Uint() == 0 || (testFieldBySourceName(num, "cell").Uint() != testFieldBySourceName(z, "failedRef").Uint() && testFieldBySourceName(num, "cell").Uint() != testFieldBySourceName(z, "failedNullRef").Uint()) || testFieldBySourceName(den, "cell").Uint() != 0 || testFieldBySourceName(den, "val").Int() != 1 {
				t.Fatal("unexpected invalid published parameter numeral", e)
			}
			return
		}
		if big {
			bigs++
		}
	}
	for i, frame := range c.SetFrames {
		if !frame.Active {
			continue
		}
		if !frame.Acquired || frame.Invocation == 0 || frame.Generation != c.Blocks[i].Generation || frame.Next.Kind != ParamNumeral {
			t.Fatal("invalid actual acquired setter local")
		}
		if frame.Published {
			if frame.MatchedIndex >= c.Blocks[i].Size {
				t.Fatal("published replacement index outside block")
			}
			entry := c.Entry(&c.Blocks[i], frame.MatchedIndex)
			if entry.key != frame.Next.key || entry.Kind != ParamNumeral {
				t.Fatal("published replacement lost its entry")
			}
			continue // The block traversal below audits this one actual owner.
		}
		auditNumeral(&frame.Next.Rat)
	}
	for i := range c.Blocks {
		b := &c.Blocks[i]
		if !b.Live {
			continue
		}
		var refs uint32
		for _, r := range roots {
			if r.Slot == uint32(i+1) {
				refs++
			}
		}
		for _, frame := range c.SourceFrames {
			if frame.Active && frame.HeldSlot == uint32(i+1) && !frame.decrementStarted {
				if frame.Invocation == 0 || frame.HeldGeneration != b.Generation {
					t.Fatal("invalid actual held source reference")
				}
				refs++
			}
		}
		if refs != b.Refs {
			t.Fatal("reference roots", i, refs, b.Refs)
		}
		for j := uint32(0); j < b.Size; j++ {
			x := c.Entry(b, j)
			if x.text.present && !x.text.borrowed {
				if _, e := x.text.owned.read(); e != nil {
					t.Fatal("invalid native string root", e)
				}
			}
			if x.Kind == ParamNumeral {
				auditNumeral(&x.Rat)
			}
		}
	}
	got, e := q.Stats()
	paramsMust(t, e)
	pending := paramsPendingNumericObject(t, q)
	if got.PendingCellObjects != pending {
		t.Fatal("pending payload stats disagree with real named raw root", got.PendingCellObjects, pending)
	}
	if got.LiveCells != base+bigs || got.LivePayloads != basePayloads+bigs+pending {
		t.Fatal("lost numeric cell/payload temporary", got.LiveCells, got.LivePayloads, base+bigs, basePayloads+bigs)
	}
	for pi := range c.pages {
		p := &c.pages[pi]
		for bi, meta := range p.states {
			if meta.used {
				continue
			}
			for j := uint32(0); j < p.class; j++ {
				if p.entries[uint32(bi)*p.class+j] != (paramEntry{}) {
					t.Fatal("unpublished entry owner")
				}
			}
		}
	}
}

// The actual unassigned physical root is separate from numeric descriptors.
// This census reads its exact allocator location and Runtime backing, never
// reconstructs a token or treats cold locator entries as owning numerals.
func paramsPendingNumericObject(t *testing.T, q *unumeral.RationalService) uint32 {
	t.Helper()
	z := testFieldBySourceName(testFieldBySourceName(reflect.ValueOf(q).Elem(), "manager").Elem(), "z").Elem()
	pending := testFieldBySourceName(z, "pendingCell")
	object := testFieldBySourceName(pending, "object")
	if testFieldBySourceName(object, "owner").Uint() == 0 {
		return 0
	}
	allocator := testFieldBySourceName(z, "allocator").Elem()
	slot := testFieldBySourceName(object, "slot").Uint()
	locations := testFieldBySourceName(allocator, "locations")
	if testFieldBySourceName(object, "owner").Uint() != testFieldBySourceName(allocator, "origin").Uint() || slot == 0 || slot > uint64(locations.Len()) {
		t.Fatal("foreign pending physical object")
	}
	loc := locations.Index(int(slot - 1))
	if !testFieldBySourceName(loc, "live").Bool() || testFieldBySourceName(loc, "free").Bool() || testFieldBySourceName(loc, "generation").Uint() != testFieldBySourceName(object, "generation").Uint() || testFieldBySourceName(loc, "address").Uint() != testFieldBySourceName(object, "address").Uint() {
		t.Fatal("stale pending physical object")
	}
	raw := testFieldBySourceName(pending, "raw")
	mapped := testFieldBySourceName(loc, "raw")
	for _, name := range []string{"origin", "slot", "generation"} {
		if testFieldBySourceName(raw, name).Uint() != testFieldBySourceName(mapped, name).Uint() {
			t.Fatal("pending object/raw mismatch", name)
		}
	}
	offset, extent, requested := testFieldBySourceName(pending, "offset").Uint(), testFieldBySourceName(pending, "extent").Uint(), testFieldBySourceName(pending, "requested").Uint()
	if offset != testFieldBySourceName(loc, "offset").Uint() || (testFieldBySourceName(loc, "kind").Uint() != 3 && extent != testFieldBySourceName(loc, "extent").Uint()) || requested != testFieldBySourceName(loc, "requested").Uint() || extent < requested || requested < 8 {
		t.Fatal("pending object physical extent")
	}
	runtime := reflect.ValueOf(q.RuntimeOwner()).Elem()
	rslot := testFieldBySourceName(raw, "slot").Uint()
	blocks := testFieldBySourceName(runtime, "blocks")
	if testFieldBySourceName(raw, "origin").Uint() != testFieldBySourceName(runtime, "origin").Uint() || rslot == 0 || rslot > uint64(blocks.Len()) {
		t.Fatal("pending raw Runtime identity")
	}
	block := blocks.Index(int(rslot - 1))
	if !testFieldBySourceName(block, "live").Bool() || testFieldBySourceName(block, "generation").Uint() != testFieldBySourceName(raw, "generation").Uint() {
		t.Fatal("pending raw consumed")
	}
	header := testFieldBySourceName(testFieldBySourceName(runtime, "config"), "SizeTBits").Uint() / 8
	data := testFieldBySourceName(block, "data")
	if testFieldBySourceName(loc, "kind").Uint() == 3 && (offset != 0 || uint64(data.Len()) != header+extent) {
		t.Fatal("direct pending actual malloc extent")
	}
	at := header + offset + 4
	if header+offset+extent > uint64(data.Len()) || at+4 > uint64(data.Len()) {
		t.Fatal("pending raw physical span")
	}
	initialized := testFieldBySourceName(block, "initialized")
	var capacity uint32
	for i := uint64(0); i < 4; i++ {
		pos := at + i
		if initialized.Index(int(pos/64)).Uint()&(uint64(1)<<(pos%64)) == 0 {
			t.Fatal("pending capacity uninitialized")
		}
		capacity |= uint32(data.Index(int(pos)).Uint()) << uint(8*i)
	}
	if requested != uint64(uint32(8)+4*capacity) {
		t.Fatal("pending source requested extent", requested, capacity)
	}
	return 1
}

func TestParamsReferenceReservationFailureSweeps(t *testing.T) {
	for op := 0; op < 8; op++ {
		completed := false
		failures := 0
		for prefix := int64(0); prefix < 160; prefix++ {
			c, sy, q := paramsTestOwner(t)
			a, e := c.NewValue()
			paramsMust(t, e)
			b, e := c.NewValue()
			paramsMust(t, e)
			k := internSymbol(t, sy, "number")
			strkey := internSymbol(t, sy, "text")
			unknown := internSymbol(t, sy, "unknown")
			name := "new-fault-intern-key"
			wide, e := q.NewValue()
			paramsMust(t, e)
			digits := 100
			if op == 3 || op == 4 {
				// Copy a genuine direct object (>256 bytes), so a warmed pool
				// cannot bypass every physical allocation refusal in this sweep.
				digits = 800
			}
			paramsMust(t, q.SetString(&wide, "1"+strings.Repeat("0", digits)))
			var numberBank [16]unumeral.OwnedRational
			for i := range numberBank {
				numberBank[i], e = q.NewValue()
				paramsMust(t, e)
				paramsMust(t, q.Copy(&numberBank[i], &wide))
			}
			qs, e := q.Stats()
			paramsMust(t, e)
			base := qs.LiveCells
			text := "text"
			long := strings.Repeat("x", 4096)
			paramsMust(t, c.SetBool(&a, k, true))
			paramsMust(t, c.SetRat(&b, k, &wide))
			if op == 2 {
				paramsMust(t, c.SetString(&b, strkey, &long))
			} else {
				paramsMust(t, c.SetString(&b, strkey, &text))
			}
			bank, e := c.NewValue()
			paramsMust(t, e)
			if op == 0 || op == 1 {
				paramsMust(t, c.Assign(&bank, &a))
				paramsMust(t, c.Assign(&a, &b))
			}
			if op == 5 {
				paramsMust(t, c.SetString(&a, strkey, &long))
			}
			var d *ParamDescrs
			if op == 6 {
				d, e = NewParamDescrs(sy, ParamDescrsConfig{Budget: c.BudgetField, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: c.BudgetField}}})
				paramsMust(t, e)
				descr := "description"
				for i := 0; i < 10; i++ {
					paramsMust(t, d.Insert(internSymbol(t, sy, strconv.Itoa(i)), ParamBool, &descr, nil, nil))
				}
				paramsMust(t, c.SetBool(&a, unknown, true))
			}
			dst := []byte(strings.Repeat("z", 8192))
			c.BudgetField.FailAfter(prefix)
			switch op {
			case 0:
				e = c.SetRat(&a, k, &wide)
			case 1:
				e = c.SetString(&a, strkey, &long)
			case 2:
				e = c.Copy(&a, &b)
			case 3:
				e = c.SetRat(&a, strkey, &wide)
			case 4:
				e = c.SetRat(&a, k, &wide)
			case 5:
				_, e = c.FormatInto(&a, dst)
			case 6:
				e = c.Validate(&a, d)
			case 7:
				e = c.SetBoolName(&a, &name, true)
			}
			paramsAuditRoots(t, c, q, base, qs.LivePayloads, []ParamsRef{a, b, bank})
			c.BudgetField.FailAfter(-1)
			if (op == 0 || op == 1) && a.Slot != b.Slot {
				wantRefs := uint32(1)
				if c.SourceFrames[a.Slot-1].Active {
					wantRefs++
				}
				if c.Blocks[b.Slot-1].Refs != wantRefs {
					t.Fatal("source copy completion frontier refs", c.Blocks[b.Slot-1].Refs, wantRefs)
				}
				copied := &c.Blocks[a.Slot-1]
				source := &c.Blocks[b.Slot-1]
				if copied.Size > source.Size {
					t.Fatal("unexpected partial-copy suffix")
				}
				for i := uint32(0); i < copied.Size; i++ {
					if c.Entry(copied, i).key != c.Entry(source, i).key {
						t.Fatal("copy prefix order")
					}
				}
			}
			injected := numericStorageReservationRefusal(e)
			if injected {
				failures++
			} else {
				if op == 6 {
					if e == nil {
						t.Fatal("unknown accepted")
					}
				} else {
					paramsMust(t, e)
				}
				completed = true
			}
			if a.Slot != 0 && c.SetFrames[a.Slot-1].Active {
				receipt, stopped := c.StoppedSet(&a)
				paramsMust(t, stopped)
				paramsMust(t, c.CleanupSet(receipt))
			}
			paramsMust(t, c.Del(&a))
			paramsMust(t, c.Del(&b))
			paramsMust(t, c.Del(&bank))
			for i := range numberBank {
				paramsMust(t, q.Del(&numberBank[i]))
			}
			paramsMust(t, q.Del(&wide))
			if d != nil {
				paramsMust(t, d.Close())
			}
			paramsMust(t, c.Close())
			paramsMust(t, q.Close())
			paramsMust(t, sy.Close())
			if c.BudgetField.Used() != 0 {
				t.Fatal("failure graph retained", op, prefix, c.BudgetField.Used())
			}
			if completed {
				break
			}
		}
		goOnly := op == 5 || op == 6 || op == 7
		if !completed || goOnly && failures != 0 || !goOnly && failures == 0 {
			t.Fatal("incomplete failure sweep", op, completed, failures)
		}
	}
}

func TestParamsReferenceConstructorFaultAndRecordShape(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(ParamsRef{}), reflect.TypeOf(paramsBlock{}), reflect.TypeOf(paramsEntriesRef{})} {
		paramsMust(t, arena.ValidateRecordType(typ))
	}
	b := arena.NewBudget(^uint64(0))
	sy, e := usymbol.NewManager(usymbol.Config{Budget: b, PageSize: 64, DescriptorPageSize: 2})
	paramsMust(t, e)
	q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{PageSize: 2, Budget: b}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}}}})
	paramsMust(t, e)
	baseline := b.Used()
	completed := false
	for n := int64(0); n < 20; n++ {
		b.FailAfter(n)
		c, e := NewParamsContext(sy, q, ParamsConfig{Budget: b, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}}})
		b.FailAfter(-1)
		if e == nil {
			paramsMust(t, c.Close())
			completed = true
		} else if e != arena.ErrInjectedFailure {
			t.Fatal(e)
		}
		if b.Used() != baseline {
			t.Fatal("constructor retained", n, b.Used(), baseline)
		}
		if completed {
			break
		}
	}
	if !completed {
		t.Fatal("constructor sweep incomplete")
	}
	paramsMust(t, q.Close())
	paramsMust(t, sy.Close())
	if b.Used() != 0 {
		t.Fatal(b.Used())
	}
}

func TestParamsReferenceWarmOperationsAndGeneration(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	k := internSymbol(t, sy, "key")
	paramsMust(t, c.SetUInt(&a, k, 3))
	dst := make([]byte, 128)
	_, e = c.FormatInto(&a, dst)
	paramsMust(t, e)
	var failure error
	allocs := testing.AllocsPerRun(50, func() {
		if failure != nil {
			return
		}
		failure = c.SetUInt(&a, k, 4)
		if failure == nil {
			_, failure = c.GetUInt(&a, k, 0)
		}
		if failure == nil {
			_, failure = c.FormatInto(&a, dst)
		}
	})
	paramsMust(t, failure)
	if allocs != 0 {
		t.Fatal("warm allocations", allocs)
	}
	stale := a
	paramsMust(t, c.Del(&a))
	b, e := c.NewValue()
	paramsMust(t, e)
	paramsMust(t, c.SetBool(&b, k, true))
	if _, e = c.Empty(&stale); e != ErrParamsOwner {
		t.Fatal("stale generation", e)
	}
	paramsMust(t, c.Del(&b))
}

func TestParamsReferenceSelfCopySourceRatIdentity(t *testing.T) {
	c, sy, q := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	k := internSymbol(t, sy, "r")
	wide, e := q.NewValue()
	paramsMust(t, e)
	paramsMust(t, q.SetString(&wide, "1"+strings.Repeat("0", 100)))
	var bank [16]unumeral.OwnedRational
	for i := range bank {
		bank[i], e = q.NewValue()
		paramsMust(t, e)
		paramsMust(t, q.Copy(&bank[i], &wide))
	}
	paramsMust(t, c.SetRat(&a, k, &wide))
	c.BudgetField.FailAfter(0)
	e = c.Copy(&a, &a)
	c.BudgetField.FailAfter(-1)
	paramsMust(t, e)
	paramsMust(t, c.Del(&a))
	for i := range bank {
		paramsMust(t, q.Del(&bank[i]))
	}
	paramsMust(t, q.Del(&wide))
}

func TestParamsReferenceExactContextAccountingAndGrowthPeak(t *testing.T) {
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	one := internSymbol(t, sy, "one")
	two := internSymbol(t, sy, "two")
	three := internSymbol(t, sy, "three")
	baseline := c.BudgetField.Used()
	s := c.Stats()
	cold := s.HeaderBytes
	borrowed := baseline - cold
	check := func() {
		s := c.Stats()
		total := s.HeaderBytes + s.BlockBytes + s.ReceiptBytes + s.SourceFrameBytes + s.SourceSetBytes + s.ReplacementBytes + s.PageBytes + s.DirectoryBytes + s.FormatBytes
		if c.BudgetField.Used() != borrowed+total {
			t.Fatal("actual context charge", c.BudgetField.Used(), borrowed+total)
		}
	}
	paramsMust(t, c.SetUInt(&a, one, 1))
	paramsMust(t, c.SetUInt(&a, two, 2))
	check()
	// Both old capacity-2 records and the new physical capacity-4 array must
	// coexist until source publication. Block metadata is also real storage.
	need := 4*uint64(reflect.TypeOf(paramEntry{}).Size()) + uint64(reflect.TypeOf(paramsEntryState{}).Size())
	used := c.BudgetField.Used()
	bank := c.BudgetField.Limit() - used - need + 1
	paramsMust(t, c.BudgetField.Reserve(bank))
	e = c.SetUInt(&a, three, 3)
	if e != arena.ErrBudget {
		t.Fatal("missing old/new peak", e)
	}
	c.BudgetField.Release(bank)
	if c.Blocks[a.Slot-1].capacity != 2 || c.Blocks[a.Slot-1].Size != 2 {
		t.Fatal("failed growth published")
	}
	check()
	paramsMust(t, c.SetUInt(&a, three, 3))
	check()
	text := "owned"
	paramsMust(t, c.SetString(&a, one, &text))
	dst := make([]byte, 128)
	_, e = c.FormatInto(&a, dst)
	paramsMust(t, e)
	check()
	paramsMust(t, c.Reset(&a))
	check()
	paramsMust(t, c.Del(&a))
}

func TestParamsReferenceFiniteSignedRatAndNullDefaults(t *testing.T) {
	c, sy, q := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	k := internSymbol(t, sy, "uint")
	def, e := q.NewValue()
	paramsMust(t, e)
	paramsMust(t, q.SetInt64(&def, 17))
	for _, n := range []uint32{0, 1, 2147483647, 2147483648, 4294967295} {
		paramsMust(t, c.SetUIntName(&a, pdLiteral("uint"), n))
		direct, e := c.GetUInt(&a, k, 0)
		paramsMust(t, e)
		named, e := c.GetUIntName(&a, pdLiteral("uint"), 0)
		paramsMust(t, e)
		if direct != n || named != n {
			t.Fatal("uint overloads")
		}
		r, e := c.GetRatName(&a, pdLiteral("uint"), &def)
		paramsMust(t, e)
		got, e := q.String(&r)
		paramsMust(t, e)
		if got != strconv.FormatInt(int64(int32(n)), 10) {
			t.Fatal(n, got)
		}
		paramsMust(t, q.Del(&r))
	}
	paramsMust(t, c.SetBoolName(&a, nil, true))
	flag, e := c.GetBoolName(&a, nil, false)
	paramsMust(t, e)
	if !flag {
		t.Fatal("null key")
	}
	paramsMust(t, c.SetString(&a, k, pdLiteral("")))
	s, p, e := c.GetString(&a, k, nil)
	paramsMust(t, e)
	if !p || s != "" {
		t.Fatal("empty differs from null")
	}
	paramsMust(t, q.SetFractionSigned(&def, 1, 3))
	paramsMust(t, c.SetRatName(&a, pdLiteral("uint"), &def))
	returned, e := c.GetRatName(&a, pdLiteral("uint"), &def)
	paramsMust(t, e)
	ratText, e := q.String(&returned)
	paramsMust(t, e)
	if ratText != "1/3" {
		t.Fatal(ratText)
	}
	paramsMust(t, q.Del(&returned))
	paramsMust(t, q.SetInt64(&def, 17))
	paramsMust(t, c.Reset(&a))
	r, e := c.GetRat(&a, k, &def)
	paramsMust(t, e)
	paramsMust(t, q.SetInt64(&r, 99))
	got, e := q.String(&def)
	paramsMust(t, e)
	if got != "17" {
		t.Fatal("returned default aliases")
	}
	paramsMust(t, q.Del(&r))
	paramsMust(t, q.Del(&def))
	paramsMust(t, c.Del(&a))
}

func TestParamsReferenceProvenanceBeforeMutation(t *testing.T) {
	c, sy, q := paramsTestOwner(t)
	other, foreign, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	b, e := other.NewValue()
	paramsMust(t, e)
	key := internSymbol(t, sy, "key")
	fk := internSymbol(t, foreign, "key")
	paramsMust(t, c.SetBool(&a, key, true))
	before := c.Stats()
	if e = c.SetBool(&a, fk, false); e == nil {
		t.Fatal("foreign symbol accepted")
	}
	if e = c.Copy(&a, &b); e != ErrParamsOwner {
		t.Fatal(e)
	}
	if _, e = c.GetBool(&b, key, false); e != ErrParamsOwner {
		t.Fatal(e)
	}
	if c.Stats() != before {
		t.Fatal("provenance mutated graph")
	}
	var raw ParamsRef
	if _, e = c.Empty(&raw); e != ErrParamsOwner {
		t.Fatal("raw zero must use NewValue", e)
	}
	var copyOwner ParamsContext
	reflect.ValueOf(&copyOwner).Elem().Set(reflect.ValueOf(c).Elem())
	if _, e = copyOwner.NewValue(); e != ErrParamsOwner {
		t.Fatal("copied owner", e)
	}
	paramsMust(t, c.Del(&a))
	paramsMust(t, other.Del(&b))
	_ = q
}

func TestParamsReferenceCStringPrefix(t *testing.T) {
	// Stop at the first NUL before measuring the host prefix; later bytes are not
	// part of a source C string and must not affect the stored length.
	c, sy, _ := paramsTestOwner(t)
	a, e := c.NewValue()
	paramsMust(t, e)
	text := "x\x00tail"
	paramsMust(t, c.SetString(&a, internSymbol(t, sy, "s"), &text))
	x := c.Entry(&c.Blocks[a.Slot-1], 0)
	if len(x.text.owned.value) != 1 {
		t.Fatal(x.text.owned.value)
	}
	paramsMust(t, c.Del(&a))
}

func TestParamsEntryGrowthBounds(t *testing.T) {
	if n, e := pdGrowth(0); e != nil || n != 2 {
		t.Fatal(n, e)
	}
	if n, e := pdGrowth(5); e != nil || n != 8 {
		t.Fatal(n, e)
	}
	if uint64(^uint(0)>>1) > uint64(^uint32(0)) {
		n := int(uint64(^uint32(0)))
		if _, e := pdGrowth(n); e != arena.ErrBounds {
			t.Fatal("uint32 capacity overflow", e)
		}
	}
}
