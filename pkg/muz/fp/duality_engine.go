package fp

// Source: muz/duality/duality_dl_interface.cpp and dl_register_engine.cpp.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// dualityReportWriter is std::cout for the Duality StreamReporter: an
// insertion failure sets the stream's badbit and later events are no-ops,
// but the search goes on.
type dualityReportWriter struct {
	out io.Writer
	bad *bool
}

func (w dualityReportWriter) Write(body []byte) (int, error) {
	if !*w.bad {
		if _, err := w.out.Write(body); err != nil {
			*w.bad = true
		}
	}
	return len(body), nil
}

// SetDualityReportOutput selects the borrowed writer that stands for
// std::cout in Duality's report; nil restores os.Stdout.
func (e *PropositionalEngine) SetDualityReportOutput(out io.Writer) error {
	if err := e.open(); err != nil {
		return err
	}
	e.dualityReport = out
	return nil
}

func (e *PropositionalEngine) closeDuality() error {
	if e.duality != nil {
		if err := e.duality.Close(); err != nil {
			return err
		}
		e.duality = nil
	}
	if e.dualityCooperation != nil {
		if err := e.dualityCooperation.Close(); err != nil {
			return err
		}
		e.dualityCooperation = nil
	}
	return nil
}

func (e *PropositionalEngine) queryDuality(query ast.ASTHandle) (status lbool.LBool, result error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, view.Close()) }()
	quantifyArrays, err := view.Bool("xform.quantify_arrays", false)
	if err != nil {
		return lbool.Undef, err
	}
	if quantifyArrays {
		// Source context::query flushes before check_rules and backend
		// construction. A rejected rule still crosses that display boundary.
		if err := e.flushPendingRules(); err != nil {
			return lbool.Undef, err
		}
		if err := e.validateFragment(); err != nil {
			return lbool.Undef, err
		}
	}
	mbqi, err := view.Bool("duality.mbqi", true)
	if err != nil {
		return lbool.Undef, err
	}
	bound, err := view.UInt("duality.recursion_bound", ^uint32(0))
	if err != nil {
		return lbool.Undef, err
	}
	if e.duality != nil && e.dualityMBQI != mbqi {
		if err := e.closeDuality(); err != nil {
			return lbool.Undef, err
		}
	}
	if e.duality == nil {
		if e.dualityCooperation == nil {
			e.dualityCooperation, err = cooperate.NewCooperation(e.manager.Budget())
			if err != nil {
				return lbool.Undef, err
			}
		}
		services := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: e.dualityCooperation}
		e.duality, err = duality.NewBackend(e.manager, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals, ModelEvaluator: services}, mbqi)
		if err != nil {
			return lbool.Undef, err
		}
		e.dualityMBQI = mbqi
	}
	options, err := duality.ReadQueryOptions(e.params, e.globals, view.local)
	if err != nil {
		return lbool.Undef, err
	}
	options.Report, err = diagnostic.ProcessVerboseEnabled(1)
	if err != nil {
		return lbool.Undef, err
	}
	if options.Report {
		// IF_VERBOSE(1, rs->SetOption("report","1")) selects
		// CreateStdoutReporter: the events go to std::cout, not to
		// verbose_stream() (AUDIT.md FUZZ-F5).
		out := e.dualityReport
		if out == nil {
			out = os.Stdout
		}
		if err = e.duality.SetReportOutput(dualityReportWriter{out: out, bad: new(bool)}); err != nil {
			return lbool.Undef, err
		}
	} else if err = e.duality.SetReportOutput(nil); err != nil {
		return lbool.Undef, err
	}
	if err := e.duality.SetOptions(options); err != nil {
		return lbool.Undef, err
	}
	// dl_interface::query replaces _d before transforming the rules, so a
	// failure from here on leaves the next query no old solver to learn from.
	defer func() {
		if result != nil {
			e.duality.AbandonQuery()
		}
	}()
	var prepared *duality.PreparedQuery
	if quantifyArrays {
		prepared, err = e.prepareDualityTransformedQuery(query, bound)
		if err != nil {
			return lbool.Undef, err
		}
	} else {
		forms, _, bounds, err := e.RawRules()
		if err != nil {
			return lbool.Undef, err
		}
		defer func() { result = errors.Join(result, closeASTRefs(e.manager, forms)) }()
		handles := make([]ast.ASTHandle, len(forms))
		for i, ref := range forms {
			handles[i], err = e.manager.RefGet(ref)
			if err != nil {
				return lbool.Undef, err
			}
		}
		background, err := e.dualityBackground()
		if err != nil {
			return lbool.Undef, err
		}
		prepared, err = e.duality.PrepareQuery(handles, bounds, query, background, e.predicateOrder, bound)
		if err != nil {
			return lbool.Undef, err
		}
	}
	defer func() { result = errors.Join(result, prepared.Close()) }()
	return e.duality.Query(prepared)
}

// dualityBackground lists the context's assertions, m_background, that
// dl_interface::query asserts one by one (get_assertion(i)) as background
// axioms, true included. The conjunction get_background_assertion (the
// engine's background, which PDR and Spacer take) is not one of them.
func (e *PropositionalEngine) dualityBackground() ([]ast.ASTHandle, error) {
	axioms := make([]ast.ASTHandle, 0, len(e.assertions))
	for _, ref := range e.assertions {
		h, err := e.manager.RefGet(ref)
		if err != nil {
			return nil, err
		}
		axioms = append(axioms, h)
	}
	return axioms, nil
}
