package rewriter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestEnum2BVApplicationArgumentOwnersBeforeNextPredicate(t *testing.T) {
	for _, mode := range []string{"equality", "distinct"} {
		t.Run(mode, func(t *testing.T) {
			m, sort, cs := enum2BVFixture(t)
			r, err := enum2BVNewForTest(t, m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, r.Close()) })
			apps := make([]ast.ASTHandle, 3)
			values := make([]ast.ASTHandle, 3)
			before := make([]uint32, 3)
			for i := range apps {
				apps[i], err = m.MkApp(cs[i], nil)
				astTestMust(t, err)
				astTestPin(t, m, apps[i])
				values[i], err = m.BitVectorNumeralUnsigned(uint32(i), 2)
				astTestMust(t, err)
				astTestPin(t, m, values[i])
				before[i], err = m.GetRefCount(values[i])
				astTestMust(t, err)
			}
			calls := 0
			stop := errors.New("next source predicate stopped")
			astTestMust(t, r.SetSortPredicate(func(s ast.ASTHandle) (bool, error) {
				if s != sort {
					t.Fatal("unexpected source predicate sort")
				}
				i := calls
				calls++
				if i > 0 {
					count, err := m.GetRefCount(values[0])
					if err != nil {
						return false, err
					}
					want := before[0] + 1
					if mode == "distinct" && i == 1 {
						want++
					}
					if count != want {
						t.Fatalf("%s operand%d lost prior lexical owner: got%d want%d", mode, i, count, want)
					}
				}
				if mode == "equality" && i == 1 || mode == "distinct" && i == 2 {
					return false, stop
				}
				return true, nil
			}))
			var application ast.ASTHandle
			var args []ast.ASTHandle
			if mode == "equality" {
				args = apps[:2]
				application, err = m.MkEq(args[0], args[1])
			} else {
				args = apps
				application, err = m.MkDistinct(args)
			}
			astTestMust(t, err)
			astTestPin(t, m, application)
			decl, err := m.GetDecl(application)
			astTestMust(t, err)
			out, proof := enum2BVRefs(t, m)
			_, err = r.config.ReduceApp(decl, ast.SliceArguments(args), out, proof)
			if !errors.Is(err, stop) {
				t.Fatal("predicate error did not unwind source locals", err)
			}
			for i, value := range values {
				count, err := m.GetRefCount(value)
				astTestMust(t, err)
				if count != before[i] {
					t.Fatal("application unwind retained argument/tmp owner", i, count, before[i])
				}
			}
			wantCount := uint32(1)
			if mode == "distinct" {
				wantCount = 2
			}
			if r.translated != wantCount || len(r.pending) != 0 {
				t.Fatal("unwind changed earlier source counters or left cleanup", r.translated, len(r.pending))
			}
		})
	}
}

func enum2BVFillReferencePage(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, free int) {
	t.Helper()
	var pins []ast.ASTRef
	for i := 0; i < free; i++ {
		pin, err := m.NewRef(h)
		astTestMust(t, err)
		pins = append(pins, pin)
	}
	for i := 0; i < 4096; i++ {
		m.Budget().FailAfter(0)
		pin, err := m.NewRef(h)
		m.Budget().FailAfter(-1)
		if errors.Is(err, arena.ErrInjectedFailure) {
			break
		}
		astTestMust(t, err)
		pins = append(pins, pin)
		if i == 4095 {
			t.Fatal("reference-page frontier not reached")
		}
	}
	for i := len(pins) - free; i < len(pins); i++ {
		astTestMust(t, m.DelRef(&pins[i]))
	}
	t.Cleanup(func() {
		for i := range pins {
			if pins[i] != (ast.ASTRef{}) {
				astTestMust(t, m.DelRef(&pins[i]))
			}
		}
	})
}

func TestEnum2BVDefinitionOwnerAndPublishedReferencePrefix(t *testing.T) {
	ownedTailFailures, publishedFailures, successes := 0, 0, 0
	for ordinal := int64(0); ordinal < 96 && successes == 0; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, sort, cs := enum2BVFixture(t)
			r, err := enum2BVNewForTest(t, m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, r.Close()) })
			_, err = r.utility.IsEnumSort(sort)
			astTestMust(t, err)
			_, err = r.config.utility.Constructors(sort)
			astTestMust(t, err)
			// Warm literal/constructor nodes; the final constructor value remains
			// borrowed so failed definition ownership can be observed by its deletion.
			for i := uint32(0); i < 3; i++ {
				_, err = m.BitVectorNumeralUnsigned(i, 2)
				astTestMust(t, err)
			}
			tail, err := m.MkApp(cs[2], nil)
			astTestMust(t, err)
			argument, err := m.MkConst(astTestSymbol(t, m, "expr-publication"), sort)
			astTestMust(t, err)
			astTestPin(t, m, argument)
			decl, err := m.GetDecl(argument)
			astTestMust(t, err)
			before, err := m.GetRefCount(decl)
			astTestMust(t, err)
			out := astTestPin(t, m, ast.ASTHandle{})
			enum2BVFillReferencePage(t, m, argument, 3)
			m.Budget().FailAfter(ordinal)
			_, err = r.reduceArgument(argument, &out)
			m.Budget().FailAfter(-1)
			if err == nil {
				successes++
				return
			}
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("unexpected definition refusal", ordinal, err)
			}
			value, err2 := m.RefGet(out)
			astTestMust(t, err2)
			if r.translated != 0 {
				t.Fatal("source fresh counter advanced before definition/owner completion")
			}
			if !value.IsNull() && len(r.bounds) == 1 {
				if !m.IsLiveHandle(tail) {
					ownedTailFailures++
				}
			}
			if len(r.entries) == 1 {
				entry := r.entries[0]
				if entry.owned[0] != (ast.ASTRef{}) && entry.owned[1] == (ast.ASTRef{}) {
					publishedFailures++
					if r.enum2BV[decl] != entry.bitVector || r.enum2Def[decl] != entry.definition || r.bv2Enum[entry.bitVector] != decl || !r.closing {
						t.Fatal("failed second vector pin lost source map/owner prefix")
					}
					count, err := m.GetRefCount(decl)
					astTestMust(t, err)
					if count != before+1 {
						t.Fatal("published original declaration pin was rolled back", count, before)
					}
					if r.EnumDefinitions() != nil {
						t.Fatal("stopped borrowed mappings remained publicly usable")
					}
					astTestMust(t, r.Close())
					count, err = m.GetRefCount(decl)
					astTestMust(t, err)
					if count != before {
						t.Fatal("Close lost/retained a published prefix pin", count, before)
					}
					survivor, err := m.RefGet(out)
					astTestMust(t, err)
					if survivor.IsNull() {
						t.Fatal("source result assignment prefix disappeared")
					}
					_, err = m.GetKind(survivor)
					astTestMust(t, err)
				}
			}
		})
	}
	if successes != 1 || ownedTailFailures == 0 || publishedFailures == 0 {
		t.Fatal("allocation census did not reach definition ownership and second vector-pin refusal", successes, ownedTailFailures, publishedFailures)
	}
}

func TestEnum2BVArgumentAssertionConsultsActualOwner(t *testing.T) {
	m, sort, cs := enum2BVFixture(t)
	r, err := enum2BVNewForTest(t, m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	_, err = r.utility.IsEnumSort(sort)
	astTestMust(t, err)
	value, err := m.MkApp(cs[0], nil)
	astTestMust(t, err)
	astTestPin(t, m, value)
	out := astTestPin(t, m, ast.ASTHandle{})
	budget := arena.NewBudget(1 << 20)
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: budget}})
	astTestMust(t, err)
	debug, err := diagnostic.NewDebugContext(region, budget)
	astTestMust(t, err)
	previous, err := diagnostic.BindProcessDebugContext(debug)
	astTestMust(t, err)
	t.Cleanup(func() {
		_, err := diagnostic.BindProcessDebugContext(previous)
		astTestMust(t, err)
		astTestMust(t, debug.Close())
		astTestMust(t, region.Close())
		if budget.Used() != 0 {
			t.Fatal("debug owner retained bytes")
		}
	})
	astTestMust(t, debug.Close())
	if diagnostic.DebugBuild {
		if err := enum2BVAppAssertion(ast.ASTApp); !errors.Is(err, arena.ErrClosed) {
			t.Fatal("source assertion103 boundary ignored actual owner", err)
		}
	}
	_, err = r.reduceArgument(value, &out)
	if diagnostic.DebugBuild {
		if !errors.Is(err, arena.ErrClosed) {
			t.Fatal("source SASSERT103 ignored actual closed owner", err)
		}
	} else {
		astTestMust(t, err)
	}
}

func TestEnum2BVStoppedLexicalCleanupPrecedesMembers(t *testing.T) {
	s, q, b := astTestOwners(t)
	writer := &enum2BVStopWriter{}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: s, Numerals: q, Budget: b, TraceTags: tags})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	astTestMust(t, m.RegisterDeclarationPlugins())
	r, err := enum2BVNewForTest(t, m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	truth, err := m.MkTrue()
	astTestMust(t, err)
	astTestPin(t, m, truth)
	before, err := m.GetRefCount(truth)
	astTestMust(t, err)
	scalar := &enum2BVScalarScope{manager: m}
	scalar.ref, err = m.NewRef(truth)
	astTestMust(t, err)
	app := &enum2BVAppScope{manager: m}
	app.a0, err = m.NewRef(truth)
	astTestMust(t, err)
	app.a1, err = m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	vector, err := m.NewRef(truth)
	astTestMust(t, err)
	app.args = append(app.args, vector)
	fp, err := m.FloatingPointSort(3, 4)
	astTestMust(t, err)
	writer.action = func() error {
		var err error
		r.finishScalar(scalar, &err)
		r.finishApp(app, &err)
		return errors.Join(err, r.Close())
	}
	tags.Enable("ast")
	if !tags.ShouldTrace("ast") {
		astTestMust(t, scalar.close())
		astTestMust(t, app.close())
		t.Skip("requires source trace build")
	}
	_, err = m.MkFPAZeroOfSort(fp, false)
	astTestMust(t, err)
	if !writer.called || !errors.Is(writer.err, ast.ErrASTPending) || len(r.pending) != 2 || r.pending[0].scalar != scalar || r.pending[1].app != app {
		t.Fatal("lexical cleanup lost inner-before-outer owner queue", writer.called, writer.err, len(r.pending))
	}
	count, err := m.GetRefCount(truth)
	astTestMust(t, err)
	if count != before+3 {
		t.Fatal("unconsumed lexical failure lost its actual pins", count, before)
	}
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	if _, err = r.config.utility.IsEnumSort(boolean); err != nil {
		t.Fatal("stopped lexical cleanup entered configuration member teardown", err)
	}
	astTestMust(t, r.Close())
	astTestMust(t, r.Close())
	count, err = m.GetRefCount(truth)
	astTestMust(t, err)
	if count != before || len(r.pending) != 0 {
		t.Fatal("owning retry retained or replayed lexical refs", count, before)
	}
}

func TestEnum2BVNonFiniteApplicationNeedsNoReferenceReservation(t *testing.T) {
	m, _, _ := enum2BVFixture(t)
	r, err := enum2BVNewForTest(t, m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	application, err := m.MkAndArray([]ast.ASTHandle{truth, falsity})
	astTestMust(t, err)
	astTestPin(t, m, application)
	decl, err := m.GetDecl(application)
	astTestMust(t, err)
	out, proof := enum2BVRefs(t, m)
	enum2BVFillReferencePage(t, m, truth, 0)
	m.Budget().FailAfter(0)
	status, err := r.config.ReduceApp(decl, ast.SliceArguments([]ast.ASTHandle{truth, falsity}), out, proof)
	m.Budget().FailAfter(-1)
	astTestMust(t, err)
	if status != RewriteFailed || len(r.pending) != 0 {
		t.Fatal("ordinary Boolean application allocated/retained enum locals", status)
	}
}
