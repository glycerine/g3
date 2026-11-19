package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *WCNFFrontend) Run(input io.Reader) error {
	return f.RunContext(context.Background(), input)
}

func (f *WCNFFrontend) RunContext(ctx context.Context, input io.Reader) error {
	if input == nil {
		return ErrWCNFFrontend
	}
	return f.run(ctx, func() (io.Reader, io.Closer, error) { return input, nil, nil })
}

func (f *WCNFFrontend) run(ctx context.Context, openInput func() (io.Reader, io.Closer, error)) error {
	if f == nil || f.context == nil || openInput == nil || f.finished || !f.started.IsZero() {
		return ErrWCNFFrontend
	}
	f.started = time.Now()
	return runOptimizationFrontendProcess(ctx, f.context, f.output, func(stopped func() bool) (lbool.LBool, error) {
		input, inputCloser, err := openInput()
		if err != nil {
			return lbool.Undef, err
		}
		if input == nil {
			return lbool.Undef, ErrWCNFFrontend
		}
		if _, err = f.context.Manager(); err != nil {
			if inputCloser != nil {
				err = errors.Join(err, inputCloser.Close())
			}
			return lbool.Undef, err
		}
		f.initialized = true
		if stopped() {
			if inputCloser != nil {
				err = inputCloser.Close()
			}
			return lbool.Undef, err
		}
		document, err := opt.ParseWCNF(input)
		if inputCloser != nil {
			err = errors.Join(err, inputCloser.Close())
		}
		if err != nil {
			return lbool.Undef, err
		}
		if stopped() {
			return lbool.Undef, nil
		}
		f.document = document.Copy()
		f.summary, err = f.context.LoadWCNF(document)
		if err != nil {
			return lbool.Undef, err
		}
		if stopped() {
			return lbool.Undef, nil
		}
		f.context.SetOptimizationFrontend()
		status, err := f.context.CheckSat(nil)
		if err != nil {
			return status, err
		}
		message, found, err := f.context.OptimizationException()
		if err != nil {
			return lbool.Undef, err
		}
		if found {
			_, _ = fmt.Fprintln(f.diagnostics, message)
		}
		return status, nil
	}, func(status lbool.LBool) error {
		f.status = status
		f.elapsed = time.Since(f.started)
		f.finished = true
		return f.WriteResult()
	}, func() error {
		f.status = lbool.Undef
		f.elapsed = time.Since(f.started)
		f.finished = true
		return f.WriteProgress()
	})
}
