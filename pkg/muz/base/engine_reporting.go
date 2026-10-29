package base

import (
	"github.com/glycerine/g3/pkg/util/statistics"
	"io"
)

func (e *EngineBase) ResetStatistics() error                         { return e.open() }
func (e *EngineBase) CollectStatistics(*statistics.Statistics) error { return e.open() }
func (e *EngineBase) DisplayProfile(io.Writer) error                 { return e.open() }
func (e *EngineBase) DisplayCertificate(io.Writer) error {
	if x := e.open(); x != nil {
		return x
	}
	return e.unsupported("certificates")
}
func (e *EngineBase) Cancel() error           { return e.open() }
func (e *EngineBase) Cleanup() error          { return e.open() }
func (e *EngineBase) UpdateParameters() error { return e.open() }
