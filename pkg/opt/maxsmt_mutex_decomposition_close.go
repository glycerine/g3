package opt

import "errors"

// Source: src/opt/maxsmt.cpp m_trail release after mutex preprocessing.

func (d *maxSMTMutexDecomposition) close() error {
	if d == nil {
		return nil
	}
	err := d.owner.close()
	if err == nil {
		for k, w := range d.weights {
			err = errors.Join(err, d.q.Del(w))
			delete(d.weights, k)
		}
		err = errors.Join(err, d.q.Del(&d.offset))
		d.weights, d.soft = nil, nil
	}
	return err
}
