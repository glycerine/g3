package shell

import (
	"io"
	"os"

	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *DIMACSFrontend) prepareFile() (sat.Model, bool, error) {
	file, ok := f.input.(*os.File)
	if !ok {
		return nil, false, nil
	}
	info, err := file.Stat()
	// Amortize the probe on larger inputs and bound the extra scan on a miss.
	if err != nil || !info.Mode().IsRegular() || info.Size() < 4096 || info.Size() > 4<<20 {
		return nil, false, nil
	}
	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil || position != 0 {
		return nil, false, nil
	}
	core, err := f.coreEnabled()
	if err != nil || core {
		return nil, false, nil
	}
	// The runner opened the selected file at byte zero. ReadAt leaves the
	// original file position intact if the verdict needs the ordinary solver.
	// Read one byte past the observed size to detect growth during the scan.
	read := io.NewSectionReader(file, 0, info.Size()+1)
	count, value := f.solver.ProbeDIMACSUniformModel(read)
	if value == lbool.Undef {
		if _, err := read.Seek(0, io.SeekStart); err != nil {
			return nil, false, nil
		}
		return f.solver.PrepareDIMACSFile(read, int(info.Size()))
	}
	end, err := read.Seek(0, io.SeekCurrent)
	if err != nil || end != info.Size() {
		return nil, false, nil
	}
	model := make(sat.Model, int(count)+1)
	for i := range model {
		model[i] = value
	}
	return model, false, nil
}
