package fp

// Source: src/api/api_datalog.cpp fixedpoint_context external relation hooks.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *PropositionalEngine) externalRelations() (*rel.ExternalRelationPlugin, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if e.external == nil {
		plugin, err := rel.NewExternalRelationPlugin(e.relations, rel.ExternalRelationCallbacks{})
		if err != nil {
			return nil, err
		}
		e.external = plugin
	}
	return e.external, nil
}

// SetExternalRelationCallbacks installs the context-wide typed reductions
// used by subsequently created and already-live external relation values.
func (e *PropositionalEngine) SetExternalRelationCallbacks(callbacks rel.ExternalRelationCallbacks) error {
	plugin, err := e.externalRelations()
	if err != nil {
		return err
	}
	if err = plugin.SetCallbacks(callbacks); err != nil {
		return err
	}
	return e.registerExternalRelationPlugin()
}

// registerExternalRelationPlugin is fixedpoint_context::set_state
// (Z3_fixedpoint_init): get_rel_context() instantiates the configured engine
// and, when it is the datalog engine, its relation manager registers
// external_relation_plugin after rel_context's own relation plugins.
func (e *PropositionalEngine) registerExternalRelationPlugin() error {
	if e.externalPluginRegistered() {
		return nil
	}
	if err := e.selectFactEngine(); err != nil {
		return err
	}
	if e.backend != "datalog" {
		return nil
	}
	if err := e.registerRelContextPlugins(); err != nil {
		return err
	}
	return e.relations.RegisterRelationPlugin(e.external)
}

// externalPluginRegistered reports that the relation manager has the
// external relation plugin (get_relation_plugin("external_relation")).
func (e *PropositionalEngine) externalPluginRegistered() bool {
	return e.external != nil && e.relations != nil && e.relations.GetRelationPlugin("external_relation") != nil
}

// NewExternalRelation creates a callback-backed relation owned by this
// fixedpoint context. The relation must close before the engine closes.
func (e *PropositionalEngine) NewExternalRelation(sorts []ast.ASTHandle) (*rel.ExternalRelation, error) {
	plugin, err := e.externalRelations()
	if err != nil {
		return nil, err
	}
	return plugin.NewForeign(sorts)
}
