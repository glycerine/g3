package polynomial

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func algebraicManagerFixture(t *testing.T) (AlgebraicManagerConfig, *uparams.ParamsContext, *arena.Budget) {
	return algebraicManagerFixtureWithParameterKeys(t, true)
}

func algebraicManagerFixtureWithParameterKeys(t *testing.T, warm bool) (AlgebraicManagerConfig, *uparams.ParamsContext, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(1 << 27)
	_, sy, q := sxOwner(t, b)
	context, e := uparams.NewParamsContext(sy, q, uparams.ParamsConfig{Budget: b, PageEntries: 2})
	serviceMust(t, e)
	catalog, e := uparams.NewGlobalParamsCatalog(sy, uparams.GlobalParamsCatalogConfig{Budget: b, Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{Budget: b, PageSize: 2}}, Text: arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{Budget: b, PageSize: 2}}})
	serviceMust(t, e)
	globals, e := uparams.NewGlobalParamsService(sy, context, uparams.GlobalParamsConfig{Budget: b, Catalog: catalog})
	serviceMust(t, e)
	serviceMust(t, globals.InitializeSource())
	p, e := context.NewValue()
	serviceMust(t, e)
	limit, e := control.NewReslimit(b)
	serviceMust(t, e)
	coop, e := cooperate.NewCooperation(b)
	serviceMust(t, e)
	primes, e := primegen.NewGenerator(b)
	serviceMust(t, e)
	// Source namespace symbols are borrowed global services, independent of the
	// constructor under test. Keep their cold interning outside its fault sweep.
	if warm {
		for _, name := range []string{"algebraic", "min_mag", "factor", "factor_max_prime", "factor_num_primes", "factor_search_size", "zero_accuracy"} {
			_, e = sy.Intern(name)
			serviceMust(t, e)
		}
	}
	t.Cleanup(func() {
		b.FailAfter(-1)
		serviceMust(t, primes.Close())
		serviceMust(t, coop.Close())
		limit.Close()
		serviceMust(t, context.Del(&p))
		serviceMust(t, globals.Close())
		serviceMust(t, catalog.Close())
		serviceMust(t, context.Close())
	})
	return AlgebraicManagerConfig{Rationals: q, Limit: limit, Cooperation: coop, Params: &p, Globals: globals, Factorization: UPolynomialFactorizationConfig{Primes: primes}}, context, b
}
func TestAlgebraicManagerEagerMembersBorrowedServicesAndArithmetic(t *testing.T) {
	for _, borrow := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "borrowed"}[borrow], func(t *testing.T) {
			cfg, _, _ := algebraicManagerFixture(t)
			if borrow {
				a, e := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: cfg.Rationals.Manager.Integers().RuntimeOwner(), Profile: umemory.SmallAllocatorProfile{PointerBits: 64}})
				serviceMust(t, e)
				cfg.Allocator = a
				defer func() { serviceMust(t, a.Close()) }()
			}
			m, e := NewAlgebraicManager(cfg)
			serviceMust(t, e)
			defer func() { serviceMust(t, m.Close()) }()
			c := m.AlgebraicRootCore
			rationals, e := m.RationalManager()
			serviceMust(t, e)
			binary, e := m.BinaryRationalManager()
			serviceMust(t, e)
			if rationals != cfg.Rationals.Manager || binary != m.binary {
				t.Fatal("source numeric accessor identity")
			}
			if c.multivariate == nil || c.multivariate != m.polynomial || c.config.Polynomials != m.univariate || c.resultantVariableCount != 2 || c.resultantVariables != [2]uint32{0, 1} {
				t.Fatal("source eager manager identity")
			}
			for _, p := range []UPolynomial{c.isolateTmp1, c.tmp2, c.tmp3, c.evalSignTmp, c.addTmp} {
				if p.slot == 0 {
					t.Fatal("source empty polynomial member was lazy")
				}
				size, e := m.univariate.Size(p)
				serviceMust(t, e)
				if size != 0 {
					t.Fatal("nonempty source default vector")
				}
			}
			if c.factors == nil || c.roots == nil || c.lowers == nil || c.uppers == nil {
				t.Fatal("source eager isolation member missing")
			}
			if c.minMagnitude != -16 || !c.factor || c.zeroAccuracy != 0 || c.factorParams != (UPolynomialFactorParams{MaxPrime: 31, PrimeTrials: 1, MaxSearchSize: 5000}) {
				t.Fatal("source parameter defaults")
			}
			if cfg.Limit.Count() != 0 {
				t.Fatal("constructor consumed a source checkpoint")
			}
			n := unumeral.NewInt(2)
			var value, square Algebraic
			serviceMust(t, m.SetInteger(&value, &n))
			serviceMust(t, m.Root(&value, 2, &value))
			algebraicExpectBetween(t, c, &value, 1, 2)
			serviceMust(t, m.Power(&value, 2, &square))
			q := unumeral.NewRational()
			serviceMust(t, cfg.Rationals.Manager.SetInt32(&q, 2))
			cmp, e := m.CompareRational(&square, &q)
			serviceMust(t, e)
			serviceMust(t, cfg.Rationals.Manager.Del(&q))
			if cmp != 0 {
				t.Fatal("owned-manager root identity", cmp)
			}
			if e = m.Close(); !errors.Is(e, ErrAlgebraicLive) {
				t.Fatal("closed live caller values", e)
			}
			serviceMust(t, m.Del(&square))
			serviceMust(t, m.Del(&value))
			allocator := m.allocator
			serviceMust(t, m.Close())
			if borrow {
				if _, e = allocator.NumFreeObjects(); e != nil {
					t.Fatal("borrowed allocator closed", e)
				}
			} else {
				if _, e = allocator.NumFreeObjects(); !errors.Is(e, arena.ErrClosed) {
					t.Fatal("owned allocator left open", e)
				}
			}
			serviceMust(t, cfg.Rationals.Manager.CheckOwnerReady())
			serviceMust(t, cfg.Factorization.Primes.Check())
			snapshot, e := cfg.Globals.SourceGetModuleName("algebraic")
			serviceMust(t, e)
			serviceMust(t, cfg.Globals.Context.Del(&snapshot))
			if cfg.Limit.Closed() {
				t.Fatal("borrowed limit closed")
			}
			if _, e = cfg.Cooperation.Enabled(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestAlgebraicManagerConstructorParametersAndPublicUpdateNoop(t *testing.T) {
	cfg, context, _ := algebraicManagerFixture(t)
	values := map[string]uint32{"min_mag": uint32(1<<31) + 3, "factor_max_prime": 19, "factor_num_primes": 3, "factor_search_size": 17, "zero_accuracy": uint32(1<<31) + 7}
	for key, value := range values {
		serviceMust(t, context.SetUIntName(cfg.Params, &key, value))
	}
	factor := "factor"
	serviceMust(t, context.SetBoolName(cfg.Params, &factor, false))
	m, e := NewAlgebraicManager(cfg)
	serviceMust(t, e)
	defer func() { serviceMust(t, m.Close()) }()
	if m.minMagnitude != 2147483645 || m.zeroAccuracy != 2147483641 || m.factor || m.factorParams != (UPolynomialFactorParams{MaxPrime: 19, PrimeTrials: 3, MaxSearchSize: 17}) {
		t.Fatal("literal signed32 configuration", m.minMagnitude, m.zeroAccuracy, m.factor, m.factorParams)
	}
	key := "min_mag"
	serviceMust(t, context.SetUIntName(cfg.Params, &key, 9))
	serviceMust(t, context.SetBoolName(cfg.Params, &factor, true))
	serviceMust(t, m.UpdateParams(cfg.Params))
	serviceMust(t, m.UpdateParams(nil))
	if m.minMagnitude != 2147483645 || m.factor {
		t.Fatal("public source update changed state")
	}
	m.compareCheap, m.compareSturm, m.compareRefine, m.comparePoly = 2, 3, 5, 7
	stats := statistics.New()
	serviceMust(t, m.CollectStatistics(stats))
	// AUDIT.md FUZZ-F7: release builds report no algebraic statistics.
	if stats.Size() != 0 {
		t.Fatal("release-profile statistics", stats.Size())
	}
	serviceMust(t, m.ResetStatistics())
	stats.Reset()
	serviceMust(t, m.CollectStatistics(stats))
	if stats.Size() != 0 {
		t.Fatal("statistics reset")
	}
}

func TestAlgebraicManagerGlobalParameterFallbackAndLocalPrecedence(t *testing.T) {
	cfg, context, _ := algebraicManagerFixture(t)
	module, e := cfg.Globals.Symbols.Intern("algebraic")
	serviceMust(t, e)
	globals, e := cfg.Globals.Current.GetParams(module)
	serviceMust(t, e)
	min, prime, factor := "min_mag", "factor_max_prime", "factor"
	serviceMust(t, context.SetUIntName(globals, &min, 11))
	serviceMust(t, context.SetUIntName(globals, &prime, 23))
	serviceMust(t, context.SetBoolName(globals, &factor, false))
	serviceMust(t, context.SetUIntName(cfg.Params, &min, 5))
	m, e := NewAlgebraicManager(cfg)
	serviceMust(t, e)
	defer func() { serviceMust(t, m.Close()) }()
	if m.minMagnitude != -5 || m.factor || m.factorParams.MaxPrime != 23 {
		t.Fatal("source local/global fallback", m.minMagnitude, m.factor, m.factorParams)
	}
	// The generated helper acquired and destroyed a snapshot during construction.
	serviceMust(t, context.SetUIntName(globals, &prime, 29))
	if m.factorParams.MaxPrime != 23 {
		t.Fatal("borrowed globals remained a live configuration view")
	}
}
