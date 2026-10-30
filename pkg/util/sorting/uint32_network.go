// Package sorting implements concrete source sorting networks used by g3.
package sorting

// Source: src/util/sorting_network.h:28-110, instantiated as unsigned_ext in
// src/test/sorting_network.cpp. Copyright (c) 2013 Nikolaj Bjorner.

// Uint32Network is the concrete unsigned specialization of the source network.
// Inputs are padded with zero to the next power of two, as in mk_default.
type Uint32Network struct {
	currentv      []uint32
	nextv         []uint32
	currentIsNext bool
}

func (s *Uint32Network) Sort(input []uint32) []uint32 {
	output := append([]uint32(nil), input...)
	if len(output) <= 1 {
		return output
	}
	for !powerOfTwo(uint32(len(output))) {
		output = append(output, 0)
	}

	current, next := &s.currentv, &s.nextv
	if s.currentIsNext {
		current, next = next, current
	}
	for i := range output {
		*current = append(*current, uint32(i))
		*next = append(*next, uint32(i))
	}
	for width := uint32(2); width <= uint32(len(output)); width <<= 1 {
		sourceSort(&current, &next, output, width)
	}
	s.currentIsNext = current == &s.nextv
	return output
}

func sourceSort(current, next **[]uint32, output []uint32, width uint32) {
	if width == 2 {
		for i := uint32(0); i < uint32(len(output))/2; i++ {
			exchange(output, (**current)[2*i], (**current)[2*i+1])
			(**next)[2*i] = (**current)[2*i]
			(**next)[2*i+1] = (**current)[2*i+1]
		}
		*current, *next = *next, *current
		return
	}
	for i := uint32(0); i < uint32(len(output))/width; i++ {
		base := width * i
		for j := uint32(0); j < width/2; j++ {
			(**next)[base+j] = (**current)[base+2*j]
			(**next)[base+width/2+j] = (**current)[base+2*j+1]
		}
	}
	*current, *next = *next, *current
	sourceSort(current, next, output, width/2)
	for i := uint32(0); i < uint32(len(output))/width; i++ {
		base := width * i
		for j := uint32(0); j < width/2; j++ {
			(**next)[base+2*j] = (**current)[base+j]
			(**next)[base+2*j+1] = (**current)[base+width/2+j]
		}
		for j := uint32(0); j < width/2-1; j++ {
			exchange(output, (**next)[base+2*j+1], (**next)[base+2*(j+1)])
		}
	}
	*current, *next = *next, *current
}

func exchange(values []uint32, i, j uint32) {
	if i >= j {
		return
	}
	left, right := values[i], values[j]
	if left <= right {
		values[i], values[j] = left, right
	} else {
		values[i], values[j] = right, left
	}
}

func powerOfTwo(value uint32) bool { return value != 0 && value&(value-1) == 0 }
