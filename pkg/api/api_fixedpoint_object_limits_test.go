package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestAPIFixedpointQueryLimitsFollowLatestUpdateAndParent(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.c.SetUIntOption("timeout", 77))
	astTestMust(t, f.c.SetUIntOption("rlimit", 5))
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetSymbol("engine", "pdr"))
	astTestMust(t, params.SetUInt("timeout", 42))
	astTestMust(t, f.fp.SetParameters(params))
	astTestMust(t, params.Close())
	if value, e := f.fp.effectiveQueryTimeout(); e != nil || value != 42 {
		t.Fatal("local timeout", value, e)
	}
	params, err = f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetBool("xform.slice", false))
	astTestMust(t, f.fp.SetParameters(params))
	astTestMust(t, params.Close())
	if value, e := f.fp.effectiveQueryTimeout(); e != nil || value != 77 {
		t.Fatal("latest update should fall back to parent timeout", value, e)
	}
	astTestMust(t, f.c.SetUIntOption("timeout", 91))
	limit := f.m.ResourceLimit()
	beforeLimit, beforeDepth := limit.Limit, len(limit.Limits)
	marker := errors.New("test query failure")
	status, err := f.fp.executeQuery(true, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		name := "timeout"
		value, e := f.fp.params.params.GetUIntName(p, &name, 0)
		if e != nil || value != 91 {
			t.Fatal("effective query timeout", value, e)
		}
		if len(limit.Limits) != beforeDepth+1 || limit.Limit != limit.Count()+5 {
			t.Fatal("query resource scope", limit.Limit, limit.Count(), len(limit.Limits))
		}
		return lbool.Undef, marker
	})
	if status != lbool.Undef || !errors.Is(err, marker) {
		t.Fatal("query error", status, err)
	}
	if limit.Limit != beforeLimit || len(limit.Limits) != beforeDepth {
		t.Fatal("resource scope was not restored")
	}
	_, err = f.fp.executeQuery(false, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		if len(limit.Limits) != beforeDepth {
			t.Fatal("relation query installed a resource scope")
		}
		return lbool.True, nil
	})
	astTestMust(t, err)
}

func TestAPIFixedpointQueryResourceLimitAndRetry(t *testing.T) {
	for _, engine := range []string{"pdr", "duality"} {
		t.Run(engine, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			params, err := f.c.NewParameterSet()
			astTestMust(t, err)
			astTestMust(t, params.SetSymbol("engine", engine))
			astTestMust(t, f.fp.SetParameters(params))
			astTestMust(t, params.Close())
			astTestMust(t, f.fp.AddFact(f.p, nil))
			// Source normalization can publish an empty prefix on cancellation;
			// flush the fact first so this exercises query cancellation and retry.
			status, err := f.fp.Query(f.p)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("initial fixedpoint query", status)
			}
			astTestMust(t, f.c.SetUIntOption("rlimit", 1))
			limit := f.m.ResourceLimit()
			beforeDepth, beforeLimit := len(limit.Limits), limit.Limit
			status, _ = f.fp.Query(f.p)
			if status != lbool.Undef {
				t.Fatal("fixedpoint query exceeded resource limit", status)
			}
			if len(limit.Limits) != beforeDepth || limit.Limit != beforeLimit {
				t.Fatal("fixedpoint query did not restore resource limit")
			}
			astTestMust(t, f.c.SetUIntOption("rlimit", 0))
			status, err = f.fp.Query(f.p)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("fixedpoint retry after resource limit", status)
			}
		})
	}
}
