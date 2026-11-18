package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::process_mutex, exact rational
// specialization used by MaxRes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (e *maxResEngine) transformMutex(mutex []ast.ASTHandle, newSoft *softWeightMap) (err error) {
	if len(mutex) <= 1 {
		return nil
	}
	q := e.q()
	sorted := append([]ast.ASTHandle(nil), mutex...)
	byWeight := make(map[ast.ASTHandle]*numeral.Rational, len(sorted))
	for _, expression := range sorted {
		weight, err := newSoft.find(expression)
		if err != nil {
			return err
		}
		if weight == nil {
			return errMaxResState
		}
		byWeight[expression] = weight
	}
	e.sortMaxResMutex(sorted, byWeight)
	// vector<rational> weights; rational weight(0), sum1, sum2
	weights := make([]numeral.Rational, len(sorted))
	for i := range weights {
		weights[i] = numeral.NewRational()
	}
	weight, w, step := numeral.NewRational(), numeral.NewRational(), numeral.NewRational()
	defer func() {
		err = errors.Join(err, q.Del(&step), q.Del(&w), q.Del(&weight))
		for i := range weights {
			err = errors.Join(err, q.Del(&weights[i]))
		}
	}()
	for i, expression := range sorted {
		if err = q.Copy(&weights[i], byWeight[expression]); err != nil {
			return err
		}
		if err := newSoft.remove(expression); err != nil {
			return err
		}
	}
	for i := len(sorted); i > 0; {
		i--
		soft, err := e.baseTrail.hold(e.solver.manager.MkOrArgs(sorted[:i+1]))
		if err != nil {
			return err
		}
		if err = q.Copy(&w, &weights[i]); err == nil {
			err = q.Sub(&w, &weight, &weight)
		}
		if err == nil {
			err = q.SetInt64(&step, int64(i))
		}
		if err == nil {
			err = q.Mul(&weight, &step, &step)
		}
		if err == nil {
			err = q.Add(&e.lower, &step, &e.lower)
		}
		if err != nil {
			return err
		}
		if err = e.solver.mutexVerbose(i+1, &weight); err != nil {
			return err
		}
		if err = newSoft.insert(soft, &weight); err != nil {
			return err
		}
		e.stats.MutexTerms++
		for i > 0 && e.cmp(&weights[i-1], &w) == 0 {
			i--
		}
		if err = q.Copy(&weight, &w); err != nil {
			return err
		}
	}
	e.stats.Mutexes++
	return e.ratErr
}
