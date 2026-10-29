package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp imp parameter selectors.

import (
	"math"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (r *PB2BVRewriter) profileSymbol(name, fallback string) (string, error) {
	empty := usymbol.Symbol{}
	s, e := r.paramsContext.GetSymbolName(&r.parameters, &name, empty)
	if e != nil || s == empty {
		return fallback, e
	}
	text, present, e := r.paramsContext.SymbolsField.BareString(s)
	if e != nil || !present {
		return fallback, e
	}
	return text, nil
}

func (r *PB2BVRewriter) loadProfile() error {
	keepName := "keep_cardinality_constraints"
	keep, e := r.paramsContext.GetBoolName(&r.parameters, &keepName, false)
	if e != nil {
		return e
	}
	for _, name := range []string{"sat.cardinality.solver", "cardinality.solver"} {
		v, x := r.paramsContext.GetBoolName(&r.parameters, &name, false)
		if x != nil {
			return x
		}
		keep = keep || v
	}
	solver, e := r.profileSymbol("sat.pb.solver", "")
	if e != nil {
		return e
	}
	if solver == "" {
		solver, e = r.profileSymbol("pb.solver", "solver")
		if e != nil {
			return e
		}
	}
	minName := "sat.pb.min_arity"
	minimum, e := r.paramsContext.GetUIntName(&r.parameters, &minName, math.MaxUint32)
	if e != nil {
		return e
	}
	if minimum == math.MaxUint32 {
		minName = "pb.min_arity"
		minimum, e = r.paramsContext.GetUIntName(&r.parameters, &minName, 9)
		if e != nil {
			return e
		}
	}
	r.keepCardinality, r.pbSolver, r.minArity = keep, solver, minimum
	return nil
}
