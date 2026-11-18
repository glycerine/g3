package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::process_mutex.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (d *maxSMTMutexDecomposition) transform(mutex []ast.ASTHandle) (err error) {
	q := d.q
	active := make([]ast.ASTHandle, 0, len(mutex))
	for _, expression := range mutex {
		weight := d.weights[expression]
		if weight == nil {
			continue
		}
		sign, e := d.sign(weight)
		if e != nil {
			return e
		}
		if sign > 0 {
			active = append(active, expression)
		}
	}
	if len(active) <= 1 {
		return nil
	}
	if err = maxSMTSortMutex(q, active, d.weights); err != nil {
		return err
	}
	values := make([]numeral.Rational, len(active))
	for i := range values {
		values[i] = numeral.NewRational()
	}
	previous, delta, step := numeral.NewRational(), numeral.NewRational(), numeral.NewRational()
	defer func() {
		err = errors.Join(err, q.Del(&step), q.Del(&delta), q.Del(&previous))
		for i := range values {
			err = errors.Join(err, q.Del(&values[i]))
		}
	}()
	for i, expression := range active {
		weight := d.weights[expression]
		if err = q.Copy(&values[i], weight); err != nil {
			return err
		}
		delete(d.weights, expression)
		if err = q.Del(weight); err != nil {
			return err
		}
		if err := d.soft.Erase(expression); err != nil {
			return err
		}
	}
	for i := len(active) - 1; i >= 0; i-- {
		if err = q.Sub(&values[i], &previous, &delta); err == nil {
			err = q.Copy(&previous, &values[i])
		}
		if err != nil {
			return err
		}
		sign, e := d.sign(&delta)
		if e != nil {
			return e
		}
		if sign == 0 {
			continue
		}
		prefix, err := d.owner.hold(d.owner.manager.MkOrArgs(active[:i+1]))
		if err != nil {
			return err
		}
		if old := d.weights[prefix]; old != nil {
			if err = q.Add(old, &delta, old); err != nil {
				return err
			}
		} else {
			weight := numeral.NewRational()
			if err = q.Copy(&weight, &delta); err != nil {
				return err
			}
			d.weights[prefix] = &weight
			if err = d.soft.Insert(prefix, [2]ast.ASTHandle{}); err != nil {
				return err
			}
		}
		if err = q.SetInt64(&step, int64(i)); err == nil {
			err = q.Mul(&delta, &step, &step)
		}
		if err == nil {
			err = q.Add(&d.offset, &step, &d.offset)
		}
		if err != nil {
			return err
		}
		if diagnostic.VerboseProgress {
			text, e := q.String(&delta)
			if e != nil {
				return e
			}
			if err = mutexVerboseText(i+1, text); err != nil {
				return err
			}
		}
	}
	return nil
}
