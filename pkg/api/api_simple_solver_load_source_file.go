package api

// Source: src/api/api_solver.cpp:161-185 Z3_solver_from_file.

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func (s *APISimpleSolver) LoadFile(path string) (resultErr error) {
	return s.loadFile(path, nil)
}

// loadFile with a non-nil out reports SMT-LIB parse errors there (see
// loadSMT2).
func (s *APISimpleSolver) loadFile(path string, out io.Writer) (resultErr error) {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	if filepath.Ext(path) == ".dimacs" {
		return s.LoadDIMACS(input)
	}
	return s.loadSMT2(input, out)
}
