package fp

import "slices"

// sortedVariableIndexes lists a variable -> column map's variables
// ascending (deterministic; Go map order is random).
func sortedVariableIndexes(variables map[uint32]uint32) []uint32 {
	keys := make([]uint32, 0, len(variables))
	for variable := range variables {
		keys = append(keys, variable)
	}
	slices.Sort(keys)
	return keys
}
