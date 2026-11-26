package shell

import (
	"io"
	"os"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *DIMACSFrontend) probeUniformModel() (uint32, lbool.LBool) {
	file, ok := f.input.(*os.File)
	if !ok {
		return 0, lbool.Undef
	}
	info, err := file.Stat()
	// Amortize the probe on larger inputs and bound the extra scan on a miss.
	if err != nil || !info.Mode().IsRegular() || info.Size() < 4096 || info.Size() > 4<<20 {
		return 0, lbool.Undef
	}
	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil || position != 0 {
		return 0, lbool.Undef
	}
	core, err := f.coreEnabled()
	if err != nil || core {
		return 0, lbool.Undef
	}
	// The runner opened the selected file at byte zero. ReadAt leaves the
	// original file position intact if the verdict needs the ordinary solver.
	// Read one byte past the observed size to detect growth during the scan.
	read := io.NewSectionReader(file, 0, info.Size()+1)
	count, value := f.solver.ProbeDIMACSUniformModel(read)
	end, err := read.Seek(0, io.SeekCurrent)
	if err != nil || end != info.Size() {
		return 0, lbool.Undef
	}
	return count, value
}
