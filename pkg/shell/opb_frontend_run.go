package shell

import (
	"io"
	"time"

	"github.com/glycerine/g3/pkg/opt"
)

func (f *OPBFrontend) Run(input io.Reader) error {
	if f == nil || f.context == nil || input == nil || f.finished || !f.started.IsZero() {
		return ErrOPBFrontend
	}
	f.started = time.Now()
	document, err := opt.ParseOPB(input)
	if err != nil {
		return err
	}
	f.document = document.Copy()
	f.summary, err = f.context.LoadOPB(document)
	if err != nil {
		return err
	}
	f.status, err = f.context.CheckSat(nil)
	if err != nil {
		return err
	}
	f.elapsed = time.Since(f.started)
	f.finished = true
	return f.WriteResult()
}
