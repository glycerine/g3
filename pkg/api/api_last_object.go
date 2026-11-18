package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Source: src/api/api_context.h:73 (ref<api::object> m_last_obj);
// src/api/api_context.cpp:105-119,238-246 (~context, reset_last_result,
// save_object); src/util/ref.h (operator=: inc_ref the new object, then
// dec_ref the old one).
//
// Every API call that returns an object (a solver, model, goal, vector,
// statistics, parameter set, tactic, ...) saves it in m_last_obj, which
// holds a reference until the next such call replaces it, a datatype
// constructor calls reset_last_result, or the context dies. So
// Z3_solver_dec_ref of the last object returned does not destroy it yet:
// its ASTs live on until the reference is released.
//
// The Go API mirrors this with apiLastObjectSlot. Close makes a Go handle
// unusable at once; when the context still holds the object as its last
// object, the destruction (and the AST releases it makes) waits for the
// slot's release. The replay layer's api::context
// (apiContextResults.saveObject/lastObject) does the same for its
// reference-counted wrappers.

// apiLastObject is an object m_last_obj can hold. lastRelease drops the
// slot's reference and destroys the object if its handle was closed.
type apiLastObject interface {
	lastRelease() error
}

// apiLastHold is the per-object state: held while the context's slot
// references the object; closed once its Go handle was closed.
type apiLastHold struct {
	held, closed bool
}

// close is the handle's Close: it reports whether destruction must run
// now (no slot reference), and makes the handle unusable either way.
func (h *apiLastHold) close() (destroyNow bool) {
	h.closed = true
	return !h.held
}

// release drops the slot reference; destroy runs if the handle is closed.
func (h *apiLastHold) release(destroy func() error) error {
	h.held = false
	if h.closed {
		return destroy()
	}
	return nil
}

// saveObject is api::context::save_object: m_last_obj = o (o may be nil,
// for a returned object whose lifetime does not reach the AST manager,
// such as statistics or parameter descriptions; the previous object is
// still released). hold is o's state, nil when o is nil.
func (c *smtContextState) saveObject(o apiLastObject, hold *apiLastHold) error {
	if c == nil {
		return ErrSMTContextOwner
	}
	if c.lastObjectByResults {
		return nil
	}
	if hold != nil {
		hold.held = true
	}
	previous := c.lastObject
	c.lastObject = o
	if previous != nil {
		return previous.lastRelease()
	}
	return nil
}

// saveHeld is save_object of a returned object o whose handle state is
// hold. On failure the slot still holds o, which dies on its release.
func (c *smtContextState) saveHeld(o apiLastObject, hold *apiLastHold) error {
	if err := c.saveObject(o, hold); err != nil {
		hold.closed = true
		return err
	}
	return nil
}

// saveStatistics is save_object of a returned Z3_stats_ref, which holds no
// AST: the previous last object is released.
func (c *smtContextState) saveStatistics(s *APIStatistics, err error) (*APIStatistics, error) {
	if err != nil {
		return nil, err
	}
	if err = c.saveObject(nil, nil); err != nil {
		return nil, errors.Join(err, s.Close())
	}
	return s, nil
}

// saveDescriptions is save_object of a returned Z3_param_descrs_ref (no
// AST).
func (c *smtContextState) saveDescriptions(d *ParameterDescriptions, err error) (*ParameterDescriptions, error) {
	if err != nil {
		return nil, err
	}
	if err = c.saveObject(nil, nil); err != nil {
		return nil, errors.Join(err, d.Close())
	}
	return d, nil
}

// saveTactic is RETURN_TACTIC's save_object of a new Z3_tactic_ref (a
// tactic can own solvers, e.g. qsat's smt kernels).
func (c *smtContextState) saveTactic(t *APITactic, err error) (*APITactic, error) {
	if err != nil {
		return nil, err
	}
	if err = c.saveHeld(t, &t.last); err != nil {
		return nil, err
	}
	return t, nil
}

// saveProbe is RETURN_PROBE's save_object of a new Z3_probe_ref (no AST).
func (c *smtContextState) saveProbe(p *APIProbe, err error) (*APIProbe, error) {
	if err != nil {
		return nil, err
	}
	if err = c.saveObject(nil, nil); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	return p, nil
}

// resetLastResult is api::context::reset_last_result (m_last_obj = 0).
func (c *smtContextState) resetLastResult() error {
	return c.saveObject(nil, nil)
}

// apiLastVector is the Z3_ast_vector_ref an API call such as
// Z3_solver_get_assertions or Z3_solver_get_unsat_core returns, when the
// Go API returns its elements as a slice: m_last_obj keeps the vector,
// and with it a reference to each element, until it is replaced. The
// elements are released front to back (~ast_ref_vector).
type apiLastVector struct {
	context *smtContextState
	values  []ast.ASTHandle
	hold    apiLastHold
}

func (v *apiLastVector) lastRelease() error {
	return v.hold.release(func() error {
		m, err := v.context.command.Manager()
		if err != nil {
			return err
		}
		for len(v.values) != 0 {
			if err := m.DecRef(v.values[0]); err != nil {
				return err
			}
			v.values = v.values[1:]
		}
		return nil
	})
}

// saveVector saves a returned vector of ASTs as the last object.
func (c *smtContextState) saveVector(values []ast.ASTHandle) error {
	if c == nil || c.command == nil {
		return ErrSMTContextOwner
	}
	m, err := c.command.Manager()
	if err != nil {
		return err
	}
	v := &apiLastVector{context: c, values: append([]ast.ASTHandle(nil), values...)}
	for i, h := range v.values {
		if err := m.IncRef(h); err != nil {
			v.values = v.values[:i]
			v.hold.closed = true
			return v.lastRelease()
		}
	}
	// The Go caller never holds the vector itself: it is released (and
	// its references dropped) when the slot lets go of it.
	v.hold.closed = true
	return c.saveObject(v, &v.hold)
}

// saveVectorRefs saves the vector of an API call that allocates and saves
// its vector before computing the elements (Z3_optimize_get_objectives,
// Z3_fixedpoint_get_rules, ...): the previous last object is released
// first, then compute runs and the vector takes its elements.
func (c *smtContextState) saveVectorRefs(compute func() ([]ast.ASTRef, error)) ([]ast.ASTRef, error) {
	if c == nil || c.command == nil {
		return nil, ErrSMTContextOwner
	}
	m, err := c.command.Manager()
	if err != nil {
		return nil, err
	}
	v := &apiLastVector{context: c}
	v.hold.closed = true
	if err = c.saveObject(v, &v.hold); err != nil {
		return nil, err
	}
	refs, err := compute()
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		h, err := m.RefGet(ref)
		if err == nil {
			err = m.IncRef(h)
		}
		if err != nil {
			for i := range refs {
				err = errors.Join(err, m.DelRef(&refs[i]))
			}
			return nil, err
		}
		v.values = append(v.values, h)
	}
	return refs, nil
}

// releaseLastObject is ~context's m_last_obj = nullptr.
func (c *smtContextState) releaseLastObject() error {
	return c.resetLastResult()
}
