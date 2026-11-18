package duality

// Source: duality_dl_interface.cpp, query background-axiom installation.

import (
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// AddBackground adds an axiom that Query asserts when it installs the
// theory; Backend.PrepareQuery asserts at the source's AssertAxiom point
// instead.
func (q *PreparedQuery) AddBackground(formula ast.ASTHandle) error {
	if q == nil || q.manager == nil || q.self != q || q.closing {
		return graph.ErrWrapperOwner
	}
	if q.run != nil {
		// The engine's run already holds the asserted theory.
		return graph.ErrWrapperOwner
	}
	return q.keepBackground(formula)
}

func (q *PreparedQuery) keepBackground(formula ast.ASTHandle) error {
	ref, err := q.manager.NewRef(formula)
	if err != nil {
		return err
	}
	q.background = append(q.background, ref)
	return nil
}

// startQuery is dl_interface::query up to the new duality_data: the
// published counterexample is swapped back into the old solver (the Go view
// is invalidated, while LearnFrom can still consume the old derivation),
// and a previous attempt's refused background-scope Pop is drained before
// either borrowed solver is used again.
func (e *QueryEngine) startQuery() error {
	if err := e.invalidatePublication(); err != nil {
		return err
	}
	return e.reap()
}

// invalidatePublication: every attempted query invalidates publication,
// including setup failures.
func (e *QueryEngine) invalidatePublication() error {
	e.status = lbool.Undef
	if e.active != nil {
		return e.active.engine.InvalidateCounterexampleView()
	}
	return nil
}

func (e *QueryEngine) ownsRun(run *queryRun) bool {
	for _, r := range e.runs {
		if r == run {
			return true
		}
	}
	return false
}

// assertAxiom is RPFP::AssertAxiom -> iZ3LogicSolver::assert_axiom: a large
// distinct is converted once, and the converted term is asserted in the
// interpolating solver (AssertInterpolationAxiom) and kept as the theory
// (get_axioms), while the raw axiom stays for printing. The interpolating
// solver is the borrowed native solver, in a scope the run owns; its
// auxiliary solver (Transformer::SubsetEq covering checks) never sees it.
func (e *QueryEngine) assertAxiom(run *queryRun, theory *[]*graph.Expr, axiom *graph.Expr) error {
	converted, changed, err := graph.InterpolationAxiom(e.context, axiom)
	if err != nil {
		return err
	}
	if changed {
		run.converted = append(run.converted, converted)
		axiom = converted
	}
	*theory = append(*theory, axiom)
	native := e.config.Native
	if len(run.scopes) == 0 {
		if err := run.pushScope(native); err != nil {
			return err
		}
	}
	h, err := axiom.Raw()
	if err != nil {
		return err
	}
	return native.Assert(h)
}
