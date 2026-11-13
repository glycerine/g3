package clp

// Source: src/muz/clp/clp_context.cpp clp::imp::query after mk_query and the
// default transformation: ground the query rule head and search from it.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *Engine) Query(query ast.ASTHandle) (status lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	// m_goals.reset() (a no-op when the context reset it with m_solver).
	if err := e.state.ResetGoals(); err != nil {
		return lbool.Undef, err
	}
	// expr_ref head(rv[0]->get_head(), m); ground(head);
	head, err := e.state.ground.ground(query)
	if err != nil {
		return lbool.Undef, err
	}
	owner := &clpRefOwner{manager: e.manager}
	owner.take(head)
	defer func() {
		resultErr = owner.finish(resultErr)
		if e.ownsState {
			// A standalone engine keeps no imp state between queries.
			resultErr = errors.Join(resultErr, e.state.ResetGoals(), e.state.resetGround())
		}
		if resultErr != nil {
			status = lbool.Undef
		}
	}()
	value, err := e.manager.RefGet(head)
	if err != nil {
		return lbool.Undef, err
	}
	// m_goals.push_back(to_app(head))
	goal, err := e.manager.NewRef(value)
	if err != nil {
		return lbool.Undef, err
	}
	e.state.goals = append(e.state.goals, goal)
	return e.search(e.depth, 0)
}
