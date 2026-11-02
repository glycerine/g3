package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointParametersSelectFactStorageBeforeQuery(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.AddFact(f.p, nil))
	rules, err := f.fp.Rules()
	astTestMust(t, err)
	if len(rules) != 1 {
		t.Fatal("module-selected PDR fact was not a rule", len(rules))
	}
	releaseAPIRefs(t, f.m, rules)

	datalog, err := f.c.NewFixedpoint()
	astTestMust(t, err)
	defer func() { astTestMust(t, datalog.Close()) }()
	relation, err := datalog.DeclareRelation("api-object-direct-fact", nil)
	astTestMust(t, err)
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetSymbol("engine", "datalog"))
	astTestMust(t, datalog.SetParameters(parameters))
	astTestMust(t, parameters.Close())
	astTestMust(t, datalog.AddFact(relation, nil))
	rules, err = datalog.Rules()
	astTestMust(t, err)
	if len(rules) != 0 {
		releaseAPIRefs(t, f.m, rules)
		t.Fatal("Datalog tuple appeared in rule enumeration", len(rules))
	}

	// Source caches the selected engine type. Later parameter updates do not
	// reinterpret already stored tuples or change the fact-ingestion path.
	parameters, err = f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetSymbol("engine", "pdr"))
	astTestMust(t, datalog.SetParameters(parameters))
	astTestMust(t, parameters.Close())
	astTestMust(t, datalog.AddFact(relation, nil))
	rules, err = datalog.Rules()
	astTestMust(t, err)
	if len(rules) != 0 {
		releaseAPIRefs(t, f.m, rules)
		t.Fatal("cached Datalog selection changed after parameter update", len(rules))
	}
	status, err := datalog.QueryRelations([]ast.ASTHandle{relation})
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("direct tuple query", status)
	}
}
