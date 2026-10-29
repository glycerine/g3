package api

func (r *Z3Replayer) pushArray(size uint64, element z3ReplayKind) error {
	if size > uint64(len(r.args)) {
		return r.failure("invalid array size")
	}
	start := len(r.args) - int(size)
	for i := start; i < len(r.args); i++ {
		if r.args[i].kind != element {
			return r.failure("invalid array: mixed value types")
		}
	}
	var value z3ReplayValue
	switch element {
	case replayUint64:
		row := make([]uint32, size)
		for i := range row {
			row[i] = uint32(r.args[start+i].u)
		}
		value = z3ReplayValue{kind: replayUintArray, u: uint64(len(r.uintArrays))}
		r.uintArrays = append(r.uintArrays, row)
	case replayInt64:
		row := make([]int32, size)
		for i := range row {
			row[i] = int32(r.args[start+i].i)
		}
		value = z3ReplayValue{kind: replayIntArray, u: uint64(len(r.intArrays))}
		r.intArrays = append(r.intArrays, row)
	case replaySymbol:
		row := make([]string, size)
		for i := range row {
			row[i] = r.args[start+i].s
		}
		value = z3ReplayValue{kind: replaySymbolArray, u: uint64(len(r.symbolArrays))}
		r.symbolArrays = append(r.symbolArrays, row)
	case replayObject:
		row := make([]uint64, size)
		for i := range row {
			row[i] = r.args[start+i].u
		}
		value = z3ReplayValue{kind: replayObjectArray, u: uint64(len(r.objectArrays))}
		r.objectArrays = append(r.objectArrays, row)
	default:
		return r.failure("unsupported array type")
	}
	r.args = append(r.args[:start], value)
	return nil
}

func (r *Z3Replayer) UintArray(p uint32) ([]uint32, error) {
	v, err := r.value(p, replayUintArray)
	if err != nil {
		return nil, err
	}
	return r.uintArrays[v.u], nil
}
func (r *Z3Replayer) IntArray(p uint32) ([]int32, error) {
	v, err := r.value(p, replayIntArray)
	if err != nil {
		return nil, err
	}
	return r.intArrays[v.u], nil
}
func (r *Z3Replayer) SymbolArray(p uint32) ([]string, error) {
	v, err := r.value(p, replaySymbolArray)
	if err != nil {
		return nil, err
	}
	return r.symbolArrays[v.u], nil
}
func (r *Z3Replayer) ObjectArray(p uint32) ([]uint64, error) {
	v, err := r.value(p, replayObjectArray)
	if err != nil {
		return nil, err
	}
	return r.objectArrays[v.u], nil
}
