package main

// g3 executes solver input files through the native Go frontends.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/api"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/shell"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

func run(input io.Reader, output io.Writer) (result error) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, context.Close()) }()
	return smt2.ParseShellCommands(context, input, output)
}

func runArguments(args []string, input io.Reader, output io.Writer) error {
	return runArgumentsWithDiagnostics(args, input, output, nil)
}

func runArgumentsWithDiagnostics(args []string, input io.Reader, output, diagnostics io.Writer) (result error) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, context.Close()) }()
	globals, err := context.GlobalParameters()
	if err != nil {
		return err
	}
	command := shell.NewCommandLine(nil, nil, globals)
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
	filename, selected := command.Input.Selected()
	if selected && command.StandardInput {
		return fmt.Errorf("using standard input to read formula")
	}
	if !selected && !command.StandardInput {
		return fmt.Errorf("input file was not specified")
	}
	kind, err := command.ResolveInputKind()
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
	if kind != shell.InputSMT2 && kind != shell.InputDIMACS && kind != shell.InputOPB && kind != shell.InputWCNF && kind != shell.InputZ3Log {
		return fmt.Errorf("selected input frontend is not ported yet")
	}
	if selected {
		file, err := os.Open(filename)
		if err != nil {
			if kind == shell.InputZ3Log {
				return &z3LogOpenError{filename: filename, cause: err}
			}
			return err
		}
		defer func() { result = errors.Join(result, file.Close()) }()
		input = file
	}
	if err = context.RefreshGlobalParameters(); err != nil {
		return err
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
		frontend, err := shell.NewOPBFrontend(context, output, shell.OPBFrontendOptions{DisplayStatistics: command.DisplayStatistics})
		if err != nil {
			return err
		}
		return frontend.Run(input)
	}
	if kind == shell.InputWCNF {
		frontend, err := shell.NewWCNFFrontend(context, output, shell.WCNFFrontendOptions{DisplayStatistics: command.DisplayStatistics})
		if err != nil {
			return err
		}
		return frontend.Run(input)
	}
	if kind == shell.InputDIMACS {
		frontend, err := shell.NewDIMACSFrontend(shell.DIMACSFrontendConfig{Globals: globals, Input: input, Writer: output, DisplayStatistics: command.DisplayStatistics})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, frontend.Close()) }()
		return frontend.Run()
	}
	if err = context.SetShellDiagnosticOutput(diagnostics); err != nil {
		return err
	}
	total := stopwatch.New(nil)
	if command.DisplayStatistics {
		if err = total.Start(); err != nil {
			return err
		}
	}
	parseErr := smt2.ParseShellCommands(context, input, output)
	if command.DisplayStatistics {
		stopErr := total.Stop()
		seconds, secondsErr := total.Seconds()
		if stopErr == nil && secondsErr == nil {
			err = context.SetStringOption("regular-output-channel", "stdout")
			if err == nil {
				err = context.WriteStatisticsWithTotalTime(output, seconds)
			}
		}
		err = errors.Join(stopErr, secondsErr, err)
	}
	return errors.Join(parseErr, err)
}

func main() {
	if err := runArgumentsWithDiagnostics(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		var openError *z3LogOpenError
		if errors.As(err, &openError) {
			fmt.Fprintln(os.Stderr, openError.Error())
			os.Exit(108) // ERR_OPEN_FILE in src/util/error_codes.h.
		}
		fmt.Fprintf(os.Stdout, "(error \"%s\")\n", strings.ReplaceAll(err.Error(), "\"", "\"\""))
		os.Exit(1)
	}
}
