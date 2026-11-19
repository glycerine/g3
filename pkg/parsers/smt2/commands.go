package smt2

// Source: src/parsers/smt2/smt2parser.cpp command dispatch and retained
// declaration/assertion/check commands; src/cmd_context/basic_cmds.cpp.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/escaped"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type commandParser struct {
	sortContext        string // parse_sort's context for unknown_sort messages
	context            *cmd_context.Context
	scanner            *Scanner
	filename           string
	freshResetSortGap  bool
	token              Token
	parenDepth         int
	cacheEnd           int
	out                io.Writer
	manager            *ast.ASTManager
	refs               []ast.ASTRef
	env                smt2LocalEnv
	exprs              localExprStack
	locals             localExprStack
	sorts              localSortStack
	bindings           uint32
	shifter            *rewriter.VarShifter
	sortParams         map[string]ast.ASTHandle
	datatypeHeaders    map[string]uint32
	ignoreUserPatterns bool
	ignoreBadPatterns  bool
	errorForVS         bool
	stop               func() bool
	ignoreChecks       bool
	queryCollector     *fixedpointQueryCollector
	tacticExpansion    []string
	softWeight         unumeral.Rational // assert-soft :weight parameter, a rational of scanner.Numbers()
	softID             string
}

// ReportedParseError indicates that ParseShellCommands already emitted the
// source parser's error response and recovered through the rest of the input.
type ReportedParseError struct{ cause error }

func (e *ReportedParseError) Error() string { return e.cause.Error() }
func (e *ReportedParseError) Unwrap() error { return e.cause }

// ParseCommands executes the retained SMT-LIB command grammar against Context.
// Unsupported commands fail explicitly. Command-local AST references are
// released after execution; the context owns declarations and assertions.
func ParseCommands(context *cmd_context.Context, input io.Reader, out io.Writer) (result error) {
	return parseCommands(context, input, out, false, nil)
}

// ParseShellCommands uses the shell's default stdout identity for its output.
func ParseShellCommands(context *cmd_context.Context, input io.Reader, out io.Writer) error {
	return parseCommandsWithOutput(context, input, out, false, nil, true)
}

// ParseShellCommandsWithStop is ParseShellCommands for a run the shell can
// end from outside: once stop reports true no further command starts. It
// stands for the source timeout handler's exit(0) (smtlib_frontend.cpp
// on_timeout), which ends the process in the middle of the command stream.
func ParseShellCommandsWithStop(context *cmd_context.Context, input io.Reader, out io.Writer, stop func() bool) error {
	return parseCommandsWithStop(context, input, out, false, nil, true, "", true, stop)
}

// ParseAssertions parses a solver-loading SMT-LIB stream. Check commands are
// syntax-checked but not executed or printed; callers consume the context's
// final assertion set only, matching api_solver.cpp solver_from_stream.
func ParseAssertions(context *cmd_context.Context, input io.Reader) error {
	return parseCommands(context, input, io.Discard, true, nil)
}

// ParseAssertionsReporting is ParseAssertions with the temporary
// cmd_context's regular stream left at out (api_solver.cpp
// solver_from_stream never redirects it): parse errors are printed as
// (error "...") and the result is a *ReportedParseError.
func ParseAssertionsReporting(context *cmd_context.Context, input io.Reader, out io.Writer) error {
	return parseCommandsWithOptions(context, input, out, true, nil, false, "", true)
}

func parseCommands(context *cmd_context.Context, input io.Reader, out io.Writer, ignoreChecks bool, collector *fixedpointQueryCollector) (result error) {
	return parseCommandsWithOutput(context, input, out, ignoreChecks, collector, false)
}

func parseCommandsWithOutput(context *cmd_context.Context, input io.Reader, out io.Writer, ignoreChecks bool, collector *fixedpointQueryCollector, shell bool) (result error) {
	return parseCommandsWithOptions(context, input, out, ignoreChecks, collector, shell, "", shell)
}

func parseCommandsWithOptions(context *cmd_context.Context, input io.Reader, out io.Writer, ignoreChecks bool, collector *fixedpointQueryCollector, shell bool, filename string, reportErrors bool) (result error) {
	return parseCommandsWithStop(context, input, out, ignoreChecks, collector, shell, filename, reportErrors, nil)
}

func parseCommandsWithStop(context *cmd_context.Context, input io.Reader, out io.Writer, ignoreChecks bool, collector *fixedpointQueryCollector, shell bool, filename string, reportErrors bool, stop func() bool) (result error) {
	if stop != nil && stop() {
		return nil
	}
	var err error
	if shell {
		err = context.SetShellRegularOutput(out)
	} else {
		err = context.SetRegularOutput(out)
	}
	if err != nil {
		return err
	}
	// Source gparams is process-wide. During this single-goroutine command
	// session, lazy AST algebraic owners and printers must see this context's
	// parameter service. Restore the enclosing session after parser cleanup.
	globals, err := context.GlobalParameters()
	if err != nil {
		return err
	}
	previous, err := uparams.BindProcessGlobalParamsService(globals)
	if err != nil {
		return err
	}
	defer func() {
		_, err := uparams.BindProcessGlobalParamsService(previous)
		result = errors.Join(result, err)
	}()
	compliant, err := context.Compliant()
	if err != nil {
		return err
	}
	scanner, err := NewScanner(input, compliant, context.NumeralService().Manager)
	if err != nil {
		return err
	}
	p := &commandParser{context: context, scanner: scanner, filename: filename, out: context.RegularOutput(), ignoreChecks: ignoreChecks, queryCollector: collector, stop: stop, softWeight: unumeral.NewRational()}
	defer func() {
		result = errors.Join(result, scanner.Numbers().Del(&p.softWeight), scanner.Close())
	}()
	if err = scanner.Numbers().SetInt32(&p.softWeight, 1); err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, p.clearCommand())
		if p.shifter != nil {
			result = errors.Join(result, p.shifter.Close())
		}
	}()
	if err = p.configurePatternOptions(); err != nil {
		return err
	}
	var firstParseError error
	if p.stop != nil && p.stop() {
		return nil
	}
	if err = p.advance(); err != nil {
		firstParseError = parserScannerError(err, scanner)
		report := func(cause error) error { return nil }
		if reportErrors {
			report = func(cause error) error { return reportParserError(p, cause) }
		}
		if err = report(firstParseError); err != nil {
			return reportFailure(firstParseError, err)
		}
		if err = p.syncAfterError(report); err != nil {
			return reportFailure(firstParseError, err)
		}
	}
	for p.token != EOFToken {
		if p.stop != nil && p.stop() {
			return nil
		}
		exit, err := p.command()
		var processExit *z3exception.ProcessExit
		if errors.As(err, &processExit) {
			// A source exit(status) below the command: no report, no
			// further commands.
			return processExit
		}
		if err != nil {
			parseErr := parserScannerError(err, scanner)
			if firstParseError == nil {
				firstParseError = parseErr
			}
			report := func(cause error) error { return nil }
			if reportErrors {
				report = func(cause error) error { return reportParserError(p, cause) }
			}
			if err = report(parseErr); err != nil {
				return reportFailure(firstParseError, err)
			}
			if err = p.clearCommand(); err != nil {
				return errors.Join(firstParseError, err)
			}
			if p.shifter != nil {
				if err = p.shifter.Close(); err != nil {
					return errors.Join(firstParseError, err)
				}
				p.shifter = nil
			}
			if err = p.syncAfterError(func(cause error) error {
				if firstParseError == nil {
					firstParseError = cause
				}
				return report(cause)
			}); err != nil {
				return reportFailure(firstParseError, err)
			}
			continue
		}
		if err = p.clearCommand(); err != nil {
			return err
		}
		if exit {
			if firstParseError != nil && reportErrors {
				return &ReportedParseError{cause: firstParseError}
			}
			return firstParseError
		}
	}
	if firstParseError != nil {
		if reportErrors {
			return &ReportedParseError{cause: firstParseError}
		}
		return firstParseError
	}
	return nil
}

func (p *commandParser) advance() error {
	switch p.token {
	case LeftParenToken:
		p.parenDepth++
	case RightParenToken:
		p.parenDepth--
	}
	p.cacheEnd = p.scanner.CacheSize()
	var err error
	p.token, err = p.scanner.Scan()
	return err
}

func parserScannerError(err error, scanner *Scanner) *ScannerError {
	var parseError *ScannerError
	if errors.As(err, &parseError) {
		return parseError
	}
	return &ScannerError{Message: z3exception.SourceMessage(err), Line: scanner.Line(), Column: scanner.Position()}
}

// reportFailure ends the parse after a failed report. An immediate-exit
// report is returned alone so the shell exits with status 1.
func reportFailure(first, err error) error {
	var reported *ReportedParseError
	if errors.As(err, &reported) {
		return reported
	}
	return errors.Join(first, err)
}

// reportParserError is parser::error: it clears the cancel flag, writes the
// Visual Studio form to the diagnostic stream when
// parser.error_for_visual_studio is on (else the (error ...) form to the
// regular stream), and with :error-behavior immediate-exit ends the run,
// which the source does with _Exit(1).
func reportParserError(p *commandParser, err error) error {
	if cause := p.context.SetCancel(false); cause != nil {
		return cause
	}
	var parseError *ScannerError
	if !errors.As(err, &parseError) {
		parseError = &ScannerError{Message: z3exception.SourceMessage(err), Line: p.scanner.Line(), Column: p.scanner.Position()}
	}
	var cause error
	if p.errorForVS {
		message := parseError.Message
		_, cause = fmt.Fprintf(p.context.DiagnosticOutput(), "Z3(%d, %d): ERROR: %s", parseError.Line, parseError.Column, message)
		if cause == nil && !strings.HasSuffix(message, "\n") {
			_, cause = fmt.Fprintln(p.context.DiagnosticOutput())
		}
	} else {
		var message string
		message, cause = escaped.String(&parseError.Message, true, 0)
		if cause != nil {
			return cause
		}
		if p.filename != "" {
			_, cause = fmt.Fprintf(p.out, "(error \"%s: line %d column %d: %s\")\n", p.filename, parseError.Line, parseError.Column, message)
		} else {
			_, cause = fmt.Fprintf(p.out, "(error \"line %d column %d: %s\")\n", parseError.Line, parseError.Column, message)
		}
	}
	if cause == nil && p.context.ExitOnError {
		return &ReportedParseError{cause: parseError}
	}
	return cause
}

func (p *commandParser) syncAfterError(report func(error) error) error {
	if p.parenDepth < 0 {
		p.parenDepth = 0
	}
	for p.token != EOFToken {
		if p.parenDepth == 0 && p.token == LeftParenToken {
			return nil
		}
		if err := p.advance(); err != nil {
			if cause := report(err); cause != nil {
				return cause
			}
		}
		if p.parenDepth < 0 {
			p.parenDepth = 0
		}
	}
	return nil
}
func (p *commandParser) expect(token Token) error {
	if p.token != token {
		return fmt.Errorf("unexpected input, expected token %d", token)
	}
	return p.advance()
}

func (p *commandParser) requireEnd() error {
	if p.token != RightParenToken {
		return fmt.Errorf("invalid command, ')' expected")
	}
	return nil
}

// optionValueEnd is parse_ext_cmd's check after a command's last argument:
// any further token is "too many arguments".
func (p *commandParser) optionValueEnd() error {
	if p.token != RightParenToken {
		return fmt.Errorf("invalid command, too many arguments")
	}
	return nil
}

func (p *commandParser) symbol() (string, error) {
	if p.token != SymbolToken {
		return "", fmt.Errorf("symbol expected")
	}
	value := p.scanner.ID()
	return value, p.advance()
}
func (p *commandParser) unsigned() (uint32, error) {
	if p.token != IntegerToken {
		return 0, fmt.Errorf("unsigned numeral expected")
	}
	value, ok, err := p.scannerUnsigned()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("unsigned numeral is out of range")
	}
	return value, p.advance()
}

// assertSoft reads assert-soft's options into weight and the parser's
// persistent :weight/:id values, then registers the soft constraint.
func (p *commandParser) assertSoft(q *unumeral.RationalManager, expression ast.ASTHandle, weight *unumeral.Rational) (err error) {
	if err = q.Copy(weight, &p.softWeight); err != nil {
		return err
	}
	id := p.softID
	for p.token != RightParenToken {
		if p.token != KeywordToken {
			return fmt.Errorf("assert-soft option keyword expected")
		}
		rawKey := p.scanner.ID()
		key := uparams.NormParamName(&rawKey)
		if err = p.advance(); err != nil {
			return err
		}
		switch key {
		case "weight":
			err = p.rational(weight)
			if err == nil {
				err = q.Copy(&p.softWeight, weight)
			}
		case "id":
			id, err = p.symbol()
			if err == nil {
				p.softID = id
			}
		default:
			return fmt.Errorf("unsupported assert-soft option %s", rawKey)
		}
		if err != nil {
			return err
		}
	}
	return p.context.AddSoftConstraint(expression, weight, id)
}

// scannerUnsigned is curr_numeral().is_unsigned() / get_unsigned().
func (p *commandParser) scannerUnsigned() (uint32, bool, error) {
	q, n := p.scanner.Numbers(), p.scanner.Number()
	ok, err := q.IsUnsigned(n)
	if err != nil || !ok {
		return 0, false, err
	}
	value, err := q.Unsigned(n)
	return value, err == nil, err
}

// rational copies the current numeral token into out (a rational of
// scanner.Numbers()) and advances.
func (p *commandParser) rational(out *unumeral.Rational) error {
	if p.token != IntegerToken && p.token != DecimalToken {
		// assert-soft's :weight is a CPK_NUMERAL argument.
		return fmt.Errorf("invalid command argument, numeral expected")
	}
	if err := p.scanner.Numbers().Copy(out, p.scanner.Number()); err != nil {
		return err
	}
	return p.advance()
}
func (p *commandParser) astManager() (*ast.ASTManager, error) {
	if p.manager != nil {
		return p.manager, nil
	}
	m, err := p.context.Manager()
	if err != nil {
		return nil, err
	}
	p.manager = m
	p.shifter, err = rewriter.NewVarShifter(m)
	p.freshResetSortGap = false
	return m, err
}

// varShifter is parser::shifter(): reset() drops m_var_shifter after an
// error and the next use allocates a new one.
func (p *commandParser) varShifter() (*rewriter.VarShifter, error) {
	if p.shifter == nil && p.manager != nil {
		shifter, err := rewriter.NewVarShifter(p.manager)
		if err != nil {
			return nil, err
		}
		p.shifter = shifter
	}
	return p.shifter, nil
}

func (p *commandParser) pin(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := p.astManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.refs = append(p.refs, ref)
	return value, nil
}
func (p *commandParser) clearCommand() (result error) {
	if p.manager != nil {
		result = errors.Join(result, p.sorts.reset(p.manager))
		result = errors.Join(result, p.exprs.reset(p.manager))
		result = errors.Join(result, p.locals.reset(p.manager))
		for i := len(p.refs) - 1; i >= 0; i-- {
			result = errors.Join(result, p.manager.DelRef(&p.refs[i]))
		}
	}
	p.refs = nil
	p.env.reset()
	p.bindings = 0
	p.tacticExpansion = nil
	return result
}

func (p *commandParser) fixedpointQuery(declaration ast.ASTHandle) (status lbool.LBool, printAnswer, printStatistics, printCertificate bool, resultErr error) {
	if err := p.validateFixedpointQueryDeclaration(declaration); err != nil {
		return lbool.Undef, false, false, false, err
	}
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return lbool.Undef, false, false, false, err
	}
	context := globals.Context
	// parametric_cmd::prepare resets the command's persistent m_params.
	options, err := p.context.FixedpointQueryCommandParams()
	if err != nil {
		return lbool.Undef, false, false, false, err
	}
	descriptions, err := globals.SourceModuleDescriptorsName("fixedpoint")
	if err != nil {
		return lbool.Undef, false, false, false, err
	}
	for p.token != RightParenToken {
		if p.token != KeywordToken {
			return lbool.Undef, false, false, false, fmt.Errorf("fixedpoint query parameter expected")
		}
		rawKey := p.scanner.ID()
		key := uparams.NormParamName(&rawKey)
		if err = p.advance(); err != nil {
			return lbool.Undef, false, false, false, err
		}
		kind, x := descriptions.KindName(key)
		if x != nil {
			return lbool.Undef, false, false, false, x
		}
		if kind == uparams.ParamInvalid {
			return lbool.Undef, false, false, false, fmt.Errorf("unsupported fixedpoint query parameter :%s", key)
		}
		if err = p.commandParameter(context, options, key, kind); err != nil {
			return lbool.Undef, false, false, false, err
		}
	}
	if err = p.requireEnd(); err != nil {
		return lbool.Undef, false, false, false, err
	}
	module, err := globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return lbool.Undef, false, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&module)) }()
	effective := options
	if !p.ignoreChecks {
		// dl_query_cmd::execute: dlctx.updt_params(m_params). The print
		// options are then read back from the context's parameters.
		if effective, err = p.context.UpdateFixedpointContextParams(); err != nil {
			return lbool.Undef, false, false, false, err
		}
	}
	answerName, statisticsName, certificateName := "print_answer", "print_statistics", "print_certificate"
	printAnswer, err = context.GetBoolFallbackName(effective, &answerName, &module, false)
	if err == nil {
		printStatistics, err = context.GetBoolFallbackName(effective, &statisticsName, &module, false)
	}
	if err == nil {
		printCertificate, err = context.GetBoolFallbackName(effective, &certificateName, &module, false)
	}
	if err != nil || p.ignoreChecks {
		return lbool.Undef, printAnswer, printStatistics, printCertificate, err
	}
	status, err = p.context.QueryFixedpointCommandWithParams(declaration, effective)
	return status, printAnswer, printStatistics, printCertificate, err
}

func (p *commandParser) command() (bool, error) {
	commandLine, commandPosition := p.scanner.Line(), p.scanner.Position()
	// parse(): "invalid command, '(' expected"; parse_cmd:
	// check_identifier("invalid command, symbol expected").
	if p.token != LeftParenToken {
		return false, fmt.Errorf("invalid command, '(' expected")
	}
	if err := p.advance(); err != nil {
		return false, err
	}
	if p.token != SymbolToken {
		return false, fmt.Errorf("invalid command, symbol expected")
	}
	name, err := p.symbol()
	if err != nil {
		return false, err
	}
	success := true
	switch name {
	case "dbg-set", "dbg-get-qbody", "dbg-pp-var", "dbg-size", "dbg-shift-vars", "dbg-num-shared", "dbg-pp-shared", "dbg-subst", "dbg-instantiate", "dbg-instantiate-nested", "dbg-elim-unused-vars", "dbg-bool-rewriter", "dbg-bool-flat-rewriter", "dbg-elim-and", "dbg-used-vars", "dbg-lt", "dbg-some-value", "dbg-params", "dbg-set-next-id", "dbg-translator", "dbg-sexpr", "display-dimacs":
		if err = p.debugCommand(name); err != nil {
			return false, err
		}
		success = false
	case "set-logic":
		// set_logic_cmd::set_next_arg runs cmd_context::set_logic on the
		// symbol before next() and before the closing ')' is checked.
		logicLine, logicPos := p.scanner.Line(), p.scanner.Position()
		logic, err := p.symbol()
		if err != nil {
			return false, err
		}
		if err = p.context.SetLogic(logic); err != nil {
			if !errors.Is(err, cmd_context.ErrUnsupportedLogic) {
				return false, positionedError(err, logicLine, logicPos)
			}
			// ctx.print_unsupported(symbol("ignoring unsupported logic "+arg), m_line, m_pos)
			if _, err = fmt.Fprintln(p.out, "unsupported"); err != nil {
				return false, err
			}
			if _, err = fmt.Fprintf(p.context.DiagnosticOutput(), "; ignoring unsupported logic %s line: %d position: %d\n", logic, commandLine, commandPosition); err != nil {
				return false, err
			}
			success = false
		}
		// set_logic_cmd has arity 1: parse_ext_cmd's check.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
	case "set-option":
		// parse_ext_cmd over set_option_cmd: CPK_KEYWORD, then
		// CPK_OPTION_VALUE applied by set_next_arg as it is read.
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid command, argument(s) missing")
		}
		if p.token != KeywordToken {
			return false, fmt.Errorf("invalid command argument, keyword expected")
		}
		keyword := p.scanner.ID()
		if err := p.advance(); err != nil {
			return false, err
		}
		unsupported := false
		switch p.token {
		case RightParenToken:
			return false, fmt.Errorf("invalid command, argument(s) missing")
		case IntegerToken, DecimalToken, BitVectorToken:
			err = p.context.SetOptionNumeral(keyword, p.scanner.Number())
		case SymbolToken:
			unsupported, err = p.context.SetOptionSymbol(keyword, p.scanner.ID())
		case StringToken:
			err = p.context.SetOptionString(keyword, p.scanner.String())
		default:
			err = fmt.Errorf("invalid option value")
		}
		if err != nil {
			return false, err
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		// execute: print_unsupported or print_success.
		if unsupported {
			if _, err = fmt.Fprintln(p.out, "unsupported"); err != nil {
				return false, err
			}
			if _, err = fmt.Fprintf(p.context.DiagnosticOutput(), "; %s line: %d position: %d\n", keyword, commandLine, commandPosition); err != nil {
				return false, err
			}
			success = false
		}
		// parse_ext_cmd's updt_params hack after every set-option.
		if err = p.configurePatternOptions(); err != nil {
			return false, err
		}
	case "set-info":
		// set_info_cmd through parse_ext_cmd: a CPK_KEYWORD then a
		// CPK_OPTION_VALUE (numeral, symbol or string), arity 2.
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid command, argument(s) missing")
		}
		if p.token != KeywordToken {
			return false, fmt.Errorf("invalid command argument, keyword expected")
		}
		key := p.scanner.ID()
		if err := p.advance(); err != nil {
			return false, err
		}
		switch p.token {
		case RightParenToken:
			return false, fmt.Errorf("invalid command, argument(s) missing")
		case SymbolToken:
			// set_next_arg(symbol) runs before next().
			if key == ":status" {
				if err = p.context.SetInfoStatus(p.scanner.ID()); err != nil {
					return false, err
				}
			}
		case IntegerToken, DecimalToken, BitVectorToken, StringToken:
		default:
			return false, fmt.Errorf("invalid option value")
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
	case "declare-datatype", "declare-datatypes":
		if err = p.declareDatatypes(name == "declare-datatype"); err != nil {
			return false, err
		}
	case "declare-sort":
		if p.token == SymbolToken {
			if _, _, found := p.context.FindPSortDecl(p.scanner.ID()); found {
				return false, fmt.Errorf("invalid sort declaration, sort already declared/defined")
			}
		}
		name, err := p.symbol()
		if err != nil {
			return false, err
		}
		if p.token == RightParenToken {
			if err = p.requireEnd(); err != nil {
				return false, err
			}
			if _, err = p.context.DeclareSortConstructor(name, 0); err != nil {
				return false, err
			}
			break
		}
		// parse_declare_sort inserts the declaration while the arity
		// numeral is still the current token, then checks the ')'.
		if p.token != IntegerToken {
			return false, fmt.Errorf("invalid sort declaration, arity (<numeral>) or ')' expected")
		}
		arity, ok, err := p.scannerUnsigned()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, fmt.Errorf("unsigned numeral is out of range")
		}
		if _, err = p.context.DeclareSortConstructor(name, arity); err != nil {
			return false, err
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		if p.token != RightParenToken {
			return false, fmt.Errorf("invalid sort declaration, ')' expected")
		}
	case "declare-map":
		mapName, err := p.symbol()
		if err != nil {
			return false, err
		}
		if err = p.expect(LeftParenToken); err != nil {
			return false, err
		}
		var indices []ast.ASTHandle
		for p.token != RightParenToken {
			sort, e := p.sort()
			if e != nil {
				return false, e
			}
			indices = append(indices, sort)
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		// declare_map_cmd::set_next_arg(sorts) runs once the sort list is
		// consumed, before the function reference is parsed.
		if len(indices) == 0 {
			return false, fmt.Errorf("invalid map declaration, empty sort list")
		}
		function, err := p.functionDeclarationReference()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if _, err = p.context.DeclareMap(mapName, indices, function); err != nil {
			return false, err
		}
	case "define-sort":
		if p.token == SymbolToken {
			if _, _, found := p.context.FindPSortDecl(p.scanner.ID()); found {
				return false, fmt.Errorf("invalid sort definition, sort already declared/defined")
			}
		}
		name, err := p.symbol()
		if err != nil {
			return false, err
		}
		if err = p.expect(LeftParenToken); err != nil {
			return false, err
		}
		var parameterNames []string
		for p.token != RightParenToken {
			parameter, e := p.symbol()
			if e != nil {
				return false, e
			}
			parameterNames = append(parameterNames, parameter)
		}
		if err = p.expect(RightParenToken); err != nil {
			return false, err
		}
		formals, byName, err := p.datatypeFormalParameters(parameterNames)
		if err != nil {
			return false, err
		}
		previous := p.sortParams
		p.sortParams = byName
		// parse_psort: the body's sort applications instantiate with the
		// definition.
		p.context.BeginPSort()
		body, bodyErr := p.sort()
		p.sortParams = previous
		if bodyErr != nil {
			return false, errors.Join(bodyErr, p.context.AbortPSort())
		}
		// Source parse_define_sort: m_ctx.insert runs before check_rparen.
		if err = p.context.DefineSort(name, formals, body); err != nil {
			return false, err
		}
		if p.token != RightParenToken {
			return false, errors.New("invalid sort definition, ')' expected")
		}
	case "declare-fun", "declare-const":
		function, err := p.symbol()
		if err != nil {
			return false, err
		}
		var domain []ast.ASTHandle
		if name == "declare-fun" {
			if err = p.expect(LeftParenToken); err != nil {
				return false, err
			}
			for p.token != RightParenToken {
				sort, err := p.sortIn("Parsing function declaration. Expecting sort list '('")
				if err != nil {
					return false, err
				}
				domain = append(domain, sort)
			}
			if err = p.advance(); err != nil {
				return false, err
			}
		}
		rangeContext := "Invalid function declaration"
		if name == "declare-const" {
			rangeContext = "Invalid constant declaration"
		}
		rangeSort, err := p.sortIn(rangeContext)
		if err != nil {
			return false, err
		}
		// Source parse_declare_fun / parse_declare_const: m_ctx.insert
		// runs before check_rparen.
		if _, err = p.context.DeclareFunction(function, domain, rangeSort); err != nil {
			return false, err
		}
		if p.token != RightParenToken {
			if name == "declare-const" {
				return false, errors.New("invalid constant declaration, ')' expected")
			}
			return false, errors.New("invalid function declaration, ')' expected")
		}
	case "declare-rel":
		// dl_declare_rel_cmd through parse_ext_cmd: CPK_SYMBOL, CPK_SORT_LIST,
		// then CPK_SYMBOL representations; execute needs the first two.
		if p.token == RightParenToken {
			return false, errors.New("at least 2 arguments expected")
		}
		if p.token != SymbolToken {
			return false, errors.New("invalid command argument, symbol expected")
		}
		relation, err := p.symbol()
		if err != nil {
			return false, err
		}
		if p.token == RightParenToken {
			return false, errors.New("at least 2 arguments expected")
		}
		if p.token != LeftParenToken {
			return false, errors.New("expecting sort list starting with '('")
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		var domain []ast.ASTHandle
		for p.token != RightParenToken {
			sort, x := p.sort()
			if x != nil {
				return false, x
			}
			domain = append(domain, sort)
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		var representations []string
		for p.token != RightParenToken {
			if p.token != SymbolToken {
				return false, errors.New("invalid command argument, symbol expected")
			}
			representation, x := p.symbol()
			if x != nil {
				return false, x
			}
			representations = append(representations, representation)
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		declaration, x := p.context.DeclareRelation(relation, domain)
		if x != nil {
			return false, x
		}
		if err = p.context.SetFixedpointPredicateRepresentation(declaration, representations); err != nil {
			return false, err
		}
		if p.queryCollector != nil {
			if err = p.queryCollector.addRelation(p, declaration); err != nil {
				return false, err
			}
		}
	case "declare-var":
		variable, err := p.symbol()
		if err != nil {
			return false, err
		}
		sort, err := p.sort()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.DeclareFixedpointVariable(variable, sort); err != nil {
			return false, err
		}
	case "define-fun", "define-const":
		if err := p.defineFunction(name == "define-fun"); err != nil {
			return false, err
		}
	case "define-fun-rec", "define-funs-rec":
		if err := p.defineRecursiveFunctions(name == "define-funs-rec"); err != nil {
			return false, err
		}
	case "assert":
		expression, named, displayText, err := p.assertionExpression()
		if err != nil {
			return false, err
		}
		// Source parse_assert: the term must be Boolean.
		isBool, err := p.booleanExpression(expression)
		if err != nil {
			return false, err
		}
		if !isBool {
			return false, errors.New("invalid assert command, term is not Boolean")
		}
		// Source parse_assert: assert_expr runs before check_rparen.
		if err = p.context.AssertWithText(expression, named, displayText); err != nil {
			return false, err
		}
		if p.token != RightParenToken {
			return false, errors.New("invalid assert command, ')' expected")
		}
	case "rule":
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid rule, expected formula")
		}
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		name := symbol.Symbol{}
		bound := ^uint32(0)
		if p.token != RightParenToken {
			text, err := p.symbol()
			if err != nil {
				return false, err
			}
			manager, err := p.astManager()
			if err != nil {
				return false, err
			}
			// A supplied quoted empty symbol is distinct from an absent name.
			// SMT-LIB symbol tokens are strings, including the spelling k!7.
			name, err = manager.Symbols.Intern(text)
			if err != nil {
				return false, err
			}
		}
		if p.token != RightParenToken {
			bound, err = p.unsigned()
			if err != nil {
				return false, err
			}
		}
		// dl_rule_cmd expects symbols for every argument after the bound;
		// each one replaces the previous name.
		for p.token != RightParenToken {
			text, err := p.symbol()
			if err != nil {
				return false, err
			}
			manager, err := p.astManager()
			if err != nil {
				return false, err
			}
			name, err = manager.Symbols.Intern(text)
			if err != nil {
				return false, err
			}
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if p.queryCollector != nil {
			if err = p.queryCollector.addRule(p, expression, name); err != nil {
				return false, err
			}
		} else if err = p.context.AddFixedpointRuleWithSymbolAndBound(expression, name, bound); err != nil {
			return false, err
		}
	case "fixedpoint-push", "fixedpoint-pop":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if name == "fixedpoint-push" {
			err = p.context.PushFixedpoint()
		} else {
			err = p.context.PopFixedpoint()
		}
		if err != nil {
			return false, err
		}
		if p.queryCollector != nil {
			if name == "fixedpoint-push" {
				p.queryCollector.push(1)
			} else if err = p.queryCollector.pop(1); err != nil {
				return false, err
			}
		}
	case "assert-soft":
		if p.token == RightParenToken {
			return false, fmt.Errorf("assert-soft requires a formulas as argument.")
		}
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		m, err := p.astManager()
		if err != nil {
			return false, err
		}
		sort, err := m.GetSort(expression)
		if err != nil {
			return false, err
		}
		boolean, err := m.MkBoolSort()
		if err != nil {
			return false, err
		}
		if sort != boolean {
			// Source assert_soft_cmd::set_next_arg (opt_cmds.cpp).
			return false, fmt.Errorf("Invalid type for expression. Expected Boolean type.")
		}
		// rational weight = ps().get_rat(symbol("weight"), rational::one());
		q := p.scanner.Numbers()
		weight := unumeral.NewRational()
		if err = p.assertSoft(q, expression, &weight); err != nil {
			return false, errors.Join(err, q.Del(&weight))
		}
		if err = q.Del(&weight); err != nil {
			return false, err
		}
	case "maximize", "minimize":
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid command, argument(s) missing")
		}
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		// The source registers the objective in set_next_arg, before the
		// parser diagnoses an extra command argument.
		if err = p.context.AddObjective(expression, name == "maximize"); err != nil {
			return false, err
		}
		if p.context.PrintSuccess {
			if _, err = fmt.Fprintln(p.out, "success"); err != nil {
				return false, err
			}
		}
		success = false
		if err = p.requireEnd(); err != nil {
			return false, err
		}
	case "push", "pop":
		// parse_opt_unsigned (its texts name push for pop as well).
		n := uint32(1)
		if p.token != RightParenToken {
			if p.token != IntegerToken {
				return false, fmt.Errorf("invalid push command, integer expected")
			}
			negative, err := p.scanner.Numbers().IsNeg(p.scanner.Number())
			if err != nil {
				return false, err
			}
			if negative {
				return false, fmt.Errorf("invalid push command, value is negative.")
			}
			value, ok, err := p.scannerUnsigned()
			if err != nil {
				return false, err
			}
			if !ok {
				return false, fmt.Errorf("invalid push command, value is too big to fit in an unsigned machine integer")
			}
			n = value
			if err = p.advance(); err != nil {
				return false, err
			}
		}
		if name == "push" {
			err = p.context.Push(n)
		} else {
			err = p.context.Pop(n)
		}
		if err != nil {
			return false, err
		}
		// Source parse_push/parse_pop mutates the context before checking
		// the closing parenthesis; a trailing syntax error keeps that prefix.
		if p.token != RightParenToken {
			return false, fmt.Errorf("invalid %s command, ')' expected", name)
		}
	case "check-sat-using":
		if err := p.checkSatUsing(!p.ignoreChecks); err != nil {
			return false, err
		}
		if !p.ignoreChecks {
			p.freshResetSortGap = false
		}
		success = false
	case "declare-tactic":
		name, err := p.symbol()
		if err != nil {
			return false, err
		}
		selected, body, err := p.parseTacticDeclaration()
		if err != nil {
			return false, err
		}
		if err = selected.DecRef(); err != nil {
			return false, err
		}
		if err = p.context.DeclareUserTactic(name, body); err != nil {
			return false, err
		}
	case "apply":
		if err := p.applyTactic(!p.ignoreChecks); err != nil {
			return false, err
		}
		if !p.ignoreChecks {
			p.freshResetSortGap = false
		}
		success = false
	case "check-sat", "check-sat-assuming":
		if err := p.checkSatCommand(name == "check-sat-assuming", !p.ignoreChecks); err != nil {
			return false, err
		}
		if !p.ignoreChecks {
			p.freshResetSortGap = false
		}
		if name == "check-sat-assuming" {
			// Source parse_check_sat_assuming ends with an unchecked next().
			return false, p.advance()
		}
		success = false
	case "query":
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid query command, argument expected")
		}
		declaration, err := p.functionDeclarationReference()
		if err != nil {
			return false, err
		}
		started := time.Now()
		status, printAnswer, printStatistics, printCertificate, err := p.fixedpointQuery(declaration)
		queryException := cmd_context.FixedpointCaughtQueryException(err)
		if err != nil {
			if queryException == nil {
				return false, err
			}
			if _, err = fmt.Fprintf(p.out, "(error \"query failed: %s\")\n", queryException.Message); err != nil {
				return false, err
			}
		}
		if p.queryCollector != nil {
			if err = p.queryCollector.add(p, declaration); err != nil {
				return false, err
			}
		}
		if !p.ignoreChecks {
			text := "unknown"
			reason := ""
			bounded := false
			if status == lbool.True {
				text = "sat"
			} else if status == lbool.False {
				text = "unsat"
			} else if queryException == nil {
				var reasonErr error
				reason, reasonErr = p.context.FixedpointReasonUnknown()
				if reasonErr != nil {
					return false, reasonErr
				}
				if reason == "bounded" {
					bounded = true
					text = "bounded"
				}
			}
			if _, err = fmt.Fprintln(p.out, text); err != nil {
				return false, err
			}
			if status == lbool.Undef && !bounded {
				detail := ""
				switch reason {
				case "input error", "memory bounds exceeded", "timeout", "canceled":
					detail = reason
				case "approximated":
					detail = "approximated relations"
				}
				if detail != "" {
					if _, err = fmt.Fprintln(p.out, detail); err != nil {
						return false, err
					}
				}
			}
			if status == lbool.True && printAnswer {
				manager, y := p.context.Manager()
				if y != nil {
					return false, y
				}
				answer, y := p.context.FixedpointAnswer(declaration)
				if y != nil {
					return false, y
				}
				handle, y := manager.RefGet(answer)
				if y == nil {
					y = p.context.WriteExpression(p.out, handle)
				}
				if y == nil {
					_, y = fmt.Fprintln(p.out)
				}
				cleanupErr := manager.DelRef(&answer)
				if answer != (ast.ASTRef{}) {
					return false, &fixedpointAnswerCleanupFailure{manager: manager, reference: answer, cause: y, cleanupErr: cleanupErr}
				}
				if y = errors.Join(y, cleanupErr); y != nil {
					return false, y
				}
			}
			if printCertificate && (status != lbool.Undef || bounded) {
				_, y := p.context.WriteFixedpointCertificate(p.out, declaration)
				if y != nil {
					return false, y
				}
				if _, y = fmt.Fprintln(p.out); y != nil {
					return false, y
				}
			}
			if err = p.context.ResetFixedpointCommandStatus(); err != nil {
				return false, err
			}
			if printStatistics {
				stats, y := p.context.FixedpointStatistics()
				if y != nil {
					return false, y
				}
				stats.UpdateDouble("time", time.Since(started).Seconds())
				if y = stats.DisplaySMT2(p.out); y != nil {
					return false, y
				}
			}
		}
		success = false
	case "get-info":
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid command, argument(s) missing")
		}
		if p.token != KeywordToken {
			return false, fmt.Errorf("invalid command argument, keyword expected")
		}
		key := p.scanner.ID()
		// Source get_info_cmd::set_next_arg prints the information as the
		// keyword is read, before parse_ext_cmd sees the closing ')'.
		switch key {
		case ":all-statistics":
			err = p.context.WriteStatistics(p.out)
		case ":reason-unknown":
			err = p.context.WriteReasonUnknown(p.out)
		default:
			var value string
			var supported bool
			value, supported, err = p.context.GetInfo(key)
			if err == nil {
				if supported {
					_, err = fmt.Fprintln(p.out, value)
				} else {
					if _, err = fmt.Fprintln(p.out, "unsupported"); err == nil {
						_, err = fmt.Fprintf(p.context.DiagnosticOutput(), "; %s line: %d position: %d\n", key, commandLine, commandPosition)
					}
				}
			}
		}
		if err != nil {
			return false, err
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		success = false
	case "get-user-tactics":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		var declarations strings.Builder
		for _, declaration := range p.context.UserTacticDeclarations() {
			if declarations.Len() != 0 {
				// get_user_tactics_cmd separates entries with "\n "
				// (tactic_cmds.cpp:83).
				declarations.WriteString("\n ")
			}
			fmt.Fprintf(&declarations, "(declare-tactic %s %s)", declaration.Name, declaration.Body)
		}
		text := strings.ReplaceAll(declarations.String(), "\"", "\\\"")
		if _, err = fmt.Fprintf(p.out, "(%s)\n", text); err != nil {
			return false, err
		}
		success = false
	case "help-tactic":
		if err = p.helpTactic(); err != nil {
			return false, err
		}
		success = false
	case "help":
		if err = p.helpCommands(); err != nil {
			return false, err
		}
		success = false
	case "get-option":
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid command, argument(s) missing")
		}
		if p.token != KeywordToken {
			return false, fmt.Errorf("invalid command argument, keyword expected")
		}
		keyword := p.scanner.ID()
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		value, supported, err := p.context.GetOption(keyword)
		if err != nil {
			return false, err
		}
		if supported {
			_, err = fmt.Fprintln(p.out, value)
		} else {
			if _, err = fmt.Fprintln(p.out, "unsupported"); err == nil {
				_, err = fmt.Fprintf(p.context.DiagnosticOutput(), "; %s line: %d position: %d\n", keyword, commandLine, commandPosition)
			}
		}
		if err != nil {
			return false, err
		}
		success = false
	case "get-model":
		modelIndex := uint32(0)
		for p.token != RightParenToken {
			if p.token != IntegerToken {
				return false, fmt.Errorf("invalid command argument, unsigned integer expected")
			}
			modelIndex, err = p.unsigned()
			if err != nil {
				return false, err
			}
		}
		if err = p.context.WriteModelAtModelIndex(p.out, modelIndex); err != nil {
			return false, err
		}
		success = false
	case "get-assignment":
		// parse_ext_cmd: a zero-arity command with an argument.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteAssignment(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-objectives":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteObjectives(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-value":
		values, text, err := p.valueExpressions()
		if err != nil {
			return false, err
		}
		modelIndex := uint32(0)
		if p.token == KeywordToken && (p.scanner.ID() == ":model-index" || p.scanner.ID() == ":model_index") {
			if err = p.advance(); err != nil {
				return false, err
			}
			if p.token != IntegerToken {
				return false, fmt.Errorf("integer index expected to indexed model evaluation")
			}
			modelIndex, err = p.unsigned()
			if err != nil {
				return false, err
			}
		}
		if p.token != RightParenToken {
			return false, fmt.Errorf("invalid get-value command, ')' expected")
		}
		if err = p.context.WriteValuesWithTextAtModelIndex(p.out, values, text, modelIndex); err != nil {
			return false, err
		}
		success = false
	case "eval":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		options := cmd_context.EvaluationOptions{Timeout: ^uint32(0)}
		for p.token != RightParenToken {
			keyword, e := p.parametricKeyword()
			if e != nil {
				return false, e
			}
			// eval_cmd::init_pdescrs: model_evaluator params, model_index,
			// timeout and max_memory; parametric_cmd::set_next_arg rejects
			// any other keyword (rlimit included) at the keyword itself.
			switch keyword {
			case "completion", "cache", "array_equalities", "max_memory", "max_steps", "timeout", "model_index":
			default:
				return false, fmt.Errorf("invalid keyword argument")
			}
			if err = p.advance(); err != nil {
				return false, err
			}
			switch keyword {
			case "completion", "cache", "array_equalities":
				if p.token != SymbolToken || p.scanner.ID() != "true" && p.scanner.ID() != "false" {
					return false, fmt.Errorf("invalid command argument, true/false expected")
				}
				value := p.scanner.ID() == "true"
				switch keyword {
				case "completion":
					options.Completion = &value
				case "cache":
					options.Cache = &value
				default:
					options.ArrayEqualities = &value
				}
				if err = p.advance(); err != nil {
					return false, err
				}
			case "max_memory", "max_steps", "timeout", "model_index":
				value, e := p.unsigned()
				if e != nil {
					return false, e
				}
				switch keyword {
				case "max_memory":
					options.MaxMemory = &value
				case "max_steps":
					options.MaxSteps = &value
				case "timeout":
					options.Timeout = value
				default:
					options.ModelIndex = value
				}
			}
		}
		if err = p.context.WriteEvaluationWithOptions(p.out, expression, options); err != nil {
			return false, err
		}
		success = false
	case "get-consequences":
		if err = p.getConsequencesCommand(!p.ignoreChecks); err != nil {
			return false, err
		}
		success = false
	case "include":
		if p.token != StringToken {
			return false, fmt.Errorf("invalid command argument, string expected")
		}
		filename := p.scanner.String()
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		file, openErr := os.Open(filename)
		if openErr != nil {
			return false, fmt.Errorf("failed to open file '%s'", filename)
		}
		parseErr := parseCommandsWithStop(p.context, file, p.out, false, nil, false, filename, true, p.stop)
		closeErr := file.Close()
		var reported *ReportedParseError
		if errors.As(parseErr, &reported) {
			parseErr = nil
		}
		if parseErr != nil || closeErr != nil {
			return false, errors.Join(parseErr, closeErr)
		}
	case "get-interpolant", "compute-interpolant":
		var targets []ast.ASTHandle
		for p.token != RightParenToken {
			target, _, e := p.expression()
			if e != nil {
				return false, e
			}
			targets = append(targets, target)
		}
		if len(targets) == 0 {
			return false, fmt.Errorf("not enough arguments")
		}
		tree := targets[0]
		for _, target := range targets[1:] {
			marked, e := p.pin(p.manager.MkInterp(tree))
			if e != nil {
				return false, e
			}
			tree, e = p.pin(p.manager.MkAnd(marked, target))
			if e != nil {
				return false, e
			}
		}
		if name == "get-interpolant" {
			err = p.context.WriteInterpolants(p.out, tree)
		} else {
			err = p.context.ComputeInterpolants(p.out, tree)
		}
		if err != nil {
			return false, err
		}
		success = false
	case "get-proof":
		// parse_ext_cmd: a zero-arity command with an argument.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteProof(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-proof-graph":
		// parse_ext_cmd: a zero-arity command with an argument.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteProofGraph(); err != nil {
			return false, err
		}
		success = false
	case "get-unsat-core":
		// parse_ext_cmd: a zero-arity command with an argument.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteUnsatCore(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-unsat-assumptions":
		// parse_ext_cmd: a zero-arity command with an argument.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteUnsatAssumptions(p.out); err != nil {
			return false, err
		}
		success = false
	case "labels":
		// parse_ext_cmd: a zero-arity command with an argument.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteLabels(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-assertions":
		// parse_ext_cmd: a zero-arity command with an argument.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteAssertions(p.out); err != nil {
			return false, err
		}
		success = false
	case "echo":
		if err = p.echoCommand(); err != nil {
			return false, err
		}
		success = false
	case "display":
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid command, argument(s) missing")
		}
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteExpression(p.out, expression); err != nil {
			return false, err
		}
		if _, err = fmt.Fprintln(p.out); err != nil {
			return false, err
		}
		success = false
	// to-subpaving, to-poly, poly/isolate-roots and poly/factor are
	// installed only when _EXTERNAL_RELEASE is undefined
	// (subpaving_cmds.cpp:53, polynomial_cmds.cpp:239); g3 follows the
	// release build, so they reach the unknown-command path.
	case "simplify", "dbg-th-rewriter":
		if p.token == RightParenToken {
			return false, fmt.Errorf("invalid simplify command, argument expected")
		}
		if err = p.simplifyCommand(); err != nil {
			return false, err
		}
		success = false
	case "reset", "reset-assertions":
		if p.token != RightParenToken {
			if name == "reset" {
				return false, fmt.Errorf("invalid reset command, ')' expected")
			}
			return false, fmt.Errorf("invalid command, too many arguments")
		}
		if p.queryCollector != nil && !p.context.BorrowsManager() && (name == "reset" || !p.context.GlobalDeclarations) {
			return false, fmt.Errorf("reset while collecting fixedpoint queries is not supported")
		}
		// Release parser-owned roots and its manager-bound shifter before
		// cmd_context::reset destroys the old AST manager.
		if err = p.clearCommand(); err != nil {
			return false, err
		}
		if name == "reset" || !p.context.GlobalDeclarations {
			if p.shifter != nil {
				if err = p.shifter.Close(); err != nil {
					return false, err
				}
				p.shifter = nil
			}
			p.manager = nil
		}
		if name == "reset" {
			err = p.context.Reset()
			p.freshResetSortGap = false
		} else {
			err = p.context.ResetAssertions()
			p.freshResetSortGap = !p.context.GlobalDeclarations
		}
		if err != nil {
			return false, err
		}
		if p.context.PrintSuccess {
			if _, err = fmt.Fprintln(p.out, "success"); err != nil {
				return false, err
			}
		}
		return false, p.advance()
	case "exit":
		// parse_ext_cmd: a zero-arity command with an argument.
		if err = p.optionValueEnd(); err != nil {
			return false, err
		}
		if p.context.PrintSuccess {
			if _, err = fmt.Fprintln(p.out, "success"); err != nil {
				return false, err
			}
		}
		return true, nil
	default:
		// Source parse_unknown_cmd consumes the command's arguments, reports
		// the end location on the diagnostic stream, and emits "unsupported"
		// on the regular stream while continuing with the next command.
		for p.token != RightParenToken {
			if err = p.skipSExpr(); err != nil {
				return false, err
			}
		}
		// print_unsupported: the regular stream first, then the diagnostic.
		if _, err = fmt.Fprintln(p.out, "unsupported"); err != nil {
			return false, err
		}
		if _, err = fmt.Fprintf(p.context.DiagnosticOutput(), "; %s line: %d position: %d\n", name, p.scanner.Line(), p.scanner.Position()); err != nil {
			return false, err
		}
		return false, p.advance()
	}
	if err = p.expect(RightParenToken); err != nil {
		return false, err
	}
	if success && p.context.PrintSuccess {
		_, err = fmt.Fprintln(p.out, "success")
	}
	return false, err
}

func (p *commandParser) skipSExpr() error {
	// consume_sexpr's texts.
	if p.token == RightParenToken {
		return fmt.Errorf("invalid s-expression, unexpected ')'")
	}
	if p.token == EOFToken {
		return fmt.Errorf("invalid s-expression, unexpected end of file")
	}
	if p.token != LeftParenToken {
		return p.advance()
	}
	if err := p.advance(); err != nil {
		return err
	}
	for p.token != RightParenToken {
		if err := p.skipSExpr(); err != nil {
			return err
		}
	}
	return p.advance()
}

func (p *commandParser) expressions() ([]ast.ASTHandle, error) {
	if err := p.expect(LeftParenToken); err != nil {
		return nil, err
	}
	var values []ast.ASTHandle
	for p.token != RightParenToken {
		value, _, err := p.expression()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, p.advance()
}

func (p *commandParser) assertionExpression() (ast.ASTHandle, string, string, error) {
	if !p.context.InteractiveMode {
		expression, name, err := p.expression()
		return expression, name, "", err
	}
	compound := p.token == LeftParenToken
	p.scanner.StartCaching()
	defer func() {
		p.scanner.StopCaching()
		p.scanner.ResetCache()
	}()
	expression, name, err := p.expression()
	if err != nil {
		return ast.ASTHandle{}, "", "", err
	}
	text, err := p.scanner.CachedString(0, p.cacheEnd)
	if err != nil {
		return ast.ASTHandle{}, "", "", err
	}
	if compound {
		text = "(" + text
	}
	return expression, name, text, nil
}

// Source parse_get_value caches each original term before expanding
// definitions, lets, and coercions. Evaluate the AST, but echo that text.
func (p *commandParser) valueExpressions() ([]ast.ASTHandle, []string, error) {
	p.scanner.StartCaching()
	defer func() { p.scanner.StopCaching(); p.scanner.ResetCache() }()
	if err := p.expect(LeftParenToken); err != nil {
		return nil, nil, err
	}
	var values []ast.ASTHandle
	var text []string
	begin := 0
	for p.token != RightParenToken {
		value, _, err := p.expression()
		if err != nil {
			return nil, nil, err
		}
		ground, err := p.manager.IsGround(value)
		if err != nil {
			return nil, nil, err
		}
		quantifiers, err := p.manager.HasQuantifiers(value)
		if err != nil {
			return nil, nil, err
		}
		if !ground || quantifiers {
			return nil, nil, fmt.Errorf("invalid get-value term, term must be ground and must not contain quantifiers")
		}
		term, err := p.scanner.CachedString(begin, p.cacheEnd)
		if err != nil {
			return nil, nil, err
		}
		begin = p.cacheEnd
		values = append(values, value)
		text = append(text, term)
	}
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("invalid get-value command, empty list of terms")
	}
	return values, text, p.advance()
}

// parseIndexed consumes (_ name index...), used for sorts and applications.
func (p *commandParser) indexed() (string, []uint32, error) {
	name, err := p.symbol()
	if err != nil {
		return "", nil, err
	}
	var indices []uint32
	for p.token != RightParenToken {
		value, err := p.unsigned()
		if err != nil {
			return "", nil, err
		}
		indices = append(indices, value)
	}
	return name, indices, p.advance()
}

// qualifiedIdentifier parses the identifier portion accepted by source
// parse_indexed_identifier: either a symbol or an indexed `(_ name ...)`.
func (p *commandParser) qualifiedIdentifier() (string, []identifierIndex, error) {
	if p.token == SymbolToken {
		name, err := p.symbol()
		return name, nil, err
	}
	if p.token != LeftParenToken {
		return "", nil, errors.New("invalid (indexed) identifier, '(_' or symbol expected")
	}
	if err := p.advance(); err != nil {
		return "", nil, err
	}
	if p.token != SymbolToken || p.scanner.ID() != "_" {
		return "", nil, errors.New("invalid indexed identifier, '_' expected")
	}
	if err := p.advance(); err != nil {
		return "", nil, err
	}
	return p.indexedExpression()
}

// unknownSort is smt2parser::unknown_sort(id, context).
func (p *commandParser) unknownSort(name string) error {
	msg := p.sortContext
	if msg != "" {
		msg += ": "
	}
	return errors.New(msg + "unknown sort '" + name + "'")
}

// unknownSortError is smt2parser::unknown_sort(id) without a context, as
// parse_indexed_sort and push_sort_app_frame call it.
func unknownSortError(name string) error {
	return errors.New("unknown sort '" + name + "'")
}

// sortContextError is parse_sort's std::string(context) + " " + message.
func (p *commandParser) sortContextError(message string) error {
	if p.sortContext == "" {
		return errors.New(message)
	}
	return errors.New(p.sortContext + " " + message)
}

// sortIn is parse_sort(context): the context prefixes unknown-sort errors.
func (p *commandParser) sortIn(context string) (ast.ASTHandle, error) {
	saved := p.sortContext
	p.sortContext = context
	defer func() { p.sortContext = saved }()
	return p.sort()
}

func (p *commandParser) sort() (ast.ASTHandle, error) {
	// cmd_context::reset_assertions falls through to reset(false) when global
	// declarations are disabled. The native extension parser then performs
	// sort-name lookup before lazy manager initialization, so its next Int sort
	// lookup fails until another AST operation initializes the manager.
	if p.freshResetSortGap && p.token == SymbolToken && p.scanner.ID() == "Int" {
		return ast.ASTHandle{}, p.unknownSort("Int")
	}
	m, err := p.astManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if p.token == SymbolToken {
		// parse_sort_name resolves the identifier before next(), so its
		// errors carry the sort token's own position.
		name := p.scanner.ID()
		if formal, found := p.sortParams[name]; found {
			if err := p.advance(); err != nil {
				return ast.ASTHandle{}, err
			}
			return p.pin(formal, nil)
		}
		if arity, found := p.datatypeHeaders[name]; found {
			if arity != 0 {
				return ast.ASTHandle{}, errors.New("sort constructor expects parameters")
			}
			if err := p.advance(); err != nil {
				return ast.ASTHandle{}, err
			}
			return p.currentDatatypeSort(name, nil)
		}
		value, err := p.context.LookupSort(name)
		var unknown *cmd_context.UnknownSortError
		if errors.As(err, &unknown) {
			return ast.ASTHandle{}, p.unknownSort(name)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = p.advance(); err != nil {
			return ast.ASTHandle{}, err
		}
		return p.pin(value, nil)
	}
	// parse_sort(context): a ')' with no open sort frame, a token that is
	// neither identifier nor '(', and a non-identifier after '(' each have
	// their own message.
	if p.token == RightParenToken {
		return ast.ASTHandle{}, p.sortContextError("invalid sort, unexpected ')'")
	}
	if p.token != LeftParenToken {
		return ast.ASTHandle{}, fmt.Errorf("invalid sort, symbol, '_' or '(' expected")
	}
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	if p.token != SymbolToken {
		return ast.ASTHandle{}, p.sortContextError("invalid sort, symbol or '_' expected")
	}
	// push_sort_app_frame looks the constructor up before next(), so an
	// unknown sort reports the name token's own position.
	nameLine, namePos := p.scanner.Line(), p.scanner.Position()
	name, err := p.symbol()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if name == "_" {
		// parse_indexed_sort: check_identifier_next consumes the sort name
		// before find_psort_decl, so the error is at the following token.
		name, err := p.symbol()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !p.context.BuiltinSortAvailable(name) {
			return ast.ASTHandle{}, unknownSortError(name)
		}
		var indices []uint32
		for p.token != RightParenToken {
			value, err := p.unsigned()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			indices = append(indices, value)
		}
		// parse_indexed_sort instantiates the sort while ')' is still the
		// current token, then calls next().
		var indexed ast.ASTHandle
		if name == "FloatingPoint" && len(indices) == 2 {
			indexed, err = p.pin(m.FloatingPointSort(indices[0], indices[1]))
		} else if name == "FloatingPoint" {
			// fpa_decl_plugin::mk_sort, FLOATING_POINT_SORT case.
			return ast.ASTHandle{}, fmt.Errorf("expecting two integer parameters to floating point sort (ebits, sbits)")
		} else if name != "BitVec" || len(indices) != 1 {
			return ast.ASTHandle{}, fmt.Errorf("unsupported indexed sort %s", name)
		} else {
			indexed, err = p.pin(m.BitVectorSort(indices[0]))
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = p.advance(); err != nil {
			return ast.ASTHandle{}, err
		}
		// psort_builtin_decl::instantiate saves the indexed sort info.
		return indexed, p.context.SaveIndexedSortInfo(indexed, name, indices)
	}
	if _, found := p.datatypeHeaders[name]; !found && !p.context.SortDeclared(name) {
		return ast.ASTHandle{}, &ScannerError{Message: unknownSortError(name).Error(), Line: nameLine, Column: namePos}
	}
	if name == "Array" {
		var sorts []ast.ASTHandle
		for p.token != RightParenToken {
			value, err := p.sort()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			sorts = append(sorts, value)
		}
		// pop_sort_app_frame instantiates the sort (array_decl_plugin::
		// mk_sort) while ')' is still the current token.
		if len(sorts) < 2 {
			return ast.ASTHandle{}, fmt.Errorf("invalid array sort definition, invalid number of parameters")
		}
		if err = p.expect(RightParenToken); err != nil {
			return ast.ASTHandle{}, err
		}
		array, err := p.pin(m.MkArraySort(ast.SliceArguments(sorts[:len(sorts)-1]), sorts[len(sorts)-1]))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return array, p.context.SaveSortInfo(array, name, sorts)
	}
	if name == "Seq" || name == "RegEx" {
		var sorts []ast.ASTHandle
		for p.token != RightParenToken {
			value, err := p.sort()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			sorts = append(sorts, value)
		}
		if len(sorts) != 1 {
			if name == "Seq" {
				return ast.ASTHandle{}, fmt.Errorf("Invalid sequence sort, expecting one parameter")
			}
			return ast.ASTHandle{}, fmt.Errorf("Invalid regex sort, expecting one parameter")
		}
		if err = p.expect(RightParenToken); err != nil {
			return ast.ASTHandle{}, err
		}
		family, err := m.SequenceFamilyID()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		parameter, err := m.NewParameterAST(sorts[0])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		kind := int32(0)
		if name == "RegEx" {
			kind = 1
		}
		value, err := m.MkFamilySort(family, kind, []ast.ASTParameter{parameter})
		if err = errors.Join(err, m.DelParameter(&parameter)); err != nil {
			return ast.ASTHandle{}, err
		}
		if value, err = p.pin(value, nil); err != nil {
			return ast.ASTHandle{}, err
		}
		return value, p.context.SaveSortInfo(value, name, sorts)
	}
	var arguments []ast.ASTHandle
	for p.token != RightParenToken {
		value, e := p.sort()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments = append(arguments, value)
	}
	// pop_sort_app_frame checks the arity and instantiates at the closing
	// paren, before next() consumes it.
	var result ast.ASTHandle
	if arity, found := p.datatypeHeaders[name]; found {
		if uint32(len(arguments)) != arity {
			return ast.ASTHandle{}, errors.New("invalid number of parameters to sort constructor")
		}
		result, err = p.currentDatatypeSort(name, arguments)
	} else {
		result, err = p.pin(p.context.InstantiateSort(name, arguments))
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return result, p.expect(RightParenToken)
}

func (p *commandParser) indexedApplication(name string, indices []identifierIndex, args []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	parameters := make([]ast.ASTParameter, len(indices))
	defer func() {
		for i := range parameters {
			if parameters[i] != (ast.ASTParameter{}) {
				_ = p.manager.DelParameter(&parameters[i])
			}
		}
	}()
	for i, index := range indices {
		var err error
		if index.function.IsNull() {
			parameters[i], err = p.manager.NewParameterUint(index.number)
		} else {
			parameters[i], err = p.manager.NewParameterAST(index.function)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	ref, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.refs = append(p.refs, ref)
	if err = p.context.MakeAppInto(name, args, parameters, rangeSort, ref); err != nil {
		return ast.ASTHandle{}, err
	}
	last := len(p.refs) - 1
	p.refs[last] = ast.ASTRef{}
	p.refs = p.refs[:last]
	p.exprs.pushOwned(ref)
	return p.manager.RefGet(ref)
}

// parseBitVectorName is is_bv_num's syntax check: bv[0-9]+, bvbin[0-1]+
// or bvhex[0-9a-fA-F]+. It returns the digits' radix and text.
func parseBitVectorName(name string) (int, string, bool) {
	base, digits := 10, ""
	switch {
	case strings.HasPrefix(name, "bvbin"):
		base, digits = 2, name[len("bvbin"):]
	case strings.HasPrefix(name, "bvhex"):
		base, digits = 16, name[len("bvhex"):]
	case strings.HasPrefix(name, "bv"):
		digits = name[len("bv"):]
	default:
		return 0, "", false
	}
	if digits == "" {
		return 0, "", false
	}
	for _, digit := range digits {
		valid := '0' <= digit && digit <= '9'
		if base == 2 {
			valid = digit == '0' || digit == '1'
		} else if base == 16 {
			valid = ('0' <= digit && digit <= '9') || ('a' <= digit && digit <= 'f') || ('A' <= digit && digit <= 'F')
		}
		if !valid {
			return 0, "", false
		}
	}
	return base, digits, true
}

// bitVectorNameValue is is_bv_decimal / is_bv_binary / is_bv_hex's
// accumulation into m_last_bv_numeral: n = n*base + digit.
func bitVectorNameValue(q *unumeral.RationalManager, base int, digits string, n *unumeral.Rational) (resultErr error) {
	radix, digit := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&radix), q.Del(&digit)) }()
	if err := q.SetInt32(&radix, int32(base)); err != nil {
		return err
	}
	if err := q.SetInt32(n, 0); err != nil {
		return err
	}
	for _, c := range digits {
		var v int32
		switch {
		case '0' <= c && c <= '9':
			v = c - '0'
		case 'a' <= c && c <= 'f':
			v = 10 + c - 'a'
		default:
			v = 10 + c - 'A'
		}
		if err := q.Mul(n, &radix, n); err != nil {
			return err
		}
		if err := q.SetInt32(&digit, v); err != nil {
			return err
		}
		if err := q.Add(n, &digit, n); err != nil {
			return err
		}
	}
	return nil
}

func (p *commandParser) parseBVNumeral(name string, indices []identifierIndex) (result ast.ASTHandle, resultErr error) {
	base, digits, ok := parseBitVectorName(name)
	if !ok {
		return ast.ASTHandle{}, fmt.Errorf("invalid bit-vector numeral")
	}
	if len(indices) != 1 || !indices[0].function.IsNull() {
		return ast.ASTHandle{}, fmt.Errorf("invalid bit-vector constant, index expected")
	}
	// A decimal SMT-LIB bv numeral is integral and may exceed machine width.
	q := p.scanner.Numbers()
	value := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&value)) }()
	if err := bitVectorNameValue(q, base, digits, &value); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.pinExpression(p.manager.BitVectorNumeralFromRational(&value, indices[0].number))
}
