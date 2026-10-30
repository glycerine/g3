package base_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type engineDispatchCore struct {
	manager                              *ast.ASTManager
	answer                               ast.ASTRef
	reset, collect, profile              int
	certificate, cancel, cleanup, update int
	failure                              error
}

func (c *engineDispatchCore) Query(ast.ASTHandle) (lbool.LBool, error) {
	return lbool.True, nil
}
func (c *engineDispatchCore) Answer() (ast.ASTRef, error) { return c.manager.CopyRef(c.answer) }
func (c *engineDispatchCore) ResetStatistics() error {
	c.reset++
	return c.failure
}
func (c *engineDispatchCore) CollectStatistics(destination *statistics.Statistics) error {
	c.collect++
	destination.UpdateUint("dispatch", 3)
	return c.failure
}
func (c *engineDispatchCore) DisplayProfile(out io.Writer) error {
	c.profile++
	if c.failure != nil {
		return c.failure
	}
	_, err := io.WriteString(out, "profile")
	return err
}
func (c *engineDispatchCore) DisplayCertificate(out io.Writer) error {
	c.certificate++
	if c.failure != nil {
		return c.failure
	}
	_, err := io.WriteString(out, "certificate")
	return err
}
func (c *engineDispatchCore) Cancel() error {
	c.cancel++
	return c.failure
}
func (c *engineDispatchCore) Cleanup() error {
	c.cleanup++
	return c.failure
}
func (c *engineDispatchCore) UpdateParameters() error {
	c.update++
	return c.failure
}

func newEngineDispatchFixture(t *testing.T) (*engineDispatchCore, *base.EngineBase) {
	t.Helper()
	f := newRuleSetFixture(t)
	truth, err := f.manager.MkTrue()
	ruleMust(t, err)
	answer, err := f.manager.NewRef(truth)
	ruleMust(t, err)
	core := &engineDispatchCore{manager: f.manager, answer: answer}
	engine, err := base.NewEngineBase(f.manager, "dispatch", core)
	ruleMust(t, err)
	t.Cleanup(func() {
		ruleMust(t, engine.Close())
		ruleMust(t, f.manager.DelRef(&answer))
	})
	return core, engine
}

func TestEngineBaseDispatchesStatisticsAndProfile(t *testing.T) {
	core, engine := newEngineDispatchFixture(t)
	ruleMust(t, engine.ResetStatistics())
	values := statistics.New()
	ruleMust(t, engine.CollectStatistics(values))
	var out bytes.Buffer
	ruleMust(t, engine.DisplayProfile(&out))
	if core.reset != 1 || core.collect != 1 || core.profile != 1 || values.Size() != 1 || out.String() != "profile" {
		t.Fatal("reporting dispatch", core.reset, core.collect, core.profile, values.Size(), out.String())
	}
}

var errEngineDispatch = errors.New("engine dispatch failure")
