package api

// Source: src/api/z3_replayer.h; src/api/z3_replayer.cpp.

import (
	"bufio"
	"io"
)

type ReplayCommand func(*Z3Replayer) error

type replayCommandEntry struct {
	name string
	call ReplayCommand
}

type Z3Replayer struct {
	scanner      *z3ReplayScanner
	output       io.Writer
	args         []z3ReplayValue
	result       uint64
	heap         map[uint64]uint64
	commands     map[uint32]replayCommandEntry
	objectArrays [][]uint64
	symbolArrays [][]string
	uintArrays   [][]uint32
	intArrays    [][]int32
	closed       bool
}

func NewZ3Replayer(input io.Reader, output io.Writer) (*Z3Replayer, error) {
	if input == nil || output == nil {
		return nil, ErrZ3ReplayState
	}
	return &Z3Replayer{scanner: newZ3ReplayScanner(bufio.NewReader(input)), output: output, heap: make(map[uint64]uint64), commands: make(map[uint32]replayCommandEntry)}, nil
}

func (r *Z3Replayer) Line() uint32 {
	if r == nil || r.scanner == nil {
		return 0
	}
	return r.scanner.line
}

func (r *Z3Replayer) Close() error {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true
	r.resetArguments()
	r.heap, r.commands, r.scanner, r.output = nil, nil, nil, nil
	return nil
}
