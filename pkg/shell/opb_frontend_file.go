package shell

import (
	"errors"
	"os"
)

func (f *OPBFrontend) RunFile(path string) (resultErr error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	return f.Run(file)
}
