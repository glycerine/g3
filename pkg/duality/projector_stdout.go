package duality

import (
	"io"
	"os"
)

// projectorStdout is the variable projector's std::cout; tests replace it.
var projectorStdout io.Writer = os.Stdout
