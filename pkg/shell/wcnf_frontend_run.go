package shell

import (
	"github.com/glycerine/g3/pkg/opt"
	"io"
	"time"
)

func (f *WCNFFrontend) Run(input io.Reader) error {
	if f == nil || f.context == nil || input == nil || f.finished || !f.started.IsZero() {
		return ErrWCNFFrontend
	}
	f.started = time.Now()
	d, e := opt.ParseWCNF(input)
	if e != nil {
		return e
	}
	f.document = d.Copy()
	f.summary, e = f.context.LoadWCNF(d)
	if e != nil {
		return e
	}
	f.status, e = f.context.CheckSat(nil)
	if e != nil {
		return e
	}
	f.elapsed = time.Since(f.started)
	f.finished = true
	return f.WriteResult()
}
