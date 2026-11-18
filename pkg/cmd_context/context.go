package cmd_context

// Source: src/cmd_context/cmd_context.cpp retained solver, assertion,
// declaration and scope operations; cmd_context.h.

import (
	"errors"
	"fmt"
	satsolver "github.com/glycerine/g3/pkg/sat/sat_solver"
	"github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	muzfp "github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/escaped"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
	"github.com/glycerine/insdict"
)

type declaration struct {
	name   string
	handle ast.ASTHandle
	ref    ast.ASTRef
}
type assertion struct {
	expression, tag ast.ASTHandle
	ref, tagRef     ast.ASTRef
	displayText     string
}
type optimizationObjective struct {
	expression ast.ASTHandle
	ref        ast.ASTRef
	value      smtcore.SMTArithmeticObjectiveValue
	modelValue smtcore.SMTArithmeticObjectiveValue
	softTerms  []optimizationSoftTerm
	softValue  numeral.Rational
	// softUpper/softInterval: a MaxSMT objective displayed as
	// (interval get_lower(i) get_upper(i)), softValue being the lower end.
	softUpper    numeral.Rational
	softInterval bool
	// displayReady: display_assignment's bounds after an undecided search.
	displayReady               bool
	displayLower, displayUpper simplex.Infinitesimal
	softOffset                 numeral.Rational
	boundLower                 numeral.Rational
	boundUpper                 numeral.Rational
	id                         string
	maximize                   bool
	soft                       bool
	bitVector                  bool
	bitWidth                   uint32
	interval                   bool
	openBounds                 bool
	valueReady                 bool
	boundsReady                bool
	// verify* are a converted objective's m_adjust_value (from_fmls), used
	// with expression (m_term) by verify_model.
	verifyConverted bool
	verifyNegate    bool
	verifyOffset    numeral.Rational
}
type optimizationSoftTerm struct {
	expression ast.ASTHandle
	ref        ast.ASTRef
	weight     numeral.Rational
}
type scope struct{ functions, sorts, assertions, objectives, softTerms, macros, sortConstructors, psorts int }
type builtin struct {
	family ast.FamilyID
	kind   int32
}

// Context owns its AST manager, solver, declarations and original assertions.
// Parameter and model-evaluator services are borrowed unless constructed by
// NewStandaloneContext. The AST manager and solver initialize on first use,
// so set-logic and proof options take effect before their construction.
type Context struct {
	config           smtcore.SMTSolverConfig
	parameters       *smtcore.ContextParams
	params           uparams.ParamsRef
	paramsReady      bool
	manager          *ast.ASTManager
	debugExpressions map[string]ast.ASTRef
	// objectRefs is m_object_refs' dictionary layout over debugExpressions.
	objectRefs      *symbolDictionary
	borrowedManager bool
	fixedpoint      *muzfp.PropositionalEngine
	// fixedpointSoftTimeout is dl_context's m_fparams.m_timeout: the smt
	// timeout when install_dl_cmds created dl_context (the SMT-LIB front end
	// start), not when the first fixedpoint command runs.
	fixedpointSoftTimeout         uint32
	fixedpointSoftTimeoutCaptured bool
	// fixedpointQueryCmdParams is dl_query_cmd's parametric m_params, which
	// lives as long as the command table; fixedpointContextParams is the
	// datalog::context's m_params_ref, recreated with the context.
	fixedpointQueryCmdParams uparams.ParamsRef
	fixedpointContextParams  uparams.ParamsRef
	datalogDisplay           *base.DatalogContext
	solver                   solver.Solver
	tacticResult             *solver.SimpleCheckSatResult
	checkSatUsingCore        []ast.ASTHandle
	retainedSolverResult     solver.Solver
	logic                    string
	functions                map[string][]ast.ASTHandle
	sorts                    map[string]ast.ASTHandle
	// listDecl is pdecl_manager::m_list's instantiated List datatype
	// (init_list commits it when no logic is set); listRef holds it.
	listDecl         ast.ASTHandle
	listRef          ast.ASTRef
	sortArities      map[string]uint32
	sortConstructors map[string]sortConstructorDeclaration
	// sortInfos is pdecl_manager::m_sort2info (sortInfoOrder keeps its
	// insertion order for release); sortDefinitionInfos holds each
	// define-sort body's psort applications; psortRecording is non-nil
	// while such a body is parsed.
	sortInfos                map[ast.ASTHandle]*sortInfoRecord
	sortInfoOrder            []ast.ASTHandle
	sortDefinitionInfos      map[string][]pendingSortInfo
	psortRecording           *[]pendingSortInfo
	sortConstructorTrail     []string
	builtins                 map[string][]builtin
	functionTrail, sortTrail []declaration
	// dtUtilASTs is dt_eh's datatype::util m_asts, the references its
	// constructor, recognizer and accessor caches (dtUtilCached) hold
	// until m_dt_eh is reset.
	dtUtilASTs   []ast.ASTRef
	dtUtilCached map[ast.ASTHandle]uint8
	// optLive is m_opt != nullptr (get_opt() created it for an objective
	// or soft constraint); optScopedHard is its m_scoped_state.m_hard, the
	// hard constraints set_hard_constraints installs at check-sat, with
	// m_hard_lim in optScopedHardLim.
	optLive          bool
	optScopedHard    []ast.ASTRef
	optScopedHardLim []int
	// funcDecls and psortDecls are the m_func_decls and m_psort_decls
	// dictionary layouts; psortStack is m_psort_decls_stack.
	funcDecls, psortDecls          *symbolDictionary
	psortStack                     []string
	macroDecls                     *symbolDictionary // m_macros layout
	optimizationClausal            bool              // opt::context::m_is_clausal
	macros                         map[string][]macroDeclaration
	macroTrail                     []string
	userTactics                    *insdict.Dict[string, string]
	assertions                     []assertion
	objectives                     []optimizationObjective
	optimizationHardOverride       []ast.ASTHandle
	optimizationHardOverrideActive bool
	// optimizationUnconverted is set while an inconsistent normalization
	// left maximize/minimize objectives unconverted (from_fmls saw only false).
	optimizationUnconverted  bool
	optimizationPreprocessor tactic.ModelConverter
	// optimizationTerms are opt::context's m_hard_constraints and objective
	// terms from the last from_fmls; they live until the next from_fmls.
	optimizationTerms []ast.ASTRef
	// optimizationOptsmt is optsmt::m_objs, reset by import_scoped_state.
	optimizationOptsmt []ast.ASTRef
	// optimizationPurifier holds from_fmls's purify definitions (fresh
	// constants in the objectives, (= q t) in m_hard_constraints).
	optimizationPurifier *opt.ObjectivePurifier
	// optimizationSAT is opt::context::m_sat_solver; update_solver replaces
	// it, so it survives until the next SAT-eligible optimize.
	optimizationSAT *satsolver.Solver
	// optimizationTarget is the opt::context install_opt_cmds wired into
	// a temporary parser context; nil for an ordinary context.
	optimizationTarget *Context
	// retainedOptimizer is the last lex optimize()'s m_opt_solver.
	retainedOptimizer *opt.ArithmeticSolver
	// optimizationRunOptimizer is the optimizer runOptimize used during the
	// current check-sat; optimizationStatsResult/optimizationStatsOptimizer
	// pair the published opt check-sat result with it, so statistics
	// reach opt::context::collect_statistics while m_check_sat_result is
	// the opt context.
	optimizationRunOptimizer   *opt.ArithmeticSolver
	optimizationStatsResult    solver.CheckSatResult
	optimizationStatsOptimizer *opt.ArithmeticSolver
	// optimizationStatsPareto is the Pareto run's m_solver (the opt_solver
	// gia_pareto drives) when optimizationStatsResult came from a Pareto check.
	optimizationStatsPareto *smtcore.SMTSolver
	// optimizationRunUsedSAT records that update_solver made
	// optimizationSAT the last run's m_solver.
	optimizationRunUsedSAT bool
	// optimizationFailureDisplay holds display_assignment's bounds for an
	// optimize() that threw: what import_scoped_state, or internalize()
	// after normalize(), left in m_maxsmts and m_optsmt.
	optimizationFailureDisplay []opt.ObjectiveValue
	// optimizationFailureModel records that such an optimize() had set
	// m_model from a sat initial check of the pareto engine.
	optimizationFailureModel bool
	// optimizationFailureRawModel is the raw initial-check m_model of a
	// lex or box optimize() that threw later; optimizationFailureNormalized
	// says optimizationFailureDisplay is still in normalized terms.
	optimizationFailureRawModel   *model.ModelCore
	optimizationFailureNormalized bool
	// optimizationFrontend marks the context of the OPB/WCNF front ends,
	// which call opt::context::optimize() directly, objectives or not.
	optimizationFrontend bool
	// optimizedObjectives is opt::context::m_objectives.size(): the
	// objectives import_scoped_state copied for the last optimize (0 once
	// m_opt is dropped). optsmtReset records that pop ran m_optsmt.reset()
	// since then.
	optimizedObjectives int
	optsmtReset         bool
	// optimizationModelsRaw is set when runOptimize kept m_model and
	// m_box_models unfixed; optimizationRawResult is the check-sat result
	// published from them. Retrieval applies fix_model (get_model,
	// get_box_model) while that result is current.
	optimizationModelsRaw bool
	optimizationRawResult solver.CheckSatResult
	// optimizationSimplify and optimizationModelConverter are
	// opt::context::m_simplify and m_model_converter: simplify_fmls
	// replaces them on the next normalize.
	optimizationSimplify       tactic.Tactic
	optimizationModelConverter tactic.ModelConverter
	// preparedOptimizer is the backend init_solver creates before normalize.
	preparedOptimizer     *opt.ArithmeticSolver
	softObjectiveIndex    map[string]int
	softObjectiveTrail    []int
	optimizationBoxNext   int
	optimizationBoxActive bool
	optimizationBoxModels []*model.ModelCore
	// optimizationBoxRetired holds m_box_models after the execution that
	// produced them ended: opt::context resets them only when the next
	// execute_box starts (or with the context).
	optimizationBoxRetired []*model.ModelCore
	// optimizationOptsmtModels is optsmt::m_models; optsmt::reset never
	// clears it, so it lives with opt::context.
	optimizationOptsmtModels     opt.OptsmtModels
	optimizationBoxValues        []opt.ObjectiveValue
	optimizationBoxLabels        []usymbol.Symbol
	optimizationBoxReasonUnknown string
	// optimizationBoxDisplay is the display bounds of the last l_undef
	// box execution.
	optimizationBoxDisplay    []opt.ObjectiveValue
	optimizationException     string
	optimizationParetoActive  bool
	optimizationParetoBackend *smtcore.SMTSolver
	// optimizationParetoOptimizer is the pareto run's m_opt_solver/optsmt
	// side: the internalized arithmetic objectives, their theory variables
	// and m_lower_fmls bound atoms (its backend is optimizationParetoBackend).
	optimizationParetoOptimizer    *opt.ArithmeticSolver
	optimizationParetoPurification *paretoPurification
	// optimizationParetoInitialModel is opt::context::m_model from
	// optimize()'s initial check, held until execute_pareto's yield (or
	// clear_state) replaces it.
	optimizationParetoInitialModel *model.ModelCore
	// optimizationObjectiveRefs is opt::context::m_objective_refs: every
	// objective function declaration, kept until the context is reset.
	optimizationObjectiveRefs []ast.ASTRef
	// optimizationFilter is opt::context::m_fm, shared by its opt_solvers.
	optimizationFilter               *tactic.FilterModelConverter
	scopes                           []scope
	PrintSuccess, GlobalDeclarations bool
	InteractiveMode                  bool
	ProduceUnsatAssumptions          bool
	ProduceAssignments               bool
	NumeralAsReal                    bool
	ExitOnError                      bool
	RandomSeed                       uint32
	checked                          bool
	status                           lbool.LBool
	progressError                    error
	infoStatus                       lbool.LBool
	watch                            stopwatch.Stopwatch
	owned                            *contextServices
	closed                           bool
	initializationError              error
	regularOutput                    io.Writer
	shellOutput                      io.Writer
	shellDiagnostic                  io.Writer
	diagnosticOutput                 io.Writer
	regularOutputName                string
	diagnosticOutputName             string
	regularFile                      *os.File
	diagnosticFile                   *os.File
	mainContext                      bool
	familyTemplate                   *ast.ASTManager // see NewAuxiliaryContextFor
	// datalogFrontendManager: see UseDatalogFrontendManager.
	datalogFrontendManager bool
	// apiSolver: see UseAPISolverInitialization; solverPending: the
	// manager exists and the solver is not made yet.
	apiSolver, solverPending bool
	// noSolverFactory: m_solver_factory is null (the cmd_context of the C
	// API parsers and fixedpoint readers); it never makes a solver, and
	// assert/push/pop only update the context (if (m_solver) ...).
	noSolverFactory bool
	// interpolatingSMTSolver is set_interpolating_solver_factory(
	// mk_smt_solver_factory()), installed by the SMT-LIB front end.
	interpolatingSMTSolver bool
	diagnosticBound        bool
	diagnosticGeneration   uint64
	previousWarning        io.Writer
	previousVerbose        *utext.NumericTextStream
	diagnosticVerbose      *utext.NumericTextStream
	logicChecker           LogicChecker
}

func NewContext(config smtcore.SMTSolverConfig) (*Context, error) {
	if config.Params == nil || config.Globals == nil {
		return nil, uparams.ErrParamsOwner
	}
	c := &Context{config: config, mainContext: true, functions: make(map[string][]ast.ASTHandle), sorts: make(map[string]ast.ASTHandle), sortArities: make(map[string]uint32), sortConstructors: make(map[string]sortConstructorDeclaration), builtins: make(map[string][]builtin), macros: make(map[string][]macroDeclaration), userTactics: insdict.NewDict[string, string](), softObjectiveIndex: make(map[string]int), status: lbool.Undef}
	var err error
	c.params, err = config.Params.NewValue()
	if err != nil {
		return c, err
	}
	c.paramsReady = true
	if config.ModelEvaluator.Params != nil {
		if err = config.Params.Append(&c.params, config.ModelEvaluator.Params); err != nil {
			return c, err
		}
	}
	c.config.ModelEvaluator.Params = &c.params
	c.parameters, err = smtcore.NewContextParams(config.Params, config.Globals, config.Params.BudgetField)
	if err != nil {
		return c, err
	}
	state, err := c.parameters.State()
	if err != nil {
		return c, err
	}
	c.PrintSuccess = state.SMTLib2Compliant
	return c, nil
}

func (c *Context) valid() error {
	if c == nil || c.closed || c.parameters == nil {
		return fmt.Errorf("closed command context")
	}
	return nil
}

// SetRegularOutput borrows the command session's current output transport.
// Source: cmd_context.h set_regular_stream(std::ostream&). Existing solver
// strategies resolve this writer when printing, so a later parser session can
// change the destination without reconstructing the solver.
func (c *Context) SetRegularOutput(output io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if w, ok := output.(contextRegularOutput); ok && w.context == c {
		return nil
	}
	if c.regularFile != nil {
		if err := c.regularFile.Close(); err != nil {
			return err
		}
		c.regularFile = nil
	}
	c.regularOutput = output
	c.shellOutput = nil
	c.regularOutputName = "caller-owned"
	return nil
}

// SetShellRegularOutput supplies the shell's stdout transport. It can be a
// caller-provided writer while retaining the source shell's stdout identity.
func (c *Context) SetShellRegularOutput(output io.Writer) error {
	if w, ok := output.(contextRegularOutput); ok && w.context == c {
		return c.valid()
	}
	if err := c.SetRegularOutput(output); err != nil {
		return err
	}
	c.shellOutput = output
	c.regularOutputName = "stdout"
	return nil
}

// SetShellDiagnosticOutput supplies the shell's stderr transport without
// changing the context's current channel selection.
func (c *Context) SetShellDiagnosticOutput(output io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	c.shellDiagnostic = output
	if output != nil && (c.diagnosticOutputName == "" || c.diagnosticOutputName == "stderr") {
		c.diagnosticOutput = contextShellDiagnosticOutput{c}
		if c.mainContext {
			if err := c.bindDiagnosticGlobals(); err != nil {
				return errors.Join(err, c.unbindDiagnosticGlobals())
			}
		}
	}
	return nil
}

// SetDiagnosticOutput borrows the command session's diagnostic transport.
// Source: cmd_context.h set_diagnostic_stream(std::ostream&).
func (c *Context) SetDiagnosticOutput(output io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if err := c.unbindDiagnosticGlobals(); err != nil {
		return err
	}
	if c.diagnosticFile != nil {
		if err := c.diagnosticFile.Close(); err != nil {
			return err
		}
		c.diagnosticFile = nil
	}
	c.diagnosticOutput = output
	c.diagnosticOutputName = "caller-owned"
	return nil
}

// RegularOutput follows the current stream even when a command changes it.
func (c *Context) RegularOutput() io.Writer { return contextRegularOutput{c} }

// DiagnosticOutput follows the current diagnostic stream even when a command
// changes its destination.
func (c *Context) DiagnosticOutput() io.Writer { return contextDiagnosticOutput{c} }

func (c *Context) setOutputChannel(name string, regular bool) error {
	output, channelName, file := &c.diagnosticOutput, &c.diagnosticOutputName, &c.diagnosticFile
	defaultName := "stderr"
	if regular {
		output, channelName, file = &c.regularOutput, &c.regularOutputName, &c.regularFile
		defaultName = "stdout"
	}
	if *file != nil {
		if err := (*file).Close(); err != nil {
			return err
		}
		*file = nil
	}
	*output, *channelName = nil, defaultName
	if regular && c.shellOutput != nil {
		*output = c.shellOutput
	}
	switch name {
	case "stdout":
		if c.shellOutput != nil {
			if regular {
				*output = c.shellOutput
			} else {
				*output = contextShellOutput{c}
			}
		} else {
			*output = os.Stdout
		}
	case "stderr":
		if c.shellDiagnostic != nil {
			*output = contextShellDiagnosticOutput{c}
		} else {
			*output = os.Stderr
		}
	default:
		opened, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			if !regular {
				_ = c.unbindDiagnosticGlobals()
			}
			return fmt.Errorf("failed to set output stream '%s': %w", name, err)
		}
		*output, *file = opened, opened
	}
	*channelName = name
	if !regular && c.mainContext {
		if err := c.bindDiagnosticGlobals(); err != nil {
			return errors.Join(err, c.unbindDiagnosticGlobals())
		}
	}
	return nil
}

// bindDiagnosticGlobals implements cmd_context::set_diagnostic_stream for a
// main context. The formatter borrows the selected file or shell writer.
func (c *Context) bindDiagnosticGlobals() error {
	config := utext.NumericTextStreamConfig{Budget: c.config.Params.BudgetField}
	if file, ok := c.diagnosticOutput.(*os.File); ok {
		config.BorrowedFile = file
	} else {
		config.BorrowedWriter = c.diagnosticOutput
	}
	stream, err := utext.NewNumericTextStream(config)
	if err != nil {
		return err
	}
	warnings := diagnostic.BorrowProcessWarningService()
	if !c.diagnosticBound {
		c.previousWarning = warnings.WarningStream()
		c.previousVerbose, err = diagnostic.BorrowProcessVerboseStream()
		if err != nil {
			return errors.Join(err, stream.Close())
		}
	}
	c.diagnosticGeneration++
	if err = warnings.SetWarningStream(contextWarningOutput{c, c.diagnosticGeneration}); err != nil {
		return errors.Join(err, stream.Close())
	}
	if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
		return errors.Join(err, stream.Close())
	}
	old := c.diagnosticVerbose
	c.diagnosticVerbose, c.diagnosticBound = stream, true
	if old != nil {
		return old.Close()
	}
	return nil
}

func (c *Context) unbindDiagnosticGlobals() error {
	if !c.diagnosticBound {
		return nil
	}
	warnings := diagnostic.BorrowProcessWarningService()
	if selected, ok := warnings.WarningStream().(contextWarningOutput); ok && selected.context == c && selected.generation == c.diagnosticGeneration {
		previous := c.previousWarning
		if prior, ok := previous.(contextWarningOutput); ok && (prior.context.closed || prior.generation != prior.context.diagnosticGeneration) {
			previous = nil
		}
		if err := warnings.SetWarningStream(previous); err != nil {
			return err
		}
	}
	selected, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if selected == c.diagnosticVerbose {
		previous := c.previousVerbose
		if previous.Closed {
			previous = diagnostic.BorrowDefaultProcessVerboseStream()
		}
		if err = diagnostic.SetProcessVerboseStream(previous); err != nil {
			return err
		}
	}
	if err = c.diagnosticVerbose.Close(); err != nil {
		return err
	}
	c.diagnosticVerbose, c.previousVerbose, c.previousWarning = nil, nil, nil
	c.diagnosticBound = false
	return nil
}

type contextRegularOutput struct{ context *Context }

type contextShellOutput struct{ context *Context }

type contextShellDiagnosticOutput struct{ context *Context }

func (w contextShellDiagnosticOutput) Write(data []byte) (int, error) {
	output := w.context.shellDiagnostic
	if output == nil {
		output = os.Stderr
	}
	return output.Write(data)
}

func (w contextShellDiagnosticOutput) Flush() error { return flushOutput(w.context.shellDiagnostic) }

func (w contextShellOutput) Write(data []byte) (int, error) {
	output := w.context.shellOutput
	if output == nil {
		output = os.Stdout
	}
	return output.Write(data)
}

func (w contextShellOutput) Flush() error { return flushOutput(w.context.shellOutput) }

func (w contextRegularOutput) Write(data []byte) (int, error) {
	output := w.context.regularOutput
	if output == nil {
		output = os.Stdout
	}
	return output.Write(data)
}

type contextDiagnosticOutput struct{ context *Context }

// Each selected ostream has a distinct sticky failure state, even if its
// enclosing context subsequently opens another diagnostic destination.
type contextWarningOutput struct {
	context    *Context
	generation uint64
}

func (w contextWarningOutput) Write(data []byte) (int, error) {
	return (contextDiagnosticOutput{w.context}).Write(data)
}

func (w contextWarningOutput) Flush() error { return flushOutput(w.context.diagnosticOutput) }

func flushOutput(output io.Writer) error {
	if flusher, ok := output.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func (w contextDiagnosticOutput) Write(data []byte) (int, error) {
	output := w.context.diagnosticOutput
	if output == nil {
		output = os.Stderr
	}
	return output.Write(data)
}

func (c *Context) Compliant() (bool, error) {
	if err := c.valid(); err != nil {
		return false, err
	}
	state, err := c.parameters.State()
	return state.SMTLib2Compliant, err
}

// GlobalParameters is borrowed by the source shell's command-line scanner.
func (c *Context) GlobalParameters() (*uparams.GlobalParamsService, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	return c.config.Globals, nil
}

// RefreshGlobalParameters is reached after source command-line assignments
// and before constructing the AST manager and solver.
func (c *Context) RefreshGlobalParameters() error {
	if err := c.valid(); err != nil {
		return err
	}
	if c.manager != nil {
		return fmt.Errorf("command context is already initialized")
	}
	if err := c.parameters.UpdateGlobal(); err != nil {
		return err
	}
	state, err := c.parameters.State()
	if err == nil {
		c.PrintSuccess = state.SMTLib2Compliant
	}
	return err
}

// ErrUnsupportedLogic marks the cmd_context::set_logic false return.
var ErrUnsupportedLogic = errors.New("unsupported logic")

func (c *Context) SetLogic(logic string) error {
	if err := c.valid(); err != nil {
		return err
	}
	if c.logic != "" {
		return fmt.Errorf("the logic has already been set")
	}
	if c.manager != nil && !c.borrowedManager {
		return fmt.Errorf("logic must be set before initialization")
	}
	if !solver.SupportedLogic(logic) {
		// cmd_context::set_logic returns false: the command prints unsupported.
		return fmt.Errorf("%w %s", ErrUnsupportedLogic, logic)
	}
	c.logic = logic
	if c.manager != nil {
		return c.logicChecker.SetLogic(c.manager, logic)
	}
	return nil
}

func (c *Context) NumeralsAsReal() bool { return c.NumeralAsReal || solver.LogicHasRealsOnly(c.logic) }

func (c *Context) SetBoolOption(name string, value bool) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	switch name {
	case "print_success":
		c.PrintSuccess = value
		return nil
	case "interactive_mode", "produce_assertions":
		if c.manager != nil {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		c.InteractiveMode = value
		return nil
	case "produce_unsat_assumptions":
		if c.manager != nil {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		c.ProduceUnsatAssumptions = value
		return nil
	case "produce_assignments":
		c.ProduceAssignments = value
		return nil
	case "numeral_as_real":
		c.NumeralAsReal = value
		return nil
	case "int_real_coercions":
		manager, err := c.Manager()
		if err != nil {
			return err
		}
		return manager.EnableIntRealCoercions(value)
	case "global_declarations", "global_decls":
		if c.manager != nil {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		c.GlobalDeclarations = value
		return nil
	case "produce_models":
		name = "model"
	case "produce_proofs":
		name = "proof"
	case "produce_interpolants":
		if c.manager != nil {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		name = "proof"
	case "produce_unsat_cores":
		name = "unsat_core"
	}
	if name == "proof" || name == "model" || name == "unsat_core" {
		if c.manager != nil && (name == "proof" || name == "unsat_core") {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		text := strconv.FormatBool(value)
		if err := c.parameters.Set(&name, &text); err != nil {
			return err
		}
	} else {
		if err := c.config.Globals.SourceSetName(name, strconv.FormatBool(value)); err != nil {
			return err
		}
		return c.globalParametersUpdated()
	}
	if err := c.config.Params.SetBoolName(&c.params, &name, value); err != nil {
		return err
	}
	if c.solver != nil {
		if err := c.solver.UpdateParams(&c.params); err != nil {
			return err
		}
		if name == "model" {
			return c.solver.SetProduceModels(value)
		}
	}
	return nil
}

// SetUIntOption follows the retained unsigned front-end and module option
// paths. The source parameter catalog validates module names and types.
func (c *Context) SetUIntOption(name string, value uint32) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if name == "random_seed" {
		c.RandomSeed = value
		return nil
	}
	if name == "verbosity" {
		return diagnostic.SetProcessVerbosityLevel(value)
	}
	if name == "reproducible_resource_limit" {
		name = "rlimit"
	}
	text := strconv.FormatUint(uint64(value), 10)
	if name == "timeout" || name == "rlimit" {
		if err := c.parameters.Set(&name, &text); err != nil {
			return err
		}
		if err := c.config.Params.SetUIntName(&c.params, &name, value); err != nil {
			return err
		}
	} else {
		if err := c.config.Globals.SourceSetName(name, text); err != nil {
			return err
		}
		// Preserve the catalog's type. An integer token can also supply a
		// Double parameter; a local UInt shadow would hide that typed value.
		return c.globalParametersUpdated()
	}
	if c.solver != nil {
		return c.solver.UpdateParams(&c.params)
	}
	return nil
}

// SetDoubleOption follows the catalog-typed global parameter path. Floating
// point parameters are solver tunables rather than SMT-LIB context options.
func (c *Context) SetDoubleOption(name string, value float64) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if err := c.config.Globals.SourceSetName(name, strconv.FormatFloat(value, 'g', -1, 64)); err != nil {
		return err
	}
	return c.globalParametersUpdated()
}

// SetSymbolOption follows set_option_cmd::set_symbol's generic parameter
// path. The source catalog validates the module/name and parses its type;
// keeping that typed global value avoids an incorrectly typed local shadow.
func (c *Context) SetSymbolOption(name, value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if name == "error_behavior" {
		switch value {
		case "immediate-exit":
			c.ExitOnError = true
		case "continued-execution":
			c.ExitOnError = false
		default:
			return fmt.Errorf("error setting :error-behavior, 'immediate-execution' or 'continued-execution' expected")
		}
		return nil
	}
	if err := c.config.Globals.SourceSetName(name, value); err != nil {
		return err
	}
	return c.globalParametersUpdated()
}

// SetStringOption follows the context-local string and output-channel paths.
func (c *Context) SetStringOption(name, value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if name == "regular_output_channel" || name == "diagnostic_output_channel" {
		return c.setOutputChannel(value, name == "regular_output_channel")
	}
	if name == "dot_proof_file" || name == "trace_file_name" {
		if err := c.parameters.Set(&name, &value); err != nil {
			return err
		}
		return c.config.Params.SetStringName(&c.params, &name, &value)
	}
	if err := c.config.Globals.SourceSetName(name, value); err != nil {
		return err
	}
	return c.globalParametersUpdated()
}

// UpdateParamValue is the context_params::set path used by the external API.
// Unlike SMT-LIB set-option, it updates only this context's construction
// parameters and leaves the process-global parameter service unchanged.
func (c *Context) UpdateParamValue(name, value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	return c.parameters.Set(&name, &value)
}

// Source cmd_context::global_params_updated. Generic option values retain
// their catalog types in gparams. Only the disabled context auto_config flag
// is copied into the solver's local update, as in the native command context.
func (c *Context) globalParametersUpdated() (result error) {
	global, err := c.config.Globals.Manager()
	if err != nil {
		return err
	}
	if err = uparams.UpdateProcessEnvParams(global); err != nil {
		return err
	}
	if err := c.parameters.UpdateGlobal(); err != nil {
		return err
	}
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	if state.SMTLib2Compliant {
		c.PrintSuccess = true
	}
	if c.fixedpoint != nil {
		if err := c.fixedpoint.ParametersUpdated(); err != nil {
			return err
		}
	}
	if c.solver == nil {
		return nil
	}
	params := c.config.Params.EmptyValue()
	defer func() { result = errors.Join(result, c.config.Params.Del(&params)) }()
	if !state.AutoConfig {
		name := "auto_config"
		if err := c.config.Params.SetBoolName(&params, &name, false); err != nil {
			return err
		}
	}
	return c.solver.UpdateParams(&params)
}

func (c *Context) Manager() (manager *ast.ASTManager, result error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	if c.initializationError != nil {
		return nil, c.initializationError
	}
	defer func() {
		if result != nil {
			c.initializationError = result
		}
	}()
	if c.manager != nil {
		return c.manager, nil
	}
	p := c.config.Params
	m, err := c.parameters.MkASTManager(ast.ASTManagerConfig{Symbols: p.SymbolsField, Numerals: p.NumeralsField, Budget: p.BudgetField})
	c.manager = m
	if err != nil {
		return nil, err
	}
	if err = c.registerManagerPlugins(m); err != nil {
		return nil, err
	}
	if err = c.initBuiltins(); err != nil {
		return nil, err
	}
	if c.mainContext && c.familyTemplate == nil && !c.datalogFrontendManager {
		if c.logic == "" {
			// insert(pm().mk_plist_decl()) runs init_list first.
			if err = c.initList(m); err != nil {
				return nil, err
			}
		}
		c.recordBuiltinPSortDecls()
	}
	// read_datalog has no cmd_context, so no solver is created for it.
	if c.apiSolver {
		c.solverPending = true
	} else if !c.datalogFrontendManager {
		if err = c.mkSolver(m); err != nil {
			return nil, err
		}
	}
	if err = c.logicChecker.SetLogic(m, c.logic); err != nil {
		return nil, err
	}
	return m, nil
}

// registerManagerPlugins fixes the manager's family ids. A main command
// context creates its manager in cmd_context::init_manager: the
// pdecl_manager constructor creates the datatype family, then
// init_manager_core registers its plugins in its own order (seq before pb,
// fpa and datalog_relation). Auxiliary contexts stand for the API, whose
// manager gets reg_decl_plugins and whose parser borrows it.
func (c *Context) registerManagerPlugins(m *ast.ASTManager) error {
	if c.familyTemplate != nil {
		if err := m.CopyFamiliesPlugins(c.familyTemplate); err != nil {
			return err
		}
		return m.RegisterDeclarationPlugins()
	}
	if !c.mainContext || c.datalogFrontendManager {
		return m.RegisterDeclarationPlugins()
	}
	if _, err := m.EnsureFamilyID("datatype"); err != nil {
		return err
	}
	for _, register := range [...]func(*ast.ASTManager) (ast.FamilyID, error){
		(*ast.ASTManager).RegisterArithmeticPlugin,
		(*ast.ASTManager).RegisterBitVectorPlugin,
		(*ast.ASTManager).RegisterArrayPlugin,
		(*ast.ASTManager).RegisterDatatypePlugin,
		(*ast.ASTManager).RegisterSequencePlugin,
		(*ast.ASTManager).RegisterPseudoBooleanPlugin,
		(*ast.ASTManager).RegisterFloatingPointPlugin,
		(*ast.ASTManager).RegisterDatalogPlugin,
	} {
		if _, err := register(m); err != nil {
			return err
		}
	}
	return nil
}

// UseDatalogFrontendManager makes the manager read_datalog's own: a bare
// ast_manager whose datalog::context constructor runs reg_decl_plugins, so
// datalog_relation gets family id 9 (before seq, fpa and pb) instead of the
// cmd_context order. The family id enters every finite sort's hash, and so
// the decl_set order of the output relations (AUDIT.md muz_rel_04-F1). It
// must run before the manager exists; afterwards it reports false.
func (c *Context) UseDatalogFrontendManager() bool {
	if c.manager != nil {
		return false
	}
	c.datalogFrontendManager = true
	return true
}

// UseAPISolverInitialization makes the context's solver the C API's
// (api::context has no cmd_context): Z3_mk_solver's init_solver runs when
// the solver is first used (assert, push, pop, check), not when the manager
// is made, so no solver ASTs are created for programs that use none (a
// fixedpoint). It must run before the manager exists; afterwards it
// reports false.
func (c *Context) UseAPISolverInitialization() bool {
	if c.manager != nil {
		return false
	}
	c.apiSolver = true
	return true
}

// ensureSolver is init_solver for a context of UseAPISolverInitialization.
func (c *Context) ensureSolver() error {
	if _, err := c.Manager(); err != nil {
		return err
	}
	if !c.solverPending || c.noSolverFactory {
		return nil
	}
	c.solverPending = false
	if err := c.mkSolver(c.manager); err != nil {
		c.initializationError = err
		return err
	}
	return nil
}

// mkSolver mirrors the shell factory step independently of manager creation.
// SetInterpolatingSMTSolverFactory is set_interpolating_solver_factory(
// mk_smt_solver_factory()), as read_smtlib2_commands installs it.
func (c *Context) SetInterpolatingSMTSolverFactory() { c.interpolatingSMTSolver = true }

func (c *Context) mkSolver(m *ast.ASTManager) error {
	var err error
	var proofs, models, cores bool
	if err = c.parameters.GetSolverParams(m, &c.params, &proofs, &models, &cores); err != nil {
		return err
	}
	// produce_interpolants() is a synonym of produce_proofs() in the source;
	// with an interpolating factory installed, proofs select a plain SMT
	// solver instead of the strategic one.
	if c.interpolatingSMTSolver {
		state, e := c.parameters.State()
		if e != nil {
			return e
		}
		if state.Proof {
			direct, e := smtcore.NewSMTSolver(m, &c.params, c.logic, c.config)
			if e != nil {
				return e
			}
			c.solver = direct
			// m_interpolating_solver_factory is mk_smt_solver_factory();
			// its smt_solver keeps the kernel's model generation.
			return c.solver.SetProduceModels(models)
		}
	}
	switch c.logic {
	case "QF_FD", "SAT":
		enabled, e := m.ProofsEnabled()
		if e != nil {
			return e
		}
		if !enabled {
			c.solver, err = portfolio.NewFDSolverWithOutput(m, &c.params, c.config.ModelEvaluator, contextRegularOutput{c})
		} else {
			var direct *smtcore.SMTSolver
			direct, err = smtcore.NewSMTSolver(m, &c.params, c.logic, c.config)
			if err == nil {
				c.solver = direct
			}
		}
	case "UFNIA":
		c.solver, err = portfolio.NewUFNIAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "UFLRA":
		c.solver, err = portfolio.NewUFLRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "AUFLIA":
		c.solver, err = portfolio.NewAUFLIAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "AUFLIRA":
		c.solver, err = portfolio.NewAUFLIRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "AUFNIRA":
		c.solver, err = portfolio.NewAUFNIRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_AUFLIA":
		c.solver, err = portfolio.NewQFAUFLIAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_LRA":
		c.solver, err = portfolio.NewQFLRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_NRA":
		c.solver, err = portfolio.NewQFNRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "NRA":
		c.solver, err = portfolio.NewNRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "LRA":
		c.solver, err = portfolio.NewLRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "LIA":
		c.solver, err = portfolio.NewLIAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_FP", "QF_FPBV", "QF_BVFP":
		c.solver, err = portfolio.NewQFFPStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.logic, c.config, contextRegularOutput{c})
	case "QF_NIA":
		c.solver, err = portfolio.NewQFNIAStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "QF_UF":
		c.solver, err = portfolio.NewQFUFStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_IDL":
		c.solver, err = portfolio.NewQFIDLStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "QF_LIA":
		c.solver, err = portfolio.NewQFLIAStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "QF_BV":
		c.solver, err = portfolio.NewQFBVStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "QF_AUFBV":
		c.solver, err = portfolio.NewQFAUFBVStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_ABV":
		c.solver, err = portfolio.NewQFABVStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_UFBV":
		c.solver, err = portfolio.NewQFUFBVStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "UFBV":
		c.solver, err = portfolio.NewUFBVStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "BV":
		c.solver, err = portfolio.NewBVStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "HORN":
		c.solver, err = portfolio.NewHornStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "ALL":
		c.solver, err = portfolio.NewALLStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "":
		// Source mk_tactic_for_logic falls through to mk_default_tactic, whose
		// arithmetic probes select QSAT for quantified LIRA goals. MBQI alone
		// does not terminate on those goals, in the source as well.
		c.solver, err = portfolio.NewALLStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	default:
		// mk_tactic_for_logic falls through to mk_default_tactic.
		c.solver, err = portfolio.NewDefaultStrategicSolver(m, &c.params, proofs, models, cores, c.logic, c.config)
	}
	if err != nil {
		return err
	}
	if err = c.solver.SetProduceModels(models); err != nil {
		return err
	}
	return nil
}

func (c *Context) DeclareSort(name string) (ast.ASTHandle, error) {
	return c.DeclareSortConstructor(name, 0)
}

// UnknownSortError is find_psort_decl failing; the parser adds its context.
type UnknownSortError struct{ Name string }

func (e *UnknownSortError) Error() string { return "unknown sort " + e.Name }

// SortDeclared is find_psort_decl(name) != nullptr: a user sort, a sort
// constructor, or a builtin sort installed for the current logic.
func (c *Context) SortDeclared(name string) bool {
	if _, found := c.sortConstructors[name]; found {
		return true
	}
	if _, found := c.sorts[name]; found {
		return true
	}
	return c.BuiltinSortAvailable(name)
}

// LookupSort is find_psort_decl(name)->instantiate(pm()) for a sort name:
// the instantiation also saves the sort's printing info.
func (c *Context) LookupSort(name string) (ast.ASTHandle, error) {
	r, err := c.lookupSort(name)
	if err != nil {
		return r, err
	}
	if _, alias := c.sortDefinitionInfos[name]; alias {
		return r, c.instantiateSortDefinitionInfos(name, nil, nil)
	}
	return r, c.saveSortInfo(r, name, nil)
}

func (c *Context) lookupSort(name string) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if declaration, found := c.sortConstructors[name]; found {
		return ast.ASTHandle{}, fmt.Errorf("sort constructor expects %d parameters", declaration.arity)
	}
	if s, found := c.sorts[name]; found {
		if arity := c.sortArities[name]; arity != 0 {
			return ast.ASTHandle{}, fmt.Errorf("sort constructor expects %d parameters", arity)
		}
		return s, nil
	}
	if !c.BuiltinSortAvailable(name) {
		return ast.ASTHandle{}, &UnknownSortError{Name: name}
	}
	if name == "FloatingPoint" {
		return ast.ASTHandle{}, fmt.Errorf("invalid floating-point sort, expecting two parameters")
	}
	for i, alias := range []string{"RoundingMode", "Float16", "Float32", "Float64", "Float128"} {
		if name == alias {
			family, err := m.RegisterFloatingPointPlugin()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkFamilySort(family, int32(i+1), nil)
		}
	}
	if name == "Seq" {
		return ast.ASTHandle{}, fmt.Errorf("Invalid sequence sort, expecting one parameter")
	}
	if name == "RegEx" {
		return ast.ASTHandle{}, fmt.Errorf("Invalid regex sort, expecting one parameter")
	}
	if name == "Bool" {
		return m.MkBoolSort()
	}
	if name == "String" || name == "StringSequence" {
		family, err := m.SequenceFamilyID()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return m.MkFamilySort(family, 2, nil)
	}
	if name == "Int" || name == "Real" {
		family, err := m.ArithmeticFamilyID()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		kind := int32(0)
		if name == "Int" {
			kind = 1
		}
		return m.MkFamilySort(family, kind, nil)
	}
	return ast.ASTHandle{}, fmt.Errorf("unknown sort %s", name)
}

func (c *Context) DeclareFunction(name string, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := m.MkFuncDecl(symbol, domain, rangeSort, ast.ASTFuncDeclInfo{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.publishFunctionDeclaration(name, d)
}

func (c *Context) publishFunctionDeclaration(name string, d ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arity, err := m.GetArity(d)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		domain[i], err = m.GetDomain(d, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	ref, err := m.NewRef(d)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, m.DelRef(&ref))
		}
	}()
	if err = c.logicChecker.CheckDeclaration(d); err != nil {
		return ast.ASTHandle{}, err
	}
	if c.containsMacro(name, domain) {
		return ast.ASTHandle{}, fmt.Errorf("invalid declaration, named expression already defined with this name %s", name)
	}
	if _, found := c.builtins[name]; found {
		return ast.ASTHandle{}, fmt.Errorf("invalid declaration, builtin symbol %s", name)
	}
	for _, prior := range c.functions[name] {
		if prior == d {
			return ast.ASTHandle{}, fmt.Errorf("invalid declaration, %s '%s' (with the given signature) already declared", constantOrFunction(len(domain)), name)
		}
	}
	c.functions[name] = append(c.functions[name], d)
	c.functionTrail = append(c.functionTrail, declaration{name, d, ref})
	published = true
	if err = c.recordFuncDecl(name, d); err != nil {
		return ast.ASTHandle{}, err
	}
	return d, nil
}

func (c *Context) Assert(formula ast.ASTHandle, name string) (result error) {
	return c.AssertWithText(formula, name, "")
}

func (c *Context) AssertWithText(formula ast.ASTHandle, name, displayText string) (result error) {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	if err = c.ensureSolver(); err != nil {
		return err
	}
	if err = c.logicChecker.CheckExpression(formula); err != nil {
		return err
	}
	if err = c.invalidateOptimizationState(); err != nil {
		return err
	}
	ref, err := m.NewRef(formula)
	if err != nil {
		return err
	}
	row := assertion{expression: formula, ref: ref, displayText: displayText}
	state, err := c.parameters.State()
	if err == nil && name != "" && state.UnsatCore {
		symbol, e := m.Symbols.Intern(name)
		err = e
		if err == nil {
			b, e := m.MkBoolSort()
			err = e
			if err == nil {
				row.tag, err = m.MkConst(symbol, b)
			}
		}
		if err == nil {
			row.tagRef, err = m.NewRef(row.tag)
		}
	}
	if err == nil && c.solver != nil {
		if row.tag.IsNull() {
			err = c.solver.Assert(formula)
		} else {
			err = c.solver.AssertTracked(formula, row.tag)
		}
	}
	if err != nil {
		if row.tagRef != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&row.tagRef))
		}
		return errors.Join(err, m.DelRef(&row.ref))
	}
	c.assertions = append(c.assertions, row)
	c.checked = false
	return nil
}

func (c *Context) Push(count uint32) error {
	if count == 0 {
		return c.valid()
	}
	c.checked = false
	if err := c.clearCheckSatResult(); err != nil {
		return err
	}
	if err := c.ensureSolver(); err != nil {
		return err
	}
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	for ; count > 0; count-- {
		// m().limit().push(m_params.rlimit()) precedes m_solver->push().
		if err := c.manager.ResourceLimit().Push(state.RLimit); err != nil {
			return err
		}
		if c.solver != nil {
			if err := c.solver.Push(); err != nil {
				return err
			}
		}
		if c.optLive {
			// m_opt->push(): scoped_state::push records m_hard's size.
			c.optScopedHardLim = append(c.optScopedHardLim, len(c.optScopedHard))
		}
		c.scopes = append(c.scopes, scope{len(c.functionTrail), len(c.sortTrail), len(c.assertions), len(c.objectives), len(c.softObjectiveTrail), len(c.macroTrail), len(c.sortConstructorTrail), len(c.psortStack)})
	}
	c.checked = false
	return nil
}

func (c *Context) Pop(count uint32) error {
	if err := c.valid(); err != nil {
		return err
	}
	c.checked = false
	if err := c.clearCheckSatResult(); err != nil {
		return err
	}
	if uint64(count) > uint64(len(c.scopes)) {
		return fmt.Errorf("invalid pop command, argument is greater than the current stack depth")
	}
	if count == 0 {
		return c.invalidateOptimizationState()
	}
	if err := c.ensureSolver(); err != nil {
		return err
	}
	if c.solver != nil {
		if err := c.solver.Pop(count); err != nil {
			return err
		}
	}
	level := len(c.scopes) - int(count)
	s := c.scopes[level]
	if c.optLive {
		// m_opt->pop(n): scoped_state::pop n times resizes m_hard to its
		// limit and pops the objective term trail and the objectives.
		limit := c.optScopedHardLim[len(c.optScopedHardLim)-int(count)]
		c.optScopedHardLim = c.optScopedHardLim[:len(c.optScopedHardLim)-int(count)]
		if err := c.shrinkOptScopedHard(limit); err != nil {
			return err
		}
		if err := c.restoreObjectives(s.objectives, s.softTerms); err != nil {
			return err
		}
	}
	// m_opt->pop(n): clear_state (set_pareto(nullptr), m_model.reset()),
	// reset_maxsmts, m_optsmt.reset() and m_hard_constraints.reset().
	c.optsmtReset = true
	c.finishParetoSearch()
	if err := c.releaseParetoInitialModel(); err != nil {
		return err
	}
	if err := c.releaseParetoPurification(); err != nil {
		return err
	}
	if err := c.retainedOptimizer.ResetMaxSMTs(); err != nil {
		return err
	}
	if c.optimizationParetoOptimizer != nil {
		c.optimizationOptsmt = append(c.optimizationOptsmt, c.optimizationParetoOptimizer.DetachObjectiveTerms()...)
	}
	if err := c.releaseOptimizationHard(); err != nil {
		return err
	}
	if err := c.invalidateOptimizationState(); err != nil {
		return err
	}
	if !c.GlobalDeclarations {
		c.restorePSortDecls(s.psorts)
		if err := c.restoreMacros(s.macros); err != nil {
			return err
		}
		if err := c.restoreDeclarations(s.functions, s.sorts); err != nil {
			return err
		}
		if err := c.restoreSortConstructors(s.sortConstructors); err != nil {
			return err
		}
	}
	if err := c.restoreAssertions(s.assertions); err != nil {
		return err
	}
	if err := c.restoreObjectives(s.objectives, s.softTerms); err != nil {
		return err
	}
	c.scopes = c.scopes[:level]
	for ; count > 0; count-- {
		c.manager.ResourceLimit().Pop()
	}
	return nil
}

func (c *Context) restoreDeclarations(functions, sorts int) error {
	// restore_func_decls erases the m_func_decls_stack suffix forward.
	if functions < len(c.functionTrail) {
		for _, row := range c.functionTrail[functions:] {
			if err := c.eraseFuncDecl(row.name, row.handle); err != nil {
				return err
			}
		}
	}
	for len(c.functionTrail) > functions {
		i := len(c.functionTrail) - 1
		row := &c.functionTrail[i]
		values := c.functions[row.name]
		if len(values) == 1 {
			delete(c.functions, row.name)
		} else {
			c.functions[row.name] = values[:len(values)-1]
		}
		if row.ref != (ast.ASTRef{}) {
			if err := c.manager.DelRef(&row.ref); err != nil {
				return err
			}
		}
		c.functionTrail = c.functionTrail[:i]
	}
	for len(c.sortTrail) > sorts {
		i := len(c.sortTrail) - 1
		row := &c.sortTrail[i]
		delete(c.sorts, row.name)
		delete(c.sortArities, row.name)
		if err := c.releaseSortDefinitionInfos(row.name); err != nil {
			return err
		}
		if row.ref != (ast.ASTRef{}) {
			if err := c.manager.DelRef(&row.ref); err != nil {
				return err
			}
		}
		c.sortTrail = c.sortTrail[:i]
	}
	return nil
}

// restoreAssertions is cmd_context::restore_assertions: restore() releases
// m_assertions[old_sz..] forward, and then m_assertion_names the same way.
func (c *Context) restoreAssertions(size int) error {
	if len(c.assertions) <= size {
		return nil
	}
	for i := size; i < len(c.assertions); i++ {
		if err := c.manager.DelRef(&c.assertions[i].ref); err != nil {
			return err
		}
	}
	for i := size; i < len(c.assertions); i++ {
		row := &c.assertions[i]
		if row.tagRef != (ast.ASTRef{}) {
			if err := c.manager.DelRef(&row.tagRef); err != nil {
				return err
			}
		}
	}
	c.assertions = c.assertions[:size]
	return nil
}

func (c *Context) CheckSat(assumptions []ast.ASTHandle) (status lbool.LBool, err error) {
	// IF_VERBOSE(100, verbose_stream() << "(started \"check-sat\")" << std::endl;)
	// before init_manager (AUDIT.md FUZZ-F5; g3trace builds only).
	if diagnostic.VerboseProgress {
		if verr := diagnostic.ProgressVerbose(100, "(started \"check-sat\")\n"); verr != nil {
			return lbool.Undef, verr
		}
	}
	if err := c.ensureSolver(); err != nil {
		return lbool.Undef, err
	}
	watch, err := stopwatch.NewScopedWatch(&c.watch, true)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, watch.Close()) }()
	optimizationContinuation := c.checked
	c.checked = false
	c.progressError = nil
	if err := c.clearCheckSatResult(); err != nil {
		return lbool.Undef, err
	}
	// opt_frontend calls opt::context::optimize() whatever the objectives.
	if len(c.objectives) != 0 || c.optimizationFrontend {
		// if (!m_processing_pareto) get_opt()->set_hard_constraints(cnstr)
		if c.optLive && !(c.optimizationParetoActive && c.optimizationParetoBackend != nil) {
			if err := c.setOptHardConstraints(assumptions); err != nil {
				return lbool.Undef, err
			}
		}
		status, err = c.checkOptimize(assumptions, optimizationContinuation)
		if err != nil {
			return lbool.Undef, err
		}
	} else {
		if err = c.retireOptimizationBoxModels(); err != nil {
			return lbool.Undef, err
		}
		c.progressError = nil
		if err = c.solver.SetProgressCallback(c); err != nil {
			return lbool.Undef, err
		}
		execution, scopeErr := c.beginCheckExecution()
		if scopeErr != nil {
			return lbool.Undef, scopeErr
		}
		status, err = c.solver.CheckSat(assumptions)
		if closeErr := execution.Close(); closeErr != nil {
			return lbool.Undef, errors.Join(err, closeErr)
		}
		if err != nil {
			var fatal z3exception.Z3Error
			var source z3exception.Z3Exception
			if errors.As(err, &fatal) || !errors.As(err, &source) {
				return lbool.Undef, err
			}
			// Native cmd_context converts nonfatal solver exceptions into an
			// unknown result owned by the installed solver. In particular, this
			// prevents combined_solver from treating a malformed model value as
			// an ordinary solver-2 unknown and falling back to solver 1.
			status = lbool.Undef
			if setErr := c.solver.SetReasonUnknown(source.Message()); setErr != nil {
				return lbool.Undef, errors.Join(err, setErr)
			}
			if setErr := c.solver.SetStatus(status); setErr != nil {
				return lbool.Undef, errors.Join(err, setErr)
			}
			err = nil
		}
		c.status, c.checked = status, true
		if c.progressError != nil {
			return status, c.progressError
		}
	}
	if status == lbool.True {
		if err = c.completeModel(); err != nil {
			return status, err
		}
		if err = c.validateModel(); err != nil {
			return status, err
		}
	}
	return status, err
}

// checkExecutionScope is the cancel_eh<reslimit>, scoped_timer and
// scoped_rlimit that cmd_context::check_sat and get_consequences put around
// the solver call, built from m_params.m_timeout and m_params.rlimit().
type checkExecutionScope struct {
	handler    *contextTimeoutHandler
	timer      *control.ScopedTimer
	limitScope control.LimitScope
}

func (c *Context) beginCheckExecution() (*checkExecutionScope, error) {
	state, err := c.parameters.State()
	if err != nil {
		return nil, err
	}
	limit := c.manager.ResourceLimit()
	handler := &contextTimeoutHandler{limit: limit, active: true}
	timer, err := control.NewScopedTimer(state.Timeout, handler)
	if err != nil {
		handler.close()
		return nil, err
	}
	limitScope, err := control.NewLimitScope(limit, state.RLimit)
	if err != nil {
		err = errors.Join(err, timer.Close())
		handler.close()
		return nil, err
	}
	return &checkExecutionScope{handler: handler, timer: timer, limitScope: limitScope}, nil
}

// Close runs the destructors in reverse construction order.
func (s *checkExecutionScope) Close() error {
	s.limitScope.Close()
	err := s.timer.Close()
	s.handler.close()
	return err
}

func (c *Context) Model() (*model.ModelCore, error) {
	state, err := c.parameters.State()
	if err != nil {
		return nil, err
	}
	if !c.checked || c.status != lbool.True && c.status != lbool.Undef || !state.Model {
		return nil, fmt.Errorf("model is not available")
	}
	value, err := c.checkSatResult().AcquireModel()
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("model is not available")
	}
	return c.fixRetrievedModel(value)
}

// fixRetrievedModel is opt::context::get_model/get_box_model: mdl = the
// stored model; fix_model(mdl). It consumes md's reference.
func (c *Context) fixRetrievedModel(md *model.ModelCore) (*model.ModelCore, error) {
	if md == nil || c.optimizationRawResult == nil || c.retainedOptimizer == nil || c.checkSatResult() != c.optimizationRawResult {
		return md, nil
	}
	fixed, err := c.retainedOptimizer.FixModel(md)
	if err != nil {
		return nil, errors.Join(err, md.DecRef())
	}
	if err = md.DecRef(); err != nil {
		return nil, errors.Join(err, fixed.DecRef())
	}
	return fixed, nil
}

func (c *Context) WriteModel(out io.Writer) (result error) {
	return c.WriteModelAtModelIndex(out, 0)
}

// WriteModelAtModelIndex matches get-model's optional box-objective index.
// Without an optimizer, the source command falls back to the current model.
func (c *Context) WriteModelAtModelIndex(out io.Writer, modelIndex uint32) (result error) {
	m, err := c.modelAtIndex(modelIndex, true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DecRef()) }()
	return c.displayModel(out, m)
}

// displayModel is cmd_context::display_model.
func (c *Context) displayModel(out io.Writer, m *model.ModelCore) (result error) {
	if m == nil {
		return nil
	}
	mp, err := model.NewModelParams(c.config.Params, c.config.Globals, nil)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, mp.Close()) }()
	v1, err := mp.V1()
	if err != nil {
		return err
	}
	v2, err := mp.V2()
	if err != nil {
		return err
	}
	if v1 || v2 {
		partial, err := mp.Partial()
		if err != nil {
			return err
		}
		var buffer strings.Builder
		if err = m.WriteV2(&buffer, c.config.Params, c.config.Globals, partial); err != nil {
			return err
		}
		text := buffer.String()
		if _, err = io.WriteString(out, "\""); err != nil {
			return err
		}
		if err = escaped.Write(out, &text, true, 0); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\"\n")
		return err
	}
	if _, err = io.WriteString(out, "(model \n"); err != nil {
		return err
	}
	// model_smt2_pp(out, *this, *md, 2) prints with cmd_context's pp_env.
	if err = c.withPPEnv(func() error { return m.WriteSMT2(out, c.config.Params, c.config.Globals, 2) }); err != nil {
		return err
	}
	_, err = io.WriteString(out, ")\n")
	return err
}

// DumpModels is the tail of cmd_context::check_sat:
// if (r == l_true && m_params.m_dump_models) { get_check_sat_result()->
// get_model(md); display_model(md); }. For opt::context, get_model is
// mdl = m_model; fix_model(mdl).
func (c *Context) DumpModels(out io.Writer, status lbool.LBool) (result error) {
	state, err := c.parameters.State()
	if err != nil || status != lbool.True || !state.DumpModels {
		return err
	}
	md, err := c.checkSatResult().AcquireModel()
	if err != nil || md == nil {
		return err
	}
	if md, err = c.fixRetrievedModel(md); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, md.DecRef()) }()
	return c.displayModel(out, md)
}

func (c *Context) modelAtIndex(modelIndex uint32, fallbackWithoutOptimizer bool) (*model.ModelCore, error) {
	// Source commands first require is_model_available().
	available, err := c.IsModelAvailable()
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, fmt.Errorf("model is not available")
	}
	if modelIndex == 0 || len(c.objectives) == 0 && fallbackWithoutOptimizer {
		return c.Model()
	}
	if uint64(modelIndex) >= uint64(len(c.optimizationBoxModels)) {
		return nil, fmt.Errorf("index into models is out of bounds")
	}
	selected := c.optimizationBoxModels[modelIndex]
	if selected == nil {
		return nil, fmt.Errorf("model is not available")
	}
	if err = selected.IncRef(); err != nil {
		return nil, err
	}
	return c.fixRetrievedModel(selected)
}

// ResetAssertions follows cmd_context::reset_assertions. With global
// declarations disabled, source performs a complete owning-session reset.
// Otherwise the declaration world and scope stack survive solver recreation.
func (c *Context) ResetAssertions() error {
	if err := c.valid(); err != nil {
		return err
	}
	if !c.GlobalDeclarations {
		return c.Reset()
	}
	// Source m_check_sat_result owns its previous solver independently of the
	// replacement m_solver. Keep that result live until normal invalidation.
	if !c.checked {
		if err := c.clearCheckSatResult(); err != nil {
			return err
		}
	}
	if c.solver != nil {
		if c.checked && c.tacticResult == nil && c.retainedSolverResult == nil {
			c.retainedSolverResult = c.solver
		} else if err := c.solver.Close(); err != nil {
			return err
		}
		c.solver = nil
		if err := c.mkSolver(c.manager); err != nil {
			c.initializationError = err
			return err
		}
	}
	if err := c.restoreAssertions(0); err != nil {
		return err
	}
	if err := c.releaseOptScopedState(); err != nil {
		return err
	}
	// m_opt = nullptr (AUDIT.md FUZZ_A-F13).
	c.optimizedObjectives, c.optsmtReset = 0, false
	c.optimizationClausal = false
	for i := range c.scopes {
		c.scopes[i].assertions = 0
		c.scopes[i].objectives = 0
		c.scopes[i].softTerms = 0
		if c.solver != nil {
			if err := c.solver.Push(); err != nil {
				c.initializationError = err
				return err
			}
		}
	}
	return nil
}

// Reset follows cmd_context::reset(false) for this owning command context.
// Session options, output and parameter services survive; the AST manager is
// rebuilt lazily after all old-manager consumers have been released.
func (c *Context) Reset() error {
	if err := c.valid(); err != nil {
		return err
	}
	if !c.borrowedManager {
		return c.resetManagerState()
	}
	manager := c.manager
	if err := c.resetManagerState(); err != nil {
		return err
	}
	c.manager = manager
	if err := c.initBuiltins(); err != nil {
		return err
	}
	return c.mkSolver(manager)
}

// Also used by Close, including partial context construction where valid()
// cannot succeed yet. Parameter services belong to the enclosing context.
func (c *Context) resetManagerState() error {
	var construction *portfolio.FDConstructionFailure
	if errors.As(c.initializationError, &construction) && construction.CleanupPending() {
		return construction
	}
	if err := c.clearCheckSatResult(); err != nil {
		return err
	}
	if c.fixedpoint != nil {
		if err := c.fixedpoint.Close(); err != nil {
			return err
		}
		c.fixedpoint = nil
	}
	if c.fixedpointContextParams.Slot != 0 {
		if err := c.config.Globals.Context.Del(&c.fixedpointContextParams); err != nil {
			return err
		}
	}
	if c.datalogDisplay != nil {
		if _, err := c.manager.ProofMode(); err != nil {
			return err
		}
		if err := c.datalogDisplay.Close(); err != nil {
			return err
		}
		c.datalogDisplay = nil
	}
	if c.manager != nil {
		// cmd_context::reset order: reset_object_refs, reset_psort_decls,
		// reset_macros, reset_func_decls, restore_assertions(0), and only
		// then m_solver = nullptr; the pdecl_manager (sort infos) goes last.
		if err := c.clearDebugExpressions(); err != nil {
			return err
		}
		c.logicChecker.Reset()
		if err := c.resetPSortDecls(); err != nil {
			return err
		}
		if err := c.resetMacros(); err != nil {
			return err
		}
		if err := c.resetFuncDecls(); err != nil {
			return err
		}
		if err := c.restoreAssertions(0); err != nil {
			return err
		}
	}
	if c.solver != nil {
		if err := c.solver.Close(); err != nil {
			return err
		}
		c.solver = nil
	}
	// m_opt = nullptr: the opt::context goes after m_solver. ~context
	// destroys its members in reverse declaration order: m_simplify,
	// m_objective_refs, m_objectives, m_scoped_state, m_optsmt,
	// m_box_models, m_pareto, m_sat_solver and then m_opt_solver.
	// ~context's body runs reset_maxsmts() before the members die.
	if err := c.retainedOptimizer.ResetMaxSMTs(); err != nil {
		return err
	}
	if err := c.releaseOptimizationSimplifier(); err != nil {
		return err
	}
	for i := range c.optimizationObjectiveRefs {
		if err := c.manager.DelRef(&c.optimizationObjectiveRefs[i]); err != nil {
			return err
		}
	}
	c.optimizationObjectiveRefs = nil
	// m_fm is declared after m_model_converter and m_objectives.
	if c.optimizationFilter != nil {
		if err := c.optimizationFilter.Close(); err != nil {
			return err
		}
		c.optimizationFilter = nil
	}
	// m_model_converter is declared after m_objectives, so it dies first.
	if state := c.optimizationParetoPurification; state != nil && state.converter != nil {
		if err := state.converter.DecRef(); err != nil {
			return err
		}
		state.converter = nil
	}
	if c.retainedOptimizer != nil {
		if err := c.retainedOptimizer.ReleaseModelConverter(); err != nil {
			return err
		}
	}
	if err := c.releaseOptimizationObjectiveTerms(); err != nil {
		return err
	}
	if c.manager != nil {
		if err := c.releaseOptScopedState(); err != nil {
			return err
		}
	}
	for i := range c.optimizationOptsmt {
		if err := c.manager.DelRef(&c.optimizationOptsmt[i]); err != nil {
			return err
		}
	}
	c.optimizationOptsmt = nil
	if err := c.optimizationOptsmtModels.Close(); err != nil {
		return err
	}
	if err := c.clearOptimizationBoxModels(); err != nil {
		return err
	}
	if err := c.releaseRetiredOptimizationBoxModels(); err != nil {
		return err
	}
	if err := c.releaseParetoInitialModel(); err != nil {
		return err
	}
	if err := c.closeParetoBackend(); err != nil {
		return err
	}
	if c.optimizationSAT != nil {
		if err := c.optimizationSAT.Close(); err != nil {
			return err
		}
		c.optimizationSAT = nil
	}
	if err := c.releaseRetainedOptimizer(); err != nil {
		return err
	}
	// m_hard_constraints is declared first, so it is destroyed last.
	if err := c.releaseOptimizationTerms(); err != nil {
		return err
	}
	if c.manager != nil {
		// m_opt = nullptr (AUDIT.md FUZZ_A-F13).
		c.optimizedObjectives, c.optsmtReset = 0, false
		c.optimizationClausal = false
		if err := c.releaseDatatypeUtil(); err != nil {
			return err
		}
		c.resetDeclarationDictionaries()
		if err := c.restoreDeclarations(0, 0); err != nil {
			return err
		}
		if err := c.restoreSortConstructors(0); err != nil {
			return err
		}
		if err := c.resetSortInfo(); err != nil {
			return err
		}
		if err := c.releaseList(); err != nil {
			return err
		}
		if !c.borrowedManager {
			if err := c.manager.Close(); err != nil {
				return err
			}
		}
		c.manager = nil
	}

	clear(c.builtins)
	clear(c.sortArities)
	clear(c.sortConstructors)
	clear(c.softObjectiveIndex)
	c.softObjectiveTrail = nil
	c.sortConstructorTrail = nil
	c.logicChecker.Reset()
	c.logic = ""
	c.NumeralAsReal = false
	c.scopes = nil
	c.checked = false
	c.status = lbool.Undef
	c.initializationError = nil
	return nil
}

func (c *Context) Close() error {
	if c == nil {
		return nil
	}
	if c.closed {
		return nil
	}
	if err := c.unbindDiagnosticGlobals(); err != nil {
		return err
	}
	if err := c.resetManagerState(); err != nil {
		return err
	}
	if c.fixedpointQueryCmdParams.Slot != 0 {
		if err := c.config.Globals.Context.Del(&c.fixedpointQueryCmdParams); err != nil {
			return err
		}
	}
	if c.userTactics != nil {
		c.userTactics.DeleteAll()
		c.userTactics = nil
	}
	if c.parameters != nil {
		if err := c.parameters.Close(); err != nil {
			return err
		}
		c.parameters = nil
	}
	if c.paramsReady {
		if err := c.config.Params.Del(&c.params); err != nil {
			return err
		}
		c.paramsReady = false
	}
	if c.owned != nil {
		if err := c.owned.Close(); err != nil {
			return err
		}
		c.owned = nil
	}
	if c.regularFile != nil {
		if err := c.regularFile.Close(); err != nil {
			return err
		}
		c.regularFile = nil
	}
	if c.diagnosticFile != nil {
		if err := c.diagnosticFile.Close(); err != nil {
			return err
		}
		c.diagnosticFile = nil
	}
	c.closed = true
	return nil
}

// releaseOptimizationHard is opt::context::pop's m_optsmt.reset() and
// m_hard_constraints.reset(); m_objectives survive until the next
// import_scoped_state.
func (c *Context) releaseOptimizationHard() (result error) {
	for i := range c.optimizationOptsmt {
		result = errors.Join(result, c.manager.DelRef(&c.optimizationOptsmt[i]))
	}
	c.optimizationOptsmt = nil
	if c.optimizationPurifier != nil {
		result = errors.Join(result, c.optimizationPurifier.ReleaseHardConstraints())
	}
	return result
}

// releaseOptimizationTerms frees the previous from_fmls state front to back.
// releaseOptimizationObjectiveTerms is the part of releaseOptimizationTerms
// ~opt::context reaches first (m_objectives); m_optsmt follows
// m_scoped_state, and the purifier's m_hard_constraints go last.
func (c *Context) releaseOptimizationObjectiveTerms() (result error) {
	for i := range c.optimizationTerms {
		result = errors.Join(result, c.manager.DelRef(&c.optimizationTerms[i]))
	}
	c.optimizationTerms = nil
	if c.optimizationPurifier != nil {
		result = errors.Join(result, c.optimizationPurifier.ReleaseObjectives())
	}
	result = errors.Join(result, c.retainedOptimizer.ReleaseObjectives())
	if state := c.optimizationParetoPurification; state != nil && state.purifier != nil {
		result = errors.Join(result, state.purifier.ReleaseObjectives())
	}
	return result
}

func (c *Context) releaseOptimizationTerms() (result error) {
	// import_scoped_state: m_optsmt.reset(), then m_objectives and
	// m_hard_constraints.
	for i := range c.optimizationOptsmt {
		result = errors.Join(result, c.manager.DelRef(&c.optimizationOptsmt[i]))
	}
	c.optimizationOptsmt = nil
	for i := range c.optimizationTerms {
		result = errors.Join(result, c.manager.DelRef(&c.optimizationTerms[i]))
	}
	c.optimizationTerms = nil
	if c.optimizationPurifier != nil {
		result = errors.Join(result, c.optimizationPurifier.Close())
		c.optimizationPurifier = nil
	}
	return result
}
