package docker_stub

import (
	"context"
	"errors"
)

// ImageExistsLocally returns a boolean indicating if an image with the
// requested name, tag and architecture exists in the local docker image store
func ImageExistsLocally(ctx context.Context, imageName string, platform string) (bool, error) {
	return false, errors.New("Unsupported Operation")
}

// RemoveImage removes image from local store, the function is used to run different
// container image architectures
func RemoveImage(ctx context.Context, imageName string, force bool, pruneChildren bool) (bool, error) {
	return false, errors.New("Unsupported Operation")
}
