package base

// CopyRule directly copies source's native tail layout and assigns name.
func (m *RuleManager) CopyRule(source *Rule, name string) (*Rule, error) {
	head, err := source.Head()
	if err != nil {
		return nil, err
	}
	return m.cloneRuleState(source, head, name)
}

// CloneRule is the name-preserving convenience form of CopyRule.
func (m *RuleManager) CloneRule(source *Rule) (*Rule, error) {
	name, err := source.Name()
	if err != nil {
		return nil, err
	}
	return m.CopyRule(source, name)
}
