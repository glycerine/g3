package base

// TableCapacitiesForTest reports m_head2rules' and m_deps' table
// capacities (0 before their first use).
func (s *RuleSet) TableCapacitiesForTest() (head, deps int) {
	if s.headTable != nil {
		head = s.headTable.Capacity()
	}
	if s.depsTable != nil {
		deps = s.depsTable.Capacity()
	}
	return head, deps
}
