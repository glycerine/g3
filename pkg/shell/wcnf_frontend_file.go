package shell

import (
	"errors"
	"os"
)

func (f *WCNFFrontend) RunFile(path string) (resultErr error) {
	file, e := os.Open(path)
	if e != nil {
		return e
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	return f.Run(file)
}
