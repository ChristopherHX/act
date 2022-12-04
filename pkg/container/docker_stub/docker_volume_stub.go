package docker_stub

import (
	"context"

	"github.com/nektos/act/pkg/common"
)

func NewDockerVolumeRemoveExecutor(volume string, force bool) common.Executor {
	return func(ctx context.Context) error {
		return nil
	}
}
