package main

// g3 executes solver input files through the native Go frontends.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/glycerine/g3/pkg/api"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/shell"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func run(input io.Reader, output io.Writer) (result error) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, context.Close()) }()
	frontend, err := shell.NewSMTLIBFrontend(shell.SMTLIBFrontendConfig{Context: context, Input: input, Output: output})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, frontend.Close()) }()
	return frontend.Run()
}

func runArguments(args []string, input io.Reader, output io.Writer) error {
	return runArgumentsWithDiagnostics(args, input, output, nil)
}

func runArgumentsWithDiagnostics(args []string, input io.Reader, output, diagnostics io.Writer) (result error) {
	context, memoryPolicy, err := cmd_context.NewStandaloneShellContext()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, context.Close()) }()
	globals, err := context.GlobalParameters()
	if err != nil {
		return err
	}
	command := shell.NewCommandLine(nil, nil, globals)
	if err = command.BeginMemoryPolicy(memoryPolicy); err != nil {
		return err
	}
	if err = command.Parse(args); err != nil {
		return err
	}
	if command.Action == shell.CommandHelp {
		_, err = fmt.Fprintln(output, "g3 Go SMT and optimization runner\nUsage: g3 [-smt2|-dimacs|-bpo|-wcnf|-log] <file> | g3 [-smt2|-dimacs|-bpo|-wcnf|-log] -in\n       g3 -dl <file-or-directory> (also selected by .dl or .datalog)\nGlobal parameter options: name=value; -p, -pd, -pm, -pp:name")
		return err
	}
	if command.Action == shell.CommandVersion {
		_, err = fmt.Fprintln(output, "g3 (Go SMT-LIB backend)")
		return err
	}
	if command.Action != shell.CommandContinue {
		return command.WriteTerminalAction(output)
	}
	if err = uparams.UpdateProcessEnvParams(globals.Current); err != nil {
		return err
	}
	filename, selected := command.Input.Selected()
	if selected && command.StandardInput {
		return &shell.CommandLineError{Message: "using standard input to read formula.", Cause: shell.ErrShellInputConflict}
	}
	if !selected && !command.StandardInput {
		return &shell.CommandLineError{Message: "input file was not specified.", Cause: shell.ErrShellInputMissing}
	}
	kind, err := command.ConfigureMemoryPolicyForDispatch(memoryPolicy)
	if err != nil {
		return err
	}
	// Source main dispatches Datalog by path; the frontend distinguishes a
	// legacy file from a WPA directory and opens its own inputs.
	if kind == shell.InputDatalog {
		if !selected {
			return fmt.Errorf("Datalog input requires a file or directory path")
		}
		if err = context.RefreshGlobalParameters(); err != nil {
			return err
		}
		return shell.RunDatalogFrontend(context, filename, output)
	}
	if kind != shell.InputSMT2 && kind != shell.InputDIMACS && kind != shell.InputOPB && kind != shell.InputWCNF && kind != shell.InputZ3Log && kind != shell.InputMPS {
		return fmt.Errorf("selected input frontend is not ported yet")
	}
	if selected && kind != shell.InputSMT2 && kind != shell.InputOPB && kind != shell.InputWCNF {
		file, err := os.Open(filename)
		if err != nil {
			if kind == shell.InputZ3Log {
				return &z3LogOpenError{filename: filename, cause: err}
			}
			return &shellInputOpenError{filename: filename, cause: err}
		}
		defer func() { result = errors.Join(result, file.Close()) }()
		input = file
	}
	if err = context.RefreshGlobalParameters(); err != nil {
		return err
	}
	if kind == shell.InputSMT2 {
		var frontend *shell.SMTLIBFrontend
		if selected {
			frontend, err = shell.OpenSMTLIBFrontend(filename, shell.SMTLIBFileFrontendConfig{
				Context: context, Output: output, Diagnostics: diagnostics,
				DisplayStatistics: command.DisplayStatistics, Timer: command.Timer,
			})
		} else {
			frontend, err = shell.NewSMTLIBFrontend(shell.SMTLIBFrontendConfig{
				Context: context, Input: input, Output: output, Diagnostics: diagnostics,
				DisplayStatistics: command.DisplayStatistics, Timer: command.Timer,
			})
		}
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, frontend.Close()) }()
		return frontend.Run()
	}
	if kind == shell.InputZ3Log {
		if diagnostics == nil {
			diagnostics = io.Discard
		}
		replayer, err := api.NewZ3Replayer(input, output)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, replayer.Close()) }()
		if err = api.RegisterZ3GeneratedGlobalCommands(replayer); err != nil {
			return err
		}
		if err = api.RegisterZ3GeneratedConfigCommands(replayer); err != nil {
			return err
		}
		if err = api.RegisterZ3GeneratedContextCommands(replayer); err != nil {
			return err
		}
		if err = api.RegisterZ3GeneratedParameterCommands(replayer); err != nil {
			return err
		}
		if err = api.RegisterZ3GeneratedSolverCommands(replayer); err != nil {
			return err
		}
		if err = api.RegisterZ3GeneratedModelCommands(replayer); err != nil {
			return err
		}
		if err = api.RegisterZ3GeneratedSymbolCommands(replayer); err != nil {
			return err
		}
		if err = api.RegisterZ3GeneratedASTCommands(replayer); err != nil {
			return err
		}
		if err = api.RegisterZ3GeneratedProcessCommands(replayer); err != nil {
			return err
		}
		if err = engine.InitializeDefaultProcess(); err != nil {
			return err
		}
		processGlobals := uparams.BorrowProcessGlobalParamsService()
		if processGlobals == nil || processGlobals.Context == nil {
			return api.ErrZ3ReplayState
		}
		process := processGlobals.Context.Numerals().RuntimeOwner().ProcessOwner()
		return api.ReplayZ3LogStream(replayer, output, diagnostics, process, nil)
	}
	if kind == shell.InputOPB {
		frontend, err := shell.NewOPBFrontend(context, output, shell.OPBFrontendOptions{DisplayStatistics: command.DisplayStatistics, Timer: command.Timer, Diagnostics: diagnostics})
		if err != nil {
			return err
		}
		if selected {
			return frontend.RunFile(filename)
		}
		return frontend.Run(input)
	}
	if kind == shell.InputWCNF {
		frontend, err := shell.NewWCNFFrontend(context, output, shell.WCNFFrontendOptions{DisplayStatistics: command.DisplayStatistics, Timer: command.Timer, Diagnostics: diagnostics})
		if err != nil {
			return err
		}
		if selected {
			return frontend.RunFile(filename)
		}
		return frontend.Run(input)
	}
	if kind == shell.InputMPS {
		frontend, err := shell.NewLPFrontend(shell.LPFrontendConfig{Globals: globals, Input: input, Output: output, Name: filename})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, frontend.Close()) }()
		return frontend.Run()
	}
	if kind == shell.InputDIMACS {
		frontend, err := shell.NewDIMACSFrontend(shell.DIMACSFrontendConfig{Globals: globals, Timer: command.Timer, Input: input, Writer: output, DisplayStatistics: command.DisplayStatistics})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, frontend.Close()) }()
		return frontend.Run()
	}
	return fmt.Errorf("selected input frontend %s is not ported yet", shell.InputKindName(kind))
}

// defaultGCPercent trades heap for time in this short-lived process: it measured
// up to 30% faster for 1.2 to 2.2 times the resident memory.
const defaultGCPercent = 400

// configureGC applies the default unless the user set GOGC, and reports whether
// it did. Library users of the solver packages keep their own GC settings.
func configureGC(lookup func(string) (string, bool), set func(int) int) bool {
	if _, chosen := lookup("GOGC"); chosen {
		return false
	}
	set(defaultGCPercent)
	return true
}

func main() {
	configureGC(os.LookupEnv, debug.SetGCPercent)
	if err := runArgumentsWithDiagnostics(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		var commandError *shell.CommandLineError
		if errors.As(err, &commandError) {
			_ = shell.WriteCommandError(os.Stderr, commandError.Message)
			os.Exit(int(z3exception.ErrorCmdLine))
		}
		var reportedParseError *smt2.ReportedParseError
		if errors.As(err, &reportedParseError) {
			// read_smtlib2_commands returns result ? 0 : 1 (smtlib_frontend.cpp);
			// the shell never uses ERR_PARSER for SMT2 input.
			os.Exit(1)
		}
		var openError *z3LogOpenError
		if errors.As(err, &openError) {
			fmt.Fprintln(os.Stderr, openError.Error())
			os.Exit(int(z3exception.ErrorOpenFile))
		}
		var optimizationFileError *shell.OptimizationInputOpenError
		if errors.As(err, &optimizationFileError) {
			fmt.Fprintln(os.Stderr, optimizationFileError.Error())
			os.Exit(int(z3exception.ErrorOpenFile))
		}
		var smtlibFileError *shell.SMTLIBFileOpenError
		if errors.As(err, &smtlibFileError) {
			fmt.Fprintf(os.Stderr, "(error \"failed to open file '%s'\")\n", smtlibFileError.Name)
			os.Exit(int(z3exception.ErrorOpenFile))
		}
		var inputOpenError *shellInputOpenError
		if errors.As(err, &inputOpenError) {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", inputOpenError.Error())
			os.Exit(int(z3exception.ErrorOpenFile))
		}
		var exception z3exception.Z3Exception
		code := z3exception.ErrorInternalFatal
		if errors.As(err, &exception) && exception.HasErrorCode() {
			code = exception.ErrorCode()
		}
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(int(code))
	}
}
