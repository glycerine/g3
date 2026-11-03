package fp

import (
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// The source relational query owns its transformer. Engines constructed
// without parameter services additionally own default services for the
// transform's theory rewriter; those services must outlive the transformer.
type datalogTransformOwners struct {
	transformer *base.RuleTransformer
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	catalog     *uparams.GlobalParamsCatalog
}

func (o *datalogTransformOwners) Close() error {
	if o.transformer != nil {
		if err := o.transformer.Close(); err != nil {
			return err
		}
		o.transformer = nil
	}
	if o.globals != nil {
		if err := o.globals.Close(); err != nil {
			return err
		}
		o.globals = nil
	}
	if o.catalog != nil {
		if err := o.catalog.Close(); err != nil {
			return err
		}
		o.catalog = nil
	}
	if o.context != nil {
		if err := o.context.Close(); err != nil {
			return err
		}
		o.context = nil
	}
	return nil
}

func (e *PropositionalEngine) clearDatalogTransformCleanup() error {
	if e.datalogTransformCleanup == nil {
		return nil
	}
	if err := e.datalogTransformCleanup.Close(); err != nil {
		return err
	}
	e.datalogTransformCleanup = nil
	return nil
}
