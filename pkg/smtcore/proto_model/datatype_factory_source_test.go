package protomodel

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// A concrete value-provider extension registered through the real model API.
// All returned values and sorts are genuine manager objects owned by callers.
type datatypeBVProvider struct {
	family ast.FamilyID
	values []ast.ASTHandle
	calls  int
	some   func(int) error
	fresh  func() error
}

func (p *datatypeBVProvider) FamilyID() ast.FamilyID { return p.family }
func (p *datatypeBVProvider) SomeValue(ast.ASTHandle) (ast.ASTHandle, error) {
	i := p.calls
	p.calls++
	if p.some != nil {
		if err := p.some(i); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return p.values[i%len(p.values)], nil
}
func (p *datatypeBVProvider) FreshValue(ast.ASTHandle) (ast.ASTHandle, error) {
	if p.fresh != nil {
		if err := p.fresh(); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return p.values[len(p.values)-1], nil
}
func (p *datatypeBVProvider) SomeValues(ast.ASTHandle, ast.ASTRef, ast.ASTRef) (bool, error) {
	return false, nil
}
func (p *datatypeBVProvider) RegisterValue(ast.ASTHandle) error { return nil }
func (p *datatypeBVProvider) Close() error                      { return nil }

func datatypeFactoryBVFixture(t *testing.T, arity int) (*ast.ASTManager, *DatatypeFactory, *datatypeBVProvider, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	m, p := protoFixture(t)
	bv, err := m.BitVectorSort(8)
	protoMust(t, err)
	fid, err := m.GetFamilyID(bv)
	protoMust(t, err)
	provider := &datatypeBVProvider{family: fid}
	for _, n := range []uint64{0, 1} {
		v, err := m.BitVectorNumeralUint64(n, 8)
		protoMust(t, err)
		protoPin(t, m, v)
		provider.values = append(provider.values, v)
	}
	protoMust(t, p.RegisterFactory(provider))
	fields := make([]ast.DatatypeAccessorDefinition, arity)
	for i := range fields {
		fields[i] = ast.DatatypeAccessorDefinition{Name: protoSymbol(t, m, fmt.Sprintf("bv-field-%d", i)), Range: bv}
	}
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: protoSymbol(t, m, "FactoryBVRecord"), Constructors: []ast.DatatypeConstructorDefinition{{Name: protoSymbol(t, m, "bv-record"), Recognizer: protoSymbol(t, m, "is-bv-record"), Accessors: fields}}}})
	protoMust(t, err)
	protoPin(t, m, sorts[0])
	f, err := NewDatatypeFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	constructors, err := m.DatatypeConstructors(sorts[0])
	protoMust(t, err)
	return m, f, provider, sorts[0], constructors[0]
}

func TestDatatypeFactoryColdArgumentFailureLeavesRegistryUnpublished(t *testing.T) {
	m, f, provider, sort, _ := datatypeFactoryBVFixture(t, 2)
	before, err := m.GetRefCount(provider.values[0])
	protoMust(t, err)
	stop := errors.New("second argument stopped")
	observed := false
	provider.some = func(i int) error {
		if i == 1 {
			count, err := m.GetRefCount(provider.values[0])
			if err != nil {
				return err
			}
			observed = count == before
			return stop
		}
		return nil
	}
	_, err = f.SomeValue(sort)
	if !errors.Is(err, stop) || !observed || len(f.sets) != 0 || len(f.sorts) != 0 || len(f.values) != 0 || len(f.pending) != 0 {
		t.Fatal("cold failure pinned a borrowed argument or published the value set", err, observed)
	}
	after, err := m.GetRefCount(provider.values[0])
	protoMust(t, err)
	if after != before {
		t.Fatal("borrowed argument vector changed its input reference count")
	}
}

func TestDatatypeFactoryAlmostFreshSeedsRecursiveCache(t *testing.T) {
	m, p := protoFixture(t)
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: protoSymbol(t, m, "AlmostNat"), Constructors: []ast.DatatypeConstructorDefinition{
		{Name: protoSymbol(t, m, "almost-zero"), Recognizer: protoSymbol(t, m, "is-almost-zero")},
		{Name: protoSymbol(t, m, "almost-succ"), Recognizer: protoSymbol(t, m, "is-almost-succ"), Accessors: []ast.DatatypeAccessorDefinition{{Name: protoSymbol(t, m, "almost-pred"), Recursive: true}}},
	}}})
	protoMust(t, err)
	f, err := NewDatatypeFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	value, err := f.almostFresh(sorts[0])
	protoMust(t, err)
	if value.IsNull() || f.last[sorts[0]] != value || len(f.sets[sorts[0]].values) != 1 {
		t.Fatal("empty almost-fresh path omitted its recursive last-value publication")
	}
}

func TestDatatypeFactoryAlmostFreshWithoutPreviousLastValue(t *testing.T) {
	m, p := protoFixture(t)
	bv, err := m.BitVectorSort(8)
	protoMust(t, err)
	fid, err := m.GetFamilyID(bv)
	protoMust(t, err)
	one, err := m.BitVectorNumeralUint64(1, 8)
	protoMust(t, err)
	protoPin(t, m, one)
	protoMust(t, p.RegisterFactory(&datatypeBVProvider{family: fid, values: []ast.ASTHandle{one}}))
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: protoSymbol(t, m, "AlmostTree"), Constructors: []ast.DatatypeConstructorDefinition{
		{Name: protoSymbol(t, m, "almost-node"), Recognizer: protoSymbol(t, m, "is-almost-node"), Accessors: []ast.DatatypeAccessorDefinition{{Name: protoSymbol(t, m, "node-bits"), Range: bv}, {Name: protoSymbol(t, m, "node-child"), Recursive: true}}},
		{Name: protoSymbol(t, m, "almost-leaf"), Recognizer: protoSymbol(t, m, "is-almost-leaf")},
	}}})
	protoMust(t, err)
	constructors, err := m.DatatypeConstructors(sorts[0])
	protoMust(t, err)
	leaf, err := m.MkApp(constructors[1], nil)
	protoMust(t, err)
	f, err := NewDatatypeFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	protoMust(t, f.RegisterValue(leaf))
	value, err := f.almostFresh(sorts[0])
	protoMust(t, err)
	if value.IsNull() || value == leaf || f.last[sorts[0]] != value {
		t.Fatal("absent last-value map entry blocked almost-fresh construction")
	}
}

func TestDatatypeFactoryFreshCandidateOwnedAcrossRegistrationFailure(t *testing.T) {
	reached := false
	for ordinal := int64(0); ordinal < 64 && !reached; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, f, provider, sort, c := datatypeFactoryBVFixture(t, 1)
			_, err := f.utility.Constructors(sort)
			protoMust(t, err)
			old, err := m.MkApp(c, []ast.ASTHandle{provider.values[0]})
			protoMust(t, err)
			protoMust(t, f.RegisterValue(old))
			// Prime ordinary application storage, then retire that actual temporary.
			v, err := m.MkApp(c, []ast.ASTHandle{provider.values[1]})
			protoMust(t, err)
			h, err := m.NewRef(v)
			protoMust(t, err)
			protoMust(t, m.DelRef(&h))
			var pins []ast.ASTRef
			t.Cleanup(func() {
				m.Budget().FailAfter(-1)
				for i := range pins {
					protoMust(t, m.DelRef(&pins[i]))
				}
			})
			provider.fresh = func() error {
				m.Budget().FailAfter(0)
				for attempts := 0; attempts < 4096; attempts++ {
					ref, err := m.NewRef(ast.ASTHandle{})
					if errors.Is(err, arena.ErrInjectedFailure) {
						for n := 0; n < 2; n++ {
							i := len(pins) - 1
							if i < 0 {
								return errors.New("missing free reference frontier")
							}
							if err := m.DelRef(&pins[i]); err != nil {
								return err
							}
							pins = pins[:i]
						}
						m.Budget().FailAfter(ordinal)
						return nil
					}
					if err != nil {
						return err
					}
					pins = append(pins, ref)
				}
				return errors.New("reference page frontier not reached")
			}
			candidate, accepted, err := f.freshConstructor(sort, c, f.sets[sort])
			m.Budget().FailAfter(-1)
			if candidate.IsNull() && errors.Is(err, arena.ErrInjectedFailure) {
				return
			}
			if !errors.Is(err, arena.ErrInjectedFailure) || accepted || candidate.IsNull() {
				t.Fatal("registration frontier not reached before successful publication", candidate, accepted, err)
			}
			reached = true
			t.Logf("registration refusal after %d actual application reservations", ordinal)
			if m.IsLiveHandle(candidate) {
				t.Fatal("expr_ref candidate survived failed registration")
			}
			if len(f.sets[sort].values) != 1 || f.sets[sort].values[0] != old || len(f.pending) != 0 {
				t.Fatal("failed candidate changed the registered set or lost lexical cleanup")
			}
		})
	}
	if !reached {
		t.Fatal("actual registration allocation frontier was not exercised")
	}
}

func TestDatatypeFactoryContainsUsesQuantifierBody(t *testing.T) {
	m, p := protoFixture(t)
	f, err := NewDatatypeFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	b, err := m.MkBoolSort()
	protoMust(t, err)
	x, err := m.MkConst(protoSymbol(t, m, "contained-atom"), b)
	protoMust(t, err)
	protoPin(t, m, x)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{protoSymbol(t, m, "bound")}, Body: x})
	protoMust(t, err)
	protoPin(t, m, q)
	before := m.Stats().LiveReferences
	contains, err := f.contains(q, x)
	protoMust(t, err)
	if !contains || m.Stats().LiveReferences != before || len(f.pending) != 0 {
		t.Fatal("source contains_app skipped a quantifier body or retained lexical owners")
	}
}

func TestDatatypeFactorySourceFreshDistinctnessAssertion(t *testing.T) {
	m, f, provider, sort, c := datatypeFactoryBVFixture(t, 1)
	provider.values = provider.values[:1]
	old, err := m.MkApp(c, []ast.ASTHandle{provider.values[0]})
	protoMust(t, err)
	protoMust(t, f.RegisterValue(old))
	_, err = f.utility.Constructors(sort)
	protoMust(t, err)
	before := m.Stats().LiveReferences
	_, report := scalarFactoryDebugActions(t, "T")
	_, accepted, err := f.freshConstructor(sort, c, f.sets[sort])
	if diagnostic.DebugBuild {
		if err == nil || accepted {
			t.Fatal("duplicate fresh argument bypassed source SASSERT175")
		}
		text, e := os.ReadFile(report)
		protoMust(t, e)
		if !strings.Contains(string(text), "src/smt/proto_model/datatype_factory.cpp") || !strings.Contains(string(text), "175") {
			t.Fatal("wrong assertion site", string(text))
		}
	} else {
		protoMust(t, err)
		if accepted {
			t.Fatal("erased assertion changed duplicate membership")
		}
	}
	// The cold source is_recursive query owns one persistent sort pin;
	// only the argument/candidate references must retire on this exit.
	if m.Stats().LiveReferences != before+1 || len(f.pending) != 0 {
		t.Fatal("assertion exit lost recursion owner or retained lexical references")
	}
	_, err = f.utility.IsRecursive(sort)
	protoMust(t, err)
	if m.Stats().LiveReferences != before+1 {
		t.Fatal("assertion-path recursion result was not already cached")
	}
}

// datatype_factory.cpp:156 uses expr_ref_vector, in contrast to the
// borrowed ptr_vector in get_some_value at line34.
func TestDatatypeFactoryFreshArgumentsOwnBeforeNextProvider(t *testing.T) {
	m, f, provider, sort, constructor := datatypeFactoryBVFixture(t, 2)
	value := provider.values[len(provider.values)-1]
	before, err := m.GetRefCount(value)
	protoMust(t, err)
	stop := errors.New("second fresh-constructor argument stopped")
	observed := false
	provider.some = func(i int) error {
		count, err := m.GetRefCount(value)
		if err != nil {
			return err
		}
		observed = count == before+1
		return stop
	}
	set, err := f.valueSet(sort)
	protoMust(t, err)
	_, _, err = f.freshConstructor(sort, constructor, set)
	if !errors.Is(err, stop) || !observed || len(set.values) != 0 || len(f.pending) != 0 {
		t.Fatal("fresh argument vector lost its owner or published a value", err, observed)
	}
	after, err := m.GetRefCount(value)
	protoMust(t, err)
	if after != before {
		t.Fatal("fresh argument owner survived failed constructor", after, before)
	}
}
