package protomodel

// Source: src/smt/theory_fpa.h:48-66.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestFloatingPointFactoryNumericZeroOneAndNoRetainedValues(t *testing.T) {
	m, p := protoFixture(t)
	family, err := m.RegisterFloatingPointPlugin()
	protoMust(t, err)
	f, err := NewFloatingPointFactory(m, family)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	for _, format := range [][2]uint32{{2, 2}, {3, 4}, {8, 24}, {11, 53}, {15, 113}} {
		e, s := format[0], format[1]
		t.Run(fmt.Sprintf("%d/%d", e, s), func(t *testing.T) {
			sort, err := m.FloatingPointSort(e, s)
			protoMust(t, err)
			protoPin(t, m, sort)
			zero, err := p.SomeValue(sort)
			protoMust(t, err)
			protoPin(t, m, zero)
			first, second, ok, err := protoTestSomeValues(t, m, p, sort)
			protoMust(t, err)
			if !ok || first != zero || first == second {
				t.Fatal("source numeric pair", first, second, zero, ok)
			}
			protoPin(t, m, second)
			kind, err := m.GetDeclKind(first)
			protoMust(t, err)
			if kind != ast.FPAOpPositiveZero {
				t.Fatal("first must be positive zero", kind)
			}
			kind, err = m.GetDeclKind(second)
			protoMust(t, err)
			if kind != ast.FPAOpNumeral {
				t.Fatal("second must be numeric one, not negative zero", kind)
			}
			encoded, found, _, err := m.RewriteFPAToIEEEBVNumeral(second, false)
			protoMust(t, err)
			bias := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(e-1)), big.NewInt(1))
			expectedBits := new(big.Int).Lsh(bias, uint(s-1))
			want, err := m.BitVectorNumeralRat(new(big.Rat).SetInt(expectedBits), e+s)
			protoMust(t, err)
			if !found || encoded != want {
				t.Fatal("one IEEE encoding", format, found, encoded, want)
			}
			before, err := m.GetRefCount(second)
			protoMust(t, err)
			protoMust(t, f.RegisterValue(second))
			after, err := m.GetRefCount(second)
			protoMust(t, err)
			if after != before {
				t.Fatal("register_value must ignore its argument", before, after)
			}
			// Repeated borrowed-handle calls canonicalize. Output expr_ref ownership
			// is a separate interface audit; register_value itself retains nothing.
			a, b, ok, err := protoTestSomeValues(t, m, p, sort)
			protoMust(t, err)
			if !ok || a != first || b != second {
				t.Fatal("unstable canonical pair")
			}
		})
	}
}

func TestFloatingPointFactoryRejectsNonFloatSortsWithSourceAssertion(t *testing.T) {
	for _, pair := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			for _, action := range []string{"T", "C"} {
				t.Run(fmt.Sprintf("%t/%t/%s", pair, enabled, action), func(t *testing.T) {
					d, report := scalarFactoryDebugActions(t, action)
					m, _ := protoFixture(t)
					family, err := m.RegisterFloatingPointPlugin()
					protoMust(t, err)
					f, err := NewFloatingPointFactory(m, family)
					protoMust(t, err)
					defer func() { protoMust(t, f.Close()) }()
					sort, err := m.RoundingModeSort()
					protoMust(t, err)
					protoMust(t, d.EnableAssertions(enabled))
					if pair {
						_, _, _, err = protoTestSomeValues(t, m, f, sort)
					} else {
						_, err = f.SomeValue(sort)
					}
					if diagnostic.DebugBuild && enabled && action == "T" {
						var ex z3exception.DefaultException
						if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
							t.Fatal("source width assertion", err)
						}
					} else if !errors.Is(err, ast.ErrASTDomain) {
						t.Fatal("physical missing format parameter", err)
					}
					data, e := os.ReadFile(report)
					protoMust(t, e)
					if diagnostic.DebugBuild && enabled {
						if !strings.Contains(string(data), "File: src/ast/fpa_decl_plugin.cpp\nLine: 941\nis_float(s)\n") {
							t.Fatal("width accessor notification", string(data))
						}
					} else if len(data) != 0 {
						t.Fatal("erased width assertion evaluated", string(data))
					}
				})
			}
		}
	}
}

func TestFloatingPointFactoryFreshValueSourceFailure(t *testing.T) {
	// Throw stops UNREACHABLE before the following source exit. The release
	// profile returns the existing explicit native-unreachable guard.
	d, report := scalarFactoryDebugActions(t, "T")
	protoMust(t, d.EnableAssertions(false)) // UNREACHABLE ignores this switch.
	f := &FloatingPointFactory{}
	value, err := f.FreshValue(ast.ASTHandle{})
	if value != (ast.ASTHandle{}) {
		t.Fatal("fresh value was synthesized")
	}
	if diagnostic.DebugBuild {
		var ex z3exception.DefaultException
		if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
			t.Fatal("source fatal throw", err)
		}
	} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
		t.Fatal("source native unreachable", err)
	}
	data, e := os.ReadFile(report)
	protoMust(t, e)
	if !strings.Contains(string(data), "NOT IMPLEMENTED YET!\n") {
		t.Fatal("missing source failure message", string(data))
	}
	if diagnostic.DebugBuild && !strings.Contains(string(data), "File: src/smt/theory_fpa.h\nLine: 65\nUNREACHABLE CODE WAS REACHED.\n") {
		t.Fatal("source fatal location", string(data))
	}
}

func TestFloatingPointFactoryClosedAndNilGuards(t *testing.T) {
	m, _ := protoFixture(t)
	family, err := m.RegisterFloatingPointPlugin()
	protoMust(t, err)
	f, err := NewFloatingPointFactory(m, family)
	protoMust(t, err)
	protoMust(t, f.Close())
	for _, factory := range []*FloatingPointFactory{nil, f} {
		if _, err := factory.SomeValue(ast.ASTHandle{}); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal("closed scalar", err)
		}
		if _, _, _, err := protoTestSomeValues(t, m, factory, ast.ASTHandle{}); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal("closed pair", err)
		}
	}
}

func TestFloatingPointFactoryArrayRangeConsumer(t *testing.T) {
	m, p := protoFixture(t)
	family, err := m.RegisterFloatingPointPlugin()
	protoMust(t, err)
	f, err := NewFloatingPointFactory(m, family)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	af, err := NewArrayFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(af))
	boolean, err := m.MkBoolSort()
	protoMust(t, err)
	fp, err := m.FloatingPointSort(3, 4)
	protoMust(t, err)
	array, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), fp)
	protoMust(t, err)
	protoPin(t, m, array)
	a, b, ok, err := protoTestSomeValues(t, m, p, array)
	protoMust(t, err)
	if !ok || a == b {
		t.Fatal("array pair was not built")
	}
	kinds := map[int32]bool{}
	for i, value := range []ast.ASTHandle{a, b} {
		decl, err := m.AsArrayFunction(value)
		protoMust(t, err)
		fi, err := p.Core().FunctionInterpretation(decl)
		protoMust(t, err)
		if fi == nil || fi.NumEntries() != 1 {
			t.Fatal("array range consumer lost its interpretation")
		}
		entry, err := fi.Entry(0)
		protoMust(t, err)
		result, err := entry.Result()
		protoMust(t, err)
		kind, err := m.GetDeclKind(result)
		protoMust(t, err)
		// The pair follows the array value_set obj_hashtable order, so either
		// array may come first; together they carry numeric zero and one.
		if kind != ast.FPAOpPositiveZero && kind != ast.FPAOpNumeral || kinds[kind] {
			t.Fatal("array range did not use numeric zero/one", i, kind)
		}
		kinds[kind] = true
	}
}

func TestFloatingPointFactoryFreshContinueExitChild(t *testing.T) {
	if os.Getenv("G3_FPA_FACTORY_EXIT_CHILD") != "1" {
		return
	}
	budget := arena.NewBudget(1 << 20)
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: budget}})
	protoMust(t, err)
	d, err := diagnostic.NewDebugContext(region, budget)
	protoMust(t, err)
	streams, err := diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: budget})
	protoMust(t, err)
	input, err := diagnostic.NewDebuggerCharInput(streams, os.Stdin)
	protoMust(t, err)
	actions, err := diagnostic.NewDebugActionContext(diagnostic.DebugActionConfig{Budget: budget, Streams: streams, Input: input})
	protoMust(t, err)
	protoMust(t, d.BindAssertionStreams(streams))
	protoMust(t, d.BindAssertionActions(actions))
	previous, err := diagnostic.BindProcessDebugContext(d)
	protoMust(t, err)
	protoMust(t, d.EnableAssertions(false))
	_, err = (&FloatingPointFactory{}).FreshValue(ast.ASTHandle{})
	if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
		t.Fatal("fresh Continue must exit or reach native guard", err)
	}
	_, err = diagnostic.BindProcessDebugContext(previous)
	protoMust(t, err)
	protoMust(t, actions.Close())
	protoMust(t, input.Close())
	protoMust(t, streams.Close())
	protoMust(t, d.Close())
	protoMust(t, region.Close())
	if budget.Used() != 0 {
		t.Fatal("native guard leaked debugger owners", budget.Used())
	}
}

func TestFloatingPointFactoryFreshContinueExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFloatingPointFactoryFreshContinueExitChild$")
	command.Env = append(os.Environ(), "G3_FPA_FACTORY_EXIT_CHILD=1")
	command.Stdin = strings.NewReader("C")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatal("source exit child timed out", ctx.Err())
	}
	if diagnostic.DebugBuild {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 107 {
			t.Fatal("continued source NOT_IMPLEMENTED_YET must exit107", err, output.String())
		}
	} else {
		protoMust(t, err)
	}
	if !strings.Contains(output.String(), "NOT IMPLEMENTED YET!\n") {
		t.Fatal("source error message missing", output.String())
	}
}

func TestFloatingPointFactoryColdAllocationPrefixes(t *testing.T) {
	failed, completed, partial := false, false, false
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		done := false
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, p := protoFixture(t)
			family, err := m.RegisterFloatingPointPlugin()
			protoMust(t, err)
			f, err := NewFloatingPointFactory(m, family)
			protoMust(t, err)
			protoMust(t, p.RegisterFactory(f))
			sort, err := m.FloatingPointSort(8, 24)
			protoMust(t, err)
			protoPin(t, m, sort)
			firstRef, secondRef := protoTestOutputRefs(t, m)
			m.Budget().FailAfter(ordinal)
			ok, call := f.SomeValues(sort, firstRef, secondRef)
			m.Budget().FailAfter(-1)
			first, read := m.RefGet(firstRef)
			protoMust(t, read)
			second, read := m.RefGet(secondRef)
			protoMust(t, read)
			if call != nil {
				failed = true
				if ok {
					t.Fatal("failed pair claimed success")
				}
			} else {
				completed = true
				done = true
				if !ok || first.IsNull() || second.IsNull() {
					t.Fatal("completed pair missing output")
				}
			}
			if !first.IsNull() {
				kind, err := m.GetDeclKind(first)
				protoMust(t, err)
				if kind != ast.FPAOpPositiveZero {
					t.Fatal("published zero prefix changed", kind)
				}
				if call != nil {
					partial = true
				}
			}
		})
		if done {
			break
		}
	}
	if !failed || !completed {
		t.Fatal("cold allocation census missed failure or completion", failed, completed)
	}
	t.Log("published zero before a later refusal:", partial)
}

func TestFloatingPointFactoryBothWidthAssertionsThenLiteralParameters(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, actions := range []string{"CC", "CT"} {
			t.Run(fmt.Sprintf("%t/%s", enabled, actions), func(t *testing.T) {
				d, report := scalarFactoryDebugActions(t, actions)
				m, _ := protoFixture(t)
				family, err := m.RegisterFloatingPointPlugin()
				protoMust(t, err)
				f, err := NewFloatingPointFactory(m, family)
				protoMust(t, err)
				defer func() { protoMust(t, f.Close()) }()
				// A real foreign sort with integer parameters can physically satisfy both
				// parameter reads after Continue. The source SASSERT is not a domain gate.
				var params []ast.ASTParameter
				for _, n := range []int32{3, 4} {
					p, e := m.NewParameterInt(n)
					protoMust(t, e)
					params = append(params, p)
				}
				defer func() {
					for i := range params {
						protoMust(t, m.DelParameter(&params[i]))
					}
				}()
				info, err := m.NewSortInfo(ast.NullFamilyID, 77, params, ast.InfiniteSortSize(), false)
				protoMust(t, err)
				defer func() { protoMust(t, m.DelSortInfo(&info)) }()
				sort, err := m.MkSort(protoSymbol(t, m, "foreign_parameterized_fpa_widths"), info)
				protoMust(t, err)
				ref, err := m.NewRef(sort)
				protoMust(t, err)
				defer func() { protoMust(t, m.DelRef(&ref)) }()
				protoMust(t, d.EnableAssertions(enabled))
				value, err := f.SomeValue(sort)
				if diagnostic.DebugBuild && enabled && actions == "CT" {
					var ex z3exception.DefaultException
					if !errors.As(err, &ex) || ex.Message() != "assertion violation" || !value.IsNull() {
						t.Fatal("second width assertion did not throw", value, err)
					}
				} else {
					protoMust(t, err)
					kind, e := m.GetDeclKind(value)
					protoMust(t, e)
					if kind != ast.FPAOpPositiveZero {
						t.Fatal("continued widths did not feed numeric setter", kind)
					}
					valueSort, e := m.GetSort(value)
					protoMust(t, e)
					eBits, sBits, matched, e := m.FloatingPointSortShape(valueSort)
					protoMust(t, e)
					if !matched || eBits != 3 || sBits != 4 {
						t.Fatal("source literal parameters changed", eBits, sBits, matched)
					}
				}
				data, e := os.ReadFile(report)
				protoMust(t, e)
				if diagnostic.DebugBuild && enabled {
					first := strings.Index(string(data), "Line: 941\n")
					second := strings.Index(string(data), "Line: 946\n")
					if first < 0 || second <= first {
						t.Fatal("actual width assertion order", string(data))
					}
				} else if len(data) != 0 {
					t.Fatal("disabled width assertions evaluated", string(data))
				}
			})
		}
	}
}
