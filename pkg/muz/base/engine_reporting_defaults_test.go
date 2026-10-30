package base_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestEngineBaseReportingDefaultsAndArgumentGuards(t *testing.T) {
	f := newRuleSetFixture(t)
	truth, err := f.manager.MkTrue()
	ruleMust(t, err)
	answer, err := f.manager.NewRef(truth)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&answer)) }()
	engine, err := base.NewEngineBase(f.manager, "default-reporting", &engineFake{manager: f.manager, answer: answer})
	ruleMust(t, err)
	defer func() { ruleMust(t, engine.Close()) }()
	ruleMust(t, engine.ResetStatistics())
	ruleMust(t, engine.CollectStatistics(statistics.New()))
	var out bytes.Buffer
	ruleMust(t, engine.DisplayProfile(&out))
	if out.Len() != 0 {
		t.Fatal("default profile produced output")
	}
	if err = engine.CollectStatistics(nil); !errors.Is(err, base.ErrEngineBase) {
		t.Fatal("nil statistics accepted", err)
	}
	if err = engine.DisplayProfile(nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("nil profile writer accepted", err)
	}
}
