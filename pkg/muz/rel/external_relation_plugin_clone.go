package rel

// Source: src/api/api_datalog.cpp fixedpoint_context: one external relation
// context per datalog::context.

// CloneForManager makes the external relation plugin of a g3 query clone's
// relation manager with this plugin's callbacks; the clone shares the
// context's relation terms (ShareRelation).
func (p *ExternalRelationPlugin) CloneForManager(destination *RelationManager) (*ExternalRelationPlugin, error) {
	if p == nil || p.closed || p.manager == nil || p.manager.open() != nil || destination == nil || destination.open() != nil || destination.manager != p.ast {
		return nil, ErrTableRelation
	}
	return NewExternalRelationPlugin(destination, p.callbacks)
}
