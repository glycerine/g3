package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIStatisticsTypedValuesRejectWrongDomain(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	object, err := newAPIStatistics(f.c.smtContextState, apiStatisticsFixture())
	astTestMust(t, err)
	defer func() { astTestMust(t, object.Close()) }()
	if value, e := object.UintValue(0); e != nil || value != 7 {
		t.Fatal("uint value", value, e)
	}
	if value, e := object.DoubleValue(1); e != nil || value != 2.5 {
		t.Fatal("double value", value, e)
	}
	if _, err = object.DoubleValue(0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("uint as double", err)
	}
	if _, err = object.UintValue(1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("double as uint", err)
	}
}
