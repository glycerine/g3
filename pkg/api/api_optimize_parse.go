package api

// Source: src/api/api_opt.cpp:285-357; src/util/file_path.h get_extension.

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func (o *APIOptimize) Parse(input io.Reader) error {
	if err := o.open(); err != nil {
		return err
	}
	if input == nil {
		return ErrSMTContextOwner
	}
	return smt2.ParseAssertions(o.command, input)
}

func (o *APIOptimize) ParseString(input string) error {
	return o.Parse(strings.NewReader(input))
}

// apiFileExtension is get_extension: the text after the last '.', or
// false when the name has no '.'.
func apiFileExtension(name string) (string, bool) {
	dot := strings.LastIndexByte(name, '.')
	if dot < 0 {
		return "", false
	}
	return name[dot+1:], true
}

// ParseFile is Z3_optimize_from_file: Z3_optimize_from_stream dispatches on
// the file extension, loading .opb and .wcnf files with their own parsers
// and everything else as SMT-LIB.
func (o *APIOptimize) ParseFile(path string) (resultErr error) {
	if err := o.open(); err != nil {
		return err
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	ext, hasExt := apiFileExtension(path)
	if hasExt && ext == "opb" {
		document, err := opt.ParseOPB(input)
		if err != nil {
			return err
		}
		_, err = o.command.LoadOPB(document)
		return err
	}
	if hasExt && ext == "wcnf" {
		document, err := opt.ParseWCNF(input)
		if err != nil {
			return err
		}
		_, err = o.command.LoadWCNF(document)
		return err
	}
	return o.Parse(input)
}
