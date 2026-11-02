package duality

// Source: src/muz/duality/duality_dl_interface.cpp, dl_interface::get_model.

import (
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/model"
)

// GetModel returns an independently owned snapshot of the active annotations.
func (e *QueryEngine) GetModel() (*model.Model, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.active == nil {
		return nil, graph.ErrWrapperOwner
	}
	return e.active.graph.AnnotationModel()
}

func (b *Backend) GetModel() (*model.Model, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	return b.queries.GetModel()
}
