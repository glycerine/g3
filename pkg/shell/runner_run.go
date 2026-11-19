package shell

// Source: src/shell/main.cpp one-shot executable composition.

import (
	"context"
	"errors"
)

func RunShell(c RunnerConfig) error { return RunShellContext(context.Background(), c) }

func RunShellContext(ctx context.Context, c RunnerConfig) (result error) {
	runner, err := NewRunner(c)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, runner.Close()) }()
	return runner.RunContext(ctx)
}
