package docker_stub

import (
	"context"
	"go/types"
	"runtime"
)

// NewContainer creates a reference to a container
func NewContainer(input *NewContainerInput) ExecutionsEnvironment {
	return nil
}

func RunnerArch(ctx context.Context) string {
	return runtime.GOOS
}

func GetHostInfo(ctx context.Context) (info types.Info, err error) {
	return types.Info{}, nil
}
