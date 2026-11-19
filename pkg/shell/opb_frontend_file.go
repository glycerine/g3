package shell

import (
	"context"
	"io"
	"os"
)

func (f *OPBFrontend) RunFile(path string) error {
	return f.RunFileContext(context.Background(), path)
}

func (f *OPBFrontend) RunFileContext(ctx context.Context, path string) error {
	return f.run(ctx, func() (io.Reader, io.Closer, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, nil, &OptimizationInputOpenError{Filename: path, Cause: err}
		}
		return file, file, nil
	})
}
