package numeral

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestArbitraryFloatUnpackEntryTracePreMutationClasses(t *testing.T) {
	m, _, _ := mpfNumeralHelpersFixture(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("mpf_dbg")
	compiled := tags.ShouldTrace("mpf_dbg")
	serviceMust(t, m.BindTrace(tags))
	bottom, e := m.MkBotExp(8)
	serviceMust(t, e)
	for _, tc := range []struct {
		name      string
		class     string
		normalize bool
		build     func(*ArbitraryFloat)
	}{
		{"zero", "has_bot_exp=1 (-127) is_zero=1", true, func(x *ArbitraryFloat) { serviceMust(t, m.MkZero(8, 24, false, x)) }},
		{"normal", "has_bot_exp=0 (-127) is_zero=0", true, func(x *ArbitraryFloat) { serviceMust(t, m.MkOne(8, 24, false, x)) }},
		{"denormal", "has_bot_exp=1 (-127) is_zero=0", true, func(x *ArbitraryFloat) {
			sig := NewInt(1)
			serviceMust(t, m.SetComponents(x, 8, 24, false, bottom, &sig))
		}},
		{"denormal no normalize", "has_bot_exp=1 (-127) is_zero=0", false, func(x *ArbitraryFloat) {
			sig := NewInt(1)
			serviceMust(t, m.SetComponents(x, 8, 24, false, bottom, &sig))
		}},
	} {
		var x ArbitraryFloat
		tc.build(&x)
		output.Reset()
		serviceMust(t, m.unpackRationalNumeral(&x, tc.normalize, numeralContextRef(integerAllocationInvocationContext{})))
		got := output.String()
		if !compiled {
			if got != "" {
				t.Fatalf("%s erased unpack trace: %q", tc.name, got)
			}
		} else {
			want := "unpack src/util/mpf.cpp:1923 ---------\nunpack "
			if !strings.Contains(got, want) || !strings.Contains(got, ": ebits=8 sbits=24 normalize="+traceBoolDigit(tc.normalize)+" has_top_exp=0 (128) "+tc.class+"\n") || strings.Count(got, "------------------------------------------------\n") != 1 {
				t.Fatalf("%s entry fields before mutation: %q", tc.name, got)
			}
		}
		serviceMust(t, m.Del(&x))
	}
}

func TestArbitraryFloatUnpackTraceRefusalBeforeMutation(t *testing.T) {
	m, _, budget := mpfNumeralHelpersFixture(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("mpf_dbg")
	if !tags.ShouldTrace("mpf_dbg") {
		return
	}
	serviceMust(t, m.BindTrace(tags))
	var x ArbitraryFloat
	serviceMust(t, m.MkOne(8, 24, false, &x))
	before := x
	budget.FailAfter(0)
	e := m.unpackRationalNumeral(&x, true, numeralContextRef(integerAllocationInvocationContext{}))
	budget.FailAfter(-1)
	if !errors.Is(e, arena.ErrInjectedFailure) || x != before || !strings.Contains(output.String(), "unpack src/util/mpf.cpp:1923 ---------\nunpack ") {
		t.Fatalf("unpack entry formatter refusal changed input: %v, %#v, %q", e, x, output.String())
	}
	serviceMust(t, m.unpackRationalNumeral(&x, true, numeralContextRef(integerAllocationInvocationContext{})))
	serviceMust(t, m.Del(&x))
}

func TestArbitraryFloatUnpackSourceInsertionBoundaries(t *testing.T) {
	m, _, _ := mpfNumeralHelpersFixture(t)
	bottom, err := m.MkBotExp(8)
	serviceMust(t, err)
	for _, tc := range []struct {
		name, top, bot, zero, normalize string
		build                           func(*ArbitraryFloat)
	}{
		{"zero", "0", "1", "1", "1", func(x *ArbitraryFloat) { serviceMust(t, m.MkZero(8, 24, false, x)) }},
		{"normal", "0", "0", "0", "1", func(x *ArbitraryFloat) { serviceMust(t, m.MkOne(8, 24, false, x)) }},
		{"denormal", "0", "1", "0", "0", func(x *ArbitraryFloat) {
			sig := NewInt(1)
			serviceMust(t, m.SetComponents(x, 8, 24, false, bottom, &sig))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var x ArbitraryFloat
			tc.build(&x)
			w := &mpffArithmeticTraceWriter{}
			tags := diagnostic.NewTraceTagsWithWriter(w)
			tags.Enable("mpf_dbg")
			serviceMust(t, m.BindTrace(tags))
			serviceMust(t, m.unpackRationalNumeral(&x, tc.normalize == "1", numeralContextRef(integerAllocationInvocationContext{})))
			if tags.ShouldTrace("mpf_dbg") {
				foundValue := false
				for i := 0; i+2 < len(w.parts); i++ {
					if w.parts[i] == "unpack " && w.parts[i+1] != "" && w.parts[i+2] == ": ebits=" {
						foundValue = true
						break
					}
				}
				if !foundValue {
					t.Fatalf("missing source value insertion: %q", w.parts)
				}
				want := []string{": ebits=", "8", " sbits=", "24", " normalize=", tc.normalize,
					" has_top_exp=", tc.top, " (", "128", ")", " has_bot_exp=", tc.bot, " (", "-127", ")",
					" is_zero=", tc.zero, "\n"}
				if !hasMPFFTraceParts(w.parts, want) {
					t.Fatalf("missing source field insertions %q in %q", want, w.parts)
				}
			}
			serviceMust(t, m.Del(&x))
		})
	}
}
